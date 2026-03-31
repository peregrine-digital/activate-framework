'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const EventEmitter = require('events');

// ── Minimal mock client ──────────────────────────────────────

class MockClient extends EventEmitter {
  constructor() {
    super();
    this._mockResults = {};
    this._calls = [];
  }

  async getState() { this._calls.push('getState'); return this._mockResults.getState || {}; }
  async readTelemetryLog() { this._calls.push('readTelemetryLog'); return this._mockResults.readTelemetryLog || { entries: [] }; }
  async getConfig(scope) { this._calls.push(`getConfig:${scope}`); return this._mockResults[`config_${scope}`] || {}; }
  async setConfig(opts) { this._calls.push('setConfig'); return { ok: true }; }
  async setFileOverride(dest, override) { this._calls.push(`setFileOverride:${dest}:${override}`); return {}; }
  async listBranches() { this._calls.push('listBranches'); return this._mockResults.listBranches || []; }
}

// ── Mock vscode module ───────────────────────────────────────

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
        executeCommand: async (...args) => {
          executedCommands.push(args);
        },
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

// ── Helpers ──────────────────────────────────────────────────

function createMockWebview() {
  const messages = [];
  return {
    options: {},
    onDidReceiveMessage: () => {},
    postMessage: (msg) => messages.push(msg),
    asWebviewUri: (uri) => `https://webview/${uri.fsPath}`,
    html: '',
    _messages: messages,
  };
}

function createPanel(client) {
  const panel = new ControlPanelProvider(client, '0.2.6');
  const webview = createMockWebview();
  panel.resolveWebviewView({ webview });
  // Clear the init message sent by resolveWebviewView
  setTimeout(() => {}, 0);
  return { panel, webview };
}

// ── Tests ────────────────────────────────────────────────────

