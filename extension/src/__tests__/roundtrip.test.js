'use strict';

/**
 * Roundtrip integration test: webview adapter ↔ ControlPanelProvider.
 *
 * Wires the real VS Code adapter (simulated) to the real ControlPanelProvider
 * through a mock message channel. Tests the complete postMessage roundtrip:
 *
 *   adapter.postMessage({command, _reqId}) → provider._onMessage → respond → adapter Promise resolves
 *
 * This test catches issues that unit tests miss:
 * - Fire-and-forget messages that get silently dropped (no _reqId)
 * - Mismatched command names between adapter and provider
 * - Missing respond() calls in the provider
 * - Serialization issues in the message format
 */

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const EventEmitter = require('events');

// ── Mock client ──────────────────────────────────────────────

class MockClient extends EventEmitter {
  constructor() {
    super();
    this._calls = [];
  }
  async getState() { this._calls.push('getState'); return { files: [], categories: [], tiers: [], manifests: [], config: {}, state: {} }; }
  async getConfig(scope) { this._calls.push(`getConfig:${scope}`); return {}; }
  async setConfig(opts) { this._calls.push('setConfig'); return {}; }
  async setFileOverride(dest, ov) { this._calls.push(`setFileOverride:${dest}`); return {}; }
  async listBranches() { this._calls.push('listBranches'); return []; }
  async readTelemetryLog() { this._calls.push('readTelemetryLog'); return { entries: [] }; }
}

// ── Mock vscode module ──────────────────────────────────────

let executedCommands = [];

const Module = require('module');
const origResolve = Module._resolveFilename;
const origLoad = Module._load;

Module._resolveFilename = function (request, parent, isMain, options) {
  if (request === 'vscode') return 'vscode';
  return origResolve.call(this, request, parent, isMain, options);
};

Module._load = function (request, parent, isMain) {
  if (request === 'vscode') {
    return {
      window: {
        registerWebviewViewProvider: () => ({ dispose() {} }),
        showInformationMessage: async () => {},
        showWarningMessage: async () => {},
        showErrorMessage: async () => {},
      },
      commands: {
        executeCommand: async (...args) => { executedCommands.push(args); },
      },
      Uri: {
        file: (p) => ({ fsPath: p }),
        joinPath: (base, ...s) => ({ fsPath: [base.fsPath, ...s].join('/') }),
      },
    };
  }
  return origLoad.call(this, request, parent, isMain);
};

const { ControlPanelProvider } = require('../controlPanel.svelte');

// ── Message channel that wires adapter to provider ──────────

/**
 * Creates a bidirectional message channel:
 * - webview.postMessage(msg) → delivered to provider's _onMessage
 * - provider's respond(result) → delivered to webview's message listener
 *
 * This simulates what VS Code does when a webview sends a message.
 */
function createMessageChannel(provider) {
  let webviewMessageHandler = null;

  // Extension-side webview mock
  const webview = {
    options: {},
    onDidReceiveMessage(handler) {
      webviewMessageHandler = handler;
    },
    // Messages sent FROM provider TO webview (responses)
    postMessage(msg) {
      // Deliver to the "webview" message listener asynchronously
      // (matches real VS Code behavior)
      Promise.resolve().then(() => {
        if (onWebviewMessage) onWebviewMessage(msg);
      });
    },
    asWebviewUri: (uri) => `https://webview/${uri.fsPath}`,
    html: '',
  };

  // Track webview-side message listener
  let onWebviewMessage = null;

  // Webview-side API mock (what acquireVsCodeApi returns)
  const vsCodeApi = {
    postMessage(msg) {
      // Deliver from "webview" to provider's _onMessage
      if (webviewMessageHandler) {
        webviewMessageHandler(msg);
      }
    },
    getState: () => null,
    setState: () => {},
  };

  // Resolve the webview view to wire up the message handler
  provider.resolveWebviewView({ webview });

  return {
    /**
     * Send a message from the webview to the provider and wait for the response.
     * Simulates what the VS Code adapter does: postMessage + listen for _responseId.
     */
    request(command, params = {}) {
      return new Promise((resolve, reject) => {
        const reqId = Date.now() + Math.random();
        const timeout = setTimeout(() => {
          onWebviewMessage = null;
          reject(new Error(`Roundtrip timeout: ${command} — no response received. Provider may be missing respond() call.`));
        }, 2000);

        onWebviewMessage = (msg) => {
          if (msg._responseId === reqId) {
            clearTimeout(timeout);
            onWebviewMessage = null;
            if (msg._error) {
              reject(new Error(msg._error));
            } else {
              resolve(msg._result);
            }
          }
        };

        vsCodeApi.postMessage({ command, ...params, _reqId: reqId });
      });
    },
  };
}

