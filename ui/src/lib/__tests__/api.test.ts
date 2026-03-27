/**
 * Tests for the mock adapter and API contract.
 * Verifies the mock adapter satisfies the ActivateAPI interface
 * and returns well-formed data for all methods.
 */
import { describe, it, expect } from 'vitest';
import { createMockAPI } from '../adapters/mock';

describe('createMockAPI', () => {
  const api = createMockAPI();

  it('sets platform to dev by default', () => {
    expect(api.platform).toBe('dev');
  });

  it('accepts a custom platform', () => {
    const desktopApi = createMockAPI('desktop');
    expect(desktopApi.platform).toBe('desktop');
  });

  describe('getState', () => {
    it('returns a valid AppState', async () => {
      const state = await api.getState();
      expect(state).toBeDefined();
      expect(state.projectDir).toBeTruthy();
      expect(state.config).toBeDefined();
      expect(state.config.manifest).toBeTruthy();
      expect(state.config.tier).toBeTruthy();
      expect(state.files).toBeInstanceOf(Array);
      expect(state.files.length).toBeGreaterThan(0);
      expect(state.categories).toBeInstanceOf(Array);
      expect(state.tiers).toBeInstanceOf(Array);
      expect(state.manifests).toBeInstanceOf(Array);
    });

    it('files have required fields', async () => {
      const state = await api.getState();
      for (const f of state.files) {
        expect(f.dest).toBeTruthy();
        expect(f.displayName).toBeTruthy();
        expect(f.category).toBeTruthy();
        expect(typeof f.installed).toBe('boolean');
        expect(typeof f.inTier).toBe('boolean');
      }
    });

    it('includes files in different states', async () => {
      const state = await api.getState();
      const installed = state.files.filter((f) => f.installed && f.inTier);
      const available = state.files.filter((f) => !f.installed && f.inTier);
      const outsideTier = state.files.filter((f) => !f.inTier);
      expect(installed.length).toBeGreaterThan(0);
      expect(available.length).toBeGreaterThan(0);
      expect(outsideTier.length).toBeGreaterThan(0);
    });
  });

  describe('getConfig', () => {
    it('returns config for global scope', async () => {
      const config = await api.getConfig('global');
      expect(config).toBeDefined();
      expect(config.manifest).toBeTruthy();
    });

    it('returns config for project scope', async () => {
      const config = await api.getConfig('project');
      expect(config).toBeDefined();
    });
  });

  describe('file operations', () => {
    it('installFile does not throw', async () => {
      const state = await api.getState();
      const file = state.files.find((f) => !f.installed);
      if (file) {
        await expect(api.installFile(file)).resolves.not.toThrow();
      }
    });

    it('uninstallFile does not throw', async () => {
      const state = await api.getState();
      const file = state.files.find((f) => f.installed);
      if (file) {
        await expect(api.uninstallFile(file)).resolves.not.toThrow();
      }
    });

    it('diffFile returns a DiffResult', async () => {
      const state = await api.getState();
      const file = state.files.find((f) => f.installed);
      if (file) {
        const result = await api.diffFile(file);
        expect(result).toBeDefined();
        expect(result.file).toBeTruthy();
        expect(typeof result.diff).toBe('string');
      }
    });
  });

  describe('onStateChanged', () => {
    it('returns an unsubscribe function', () => {
      const unsub = api.onStateChanged(() => {});
      expect(typeof unsub).toBe('function');
      unsub();
    });
  });

  describe('listing methods', () => {
    it('listManifests returns array', async () => {
      const manifests = await api.listManifests();
      expect(manifests).toBeInstanceOf(Array);
    });

    it('listBranches returns array', async () => {
      const branches = await api.listBranches();
      expect(branches).toBeInstanceOf(Array);
    });
  });
});
