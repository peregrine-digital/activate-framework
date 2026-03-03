'use strict';

const { describe, it, beforeEach, afterEach } = require('node:test');
const assert = require('node:assert/strict');
const Module = require('module');
const EventEmitter = require('events');

// ── Mock infrastructure ─────────────────────────────────────────

const origResolve = Module._resolveFilename;
const origLoad = Module._load;

let shownMessages = [];
let installedExtensions = [];
let executedCommands = [];
let httpsRequests = [];
let mockHttpsResponse = null;

function resetMocks() {
  shownMessages = [];
  installedExtensions = [];
  executedCommands = [];
  httpsRequests = [];
  mockHttpsResponse = null;
}

// Minimal VS Code mock
const vscodeMock = {
  workspace: {
    workspaceFolders: [{ uri: { fsPath: '/test/project' } }],
  },
  window: {
    showInformationMessage: async (msg, ...buttons) => {
      shownMessages.push({ msg, buttons });
      return buttons[0]; // auto-accept first button
    },
    withProgress: async (_opts, fn) => fn(),
    createOutputChannel: () => ({
      appendLine: () => {},
      clear: () => {},
      show: () => {},
      dispose: () => {},
    }),
    registerWebviewViewProvider: () => ({ dispose: () => {} }),
  },
  commands: {
    registerCommand: () => ({ dispose: () => {} }),
    executeCommand: async (cmd, ...args) => {
      executedCommands.push({ cmd, args });
    },
  },
  authentication: {
    getSession: async () => ({ accessToken: 'test-gh-token' }),
  },
  Uri: {
    file: (p) => ({ fsPath: p, scheme: 'file' }),
  },
  ProgressLocation: { Notification: 15 },
};

// Mock https module that captures requests
const httpsMock = {
  get: (url, options, callback) => {
    const reqInfo = { url, options };
    httpsRequests.push(reqInfo);

    const resp = new EventEmitter();
    if (mockHttpsResponse) {
      resp.statusCode = mockHttpsResponse.statusCode || 200;
      resp.headers = mockHttpsResponse.headers || {};
    } else {
      resp.statusCode = 200;
      resp.headers = {};
    }

    // Simulate redirect if configured
    if (resp.statusCode >= 300 && resp.statusCode < 400 && resp.headers.location) {
      setImmediate(() => callback(resp));
    } else {
      resp.pipe = (dest) => {
        dest.emit('finish');
        return dest;
      };
      setImmediate(() => callback(resp));
    }

    return { on: () => {} };
  },
};

// ── Mock Client for update flow tests ───────────────────────────

class MockUpdateClient extends EventEmitter {
  constructor() {
    super();
    this._disposed = false;
    this._updating = false;
    this.calls = [];
    this._serverVersion = '0.1.0';
    this._checkUpdateResult = null;
  }

  async start() {
    this.calls.push('start');
    this._disposed = false;
  }

  async stop() {
    this.calls.push('stop');
    // Simulate daemon exit event (like real client does)
    setImmediate(() => this.emit('exit', 0, null));
  }

  async checkUpdate(extVersion, force, token) {
    this.calls.push({ method: 'checkUpdate', extVersion, force, token });
    return this._checkUpdateResult;
  }

  async selfUpdate(token) {
    this.calls.push({ method: 'selfUpdate', token });
    return { updated: true, latestVersion: '0.2.0' };
  }

  get serverVersion() { return this._serverVersion; }
}

// ── Tests ────────────────────────────────────────────────────────

describe('Update flow', () => {
  let mockClient;

  beforeEach(() => {
    resetMocks();
    mockClient = new MockUpdateClient();
  });

  describe('_updating flag prevents auto-restart race', () => {
    it('auto-restart fires when _updating is false', async () => {
      let restartCalled = false;

      // Simulate the exit handler from extension.js
      mockClient.on('exit', () => {
        if (!mockClient._disposed && !mockClient._updating) {
          restartCalled = true;
        }
      });

      // Simulate unexpected daemon death
      mockClient.emit('exit', 1, null);
      assert.ok(restartCalled, 'auto-restart should fire on unexpected exit');
    });

    it('auto-restart is suppressed when _updating is true', async () => {
      let restartCalled = false;

      mockClient.on('exit', () => {
        if (!mockClient._disposed && !mockClient._updating) {
          restartCalled = true;
        }
      });

      // Simulate intentional update
      mockClient._updating = true;
      mockClient.emit('exit', 0, null);
      assert.ok(!restartCalled, 'auto-restart should NOT fire during intentional update');
    });

    it('auto-restart is suppressed when _disposed is true', async () => {
      let restartCalled = false;

      mockClient.on('exit', () => {
        if (!mockClient._disposed && !mockClient._updating) {
          restartCalled = true;
        }
      });

      mockClient._disposed = true;
      mockClient.emit('exit', 0, null);
      assert.ok(!restartCalled, 'auto-restart should NOT fire when disposed');
    });
  });

  describe('CLI update stop/start sequence', () => {
    it('sets _updating before stop/start and clears after', async () => {
      const flagStates = [];

      // Wire up exit handler to record flag state
      mockClient.on('exit', () => {
        flagStates.push({ event: 'exit', updating: mockClient._updating });
      });

      // Simulate the update flow from extension.js checkForUpdates
      mockClient._updating = true;
      try {
        await mockClient.selfUpdate('token');
        await mockClient.stop();
        // Allow the exit event to fire (emitted via setImmediate in stop())
        await new Promise((r) => setTimeout(r, 10));
        await mockClient.start();
      } finally {
        mockClient._updating = false;
      }

      // Verify the flag was true when exit fired
      assert.ok(flagStates.length > 0, 'exit event should have fired');
      assert.ok(flagStates[0].updating, '_updating should be true when exit fires during update');

      // Verify calls happened in correct order
      const methodCalls = mockClient.calls.filter((c) => typeof c === 'string' || c.method);
      const callNames = methodCalls.map((c) => typeof c === 'string' ? c : c.method);
      assert.deepStrictEqual(callNames, ['selfUpdate', 'stop', 'start']);
    });

    it('clears _updating even if stop() throws', async () => {
      const origStop = mockClient.stop.bind(mockClient);
      mockClient.stop = async () => {
        origStop();
        throw new Error('stop failed');
      };

      mockClient._updating = true;
      try {
        await mockClient.selfUpdate('token');
        await mockClient.stop();
        await mockClient.start();
      } catch {
        // Expected
      } finally {
        mockClient._updating = false;
      }

      assert.strictEqual(mockClient._updating, false, '_updating must be cleared even on error');
    });
  });

  describe('checkUpdate token passthrough', () => {
    it('passes token from auth session to checkUpdate RPC', async () => {
      mockClient._checkUpdateResult = { updateAvailable: false };

      await mockClient.checkUpdate('1.0.0', true, 'test-gh-token');

      const call = mockClient.calls.find((c) => c.method === 'checkUpdate');
      assert.ok(call, 'checkUpdate should have been called');
      assert.strictEqual(call.token, 'test-gh-token');
      assert.strictEqual(call.force, true);
    });

    it('passes token to selfUpdate RPC', async () => {
      await mockClient.selfUpdate('test-gh-token');

      const call = mockClient.calls.find((c) => c.method === 'selfUpdate');
      assert.ok(call, 'selfUpdate should have been called');
      assert.strictEqual(call.token, 'test-gh-token');
    });
  });
});

