'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const Module = require('module');
const EventEmitter = require('events');

// ── Module interception (vscode mock) ───────────────────────────

const origResolve = Module._resolveFilename;
const origLoad = Module._load;

// Minimal VS Code mock — only needed so extension.js can be required
const vscodeMock = {
  workspace: { workspaceFolders: [{ uri: { fsPath: '/test' } }] },
  window: {
    showInformationMessage: async () => {},
    withProgress: async (_o, fn) => fn(),
    createOutputChannel: () => ({
      appendLine: () => {}, clear: () => {}, show: () => {}, dispose: () => {},
    }),
    registerWebviewViewProvider: () => ({ dispose: () => {} }),
  },
  commands: { registerCommand: () => ({ dispose: () => {} }), executeCommand: async () => {} },
  authentication: { getSession: async () => null },
  Uri: { file: (p) => ({ fsPath: p, scheme: 'file' }), joinPath: (b, ...s) => ({ fsPath: [b.fsPath, ...s].join('/') }) },
  ProgressLocation: { Notification: 15 },
};

let installed = false;
function installVscodeMock() {
  if (installed) return;
  installed = true;
  Module._resolveFilename = function (request, parent, isMain, options) {
    if (request === 'vscode') return 'vscode';
    return origResolve.call(this, request, parent, isMain, options);
  };
  Module._load = function (request, parent, isMain) {
    if (request === 'vscode') return vscodeMock;
    return origLoad.call(this, request, parent, isMain);
  };
}

// Install mock before requiring extension.js
installVscodeMock();
const { buildDownloadHeaders, performCliUpdate } = require('../extension');

// ── Mock Client ─────────────────────────────────────────────────

class MockClient extends EventEmitter {
  constructor() {
    super();
    this._disposed = false;
    this._updating = false;
    this.calls = [];
  }

  async start() {
    this.calls.push('start');
    this._disposed = false;
  }

  async stop() {
    this.calls.push('stop');
    this._disposed = true;
  }

  async selfUpdate(token) {
    this.calls.push({ method: 'selfUpdate', token });
    // Simulate daemon crash during binary replacement —
    // the real daemon process dies when go-selfupdate replaces the binary
    this.emit('exit', 0, null);
    return { updated: true, latestVersion: '0.2.0' };
  }
}

// ── buildDownloadHeaders tests ──────────────────────────────────
// These test the REAL exported function from extension.js.

describe('buildDownloadHeaders (real function)', () => {
  it('initial request includes auth and accept headers', () => {
    const h = buildDownloadHeaders('ghp_mytoken', false);
    assert.strictEqual(h['Authorization'], 'Bearer ghp_mytoken');
    assert.strictEqual(h['Accept'], 'application/octet-stream');
    assert.strictEqual(h['User-Agent'], 'activate-extension');
  });

  it('redirect request omits auth and accept', () => {
    const h = buildDownloadHeaders('ghp_mytoken', true);
    assert.strictEqual(h['Authorization'], undefined,
      'auth must not be sent on redirects — S3 pre-signed URLs reject extra auth');
    assert.strictEqual(h['Accept'], undefined);
    assert.strictEqual(h['User-Agent'], 'activate-extension');
  });

  it('no token means no auth header', () => {
    const h = buildDownloadHeaders('', false);
    assert.strictEqual(h['Authorization'], undefined);
    assert.strictEqual(h['Accept'], 'application/octet-stream');
  });

  it('null/undefined token treated as no token', () => {
    assert.strictEqual(buildDownloadHeaders(null, false)['Authorization'], undefined);
    assert.strictEqual(buildDownloadHeaders(undefined, false)['Authorization'], undefined);
  });

  it('isRedirect defaults to false', () => {
    const h = buildDownloadHeaders('tok');
    assert.strictEqual(h['Authorization'], 'Bearer tok');
    assert.strictEqual(h['Accept'], 'application/octet-stream');
  });
});

// ── performCliUpdate tests ──────────────────────────────────────
// These test the REAL exported function from extension.js.

describe('performCliUpdate (real function)', () => {
  it('_updating is true when daemon crashes during selfUpdate', async () => {
    const client = new MockClient();
    const flagDuringExit = [];

    client.on('exit', () => {
      flagDuringExit.push(client._updating);
    });

    await performCliUpdate(client, 'test-token');

    // _updating should have been true when the exit event fired during selfUpdate
    assert.ok(flagDuringExit.length > 0, 'exit event should have fired during selfUpdate()');
    assert.strictEqual(flagDuringExit[0], true,
      '_updating must be true when exit fires — prevents auto-restart race');

    // After performCliUpdate returns, flag must be cleared
    assert.strictEqual(client._updating, false);
  });

  it('calls selfUpdate → stop → start in order', async () => {
    const client = new MockClient();
    await performCliUpdate(client, 'tok');

    const ops = client.calls.map((c) => typeof c === 'string' ? c : c.method);
    assert.deepStrictEqual(ops, ['selfUpdate', 'stop', 'start']);
  });

  it('passes token to selfUpdate', async () => {
    const client = new MockClient();
    await performCliUpdate(client, 'ghp_secret');

    const call = client.calls.find((c) => c.method === 'selfUpdate');
    assert.strictEqual(call.token, 'ghp_secret');
  });

  it('clears _updating even if stop() throws', async () => {
    const client = new MockClient();
    client.stop = async () => { throw new Error('stop failed'); };

    // stop() errors are caught internally (daemon may already be dead)
    await performCliUpdate(client, 'tok');
    assert.strictEqual(client._updating, false,
      '_updating must be cleared — otherwise auto-restart is permanently suppressed');
  });

  it('clears _updating even if start() throws', async () => {
    const client = new MockClient();
    client.start = async () => { throw new Error('start failed'); };

    await assert.rejects(() => performCliUpdate(client, 'tok'));
    assert.strictEqual(client._updating, false);
  });

  it('clears _updating even if selfUpdate() throws', async () => {
    const client = new MockClient();
    client.selfUpdate = async () => { throw new Error('rpc failed'); };

    // selfUpdate errors are caught internally (daemon may die during binary replacement)
    await performCliUpdate(client, 'tok');
    assert.strictEqual(client._updating, false);
  });
});

// ── Auto-restart suppression integration ────────────────────────

describe('auto-restart suppression (real function)', () => {
  it('exit during selfUpdate does NOT trigger auto-restart', async () => {
    const client = new MockClient();
    let autoRestartFired = false;

    // Wire up the same handler as extension.js
    client.on('exit', () => {
      if (!client._disposed && !client._updating) {
        autoRestartFired = true;
      }
    });

    await performCliUpdate(client, 'tok');

    assert.strictEqual(autoRestartFired, false,
      'auto-restart must not fire during performCliUpdate');
  });

  it('exit AFTER performCliUpdate completes DOES trigger auto-restart', async () => {
    const client = new MockClient();
    let autoRestartFired = false;

    client.on('exit', () => {
      if (!client._disposed && !client._updating) {
        autoRestartFired = true;
      }
    });

    await performCliUpdate(client, 'tok');

    // Reset state — simulate a new daemon that crashes unexpectedly
    client._disposed = false;
    autoRestartFired = false;
    client.emit('exit', 1, null);
    assert.strictEqual(autoRestartFired, true,
      'auto-restart should work normally after update completes');
  });
});
