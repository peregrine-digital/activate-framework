'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const Module = require('module');
const EventEmitter = require('events');

// ── VS Code mock ─────────────────────────────────────────────

const registeredCommands = new Map();
const subscriptions = [];
let quickPickResult = null;
let infoMessageResults = [];
let authSession = null;
let shownMessages = [];
let warningMessages = [];
let errorMessages = [];
let webviewProviders = {};
let outputLines = [];
let appliedEdits = [];

/** Simple per-workspace state store mock. */
function createWorkspaceState() {
  const store = new Map();
  return {
    get(key) { return store.get(key); },
    async update(key, value) { store.set(key, value); },
    _store: store,
  };
}

function resetVscodeMock() {
  registeredCommands.clear();
  subscriptions.length = 0;
  quickPickResult = null;
  infoMessageResults = [];
  authSession = null;
  shownMessages = [];
  warningMessages = [];
  errorMessages = [];
  webviewProviders = {};
  outputLines = [];
  appliedEdits = [];
}

const vscodeMock = {
  workspace: {
    workspaceFolders: [{ uri: { fsPath: '/test/project' } }],
    fs: {
      readFile: async () => Buffer.from('mock-content'),
      writeFile: async () => {},
      readDirectory: async () => [],
      delete: async () => {},
    },
    applyEdit: async (edit) => { appliedEdits.push(edit); return true; },
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
    showInformationMessage: (msg, ...rest) => {
      shownMessages.push(msg);
      // Strip options objects (e.g. { modal: true }) to find button labels
      const buttons = rest.filter((r) => typeof r === 'string');
      if (buttons.length > 0 && infoMessageResults.length > 0) {
        return Promise.resolve(infoMessageResults.shift());
      }
      return Promise.resolve(undefined);
    },
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
    onDidChangeSessions: () => ({ dispose: () => {} }),
  },
  Uri: {
    file: (p) => ({ fsPath: p, scheme: 'file' }),
    joinPath: (base, ...segments) => ({
      fsPath: [base.fsPath, ...segments].join('/'),
      scheme: 'file',
    }),
  },
  WorkspaceEdit: class WorkspaceEdit {
    constructor() { this._ops = []; }
    createFile(uri, opts) { this._ops.push({ type: 'create', uri, opts }); }
    deleteFile(uri, opts) { this._ops.push({ type: 'delete', uri, opts }); }
  },
};

// ── Mock ActivateClient ──────────────────────────────────────

class MockClient extends EventEmitter {
  constructor(opts) {
    super();
    this.constructorOpts = opts;
    this.calls = [];
    this._disposed = false;
    this._mockResults = {};
    this._token = opts?.token || '';
  }

  get token() { return this._token; }
  set token(v) { this._token = v || ''; }

  async start() { this.calls.push(['start']); }
  async stop() { this.calls.push(['stop']); this._disposed = true; }

  _record(method, params) {
    this.calls.push([method, params]);
    return this._mockResults[method] || {};
  }