// ── Tests ────────────────────────────────────────────────────

describe('roundtrip: adapter ↔ provider message channel', () => {
  let mockClient;

  beforeEach(() => {
    executedCommands = [];
    mockClient = new MockClient();
  });

  // -- Request/response commands (return data) --

  it('getState roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    const result = await channel.request('getState');
    assert.ok(result, 'should receive state data');
    assert.ok(Array.isArray(result.files));
  });

  it('getConfig roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    const result = await channel.request('getConfig', { scope: 'global' });
    assert.ok(result !== undefined, 'should receive config');
  });

  it('listBranches roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    const result = await channel.request('listBranches');
    assert.ok(Array.isArray(result));
  });

  // -- Command messages (the ones that were broken) --

  it('openFile roundtrip — command dispatched AND response received', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);
    const file = { dest: 'instructions/general.md' };

    const result = await channel.request('openFile', { file });

    assert.strictEqual(result, null, 'response should be null');
    assert.deepEqual(executedCommands[0], ['activate-framework.openFile', file]);
  });

  it('installFile roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);
    const file = { dest: 'instructions/general.md', installed: false };

    const result = await channel.request('installFile', { file });

    assert.strictEqual(result, null);
    assert.deepEqual(executedCommands[0], ['activate-framework.installFile', file]);
  });

  it('uninstallFile roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);
    const file = { dest: 'instructions/general.md', installed: true };

    const result = await channel.request('uninstallFile', { file });

    assert.strictEqual(result, null);
    assert.deepEqual(executedCommands[0], ['activate-framework.uninstallFile', file]);
  });

  it('changeTier roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('changeTier');
    assert.deepEqual(executedCommands[0], ['activate-framework.changeTier']);
  });

  it('changeManifest roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('changeManifest');
    assert.deepEqual(executedCommands[0], ['activate-framework.changeManifest']);
  });

  it('addToWorkspace roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('addToWorkspace');
    assert.deepEqual(executedCommands[0], ['activate-framework.addToWorkspace']);
  });

  it('removeFromWorkspace roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('removeFromWorkspace');
    assert.deepEqual(executedCommands[0], ['activate-framework.removeFromWorkspace']);
  });

  it('updateAll roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('updateAll');
    assert.deepEqual(executedCommands[0], ['activate-framework.updateAll']);
  });

  it('diffFile roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);
    const file = { dest: 'instructions/general.md' };

    await channel.request('diffFile', { file });
    assert.deepEqual(executedCommands[0], ['activate-framework.diffFile', file]);
  });

  it('skipUpdate roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);
    const file = { dest: 'instructions/general.md' };

    await channel.request('skipUpdate', { file });
    assert.deepEqual(executedCommands[0], ['activate-framework.skipFileUpdate', file]);
  });

  it('checkForUpdates roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('checkForUpdates');
    assert.deepEqual(executedCommands[0], ['activate-framework.checkForUpdates']);
  });

  it('installCLI roundtrip', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    await channel.request('installCLI');
    assert.deepEqual(executedCommands[0], ['activate-framework.installCLI']);
  });

  // -- Failure detection --

  it('times out if provider does not respond (regression guard)', async () => {
    const provider = new ControlPanelProvider(mockClient, '0.2.6');
    const channel = createMessageChannel(provider);

    // Send an unknown command — provider won't respond with _result
    // (it logs a warning but sends an error response)
    await assert.rejects(
      channel.request('nonExistentCommand'),
      (err) => err.message.includes('Unknown command'),
    );
  });
});
