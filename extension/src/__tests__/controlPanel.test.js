'use strict';

const { describe, it, beforeEach } = require('node:test');
const assert = require('node:assert/strict');
const EventEmitter = require('events');

// ── Minimal mock client ──────────────────────────────────────

class MockClient extends EventEmitter {
  constructor() {
    super();
    this._mockResults = {};
  }

  async getState() { return this._mockResults.getState || {}; }
  async listManifests() { return this._mockResults.listManifests || []; }
  async readTelemetryLog() { return this._mockResults.readTelemetryLog || []; }
  async getConfig(scope) { return this._mockResults[`config_${scope}`] || {}; }
  async setConfig() { return { ok: true }; }
}

// ── Import controlPanel helpers by loading the module ────────

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
      },
      commands: { executeCommand: async () => {} },
      Uri: {
        file: (p) => ({ fsPath: p }),
        joinPath: (base, ...s) => ({ fsPath: [base.fsPath, ...s].join('/') }),
      },
    };
  }
  return origLoad.call(this, request, parent, isMain);
};

// Grab the module-private helpers by reading the source
const cpModule = require('../controlPanel');
const { ControlPanelProvider } = cpModule;

// Access private helpers via a fresh module that we can test
// Since getTierIncludes, groupByCategory, inferCategory are module-scope,
// we need to test them indirectly through ControlPanelProvider behavior

describe('ControlPanelProvider', () => {
  let mockClient;
  let panel;

  beforeEach(() => {
    mockClient = new MockClient();
    panel = new ControlPanelProvider(mockClient);
  });

  it('creates with a client reference', () => {
    assert.ok(panel);
    assert.equal(panel._client, mockClient);
  });

  it('has the expected viewType', () => {
    assert.equal(ControlPanelProvider.viewType, 'activate-framework.controlPanel');
  });

  it('starts on the main page', () => {
    assert.equal(panel._currentPage, 'main');
  });

  describe('_gatherState', () => {
    const DEFAULT_TIERS = [
      { id: 'minimal', label: 'Minimal', includes: ['core'] },
      { id: 'standard', label: 'Standard', includes: ['core', 'ad-hoc'] },
      { id: 'advanced', label: 'Advanced', includes: ['core', 'ad-hoc', 'ad-hoc-advanced'] },
    ];

    it('transforms daemon state into panel shape', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'standard', manifest: 'test-manifest', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true, installedVersion: '1.0.0' },
        tiers: DEFAULT_TIERS,
        files: [
          { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true, installedVersion: '1.0.0', bundledVersion: '1.0.0', override: '' },
          { dest: 'prompts/b.md', category: 'prompts', tier: 'ad-hoc', installed: false, bundledVersion: '1.0.0', override: '' },
          { dest: 'agents/c.md', category: 'agents', tier: 'ad-hoc-advanced', installed: false, bundledVersion: '1.0.0', override: '' },
        ],
      };
      mockClient._mockResults.listManifests = [
        { id: 'test-manifest', name: 'Test' },
        { id: 'other', name: 'Other' },
      ];

      const state = await panel._gatherState();

      assert.equal(state.isActive, true);
      assert.equal(state.version, '1.0.0');
      assert.equal(state.tier, 'standard');
      assert.equal(state.tierLabel, 'Standard');
      assert.equal(state.manifestName, 'test-manifest');
      assert.equal(state.manifestCount, 2);
      assert.equal(state.installedFiles.length, 1);
      assert.equal(state.availableFiles.length, 1); // prompts/b.md is in standard tier
      assert.equal(state.outsideTierFiles.length, 1); // agents/c.md is ad-hoc-advanced
    });

    it('builds versionMap for installed files', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'standard', manifest: 'test', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true, installedVersion: '1.0.0' },
        tiers: DEFAULT_TIERS,
        files: [
          { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true, installedVersion: '0.9.0', bundledVersion: '1.0.0' },
        ],
      };
      mockClient._mockResults.listManifests = [];

      const state = await panel._gatherState();
      const vInfo = state.versionMap.get('instructions/a.md');
      assert.ok(vInfo);
      assert.equal(vInfo.installed, '0.9.0');
      assert.equal(vInfo.bundled, '1.0.0');
    });

    it('respects excluded override', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'standard', manifest: 'test', fileOverrides: { 'a.md': 'excluded' }, skippedVersions: {} },
        state: { hasInstallMarker: true },
        tiers: DEFAULT_TIERS,
        files: [
          { dest: 'a.md', category: 'instructions', tier: 'core', installed: false, override: 'excluded' },
        ],
      };
      mockClient._mockResults.listManifests = [];

      const state = await panel._gatherState();
      assert.equal(state.installedFiles.length, 0);
      assert.equal(state.availableFiles.length, 0);
      assert.equal(state.outsideTierFiles.length, 0);
    });

    it('respects pinned override for out-of-tier files', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'minimal', manifest: 'test', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true },
        tiers: DEFAULT_TIERS,
        files: [
          { dest: 'prompts/b.md', category: 'prompts', tier: 'ad-hoc', installed: false, override: 'pinned' },
        ],
      };
      mockClient._mockResults.listManifests = [];

      const state = await panel._gatherState();
      // Pinned file should be in available, not outside-tier
      assert.equal(state.availableFiles.length, 1);
      assert.equal(state.outsideTierFiles.length, 0);
    });

    it('handles missing config gracefully', async () => {
      mockClient._mockResults.getState = { files: [] };
      mockClient._mockResults.listManifests = [];

      const state = await panel._gatherState();
      assert.equal(state.tier, 'standard');
      assert.equal(state.isActive, false);
      assert.equal(state.installedFiles.length, 0);
    });

    it('uses custom tiers from daemon', async () => {
      const customTiers = [
        { id: 'basic', label: 'Basic', includes: ['foundation'] },
        { id: 'full', label: 'Full', includes: ['foundation', 'extras'] },
      ];
      mockClient._mockResults.getState = {
        config: { tier: 'basic', manifest: 'custom', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true },
        tiers: customTiers,
        files: [
          { dest: 'a.md', category: 'instructions', tier: 'foundation', installed: false, override: '' },
          { dest: 'b.md', category: 'prompts', tier: 'extras', installed: false, override: '' },
        ],
      };
      mockClient._mockResults.listManifests = [];

      const state = await panel._gatherState();
      assert.equal(state.tierLabel, 'Basic');
      assert.equal(state.availableFiles.length, 1); // only foundation files
      assert.equal(state.outsideTierFiles.length, 1); // extras is outside basic
    });
  });

  describe('_render', () => {
    it('renders without error when no view', async () => {
      // _view is null by default, should be a no-op
      await panel._render();
    });

    it('calls readTelemetryLog for usage page', async () => {
      let logCalled = false;
      const origRead = mockClient.readTelemetryLog.bind(mockClient);
      mockClient.readTelemetryLog = async () => { logCalled = true; return []; };
      
      panel._currentPage = 'usage';
      panel._view = {
        webview: { html: '', options: {}, cspSource: '' },
      };

      // _render will try to generate HTML; we just verify the client call
      try {
        await panel._render();
      } catch {
        // HTML generation may fail without full webview mock — that's fine
      }
      assert.ok(logCalled, 'readTelemetryLog should have been called');
    });

    it('renders settings page with config scopes', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'standard', manifest: 'test-manifest', telemetryEnabled: true },
        state: { hasInstallMarker: true },
        tiers: [{ id: 'standard', label: 'Standard', includes: ['core'] }],
        projectDir: '/test/project',
      };
      mockClient._mockResults.config_global = { manifest: 'test-manifest', tier: 'standard', telemetryEnabled: true };
      mockClient._mockResults.config_project = { tier: 'standard' };

      panel._currentPage = 'settings';
      panel._view = {
        webview: { html: '', options: {}, cspSource: '' },
      };

      await panel._render();
      assert.ok(panel._view.webview.html.includes('Settings'), 'should render settings page');
      assert.ok(panel._view.webview.html.includes('Global Defaults'), 'should show global section');
      assert.ok(panel._view.webview.html.includes('Project Overrides'), 'should show project section');
      assert.ok(panel._view.webview.html.includes('Enabled'), 'should show telemetry status');
    });
  });
});

