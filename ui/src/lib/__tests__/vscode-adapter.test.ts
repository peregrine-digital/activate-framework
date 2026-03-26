/**
 * VS Code adapter contract tests.
 *
 * These tests verify that the VS Code webview adapter sends all messages
 * using the request/response pattern (_reqId). Fire-and-forget messages
 * (without _reqId) are silently dropped by VS Code's webview infrastructure.
 *
 * This test would have caught the bug where openFile, installFile, etc.
 * used fire() instead of request(), causing clicks to do nothing.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { createVSCodeAPI } from '../adapters/vscode';
import type { FileStatus } from '../types';

// Capture all messages posted by the adapter
let postedMessages: Array<Record<string, unknown>> = [];

// Mock acquireVsCodeApi — must be global before importing the adapter
vi.stubGlobal('acquireVsCodeApi', () => ({
  postMessage: (msg: Record<string, unknown>) => {
    postedMessages.push(msg);
  },
  getState: () => null,
  setState: () => {},
}));

function makeFile(): FileStatus {
  return {
    dest: 'instructions/general.md',
    displayName: 'General',
    description: 'desc',
    category: 'instructions',
    tier: 'core',
    installed: true,
    inTier: true,
    bundledVersion: '1.0.0',
    installedVersion: '1.0.0',
    updateAvailable: false,
    skipped: false,
    override: '',
  };
}

describe('VS Code adapter contract', () => {
  let api: ReturnType<typeof createVSCodeAPI>;

  beforeEach(() => {
    postedMessages = [];
    api = createVSCodeAPI();
  });

  /**
   * Every method that sends a message MUST include _reqId.
   * This is the critical invariant — without _reqId, VS Code's
   * webview infrastructure may silently drop the message.
   */
  describe('all methods use request/response pattern (_reqId)', () => {
    const cases: Array<{ name: string; call: (api: ReturnType<typeof createVSCodeAPI>) => void }> = [
      { name: 'getState', call: (a) => a.getState() },
      { name: 'getConfig', call: (a) => a.getConfig('global') },
      { name: 'setConfig', call: (a) => a.setConfig({ tier: 'core' }) },
      { name: 'refreshConfig', call: (a) => a.refreshConfig() },
      { name: 'installFile', call: (a) => a.installFile(makeFile()) },
      { name: 'uninstallFile', call: (a) => a.uninstallFile(makeFile()) },
      { name: 'diffFile', call: (a) => a.diffFile(makeFile()) },
      { name: 'skipUpdate', call: (a) => a.skipUpdate(makeFile()) },
      { name: 'setFileOverride', call: (a) => a.setFileOverride('test.md', 'pinned') },
      { name: 'updateAll', call: (a) => a.updateAll() },
      { name: 'addToWorkspace', call: (a) => a.addToWorkspace() },
      { name: 'removeFromWorkspace', call: (a) => a.removeFromWorkspace() },
      { name: 'listManifests', call: (a) => a.listManifests() },
      { name: 'listBranches', call: (a) => a.listBranches() },
      { name: 'runTelemetry', call: (a) => a.runTelemetry() },
      { name: 'readTelemetryLog', call: (a) => a.readTelemetryLog() },
      { name: 'openFile', call: (a) => a.openFile(makeFile()) },
      { name: 'changeTier', call: (a) => a.changeTier() },
      { name: 'changeManifest', call: (a) => a.changeManifest() },
      { name: 'installCLI', call: (a) => a.installCLI() },
      { name: 'checkForUpdates', call: (a) => a.checkForUpdates() },
    ];

    for (const { name, call } of cases) {
      it(`${name} sends _reqId`, () => {
        postedMessages = [];
        call(api);

        const msgs = postedMessages.filter((m) => m.command === name ||
          // Some methods map to different command names
          (name === 'setFileOverride' && m.command === 'setOverride') ||
          (name === 'runTelemetry' && m.command === 'refreshUsage') ||
          (name === 'readTelemetryLog' && m.command === 'readTelemetryLog'));

        expect(msgs.length).toBeGreaterThan(0);
        for (const msg of msgs) {
          expect(msg._reqId, `${name} must send _reqId for reliable delivery`).toBeDefined();
          expect(typeof msg._reqId).toBe('number');
        }
      });
    }
  });

  describe('message format', () => {
    it('openFile sends file object', () => {
      const file = makeFile();
      api.openFile(file);

      const msg = postedMessages.find((m) => m.command === 'openFile');
      expect(msg).toBeDefined();
      expect(msg!.file).toEqual(file);
      expect(msg!._reqId).toBeDefined();
    });

    it('messages are structuredClone-safe (no Proxy objects)', () => {
      // Svelte 5 $props() returns Proxy objects which cannot be cloned
      // by postMessage. The adapter must strip them via JSON roundtrip.
      const file = makeFile();
      postedMessages = [];
      api.openFile(file);

      const msg = postedMessages.find((m) => m.command === 'openFile');
      expect(msg).toBeDefined();
      // structuredClone throws on Proxy objects — this must not throw
      expect(() => structuredClone(msg)).not.toThrow();
    });

    it('installFile sends file object', () => {
      const file = makeFile();
      api.installFile(file);

      const msg = postedMessages.find((m) => m.command === 'installFile');
      expect(msg).toBeDefined();
      expect(msg!.file).toEqual(file);
    });

    it('setConfig sends updates object', () => {
      api.setConfig({ tier: 'workflow', scope: 'project' });

      const msg = postedMessages.find((m) => m.command === 'setConfig');
      expect(msg).toBeDefined();
      expect(msg!.updates).toEqual({ tier: 'workflow', scope: 'project' });
    });

    it('getConfig sends scope', () => {
      api.getConfig('project');

      const msg = postedMessages.find((m) => m.command === 'getConfig');
      expect(msg).toBeDefined();
      expect(msg!.scope).toBe('project');
    });
  });

  describe('response handling', () => {
    it('openFile resolves when response arrives', async () => {
      const file = makeFile();
      const promise = api.openFile(file);

      // Simulate extension responding
      const msg = postedMessages.find((m) => m.command === 'openFile');
      window.dispatchEvent(
        new MessageEvent('message', {
          data: { _responseId: msg!._reqId, _result: null },
        }),
      );

      await expect(promise).resolves.not.toThrow();
    });

    it('openFile rejects on error response', async () => {
      const file = makeFile();
      const promise = api.openFile(file);

      const msg = postedMessages.find((m) => m.command === 'openFile');
      window.dispatchEvent(
        new MessageEvent('message', {
          data: { _responseId: msg!._reqId, _error: 'File not found' },
        }),
      );

      await expect(promise).rejects.toThrow('File not found');
    });
  });
});
