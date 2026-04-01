/**
 * TypeScript types matching the Go models in cli/model/.
 * These are the data structures returned by ActivateService RPC methods.
 */

export interface Config {
  manifest: string;
  tier: string;
  preset?: string;
  repo: string;
  branch: string;
  fileOverrides: Record<string, 'pinned' | 'excluded'>;
  skippedVersions: Record<string, string>;
  telemetryEnabled?: boolean;
}

export interface TierDef {
  id: string;
  label: string;
  description: string;
}

export interface ManifestFile {
  src: string;
  dest: string;
  tier: string;
  category: string;
  description: string;
  displayName?: string;
}

export interface Manifest {
  id: string;
  name: string;
  description: string;
  basePath: string;
  tiers: TierDef[];
  files: ManifestFile[];
}

export interface FileStatus {
  dest: string;
  category: string;
  installed: boolean;
  installedVersion: string | null;
  bundledVersion: string | null;
  updateAvailable: boolean;
  inTier: boolean;
  inPreset?: boolean;
  override: '' | 'pinned' | 'excluded';
  skipped: boolean;
  description: string;
  displayName?: string;
  tier: string;
}

export interface Preset {
  id: string;
  name: string;
  description: string;
  plugin?: string;
}

export interface Category {
  id: string;
  label: string;
}

export interface AppState {
  config: Config;
  tiers: TierDef[];
  manifests: Manifest[];
  presets?: Preset[];
  files: FileStatus[];
  categories: Category[];
  state: {
    hasInstallMarker: boolean;
  };
  projectDir: string;
  telemetryLogPath: string;
}

export interface TelemetryEntry {
  date: string;
  timestamp: string;
  premium_entitlement: number | null;
  premium_remaining: number | null;
  premium_used: number | null;
  quota_reset_date_utc: string | null;
}

export interface DiffResult {
  file: string;
  diff: string;
}

/** The page currently displayed in the UI. */
export type Page = 'main' | 'usage' | 'settings' | 'workspace-settings' | 'no-cli';
