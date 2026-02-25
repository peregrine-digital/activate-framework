/**
 * JSON-RPC client for the Activate CLI daemon.
 *
 * Spawns `activate serve --stdio` and communicates via
 * Content-Length framed JSON-RPC 2.0 over stdin/stdout.
 */
'use strict';

const { spawn } = require('child_process');
const { EventEmitter } = require('events');
const path = require('path');

// ── Protocol constants (mirror cli/protocol.go) ────────────────

const Method = {
  Initialize:    'activate/initialize',
  Shutdown:      'activate/shutdown',
  StateGet:      'activate/state',
  ConfigGet:     'activate/configGet',
  ConfigSet:     'activate/configSet',
  ManifestList:  'activate/manifestList',
  ManifestFiles: 'activate/manifestFiles',
  RepoAdd:       'activate/repoAdd',
  RepoRemove:    'activate/repoRemove',
  Sync:          'activate/sync',
  Update:        'activate/update',
  FileInstall:   'activate/fileInstall',
  FileUninstall: 'activate/fileUninstall',
  FileDiff:      'activate/fileDiff',
  FileSkip:      'activate/fileSkip',
  FileOverride:  'activate/fileOverride',
  TelemetryRun:  'activate/telemetryRun',
  TelemetryLog:  'activate/telemetryLog',
};

// ── Transport: Content-Length framed reader ─────────────────────

/**
 * Parses Content-Length framed JSON messages from a readable stream.
 * Emits 'message' events with parsed JSON objects.
 */
class FrameReader extends EventEmitter {
  constructor(stream) {
    super();
    this._stream = stream;
    this._buffer = Buffer.alloc(0);
    this._contentLength = -1;

    stream.on('data', (chunk) => this._onData(chunk));
    stream.on('end', () => this.emit('close'));
    stream.on('error', (err) => this.emit('error', err));
  }

  _onData(chunk) {
    this._buffer = Buffer.concat([this._buffer, chunk]);
    this._parse();
  }

  _parse() {
    while (true) {
      if (this._contentLength === -1) {
        // Look for header separator
        const sep = this._buffer.indexOf('\r\n\r\n');
        if (sep === -1) return;

        const header = this._buffer.slice(0, sep).toString('ascii');
        const match = header.match(/Content-Length:\s*(\d+)/i);
        if (!match) {
          this.emit('error', new Error(`Missing Content-Length in: ${header}`));
          return;
        }
        this._contentLength = parseInt(match[1], 10);
        this._buffer = this._buffer.slice(sep + 4);
      }

      if (this._buffer.length < this._contentLength) return;

      const body = this._buffer.slice(0, this._contentLength);
      this._buffer = this._buffer.slice(this._contentLength);
      this._contentLength = -1;

      try {
        const msg = JSON.parse(body.toString('utf8'));
        this.emit('message', msg);
      } catch (err) {
        this.emit('error', new Error(`JSON parse error: ${err.message}`));
      }
    }
  }
}

/**
 * Writes Content-Length framed JSON messages to a writable stream.
 */
function writeFrame(stream, obj) {
  const body = Buffer.from(JSON.stringify(obj), 'utf8');
  const header = `Content-Length: ${body.length}\r\n\r\n`;
  stream.write(header);
  stream.write(body);
}

// ── Client ─────────────────────────────────────────────────────

class ActivateClient extends EventEmitter {
  /**
   * @param {object} opts
   * @param {string} opts.binPath - Path to the activate binary
   * @param {string} opts.projectDir - Workspace root path
   * @param {object} [opts.log] - Logger with debug/error methods
   */
  constructor(opts) {
    super();
    this._binPath = opts.binPath;
    this._projectDir = opts.projectDir;
    this._log = opts.log || { debug() {}, error() {} };
    this._process = null;
    this._reader = null;
    this._nextId = 1;
    this._pending = new Map(); // id → { resolve, reject }
    this._initialized = false;
    this._disposed = false;
  }

