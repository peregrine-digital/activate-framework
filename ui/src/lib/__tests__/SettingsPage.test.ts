import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect, vi } from 'vitest';
import SettingsPage from '../components/SettingsPage.svelte';
import type { ActivateAPI } from '../api.js';
import type { AppState, Config } from '../types.js';

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    manifest: 'activate-framework',
    tier: 'standard',
    preset: 'adhoc/standard',
    repo: 'peregrine-digital/activate-framework',
    branch: 'main',
    fileOverrides: {},
    skippedVersions: {},
    ...overrides,
  };
}

function makeAppState(overrides: Partial<AppState> = {}): AppState {
  return {
    config: makeConfig(),
    tiers: [
      { id: 'core', label: 'Core', description: 'Essential' },
      { id: 'standard', label: 'Standard', description: 'Recommended' },
    ],
    manifests: [],
    presets: [
      { id: 'adhoc/core', name: 'Core', description: 'Core preset' },
      { id: 'adhoc/standard', name: 'Standard', description: 'Standard preset' },
    ],
    files: [],
    categories: [],
    state: { hasInstallMarker: true },
    projectDir: '/tmp/test',
    telemetryLogPath: '/tmp/test/telemetry.log',
    ...overrides,
  };
}

function makeMockAPI(overrides: Partial<ActivateAPI> = {}): ActivateAPI {
  return {
    platform: 'vscode',
    getState: vi.fn().mockResolvedValue(makeAppState()),
    getConfig: vi.fn().mockResolvedValue(makeConfig()),
    setConfig: vi.fn().mockResolvedValue(undefined),
    refreshConfig: vi.fn().mockResolvedValue(undefined),
    installFile: vi.fn().mockResolvedValue(undefined),
    uninstallFile: vi.fn().mockResolvedValue(undefined),
    diffFile: vi.fn().mockResolvedValue({ file: '', diff: '' }),
    skipUpdate: vi.fn().mockResolvedValue(undefined),
    setFileOverride: vi.fn().mockResolvedValue(undefined),
    updateAll: vi.fn().mockResolvedValue(undefined),
    addToWorkspace: vi.fn().mockResolvedValue(undefined),
    removeFromWorkspace: vi.fn().mockResolvedValue(undefined),
    listManifests: vi.fn().mockResolvedValue([]),
    listBranches: vi.fn().mockResolvedValue([]),
    listPresets: vi.fn().mockResolvedValue([]),
    changePreset: vi.fn().mockResolvedValue(undefined),
    runTelemetry: vi.fn().mockResolvedValue(undefined),
    readTelemetryLog: vi.fn().mockResolvedValue([]),
    openFile: vi.fn().mockResolvedValue(undefined),
    changeTier: vi.fn().mockResolvedValue(undefined),
    changeManifest: vi.fn().mockResolvedValue(undefined),
    installCLI: vi.fn().mockResolvedValue(undefined),
    checkForUpdates: vi.fn().mockResolvedValue(undefined),
    onStateChanged: vi.fn().mockReturnValue(() => {}),
    ...overrides,
  };
}

describe('SettingsPage', () => {
  describe('Updates section', () => {
    it('renders Check for Updates button for vscode platform', () => {
      const api = makeMockAPI({ platform: 'vscode' });
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '0.3.3', onBack: vi.fn() },
      });

      expect(getByText('🔄 Check for Updates')).toBeTruthy();
    });

    it('calls api.checkForUpdates when button is clicked', async () => {
      const api = makeMockAPI({ platform: 'vscode' });
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '0.3.3', onBack: vi.fn() },
      });

      await fireEvent.click(getByText('🔄 Check for Updates'));
      expect(api.checkForUpdates).toHaveBeenCalledOnce();
    });

    it('displays serverVersion when provided', () => {
      const api = makeMockAPI();
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '0.3.3', onBack: vi.fn() },
      });

      expect(getByText('CLI Version')).toBeTruthy();
      expect(getByText('0.3.3')).toBeTruthy();
    });

    it('displays dash when serverVersion is empty', () => {
      const api = makeMockAPI();
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '', onBack: vi.fn() },
      });

      expect(getByText('CLI Version')).toBeTruthy();
      expect(getByText('—')).toBeTruthy();
    });

    it('displays dash when serverVersion prop is omitted', () => {
      const api = makeMockAPI();
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, onBack: vi.fn() },
      });

      expect(getByText('—')).toBeTruthy();
    });

    it('hides Check for Updates button on desktop platform', () => {
      const api = makeMockAPI({ platform: 'desktop' });
      const { queryByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '0.3.3', onBack: vi.fn() },
      });

      expect(queryByText('🔄 Check for Updates')).toBeNull();
    });

    it('renders Updates section label', () => {
      const api = makeMockAPI();
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, serverVersion: '0.3.3', onBack: vi.fn() },
      });

      expect(getByText('Updates')).toBeTruthy();
    });
  });

  describe('navigation', () => {
    it('calls onBack when back button is clicked', async () => {
      const onBack = vi.fn();
      const api = makeMockAPI();
      const { getByText } = render(SettingsPage, {
        props: { appState: makeAppState(), api, onBack },
      });

      await fireEvent.click(getByText('← Back'));
      expect(onBack).toHaveBeenCalledOnce();
    });
  });
});
