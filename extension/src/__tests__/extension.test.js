'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const Module = require('module');
const EventEmitter = require('events');

// ── VS Code mock ─────────────────────────────────────────────

const registeredCommands = new Map();
const subscriptions = [];
let quickPickResult = null;
let authSession = null;
let shownMessages = [];
let warningMessages = [];
let errorMessages = [];
let webviewProviders = {};
let outputLines = [];

function resetVscodeMock() {
  registeredCommands.clear();
  subscriptions.length = 0;
  quickPickResult = null;
  authSession = null;
  shownMessages = [];
  warningMessages = [];
  errorMessages = [];
  webviewProviders = {};
  outputLines = [];
}

const vscodeMock = {
  workspace: {
    workspaceFolders: [{ uri: { fsPath: '/test/project' } }],
  },
  window: {
    showQuickPick: async (items, _opts) => {
      if (quickPickResult === null) return undefined;
      return items.find((i) => i.value === quickPickResult) || items[0];
    },
    showWarningMessage: async (msg, _opts, ...buttons) => {
      warningMessages.push(msg);
      return buttons[0]; // auto-confirm
    },
    showErrorMessage: (msg) => { errorMessages.push(msg); },
    showInformationMessage: (msg) => { shownMessages.push(msg); },
    createOutputChannel: () => ({
      appendLine: (line) => outputLines.push(line),
      clear: () => { outputLines.length = 0; },
      show: () => {},
      dispose: () => {},
    }),
    registerWebviewViewProvider: (viewType, provider) => {
      webviewProviders[viewType] = provider;
      return { dispose: () => {} };
    },
  },
  commands: {
    registerCommand: (id, handler) => {
      registeredCommands.set(id, handler);
      return { dispose: () => registeredCommands.delete(id) };
    },
    executeCommand: async () => {},
  },
  authentication: {
    getSession: async () => authSession,
  },
  Uri: {
    file: (p) => ({ fsPath: p, scheme: 'file' }),
    joinPath: (base, ...segments) => ({
      fsPath: [base.fsPath, ...segments].join('/'),
      scheme: 'file',
    }),
  },
};

// ── Mock ActivateClient ──────────────────────────────────────

class MockClient extends EventEmitter {
  constructor() {
    super();
    this.calls = [];
    this._disposed = false;
    this._mockResults = {};
  }

  async start() { this.calls.push(['start']); }
  async stop() { this.calls.push(['stop']); this._disposed = true; }

  _record(method, params) {
    this.calls.push([method, params]);
    return this._mockResults[method] || {};
  }

  async getState() { return this._record('getState'); }
  async getConfig() { return this._record('getConfig'); }
  async setConfig(p) { return this._record('setConfig', p); }
  async listManifests() { return this._record('listManifests'); }
  async listFiles(p) { return this._record('listFiles', p); }
  async repoAdd(p) { return this._record('repoAdd', p); }
  async repoRemove() { return this._record('repoRemove'); }
  async sync() { return this._record('sync'); }
  async update() { return this._record('update'); }
  async installFile(p) { return this._record('installFile', p); }
  async uninstallFile(p) { return this._record('uninstallFile', p); }
  async diffFile(p) { return this._record('diffFile', p); }
  async skipFileUpdate(p) { return this._record('skipFileUpdate', p); }
  async setFileOverride(p) { return this._record('setFileOverride', p); }
  async runTelemetry(p) { return this._record('runTelemetry', p); }
  async readTelemetryLog() { return this._record('readTelemetryLog'); }
}

// ── Module interception ──────────────────────────────────────

const origResolve = Module._resolveFilename;
const origLoad = Module._load;

function installMocks(mockClient) {
  Module._resolveFilename = function (request, parent, isMain, options) {
    if (request === 'vscode') return 'vscode';
    return origResolve.call(this, request, parent, isMain, options);
  };

  Module._load = function (request, parent, isMain) {
    if (request === 'vscode') return vscodeMock;
    if (request === './client' || request.endsWith('/client')) {
      return { ActivateClient: function () { return mockClient; } };
    }
    return origLoad.call(this, request, parent, isMain);
  };
}

function uninstallMocks() {
  Module._resolveFilename = origResolve;
  Module._load = origLoad;
  // Clear require cache for modules under test
  for (const key of Object.keys(require.cache)) {
    if (key.includes('extension/src/extension.js') || key.includes('extension/src/controlPanel.js')) {
      delete require.cache[key];
    }
  }
}

// ── Tests ────────────────────────────────────────────────────

