'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const { PassThrough } = require('stream');
const { FrameReader, writeFrame, ActivateClient, Method } = require('../client');

// ── FrameReader tests ──────────────────────────────────────────

describe('FrameReader', () => {
  it('parses a single Content-Length framed message', (t, done) => {
    const stream = new PassThrough();
    const reader = new FrameReader(stream);

    reader.on('message', (msg) => {
      assert.deepStrictEqual(msg, { jsonrpc: '2.0', id: 1, result: 'ok' });
      done();
    });

    const body = JSON.stringify({ jsonrpc: '2.0', id: 1, result: 'ok' });
    stream.write(`Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`);
  });

  it('parses multiple messages in one chunk', (t, done) => {
    const stream = new PassThrough();
    const reader = new FrameReader(stream);
    const messages = [];

    reader.on('message', (msg) => {
      messages.push(msg);
      if (messages.length === 2) {
        assert.strictEqual(messages[0].id, 1);
        assert.strictEqual(messages[1].id, 2);
        done();
      }
    });

    const msg1 = JSON.stringify({ jsonrpc: '2.0', id: 1, result: 'a' });
    const msg2 = JSON.stringify({ jsonrpc: '2.0', id: 2, result: 'b' });
    stream.write(
      `Content-Length: ${Buffer.byteLength(msg1)}\r\n\r\n${msg1}` +
      `Content-Length: ${Buffer.byteLength(msg2)}\r\n\r\n${msg2}`
    );
  });

  it('handles message split across chunks', (t, done) => {
    const stream = new PassThrough();
    const reader = new FrameReader(stream);

    reader.on('message', (msg) => {
      assert.strictEqual(msg.id, 42);
      done();
    });

    const body = JSON.stringify({ jsonrpc: '2.0', id: 42, result: 'split' });
    const full = `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`;
    // Split in the middle
    const mid = Math.floor(full.length / 2);
    stream.write(full.slice(0, mid));
    setTimeout(() => stream.write(full.slice(mid)), 5);
  });

  it('emits error on missing Content-Length', (t, done) => {
    const stream = new PassThrough();
    const reader = new FrameReader(stream);

    reader.on('error', (err) => {
      assert.match(err.message, /Missing Content-Length/);
      done();
    });

    stream.write('Bad-Header: value\r\n\r\n{}');
  });

  it('emits close when stream ends', (t, done) => {
    const stream = new PassThrough();
    const reader = new FrameReader(stream);

    reader.on('close', () => done());
    stream.end();
  });
});

// ── writeFrame tests ───────────────────────────────────────────

describe('writeFrame', () => {
  it('writes Content-Length framed JSON', () => {
    const stream = new PassThrough();
    const chunks = [];
    stream.on('data', (chunk) => chunks.push(chunk));

    writeFrame(stream, { jsonrpc: '2.0', id: 1, method: 'test' });
    stream.end();

    const output = Buffer.concat(chunks).toString('utf8');
    assert.match(output, /^Content-Length: \d+\r\n\r\n/);
    const bodyStr = output.replace(/^Content-Length: \d+\r\n\r\n/, '');
    const parsed = JSON.parse(bodyStr);
    assert.strictEqual(parsed.id, 1);
    assert.strictEqual(parsed.method, 'test');
  });

  it('handles unicode correctly', () => {
    const stream = new PassThrough();
    const chunks = [];
    stream.on('data', (chunk) => chunks.push(chunk));

    writeFrame(stream, { text: '日本語' });
    stream.end();

    const output = Buffer.concat(chunks).toString('utf8');
    const match = output.match(/Content-Length: (\d+)/);
    const declaredLen = parseInt(match[1], 10);
    const body = output.slice(output.indexOf('\r\n\r\n') + 4);
    assert.strictEqual(Buffer.byteLength(body, 'utf8'), declaredLen);
    assert.deepStrictEqual(JSON.parse(body), { text: '日本語' });
  });
});

// ── ActivateClient tests (with mock daemon) ────────────────────

/**
 * Creates a mock daemon that responds to requests.
 * Returns { client, respondNext, notifications }
 */
function createMockClient() {
  const clientToServer = new PassThrough();
  const serverToClient = new PassThrough();

  // Read requests from the client
  const serverReader = new FrameReader(clientToServer);
  const requests = [];
  const requestResolvers = [];

  serverReader.on('message', (msg) => {
    if (requestResolvers.length > 0) {
      requestResolvers.shift()(msg);
    } else {
      requests.push(msg);
    }
  });

  function nextRequest() {
    if (requests.length > 0) {
      return Promise.resolve(requests.shift());
    }
    return new Promise((resolve) => requestResolvers.push(resolve));
  }

  function sendResponse(id, result) {
    writeFrame(serverToClient, { jsonrpc: '2.0', id, result });
  }

  function sendError(id, code, message) {
    writeFrame(serverToClient, {
      jsonrpc: '2.0',
      id,
      error: { code, message },
    });
  }

  function sendNotification(method, params) {
    writeFrame(serverToClient, { jsonrpc: '2.0', method, params });
  }

  // Create a client with mock transport (bypass spawn)
  const client = new ActivateClient({
    binPath: '/fake/activate',
    projectDir: '/fake/project',
  });

  // Override start to wire up mock transport instead of spawning
  client._process = {
    stdin: clientToServer,
    stdout: serverToClient,
    stderr: new PassThrough(),
    kill() { clientToServer.end(); },
    on() {},
  };
  client._reader = new FrameReader(serverToClient);
  client._reader.on('message', (msg) => {
    if (msg.method && msg.id === undefined) {
      client.emit('notification', msg.method, msg.params);
      return;
    }
    const id = typeof msg.id === 'number' ? msg.id : parseInt(msg.id, 10);
    const pending = client._pending.get(id);
    if (pending) {
      client._pending.delete(id);
      if (msg.error) {
        const err = new Error(msg.error.message);
        err.code = msg.error.code;
        pending.reject(err);
      } else {
        pending.resolve(msg.result);
      }
    }
  });

  return { client, nextRequest, sendResponse, sendError, sendNotification };
}