// ── Test the module-level helpers indirectly ──────────────────

describe('groupByCategory (via controlPanel module)', () => {
  // We can test groupByCategory through _gatherState behavior
  // since it's used to build the file lists

  it('groups installed files by category', async () => {
    const mockClient = new MockClient();
    const panel = new ControlPanelProvider(mockClient);

    mockClient._mockResults.getState = {
      config: { tier: 'advanced', manifest: 'test', fileOverrides: {}, skippedVersions: {} },
      state: { hasInstallMarker: true },
      tiers: [
        { id: 'minimal', label: 'Minimal', includes: ['core'] },
        { id: 'standard', label: 'Standard', includes: ['core', 'ad-hoc'] },
        { id: 'advanced', label: 'Advanced', includes: ['core', 'ad-hoc', 'ad-hoc-advanced'] },
      ],
      files: [
        { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true },
        { dest: 'instructions/b.md', category: 'instructions', tier: 'core', installed: true },
        { dest: 'prompts/c.md', category: 'prompts', tier: 'ad-hoc', installed: true },
        { dest: 'agents/d.md', category: 'agents', tier: 'ad-hoc-advanced', installed: true },
      ],
    };
    mockClient._mockResults.listManifests = [];

    const state = await panel._gatherState();
    assert.equal(state.installedFiles.length, 4);
  });

  it('preserves file shape through gather', async () => {
    const mockClient = new MockClient();
    const panel = new ControlPanelProvider(mockClient);

    mockClient._mockResults.getState = {
      config: { tier: 'standard', manifest: 'test', fileOverrides: {}, skippedVersions: {} },
      state: { hasInstallMarker: true },
      tiers: [
        { id: 'minimal', label: 'Minimal', includes: ['core'] },
        { id: 'standard', label: 'Standard', includes: ['core', 'ad-hoc'] },
      ],
      files: [
        { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: false, bundledVersion: '2.0.0', description: 'Test file' },
      ],
    };
    mockClient._mockResults.listManifests = [];

    const state = await panel._gatherState();
    assert.equal(state.availableFiles.length, 1);
    assert.equal(state.availableFiles[0].dest, 'instructions/a.md');
    assert.equal(state.availableFiles[0].bundledVersion, '2.0.0');
  });
});