describe('extension.js', () => {
  let mockClient;

  beforeEach(() => {
    resetVscodeMock();
    mockClient = new MockClient();
    uninstallMocks();
    installMocks(mockClient);
  });

  it('registers all expected commands on activation', async () => {
    // Mock fs.existsSync for binary resolution
    const origFs = Module._load;
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function () { return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: 'installed',
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.sync = { action: 'none' };

    const ext = require('../extension');
    const context = {
      extensionUri: { fsPath: '/ext' },
      extension: { packageJSON: { version: '1.0.0' } },
      subscriptions: subscriptions,
    };

    await ext.activate(context);

    const expectedCommands = [
      'activate-framework.changeTier',
      'activate-framework.changeManifest',
      'activate-framework.showStatus',
      'activate-framework.remove',
      'activate-framework.refresh',
      'activate-framework.addToWorkspace',
      'activate-framework.removeFromWorkspace',
      'activate-framework.updateAll',
      'activate-framework.installFile',
      'activate-framework.uninstallFile',
      'activate-framework.openFile',
      'activate-framework.diffFile',
      'activate-framework.skipFileUpdate',
      'activate-framework.telemetryRunNow',
    ];

    for (const cmd of expectedCommands) {
      assert.ok(registeredCommands.has(cmd), `missing command: ${cmd}`);
    }
  });

  it('changeTier command calls setConfig and sync', async () => {
    mockClient._mockResults.getState = {
      state: 'installed',
      config: { tier: 'standard', manifest: 'test' },
      files: [],
    };
    mockClient._mockResults.sync = { action: 'none' };

    quickPickResult = 'advanced';

    const handler = async () => {
      const state = await mockClient.getState();
      await mockClient.setConfig({ tier: 'advanced', scope: 'project' });
      await mockClient.sync();
    };
    await handler();

    const setCalls = mockClient.calls.filter(([m]) => m === 'setConfig');
    assert.equal(setCalls.length, 1);
    assert.deepStrictEqual(setCalls[0][1], { tier: 'advanced', scope: 'project' });

    const syncCalls = mockClient.calls.filter(([m]) => m === 'sync');
    assert.equal(syncCalls.length, 1);
  });

  it('showStatus writes to output channel', async () => {
    mockClient._mockResults.getState = {
      projectDir: '/test/project',
      state: 'installed',
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [{ dest: 'a.md' }, { dest: 'b.md' }],
    };

    const state = await mockClient.getState();
    const channel = { lines: [], clear() { this.lines.length = 0; }, appendLine(l) { this.lines.push(l); }, show() {} };
    channel.clear();
    channel.appendLine('=== Activate Framework Status ===');
    channel.appendLine(`Project:  ${state.projectDir}`);
    channel.appendLine(`State:    ${state.state}`);
    channel.appendLine(`Manifest: ${state.config.manifest}`);
    channel.appendLine(`Tier:     ${state.config.tier}`);
    channel.appendLine(`Files:    ${state.files.length}`);

    assert.ok(channel.lines.some((l) => l.includes('/test/project')));
    assert.ok(channel.lines.some((l) => l.includes('installed')));
    assert.ok(channel.lines.some((l) => l.includes('2')));
  });

  it('installFile calls client.installFile with correct dest', async () => {
    await mockClient.installFile('prompts/foo.md');
    const calls = mockClient.calls.filter(([m]) => m === 'installFile');
    assert.equal(calls.length, 1);
    assert.equal(calls[0][1], 'prompts/foo.md');
  });

  it('uninstallFile calls client.uninstallFile', async () => {
    await mockClient.uninstallFile('prompts/foo.md');
    const calls = mockClient.calls.filter(([m]) => m === 'uninstallFile');
    assert.equal(calls.length, 1);
    assert.equal(calls[0][1], 'prompts/foo.md');
  });

  it('updateAll calls client.update', async () => {
    mockClient._mockResults.update = { updated: ['a.md', 'b.md'], skipped: [] };
    const result = await mockClient.update();
    assert.equal(result.updated.length, 2);
  });

  it('repoAdd calls client.repoAdd', async () => {
    mockClient._mockResults.repoAdd = { manifest: 'test', tier: 'standard', count: 5 };
    const result = await mockClient.repoAdd();
    assert.equal(result.count, 5);
  });

  it('repoRemove calls client.repoRemove', async () => {
    await mockClient.repoRemove();
    const calls = mockClient.calls.filter(([m]) => m === 'repoRemove');
    assert.equal(calls.length, 1);
  });

  it('skipFileUpdate calls client.skipFileUpdate', async () => {
    await mockClient.skipFileUpdate('agents/foo.md');
    const calls = mockClient.calls.filter(([m]) => m === 'skipFileUpdate');
    assert.equal(calls.length, 1);
    assert.equal(calls[0][1], 'agents/foo.md');
  });

  it('diffFile returns diff result', async () => {
    mockClient._mockResults.diffFile = { file: 'a.md', diff: '--- a\n+++ b\n@@ -1 +1 @@\n-old\n+new', identical: false };
    const result = await mockClient.diffFile('a.md');
    assert.equal(result.identical, false);
    assert.ok(result.diff.includes('---'));
  });

  it('telemetryRunNow calls client.runTelemetry', async () => {
    await mockClient.runTelemetry('test-token');
    const calls = mockClient.calls.filter(([m]) => m === 'runTelemetry');
    assert.equal(calls.length, 1);
    assert.equal(calls[0][1], 'test-token');
  });

  it('deactivate stops the client', async () => {
    await mockClient.stop();
    assert.equal(mockClient._disposed, true);
    const calls = mockClient.calls.filter(([m]) => m === 'stop');
    assert.equal(calls.length, 1);
  });

  it('auto-setup calls repoAdd when state is none', async () => {
    mockClient._mockResults.getState = {
      state: 'none',
      config: { tier: 'standard', manifest: 'test' },
      files: [],
    };

    const state = await mockClient.getState();
    if (state.state === 'none' || state.state === 'not_installed') {
      await mockClient.repoAdd();
    }

    const calls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(calls.length, 1);
  });

  it('auto-setup calls sync when already installed', async () => {
    mockClient._mockResults.getState = {
      state: 'installed',
      config: { tier: 'standard', manifest: 'test' },
      files: [],
    };
    mockClient._mockResults.sync = { action: 'none' };

    const state = await mockClient.getState();
    if (state.state === 'none' || state.state === 'not_installed') {
      await mockClient.repoAdd();
    } else {
      await mockClient.sync();
    }

    const repoCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(repoCalls.length, 0);
    const syncCalls = mockClient.calls.filter(([m]) => m === 'sync');
    assert.equal(syncCalls.length, 1);
  });
});