describe('ControlPanelProvider (svelte)', () => {
  let mockClient;

  beforeEach(() => {
    mockClient = new MockClient();
    executedCommands = [];
  });

  it('creates with a client reference', () => {
    const panel = new ControlPanelProvider(mockClient, '0.2.6');
    assert.ok(panel);
    assert.equal(panel._client, mockClient);
  });

  it('has the expected viewType', () => {
    assert.equal(ControlPanelProvider.viewType, 'activate-framework.controlPanel');
  });

  describe('resolveWebviewView', () => {
    it('sets HTML that loads webview bundle', () => {
      const { webview } = createPanel(mockClient);
      assert.ok(webview.html.includes('webview.js'), 'should load webview.js');
      assert.ok(webview.html.includes('webview.css'), 'should load webview.css');
      assert.ok(webview.html.includes('<div id="app">'), 'should have app mount point');
    });

    it('enables scripts', () => {
      const { webview } = createPanel(mockClient);
      assert.equal(webview.options.enableScripts, true);
    });

    it('sends init message after resolve', async () => {
      const { webview } = createPanel(mockClient);
      // Wait for the setTimeout in resolveWebviewView
      await new Promise((r) => setTimeout(r, 200));
      const initMsg = webview._messages.find((m) => m.type === 'init');
      assert.ok(initMsg, 'should send init message');
      assert.equal(initMsg.hasCli, true);
      assert.equal(initMsg.extensionVersion, '0.2.6');
    });

    it('sends init with hasCli=false when no client', async () => {
      const panel = new ControlPanelProvider(null, '0.2.6');
      const webview = createMockWebview();
      panel.resolveWebviewView({ webview });
      await new Promise((r) => setTimeout(r, 200));
      const initMsg = webview._messages.find((m) => m.type === 'init');
      assert.ok(initMsg);
      assert.equal(initMsg.hasCli, false);
    });
  });

  describe('setClient', () => {
    it('updates client and sends init', async () => {
      const panel = new ControlPanelProvider(null, '0.2.6');
      const webview = createMockWebview();
      panel.resolveWebviewView({ webview });
      await new Promise((r) => setTimeout(r, 200));
      webview._messages.length = 0;

      panel.setClient(mockClient);
      assert.equal(panel._client, mockClient);

      const initMsg = webview._messages.find((m) => m.type === 'init');
      assert.ok(initMsg, 'setClient should send init');
      assert.equal(initMsg.hasCli, true);
    });
  });

  describe('refresh', () => {
    it('sends stateChanged to webview', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;

      await panel.refresh();
      await new Promise((r) => setTimeout(r, 200));

      const stateMsg = webview._messages.find((m) => m.type === 'stateChanged');
      assert.ok(stateMsg, 'refresh should send stateChanged');
    });

    it('debounces multiple rapid refreshes', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;

      panel.refresh();
      panel.refresh();
      panel.refresh();
      panel.refresh();
      await panel.refresh();
      await new Promise((r) => setTimeout(r, 200));

      const stateMessages = webview._messages.filter((m) => m.type === 'stateChanged');
      assert.ok(stateMessages.length <= 2, `expected at most 2 stateChanged, got ${stateMessages.length}`);
    });
  });

  describe('request/response messages', () => {
    it('getState returns daemon state', async () => {
      const { panel, webview } = createPanel(mockClient);
      const expectedState = {
        config: { tier: 'standard', manifest: 'test' },
        state: { hasInstallMarker: true },
        files: [],
        tiers: [],
      };
      mockClient._mockResults.getState = expectedState;

      await panel._onMessage({ command: 'getState', _reqId: 1 });

      const response = webview._messages.find((m) => m._responseId === 1);
      assert.ok(response, 'should send response');
      assert.deepEqual(response._result, expectedState);
    });

    it('getConfig forwards scope to client', async () => {
      const { panel, webview } = createPanel(mockClient);
      mockClient._mockResults.config_global = { manifest: 'ironarch', tier: 'workflow' };

      await panel._onMessage({ command: 'getConfig', scope: 'global', _reqId: 2 });

      const response = webview._messages.find((m) => m._responseId === 2);
      assert.ok(response);
      assert.deepEqual(response._result, { manifest: 'ironarch', tier: 'workflow' });
      assert.ok(mockClient._calls.includes('getConfig:global'));
    });

    it('setConfig forwards updates to client', async () => {
      const { panel, webview } = createPanel(mockClient);

      await panel._onMessage({
        command: 'setConfig',
        updates: { tier: 'advanced', scope: 'project' },
        _reqId: 3,
      });

      const response = webview._messages.find((m) => m._responseId === 3);
      assert.ok(response);
      assert.equal(response._error, undefined);
      assert.ok(mockClient._calls.includes('setConfig'));
    });

    it('setOverride calls setFileOverride', async () => {
      const { panel, webview } = createPanel(mockClient);

      await panel._onMessage({
        command: 'setOverride',
        file: { file: 'instructions/a.md', override: 'pinned' },
        _reqId: 4,
      });

      const response = webview._messages.find((m) => m._responseId === 4);
      assert.ok(response);
      assert.ok(mockClient._calls.includes('setFileOverride:instructions/a.md:pinned'));
    });

    it('listManifests extracts manifests from state', async () => {
      const { panel, webview } = createPanel(mockClient);
      mockClient._mockResults.getState = {
        manifests: [{ id: 'ironarch', name: 'IronArch' }, { id: 'adhoc', name: 'Ad Hoc' }],
      };

      await panel._onMessage({ command: 'listManifests', _reqId: 5 });

      const response = webview._messages.find((m) => m._responseId === 5);
      assert.ok(response);
      assert.equal(response._result.length, 2);
      assert.equal(response._result[0].id, 'ironarch');
    });

    it('listBranches delegates to client', async () => {
      const { panel, webview } = createPanel(mockClient);
      mockClient._mockResults.listBranches = ['main', 'develop', 'feat/test'];

      await panel._onMessage({ command: 'listBranches', _reqId: 6 });

      const response = webview._messages.find((m) => m._responseId === 6);
      assert.ok(response);
      assert.deepEqual(response._result, ['main', 'develop', 'feat/test']);
    });

    it('readTelemetryLog returns entries', async () => {
      const { panel, webview } = createPanel(mockClient);
      const entries = [
        { date: '2025-01-01', premium_used: 10, premium_remaining: 90, premium_entitlement: 100 },
      ];
      mockClient._mockResults.readTelemetryLog = { entries };

      await panel._onMessage({ command: 'readTelemetryLog', _reqId: 7 });

      const response = webview._messages.find((m) => m._responseId === 7);
      assert.ok(response);
      assert.deepEqual(response._result, entries);
    });

    it('responds with error on unknown request command', async () => {
      const { panel, webview } = createPanel(mockClient);

      await panel._onMessage({ command: 'nonexistent', _reqId: 99 });

      const response = webview._messages.find((m) => m._responseId === 99);
      assert.ok(response);
      assert.ok(response._error, 'should have an error');
      assert.ok(response._error.includes('Unknown'), 'error should mention unknown command');
    });

    it('responds with error when client method throws', async () => {
      const { panel, webview } = createPanel(mockClient);
      mockClient.getState = async () => { throw new Error('daemon disconnected'); };

      await panel._onMessage({ command: 'getState', _reqId: 10 });

      const response = webview._messages.find((m) => m._responseId === 10);
      assert.ok(response);
      assert.equal(response._error, 'daemon disconnected');
    });
  });

  describe('command messages (request/response)', () => {
    it('installCLI dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;

      await panel._onMessage({ command: 'installCLI', _reqId: 100 });

      assert.deepEqual(executedCommands[0], ['activate-framework.installCLI']);
      const resp = webview._messages.find((m) => m._responseId === 100);
      assert.ok(resp, 'should send response');
      assert.strictEqual(resp._result, null);
    });

    it('changeTier dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'changeTier', _reqId: 101 });
      assert.deepEqual(executedCommands[0], ['activate-framework.changeTier']);
      assert.ok(webview._messages.find((m) => m._responseId === 101));
    });

    it('changeManifest dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'changeManifest', _reqId: 102 });
      assert.deepEqual(executedCommands[0], ['activate-framework.changeManifest']);
      assert.ok(webview._messages.find((m) => m._responseId === 102));
    });

    it('addToWorkspace dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'addToWorkspace', _reqId: 103 });
      assert.deepEqual(executedCommands[0], ['activate-framework.addToWorkspace']);
      assert.ok(webview._messages.find((m) => m._responseId === 103));
    });

    it('removeFromWorkspace dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'removeFromWorkspace', _reqId: 104 });
      assert.deepEqual(executedCommands[0], ['activate-framework.removeFromWorkspace']);
      assert.ok(webview._messages.find((m) => m._responseId === 104));
    });

    it('updateAll dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'updateAll', _reqId: 105 });
      assert.deepEqual(executedCommands[0], ['activate-framework.updateAll']);
      assert.ok(webview._messages.find((m) => m._responseId === 105));
    });

    it('installFile passes file arg and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      const file = { dest: 'instructions/a.md', installed: false };
      await panel._onMessage({ command: 'installFile', file, _reqId: 106 });
      assert.deepEqual(executedCommands[0], ['activate-framework.installFile', file]);
      assert.ok(webview._messages.find((m) => m._responseId === 106));
    });

    it('uninstallFile passes file arg and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      const file = { dest: 'instructions/a.md', installed: true };
      await panel._onMessage({ command: 'uninstallFile', file, _reqId: 107 });
      assert.deepEqual(executedCommands[0], ['activate-framework.uninstallFile', file]);
      assert.ok(webview._messages.find((m) => m._responseId === 107));
    });

    it('openFile passes file arg and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      const file = { dest: 'instructions/a.md' };
      await panel._onMessage({ command: 'openFile', file, _reqId: 108 });
      assert.deepEqual(executedCommands[0], ['activate-framework.openFile', file]);
      assert.ok(webview._messages.find((m) => m._responseId === 108));
    });

    it('diffFile passes file arg and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      const file = { dest: 'instructions/a.md' };
      await panel._onMessage({ command: 'diffFile', file, _reqId: 109 });
      assert.deepEqual(executedCommands[0], ['activate-framework.diffFile', file]);
      assert.ok(webview._messages.find((m) => m._responseId === 109));
    });

    it('skipUpdate passes file arg and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      const file = { dest: 'instructions/a.md' };
      await panel._onMessage({ command: 'skipUpdate', file, _reqId: 110 });
      assert.deepEqual(executedCommands[0], ['activate-framework.skipFileUpdate', file]);
      assert.ok(webview._messages.find((m) => m._responseId === 110));
    });

    it('checkForUpdates dispatches command and responds', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;
      await panel._onMessage({ command: 'checkForUpdates', _reqId: 111 });
      assert.deepEqual(executedCommands[0], ['activate-framework.checkForUpdates']);
      assert.ok(webview._messages.find((m) => m._responseId === 111));
    });

    it('refreshUsage dispatches telemetryRunNow then refreshes', async () => {
      const { panel, webview } = createPanel(mockClient);
      webview._messages.length = 0;

      await panel._onMessage({ command: 'refreshUsage', _reqId: 112 });
      // Wait for the promise chain and debounce to settle
      await new Promise((r) => setTimeout(r, 300));

      assert.deepEqual(executedCommands[0], ['activate-framework.telemetryRunNow']);
      assert.ok(webview._messages.find((m) => m._responseId === 112));
    });
  });
});
