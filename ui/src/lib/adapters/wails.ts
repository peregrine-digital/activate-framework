/**
 * Wails adapter — bridges Go daemon-backed methods to the shared ActivateAPI.
 *
 * The Go backend spawns `activate serve --stdio` and forwards JSON-RPC calls.
 * Go methods return raw JSON which we parse here. Daemon stateChanged
 * notifications are forwarded as Wails 'stateChanged' events.
 */
import type { ActivateAPI } from '../api';
import type {
  AppState,
  Config,
  DiffResult,
  FileStatus,
  Manifest,
  TelemetryEntry,
} from '../types';

// Wails v2 injects Go bindings at `window.go.main.App`
declare global {
  interface Window {
    go: {
      main: {
        App: {
          InitWorkspace(dir: string): Promise<void>;
          CloseWorkspace(): Promise<void>;
          SelectWorkspace(): Promise<any>;
          SetWorkspaceMenuVisible(visible: boolean): Promise<void>;
          ListWorkspaces(): Promise<any[]>;
          GetState(): Promise<any>;
          GetConfig(scope: string): Promise<any>;
          SetConfig(params: any): Promise<any>;
          InstallFile(dest: string): Promise<any>;
          UninstallFile(dest: string): Promise<any>;
          DiffFile(dest: string): Promise<any>;
          SkipUpdate(dest: string): Promise<any>;
          SetOverride(dest: string, override: string): Promise<any>;
          UpdateAll(): Promise<any>;
          AddToWorkspace(): Promise<any>;
          RemoveFromWorkspace(): Promise<any>;
          ListManifests(): Promise<any>;
          ListBranches(): Promise<any>;
          RunTelemetry(): Promise<any>;
          ReadTelemetryLog(): Promise<any>;
          CheckForUpdates(): Promise<any>;
          SyncManifests(): Promise<any>;
          OpenFile(file: string): Promise<void>;
        };
      };
    };
    runtime: {
      EventsOn(event: string, callback: (...args: any[]) => void): () => void;
      EventsOff(event: string): void;
    };
  }
}

export function createWailsAPI(): ActivateAPI {
  const app = window.go.main.App;
  const listeners = new Set<() => void>();

  // Listen for stateChanged events from the Go backend (forwarded from daemon)
  function setupEventListener() {
    if (typeof window !== 'undefined' && window.runtime?.EventsOn) {
      window.runtime.EventsOn('stateChanged', () => {
        console.log('[activate:wails] stateChanged event received, notifying', listeners.size, 'listeners');
        listeners.forEach((cb) => cb());
      });
      console.log('[activate:wails] EventsOn registered for stateChanged');
    } else {
      // Runtime not ready yet, retry
      console.log('[activate:wails] runtime not ready, retrying EventsOn in 100ms');
      setTimeout(setupEventListener, 100);
    }
  }
  setupEventListener();

  return {
    platform: 'desktop',

    async getState(): Promise<AppState> {
      const raw = await app.GetState();
      console.log('[activate:wails] GetState returned:', typeof raw, raw);
      return raw as AppState;
    },

    async getConfig(scope: 'global' | 'project' | 'resolved'): Promise<Config> {
      return app.GetConfig(scope);
    },

    async setConfig(updates: Partial<Config> & { scope: 'global' | 'project' }): Promise<void> {
      const { scope, ...rest } = updates;
      await app.SetConfig({ scope, updates: rest });
    },

    async refreshConfig(): Promise<void> {
      // Re-fetch state triggers a config reload in the daemon
      listeners.forEach((cb) => cb());
    },

    async installFile(file: FileStatus): Promise<void> {
      await app.InstallFile(file.dest);
    },

    async uninstallFile(file: FileStatus): Promise<void> {
      await app.UninstallFile(file.dest);
    },

    async diffFile(file: FileStatus): Promise<DiffResult> {
      const result = await app.DiffFile(file.dest);
      return result ?? { file: file.dest, diff: '' };
    },

    async skipUpdate(file: FileStatus): Promise<void> {
      await app.SkipUpdate(file.dest);
    },

    async setFileOverride(dest: string, override: '' | 'pinned' | 'excluded'): Promise<void> {
      await app.SetOverride(dest, override);
    },

    async updateAll(): Promise<void> {
      await app.UpdateAll();
    },

    async addToWorkspace(): Promise<void> {
      await app.AddToWorkspace();
    },

    async removeFromWorkspace(): Promise<void> {
      await app.RemoveFromWorkspace();
    },

    async listManifests(): Promise<Manifest[]> {
      return (await app.ListManifests()) ?? [];
    },

    async listBranches(): Promise<string[]> {
      return (await app.ListBranches()) ?? [];
    },

    async runTelemetry(): Promise<void> {
      await app.RunTelemetry();
    },

    async readTelemetryLog(): Promise<TelemetryEntry[]> {
      return (await app.ReadTelemetryLog()) ?? [];
    },

    async openFile(file: FileStatus): Promise<void> {
      await app.OpenFile(file.dest);
    },

    async changeTier(): Promise<void> {
      // TODO: show tier picker dialog
    },

    async changeManifest(): Promise<void> {
      // TODO: show manifest picker dialog
    },

    async installCLI(): Promise<void> {
      // Desktop users install CLI via shell script
    },

    async checkForUpdates(): Promise<void> {
      await app.CheckForUpdates();
    },

    onStateChanged(callback: () => void): () => void {
      listeners.add(callback);
      return () => listeners.delete(callback);
    },
  };
}