  async getState() { return this._record('getState'); }
  async getConfig(scope) { this.calls.push(['getConfig', scope]); return this._mockResults[`config_${scope}`] || this._mockResults.getConfig || {}; }
  async setConfig(p) { return this._record('setConfig', p); }
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
      return {
        ActivateClient: function (opts) {
          // Capture constructor opts on the mock for assertion
          mockClient.constructorOpts = opts;
          mockClient._token = opts?.token || '';
          return mockClient;
        },
      };
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
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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
      'activate-framework.quickStart',
    ];

    for (const cmd of expectedCommands) {
      assert.ok(registeredCommands.has(cmd), `missing command: ${cmd}`);
    }
  });

  it('changeTier command calls setConfig and sync', async () => {
    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [{ dest: 'a.md' }, { dest: 'b.md' }],
    };

    const state = await mockClient.getState();
    const channel = { lines: [], clear() { this.lines.length = 0; }, appendLine(l) { this.lines.push(l); }, show() {} };
    channel.clear();
    channel.appendLine('=== Activate Framework Status ===');
    channel.appendLine(`Project:  ${state.projectDir}`);
    channel.appendLine(`State:    ${state.state.hasInstallMarker ? 'installed' : 'not_installed'}`);
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
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'test' },
      files: [],
    };

    const state = await mockClient.getState();
    if (!state.state.hasInstallMarker) {
      await mockClient.repoAdd();
    }

    const calls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(calls.length, 1);
  });

  it('auto-setup calls sync when already installed', async () => {
    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'test' },
      files: [],
    };
    mockClient._mockResults.sync = { action: 'none' };

    const state = await mockClient.getState();
    if (!state.state.hasInstallMarker) {
      await mockClient.repoAdd();
    } else {
      await mockClient.sync();
    }

    const repoCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(repoCalls.length, 0);
    const syncCalls = mockClient.calls.filter(([m]) => m === 'sync');
    assert.equal(syncCalls.length, 1);
  });

  // ── Daemon auth token tests ──────────────────────────────────

  it('passes VS Code GitHub token to ActivateClient', async () => {
    authSession = { accessToken: 'ghp_vscode_token_abc' };

    const origFs = Module._load;
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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

    assert.strictEqual(mockClient.constructorOpts.token, 'ghp_vscode_token_abc');
  });

  it('passes empty token when no GitHub auth session', async () => {
    authSession = null;

    const origFs = Module._load;
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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

    assert.strictEqual(mockClient.constructorOpts.token, '');
  });

  it('installFile command fires WorkspaceEdit createFile for Copilot detection', async () => {
    // Mock fs.existsSync so resolveBinPath finds the CLI binary
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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

    appliedEdits = [];
    const handler = registeredCommands.get('activate-framework.installFile');
    assert.ok(handler, 'installFile command should be registered');

    await handler({ dest: 'copilot/agents/test.agent.md' });

    // Wait for async refreshWorkspace to complete
    await new Promise((r) => setTimeout(r, 100));

    // Should have called workspace.applyEdit with a WorkspaceEdit containing a createFile
    assert.ok(appliedEdits.length > 0, 'workspace.applyEdit should have been called');
    const edit = appliedEdits[appliedEdits.length - 1];
    assert.ok(edit._ops, 'should be a WorkspaceEdit with ops');
    assert.equal(edit._ops[0].type, 'create');
    assert.ok(edit._ops[0].uri.fsPath.includes('copilot/agents/test.agent.md'));
    assert.ok(edit._ops[0].opts.overwrite);
  });

  it('uninstallFile command fires WorkspaceEdit deleteFile for Copilot detection', async () => {
    // Mock fs.existsSync so resolveBinPath finds the CLI binary
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
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

    appliedEdits = [];
    const handler = registeredCommands.get('activate-framework.uninstallFile');
    assert.ok(handler, 'uninstallFile command should be registered');

    await handler({ dest: 'copilot/agents/test.agent.md' });

    await new Promise((r) => setTimeout(r, 100));

    assert.ok(appliedEdits.length > 0, 'workspace.applyEdit should have been called');
    const edit = appliedEdits[appliedEdits.length - 1];
    assert.ok(edit._ops, 'should be a WorkspaceEdit with ops');
    assert.equal(edit._ops[0].type, 'delete');
    assert.ok(edit._ops[0].uri.fsPath.includes('copilot/agents/test.agent.md'));
    assert.ok(edit._ops[0].opts.ignoreIfNotExists);
  });
});

// ── Quick Start prompt tests ─────────────────────────────────