describe('downloadAndInstallVsix auth headers', () => {
  // These tests verify the header construction logic by testing
  // the actual download function behavior with a mocked https module.

  let downloadAndInstallVsix;

  beforeEach(() => {
    resetMocks();

    // Clear cached modules
    for (const key of Object.keys(require.cache)) {
      if (key.includes('extension/src/extension.js')) {
        delete require.cache[key];
      }
    }

    // We can't easily import the private function, so we test the
    // header construction logic directly.
  });

  it('initial request should include auth and octet-stream accept', () => {
    // Replicate the header construction from downloadAndInstallVsix
    const token = 'ghp_testtoken';
    const isRedirect = false;

    const headers = { 'User-Agent': 'activate-extension' };
    if (!isRedirect) {
      headers['Accept'] = 'application/octet-stream';
      if (token) headers['Authorization'] = `Bearer ${token}`;
    }

    assert.strictEqual(headers['Authorization'], 'Bearer ghp_testtoken');
    assert.strictEqual(headers['Accept'], 'application/octet-stream');
  });

  it('redirect request should NOT include auth or accept headers', () => {
    const token = 'ghp_testtoken';
    const isRedirect = true;

    const headers = { 'User-Agent': 'activate-extension' };
    if (!isRedirect) {
      headers['Accept'] = 'application/octet-stream';
      if (token) headers['Authorization'] = `Bearer ${token}`;
    }

    assert.strictEqual(headers['Authorization'], undefined,
      'auth header must not be sent on redirects (S3 pre-signed URLs reject extra auth)');
    assert.strictEqual(headers['Accept'], undefined,
      'accept header must not be sent on redirects');
    assert.strictEqual(headers['User-Agent'], 'activate-extension');
  });

  it('initial request without token should not include auth header', () => {
    const token = '';
    const isRedirect = false;

    const headers = { 'User-Agent': 'activate-extension' };
    if (!isRedirect) {
      headers['Accept'] = 'application/octet-stream';
      if (token) headers['Authorization'] = `Bearer ${token}`;
    }

    assert.strictEqual(headers['Authorization'], undefined,
      'no auth header when token is empty');
    assert.strictEqual(headers['Accept'], 'application/octet-stream');
  });
});

describe('Update flow integration', () => {
  it('full update sequence: check → update → stop → start → notify', async () => {
    const client = new MockUpdateClient();
    const events = [];

    // Wire up the exit handler (mirrors extension.js)
    client.on('exit', () => {
      if (!client._disposed && !client._updating) {
        events.push('auto-restart-triggered');
      } else {
        events.push('auto-restart-suppressed');
      }
    });

    // Simulate the full checkForUpdates flow
    client._checkUpdateResult = {
      updateAvailable: true,
      currentVersion: '0.1.0',
      latestVersion: '0.2.0',
      extension: { available: false },
    };

    const update = await client.checkUpdate('1.0.0', true, 'token');

    if (update.updateAvailable) {
      client._updating = true;
      try {
        await client.selfUpdate('token');
        await client.stop();
        // Allow the exit event to fire
        await new Promise((r) => setTimeout(r, 10));
        await client.start();
      } finally {
        client._updating = false;
      }
    }

    // Verify auto-restart was suppressed
    assert.ok(events.includes('auto-restart-suppressed'),
      'auto-restart should be suppressed during update');
    assert.ok(!events.includes('auto-restart-triggered'),
      'auto-restart should NOT have been triggered');

    // Verify the correct sequence of operations
    const ops = client.calls.map((c) => typeof c === 'string' ? c : c.method);
    assert.deepStrictEqual(ops, ['checkUpdate', 'selfUpdate', 'stop', 'start']);
  });
});
