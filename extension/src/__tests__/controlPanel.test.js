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
        state: { hasInstallMarker: true },
        tiers: DEFAULT_TIERS,
        manifests: [
          { id: 'test-manifest', name: 'Test' },
          { id: 'other', name: 'Other' },
        ],
        files: [
          { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true, installedVersion: '1.0.0', bundledVersion: '1.0.0', override: '', inTier: true },
          { dest: 'prompts/b.md', category: 'prompts', tier: 'ad-hoc', installed: false, bundledVersion: '1.0.0', override: '', inTier: true },
          { dest: 'agents/c.md', category: 'agents', tier: 'ad-hoc-advanced', installed: false, bundledVersion: '1.0.0', override: '', inTier: false },
        ],
      };

      const state = await panel._gatherState();

      assert.equal(state.isActive, true);
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
        state: { hasInstallMarker: true },
        tiers: DEFAULT_TIERS,
        files: [
          { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true, installedVersion: '0.9.0', bundledVersion: '1.0.0' },
        ],
      };

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
          { dest: 'a.md', category: 'instructions', tier: 'core', installed: false, override: 'excluded', inTier: true },
        ],
      };

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
          { dest: 'prompts/b.md', category: 'prompts', tier: 'ad-hoc', installed: false, override: 'pinned', inTier: false },
        ],
      };

      const state = await panel._gatherState();
      // Pinned file should be in available, not outside-tier
      assert.equal(state.availableFiles.length, 1);
      assert.equal(state.outsideTierFiles.length, 0);
    });

    it('handles missing config gracefully', async () => {
      mockClient._mockResults.getState = { files: [] };

      const state = await panel._gatherState();
      assert.equal(state.tier, '');
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
          { dest: 'a.md', category: 'instructions', tier: 'foundation', installed: false, override: '', inTier: true },
          { dest: 'b.md', category: 'prompts', tier: 'extras', installed: false, override: '', inTier: false },
        ],
      };

      const state = await panel._gatherState();
      assert.equal(state.tierLabel, 'Basic');
      assert.equal(state.availableFiles.length, 1); // only foundation files
      assert.equal(state.outsideTierFiles.length, 1); // extras is outside basic
    });

    it('uses presets from daemon state when available', async () => {
      mockClient._mockResults.getState = {
        config: { preset: 'activate/workflow', manifest: 'ironarch', tier: 'workflow', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true },
        presets: [
          { id: 'activate/standard', name: 'Activate Standard', description: 'Standard preset' },
          { id: 'activate/workflow', name: 'Activate Workflow', description: 'Workflow preset' },
        ],
        tiers: [],
        files: [
          { dest: 'instructions/a.md', category: 'instructions', installed: true, inPreset: true, inTier: true, override: '' },
          { dest: 'agents/b.md', category: 'agents', installed: false, inPreset: true, inTier: false, override: '' },
          { dest: 'agents/c.md', category: 'agents', installed: false, inPreset: false, inTier: false, override: '' },
        ],
      };

      const state = await panel._gatherState();
      assert.equal(state.preset, 'activate/workflow');
      assert.equal(state.presetLabel, 'Activate Workflow');
      assert.equal(state.presets.length, 2);
      assert.equal(state.installedFiles.length, 1);
      assert.equal(state.availableFiles.length, 1); // agents/b.md is inPreset
      assert.equal(state.outsideTierFiles.length, 1); // agents/c.md is not inPreset
    });

    it('falls back to inTier when no presets available', async () => {
      mockClient._mockResults.getState = {
        config: { tier: 'standard', manifest: 'test', fileOverrides: {}, skippedVersions: {} },
        state: { hasInstallMarker: true },
        tiers: [{ id: 'standard', label: 'Standard' }],
        files: [
          { dest: 'a.md', category: 'instructions', installed: false, inPreset: false, inTier: true, override: '' },
        ],
      };

      const state = await panel._gatherState();
      assert.equal(state.presets.length, 0);
      assert.equal(state.availableFiles.length, 1); // falls back to inTier
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

    it('renders settings page with preset when presets available', async () => {
      mockClient._mockResults.getState = {
        config: { preset: 'activate/workflow', manifest: 'ironarch', tier: 'workflow', telemetryEnabled: true },
        state: { hasInstallMarker: true },
        presets: [{ id: 'activate/workflow', name: 'Activate Workflow', description: 'Workflow' }],
        tiers: [],
        projectDir: '/test/project',
      };
      mockClient._mockResults.config_global = { preset: 'activate/workflow' };
      mockClient._mockResults.config_project = { preset: 'activate/workflow' };

      panel._currentPage = 'settings';
      panel._view = {
        webview: { html: '', options: {}, cspSource: '' },
      };

      await panel._render();
      assert.ok(panel._view.webview.html.includes('Preset'), 'should show Preset label when presets available');
      assert.ok(panel._view.webview.html.includes('activate/workflow'), 'should show preset value');
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
      assert.ok(panel._view.webview.html.includes('Save Current Setup as Global Default'), 'should show save-global-default button');
      assert.ok(panel._view.webview.html.includes("setGlobalDefault"), 'button should wire to setGlobalDefault message');
      assert.ok(panel._view.webview.html.includes('Run Quick Start Setup'), 'should show quick-start button');
      assert.ok(panel._view.webview.html.includes("runQuickStart"), 'quick-start button should wire to runQuickStart message');
      assert.ok(panel._view.webview.html.includes('Reset Global Defaults'), 'should show reset-global button');
      assert.ok(panel._view.webview.html.includes("resetGlobalDefaults"), 'reset button should wire to resetGlobalDefaults message');
    });

    it('setGlobalDefault onclick uses JSON-safe interpolation for special chars', async () => {
      mockClient._mockResults.getState = {
        config: { tier: "team's-tier", manifest: "o'reilly", telemetryEnabled: false },
        state: { hasInstallMarker: true },
        tiers: [{ id: 'standard', label: 'Standard', includes: ['core'] }],
        projectDir: '/test/project',
      };
      mockClient._mockResults.config_global = {};
      mockClient._mockResults.config_project = { tier: "team's-tier", manifest: "o'reilly" };

      panel._currentPage = 'settings';
      panel._view = {
        webview: { html: '', options: {}, cspSource: '' },
      };

      await panel._render();
      const html = panel._view.webview.html;

      // The onclick payload must contain a valid JSON object (HTML-escaped).
      // With JSON.stringify the single quotes in values become part of a
      // double-quoted JSON string, so no JS string-break is possible.
      assert.ok(
        html.includes('setGlobalDefault'),
        'should still wire to setGlobalDefault message',
      );
      // Ensure the manifest value appears without broken JS quotes
      assert.ok(
        !html.includes("manifest: 'o&#39;reilly'"),
        'should NOT use single-quoted interpolation that breaks on apostrophes',
      );
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
        { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: true, inTier: true },
        { dest: 'instructions/b.md', category: 'instructions', tier: 'core', installed: true, inTier: true },
        { dest: 'prompts/c.md', category: 'prompts', tier: 'ad-hoc', installed: true, inTier: true },
        { dest: 'agents/d.md', category: 'agents', tier: 'ad-hoc-advanced', installed: true, inTier: true },
      ],
    };

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
        { dest: 'instructions/a.md', category: 'instructions', tier: 'core', installed: false, bundledVersion: '2.0.0', description: 'Test file', inTier: true },
      ],
    };

    const state = await panel._gatherState();
    assert.equal(state.availableFiles.length, 1);
    assert.equal(state.availableFiles[0].dest, 'instructions/a.md');
    assert.equal(state.availableFiles[0].bundledVersion, '2.0.0');
  });

  describe('refresh debouncing', () => {
    it('collapses multiple rapid refresh calls into one render', async () => {
      const mockClient = new MockClient();
      const panel = new ControlPanelProvider(mockClient, '0.1.0');
      const webview = { options: {}, onDidReceiveMessage: () => {}, html: '' };
      panel.resolveWebviewView({ webview });

      mockClient._mockResults.getState = {
        state: {}, config: {}, tiers: [], categories: [], files: [],
      };

      let renderCount = 0;
      const origRender = panel._render.bind(panel);
      panel._render = async function () {
        renderCount++;
        return origRender();
      };

      // Fire 5 rapid refreshes
      panel.refresh();
      panel.refresh();
      panel.refresh();
      panel.refresh();
      await panel.refresh();

      // Wait for debounce to settle
      await new Promise((r) => setTimeout(r, 200));

      assert.ok(renderCount <= 2, `expected at most 2 renders, got ${renderCount}`);
    });
  });
});