  /** Start the daemon process and initialize. */
  async start() {
    if (this._process) return;

    this._process = spawn(this._binPath, ['serve', '--stdio'], {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: { ...process.env },
    });

    this._process.on('exit', (code, signal) => {
      this._log.debug(`Daemon exited: code=${code} signal=${signal}`);
      this._rejectAll(new Error(`Daemon exited unexpectedly (code ${code})`));
      this._process = null;
      if (!this._disposed) {
        this.emit('exit', code, signal);
      }
    });

    this._process.on('error', (err) => {
      this._log.error(`Daemon spawn error: ${err.message}`);
      this._rejectAll(err);
      this.emit('error', err);
    });

    // Capture stderr for logging
    if (this._process.stderr) {
      this._process.stderr.on('data', (chunk) => {
        this._log.debug(`[daemon stderr] ${chunk.toString().trim()}`);
      });
    }

    this._reader = new FrameReader(this._process.stdout);

    this._reader.on('message', (msg) => {
      // Notification (no id)
      if (msg.method && msg.id === undefined) {
        this.emit('notification', msg.method, msg.params);
        return;
      }
      // Response
      const id = typeof msg.id === 'number' ? msg.id : parseInt(msg.id, 10);
      const pending = this._pending.get(id);
      if (!pending) {
        this._log.debug(`Unexpected response id=${msg.id}`);
        return;
      }
      this._pending.delete(id);
      if (msg.error) {
        const err = new Error(msg.error.message);
        err.code = msg.error.code;
        err.data = msg.error.data;
        pending.reject(err);
      } else {
        pending.resolve(msg.result);
      }
    });

    this._reader.on('error', (err) => {
      this._log.error(`Frame reader error: ${err.message}`);
    });

    this._reader.on('close', () => {
      this._rejectAll(new Error('Daemon stdout closed'));
    });

    // Send initialize
    const result = await this.request(Method.Initialize, {
      projectDir: this._projectDir,
    });
    this._initialized = true;
    return result;
  }

  /** Stop the daemon gracefully. */
  async stop() {
    this._disposed = true;
    if (!this._process) return;
    try {
      await this.request(Method.Shutdown, {});
    } catch {
      // Ignore — process may already be gone
    }
    this._process.kill('SIGTERM');
    this._process = null;
    this._rejectAll(new Error('Client stopped'));
  }

  /** Send a raw JSON-RPC request and return the result. */
  request(method, params) {
    if (!this._process || !this._process.stdin.writable) {
      return Promise.reject(new Error('Daemon not running'));
    }

    const id = this._nextId++;
    return new Promise((resolve, reject) => {
      this._pending.set(id, { resolve, reject });
      writeFrame(this._process.stdin, {
        jsonrpc: '2.0',
        id,
        method,
        params: params || {},
      });
    });
  }

  // ── Typed API methods ──────────────────────────────────────────

  getState() {
    return this.request(Method.StateGet);
  }

  getConfig(scope) {
    return this.request(Method.ConfigGet, { scope });
  }

  setConfig(params) {
    return this.request(Method.ConfigSet, params);
  }

  listManifests() {
    return this.request(Method.ManifestList);
  }

  listFiles(params) {
    return this.request(Method.ManifestFiles, params || {});
  }

  repoAdd() {
    return this.request(Method.RepoAdd);
  }

  repoRemove() {
    return this.request(Method.RepoRemove);
  }

  sync() {
    return this.request(Method.Sync);
  }

  update() {
    return this.request(Method.Update);
  }

  installFile(file) {
    return this.request(Method.FileInstall, { file });
  }

  uninstallFile(file) {
    return this.request(Method.FileUninstall, { file });
  }

  diffFile(file) {
    return this.request(Method.FileDiff, { file });
  }

  skipFileUpdate(file) {
    return this.request(Method.FileSkip, { file });
  }

  setFileOverride(file, override) {
    return this.request(Method.FileOverride, { file, override });
  }

  runTelemetry(token) {
    return this.request(Method.TelemetryRun, { token });
  }

  readTelemetryLog() {
    return this.request(Method.TelemetryLog);
  }

  // ── Internal ───────────────────────────────────────────────────

  _rejectAll(err) {
    for (const [, pending] of this._pending) {
      pending.reject(err);
    }
    this._pending.clear();
  }
}

module.exports = { ActivateClient, FrameReader, writeFrame, Method };