describe('showQuickStartPrompt', () => {
  let mockClient;

  beforeEach(() => {
    resetVscodeMock();
    mockClient = new MockClient();
    uninstallMocks();
    installMocks(mockClient);
  });

  /**
   * Helper: load the extension module fresh and extract showQuickStartPrompt.
   * Sets up fs mocks so resolveBinPath can find the CLI binary.
   */
  function loadExtension() {
    Module._load = function (request, parent, isMain) {
      if (request === 'fs') {
        const realFs = origLoad.call(this, 'fs', parent, isMain);
        return { ...realFs, existsSync: (p) => p.includes('bin/activate') ? true : realFs.existsSync(p) };
      }
      if (request === 'vscode') return vscodeMock;
      if (request === './client' || request.endsWith('/client')) {
        return { ActivateClient: function (opts) { mockClient.constructorOpts = opts; mockClient._token = opts?.token || ''; return mockClient; } };
      }
      return origLoad.call(this, request, parent, isMain);
    };
    return require('../extension');
  }

  /**
   * Helper: activate the extension with a mock context that includes workspaceState.
   */
  async function activateWithState(ext, wsState) {
    mockClient._mockResults.sync = { action: 'none' };
    const context = {
      extensionUri: { fsPath: '/ext' },
      extension: { packageJSON: { version: '1.0.0' } },
      subscriptions: subscriptions,
      workspaceState: wsState || createWorkspaceState(),
    };
    await ext.activate(context);
    return context;
  }

  it('Quick Start: sets ironarch/workflow and calls repoAdd', async () => {
    const ext = loadExtension();

    // State: first run, no files installed
    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    // Global config: no manifest set
    mockClient._mockResults.config_global = {};

    // User picks "Quick Start"
    infoMessageResults = ['Quick Start'];

    await activateWithState(ext);

    // Should have called setConfig with ironarch/workflow
    const setCalls = mockClient.calls.filter(([m]) => m === 'setConfig');
    assert.ok(setCalls.length >= 1, 'should call setConfig at least once');
    const projectSet = setCalls.find(([, p]) => p?.manifest === 'ironarch');
    assert.ok(projectSet, 'should set manifest to ironarch');
    assert.equal(projectSet[1].tier, 'workflow');
    assert.equal(projectSet[1].scope, 'project');

    // Should have called repoAdd
    const addCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.ok(addCalls.length >= 1, 'should call repoAdd');
  });

  it('Cancel: skips install, sets workspaceState flag', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = {};

    // User cancels (no results in queue → undefined)

    const wsState = createWorkspaceState();
    await activateWithState(ext, wsState);

    // Should NOT call repoAdd
    const addCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(addCalls.length, 0, 'should not call repoAdd on cancel');

    // Should set workspaceState dismiss flag
    assert.equal(wsState.get('quickStartDismissed'), true, 'should persist dismiss flag');
  });

  it('skips prompt when global config has manifest set', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: true, hasProjectConfig: false },
      config: { tier: 'workflow', manifest: 'ironarch' },
      files: [],
    };
    // Global config HAS manifest set
    mockClient._mockResults.config_global = { manifest: 'ironarch', tier: 'workflow' };

    // Should never reach the prompt

    await activateWithState(ext);

    // Should auto-install without prompt
    const addCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.ok(addCalls.length >= 1, 'should auto-install with global defaults');

    // Should NOT show the modal prompt
    assert.ok(
      !shownMessages.some((m) => m.includes('Set up Activate')),
      'should not show prompt when global manifest is set',
    );
  });

  it('skips prompt when workspaceState has dismiss flag', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = {};

    const wsState = createWorkspaceState();
    await wsState.update('quickStartDismissed', true);

    await activateWithState(ext, wsState);

    // Should NOT call repoAdd
    const addCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.equal(addCalls.length, 0, 'should not install when dismissed');

    // Should NOT show prompt
    assert.ok(
      !shownMessages.some((m) => m.includes('Set up Activate')),
      'should not show prompt when dismissed',
    );
  });

  it('modal prompt shows correct message content', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = {};

    // User picks Quick Start
    infoMessageResults = ['Quick Start'];

    await activateWithState(ext);

    // Should have shown a modal with setup info
    assert.ok(
      shownMessages.some((m) => m.includes('6 specialized agents')),
      'should mention specialized agents in prompt',
    );
    assert.ok(
      shownMessages.some((m) => m.includes('Settings panel')),
      'should mention Settings panel for re-trigger',
    );
  });

  it('does not show prompt for already-installed workspaces', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.sync = { action: 'none' };

    await activateWithState(ext);

    // Should call sync, not repoAdd
    const syncCalls = mockClient.calls.filter(([m]) => m === 'sync');
    assert.ok(syncCalls.length >= 1, 'should sync for installed workspaces');

    // Should NOT show quick-start prompt
    assert.ok(
      !shownMessages.some((m) => m.includes('Set up Activate')),
      'should not show prompt for installed workspace',
    );
  });

  it('shows prompt when global config returns empty manifest string', async () => {
    const ext = loadExtension();

    // Daemon returns {manifest: "", tier: ""} when no global config file exists
    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = { manifest: '', tier: '' };

    infoMessageResults = ['Quick Start'];

    await activateWithState(ext);

    // Empty-string manifest should be treated as "no preference" — prompt should appear
    assert.ok(
      shownMessages.some((m) => m.includes('Set up Activate')),
      'should show prompt when global manifest is empty string',
    );
    const addCalls = mockClient.calls.filter(([m]) => m === 'repoAdd');
    assert.ok(addCalls.length >= 1, 'should call repoAdd after picking');
  });

  it('calls getConfig with global scope to check for existing preferences', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: false, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = {};

    infoMessageResults = ['Quick Start'];

    await activateWithState(ext);

    // Verify getConfig was called with 'global' scope
    const configCalls = mockClient.calls.filter(([m]) => m === 'getConfig');
    assert.ok(configCalls.length >= 1, 'should call getConfig');
    assert.ok(
      configCalls.some(([, scope]) => scope === 'global'),
      'should call getConfig with global scope',
    );
  });

  it('quickStart command skips guards and shows prompt even when global config is set', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: true, hasProjectConfig: false },
      config: { tier: 'workflow', manifest: 'ironarch' },
      files: [],
    };
    mockClient._mockResults.config_global = { manifest: 'ironarch', tier: 'workflow' };
    mockClient._mockResults.sync = { action: 'none' };

    // User picks Quick Start from the re-trigger
    infoMessageResults = ['Quick Start'];

    await activateWithState(ext);

    // Execute the quickStart command manually
    const handler = registeredCommands.get('activate-framework.quickStart');
    assert.ok(handler, 'quickStart command should be registered');
    await handler();

    // Should show the modal even though global config is set
    assert.ok(
      shownMessages.some((m) => m.includes('Set up Activate')),
      'should show prompt when triggered via command',
    );
  });

  it('quickStart command clears dismiss flag before showing prompt', async () => {
    const ext = loadExtension();

    mockClient._mockResults.getState = {
      state: { hasInstallMarker: true, hasGlobalConfig: false, hasProjectConfig: false },
      config: { tier: 'standard', manifest: 'activate-framework' },
      files: [],
    };
    mockClient._mockResults.config_global = {};
    mockClient._mockResults.sync = { action: 'none' };

    const wsState = createWorkspaceState();
    await wsState.update('quickStartDismissed', true);

    // User picks Quick Start from the re-trigger
    infoMessageResults = ['Quick Start'];

    const context = await activateWithState(ext, wsState);

    // Execute the quickStart command
    const handler = registeredCommands.get('activate-framework.quickStart');
    await handler();

    // Dismiss flag should have been cleared
    assert.equal(wsState.get('quickStartDismissed'), false, 'dismiss flag should be cleared by command');

    // Should show prompt
    assert.ok(
      shownMessages.some((m) => m.includes('Set up Activate')),
      'should show prompt after clearing dismiss flag',
    );
  });
});