describe('ActivateClient', () => {
  it('sends request and receives response', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.getState();
    const req = await nextRequest();

    assert.strictEqual(req.method, Method.StateGet);
    assert.strictEqual(req.jsonrpc, '2.0');

    sendResponse(req.id, { installed: true });
    const result = await resultPromise;
    assert.deepStrictEqual(result, { installed: true });
  });

  it('rejects on RPC error', async () => {
    const { client, nextRequest, sendError } = createMockClient();

    const resultPromise = client.getState();
    const req = await nextRequest();
    sendError(req.id, -32603, 'internal error');

    await assert.rejects(resultPromise, (err) => {
      assert.strictEqual(err.message, 'internal error');
      assert.strictEqual(err.code, -32603);
      return true;
    });
  });

  it('emits notifications', async () => {
    const { client, sendNotification } = createMockClient();

    const notifPromise = new Promise((resolve) => {
      client.on('notification', (method, params) => {
        resolve({ method, params });
      });
    });

    sendNotification('activate/stateChanged', null);
    const notif = await notifPromise;
    assert.strictEqual(notif.method, 'activate/stateChanged');
  });

  it('sends correct params for setConfig', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.setConfig({
      scope: 'project',
      tier: 'advanced',
    });
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.ConfigSet);
    assert.strictEqual(req.params.scope, 'project');
    assert.strictEqual(req.params.tier, 'advanced');

    sendResponse(req.id, { ok: true });
    await resultPromise;
  });

  it('sends correct params for installFile', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.installFile('instructions/general.md');
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.FileInstall);
    assert.strictEqual(req.params.file, 'instructions/general.md');

    sendResponse(req.id, { installed: true });
    await resultPromise;
  });

  it('sends correct params for uninstallFile', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.uninstallFile('prompts/test.md');
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.FileUninstall);
    assert.strictEqual(req.params.file, 'prompts/test.md');

    sendResponse(req.id, { removed: true });
    await resultPromise;
  });

  it('sends correct params for setFileOverride', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.setFileOverride('agents/plan.md', 'pinned');
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.FileOverride);
    assert.strictEqual(req.params.file, 'agents/plan.md');
    assert.strictEqual(req.params.override, 'pinned');

    sendResponse(req.id, { ok: true });
    await resultPromise;
  });

  it('handles concurrent requests', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const p1 = client.getState();
    const p2 = client.listManifests();
    const p3 = client.getConfig('resolved');

    const r1 = await nextRequest();
    const r2 = await nextRequest();
    const r3 = await nextRequest();

    // Respond out of order
    sendResponse(r3.id, { manifest: 'a' });
    sendResponse(r1.id, { installed: true });
    sendResponse(r2.id, [{ id: 'm1' }]);

    const [res1, res2, res3] = await Promise.all([p1, p2, p3]);
    assert.deepStrictEqual(res1, { installed: true });
    assert.deepStrictEqual(res2, [{ id: 'm1' }]);
    assert.deepStrictEqual(res3, { manifest: 'a' });
  });

  it('sends correct params for repoAdd', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.repoAdd();
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.RepoAdd);

    sendResponse(req.id, { files: ['a.md'] });
    const result = await resultPromise;
    assert.deepStrictEqual(result, { files: ['a.md'] });
  });

  it('sends correct params for sync', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.sync();
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.Sync);

    sendResponse(req.id, { synced: true });
    await resultPromise;
  });

  it('sends correct params for skipFileUpdate', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.skipFileUpdate('skills/build.md');
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.FileSkip);
    assert.strictEqual(req.params.file, 'skills/build.md');

    sendResponse(req.id, { skipped: true });
    await resultPromise;
  });

  it('sends correct params for runTelemetry', async () => {
    const { client, nextRequest, sendResponse } = createMockClient();

    const resultPromise = client.runTelemetry('ghp_test123');
    const req = await nextRequest();
    assert.strictEqual(req.method, Method.TelemetryRun);
    assert.strictEqual(req.params.token, 'ghp_test123');

    sendResponse(req.id, { logged: true });
    await resultPromise;
  });

  it('rejects pending requests when daemon not running', async () => {
    const client = new ActivateClient({
      binPath: '/fake/activate',
      projectDir: '/fake/project',
    });

    await assert.rejects(
      () => client.getState(),
      /Daemon not running/,
    );
  });
});
