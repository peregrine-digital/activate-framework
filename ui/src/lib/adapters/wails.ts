/**
 * Wails adapter — bridges Go bindings to the shared ActivateAPI interface.
 *
 * In Wails v2, Go methods bound via `Bind: []interface{}{app}` are available
 * in the frontend as `window.go.main.App.MethodName(args)`.
 *
 * This adapter wraps those calls into the shared ActivateAPI contract so the
 * same Svelte components work across VS Code, desktop, and dev preview.
 */
import type { ActivateAPI } from '../api';
import type {
  AppState,
  Config,
  FileStatus,
  Manifest,
  TelemetryEntry,
  UsageSummary,
  DailyUsage,
} from '../types';

// Wails v2 injects Go bindings at `window.go.main.App`
declare global {
  interface Window {
    go: {
      main: {
        App: {
          SelectWorkspace(): Promise<any>;
          InitWorkspace(dir: string): Promise<void>;
          GetState(): Promise<any>;
          GetConfig(scope: string): Promise<any>;
          SetConfig(scope: string, updates: any): Promise<any>;
          RefreshConfig(): Promise<void>;
          ListManifests(): Promise<any[]>;
          ListFiles(manifestID: string, tierID: string, category: string): Promise<any>;
          InstallFile(file: string): Promise<any>;
          UninstallFile(file: string): Promise<any>;
          DiffFile(file: string): Promise<any>;
          SkipUpdate(file: string): Promise<any>;
          SetOverride(file: string, override: string): Promise<any>;
          Sync(): Promise<any>;
          Update(): Promise<any>;
          RepoAdd(): Promise<any>;
          RepoRemove(): Promise<void>;
          ListBranches(repo: string): Promise<string[]>;
          RunTelemetry(token: string): Promise<any>;
          ReadTelemetryLog(): Promise<any[]>;
          OpenFile(file: string): Promise<void>;
        };
      };
    };
  }
}

export function createWailsAPI(): ActivateAPI {
  const app = window.go.main.App;
  const listeners: Array<() => void> = [];

  return {
    platform: 'desktop',

    async getState(): Promise<AppState> {
      return app.GetState();
    },

    async getConfig(scope: string): Promise<Config> {
      return app.GetConfig(scope);
    },

    async setConfig(scope: string, updates: Partial<Config>): Promise<void> {
      await app.SetConfig(scope, updates as any);
    },

    async refreshConfig(): Promise<void> {
      await app.RefreshConfig();
      listeners.forEach((fn) => fn());
    },

    async listManifests(): Promise<Manifest[]> {
      return app.ListManifests() ?? [];
    },

    async installFile(file: string): Promise<void> {
      await app.InstallFile(file);
      listeners.forEach((fn) => fn());
    },

    async uninstallFile(file: string): Promise<void> {
      await app.UninstallFile(file);
      listeners.forEach((fn) => fn());
    },

    async diffFile(file: string): Promise<string> {
      const result = await app.DiffFile(file);
      return result?.diff ?? '';
    },

    async skipUpdate(file: string): Promise<void> {
      await app.SkipUpdate(file);
      listeners.forEach((fn) => fn());
    },

    async setFileOverride(file: string, override: string): Promise<void> {
      await app.SetOverride(file, override);
      listeners.forEach((fn) => fn());
    },

    async updateAll(): Promise<void> {
      await app.Update();
      listeners.forEach((fn) => fn());
    },

    async addToWorkspace(): Promise<void> {
      await app.RepoAdd();
      listeners.forEach((fn) => fn());
    },

    async removeFromWorkspace(): Promise<void> {
      await app.RepoRemove();
      listeners.forEach((fn) => fn());
    },

    async changeTier(tier: string): Promise<void> {
      await app.SetConfig('project', { tier } as any);
      listeners.forEach((fn) => fn());
    },

    async changeManifest(manifest: string): Promise<void> {
      await app.SetConfig('project', { manifest } as any);
      listeners.forEach((fn) => fn());
    },

    async listBranches(): Promise<string[]> {
      const state = await app.GetState();
      const repo = state?.config?.repo ?? '';
      if (!repo) return [];
      return app.ListBranches(repo);
    },

    async openFile(file: string): Promise<void> {
      await app.OpenFile(file);
    },

    async installCLI(): Promise<void> {
      // Desktop doesn't auto-install CLI — direct user to docs
    },

    async checkForUpdates(): Promise<void> {
      await app.Sync();
      listeners.forEach((fn) => fn());
    },

    async getUsageSummary(): Promise<UsageSummary> {
      const entries = await app.ReadTelemetryLog();
      if (!entries || entries.length === 0) {
        return {
          premiumEntitlement: 0,
          premiumUsed: 0,
          premiumRemaining: 0,
          quotaResetDate: '',
          daysTracked: 0,
        };
      }
      const latest = entries[entries.length - 1];
      return {
        premiumEntitlement: latest.premium_entitlement ?? 0,
        premiumUsed: latest.premium_used ?? 0,
        premiumRemaining: latest.premium_remaining ?? 0,
        quotaResetDate: latest.quota_reset_date_utc ?? '',
        daysTracked: entries.length,
      };
    },

    async getDailyUsage(): Promise<DailyUsage[]> {
      const entries = await app.ReadTelemetryLog();
      if (!entries) return [];
      return entries.map((e: any) => ({
        date: e.date,
        premiumUsed: e.premium_used ?? 0,
        premiumEntitlement: e.premium_entitlement ?? 0,
      }));
    },

    async runTelemetry(): Promise<void> {
      await app.RunTelemetry('');
      listeners.forEach((fn) => fn());
    },

    onStateChanged(callback: () => void): void {
      listeners.push(callback);
    },
  };
}
