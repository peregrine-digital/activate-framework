import type {
  AppState,
  Config,
  DiffResult,
  FileStatus,
  Manifest,
  TelemetryEntry,
} from './types.js';

export type Platform = 'desktop' | 'vscode' | 'dev';

/**
 * Platform-agnostic API interface for the Activate service.
 *
 * Each target (VS Code, Wails, mock) provides its own implementation.
 * Methods match the ActivateService RPC surface used by the control panel.
 */
export interface ActivateAPI {
  /** Which platform this adapter is running on. */
  readonly platform: Platform;

  // ── State ──
  getState(): Promise<AppState>;
  getConfig(scope: 'global' | 'project' | 'resolved'): Promise<Config>;

  // ── Config mutations ──
  setConfig(updates: Partial<Config> & { scope: 'global' | 'project' }): Promise<void>;
  refreshConfig(): Promise<void>;

  // ── File operations ──
  installFile(file: FileStatus): Promise<void>;
  uninstallFile(file: FileStatus): Promise<void>;
  diffFile(file: FileStatus): Promise<DiffResult>;
  skipUpdate(file: FileStatus): Promise<void>;
  setFileOverride(dest: string, override: '' | 'pinned' | 'excluded'): Promise<void>;

  // ── Bulk operations ──
  updateAll(): Promise<void>;
  addToWorkspace(): Promise<void>;
  removeFromWorkspace(): Promise<void>;

  // ── Manifests ──
  listManifests(): Promise<Manifest[]>;
  listBranches(): Promise<string[]>;

  // ── Telemetry ──
  runTelemetry(): Promise<void>;
  readTelemetryLog(): Promise<TelemetryEntry[]>;

  // ── Navigation / platform actions ──
  openFile(file: FileStatus): Promise<void>;
  changeTier(): Promise<void>;
  changeManifest(): Promise<void>;
  installCLI(): Promise<void>;
  checkForUpdates(): Promise<void>;

  // ── Events ──
  onStateChanged(callback: () => void): () => void;
}
