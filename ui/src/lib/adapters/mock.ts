import type { ActivateAPI } from '../api.js';
import type {
  AppState,
  Config,
  DiffResult,
  FileStatus,
  Manifest,
  TelemetryEntry,
} from '../types.js';

const MOCK_FILES: FileStatus[] = [
  // Installed — current
  { dest: '.github/instructions/general.instructions.md', category: 'instructions', installed: true, installedVersion: '0.5.0', bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'Universal coding conventions and workflow expectations', displayName: 'general', tier: 'core' },
  { dest: '.github/instructions/security.instructions.md', category: 'instructions', installed: true, installedVersion: '0.5.0', bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'Security guardrails for all code changes', displayName: 'security', tier: 'core' },
  // Installed — outdated
  { dest: '.github/instructions/go.instructions.md', category: 'instructions', installed: true, installedVersion: '0.4.0', bundledVersion: '0.5.0', updateAvailable: true, inTier: true, override: '', skipped: false, description: 'Go language conventions and idioms', displayName: 'go', tier: 'standard' },
  // Installed — pinned
  { dest: '.github/prompts/create-adr.prompt.md', category: 'prompts', installed: true, installedVersion: '0.3.0', bundledVersion: '0.5.0', updateAvailable: true, inTier: true, override: 'pinned', skipped: false, description: 'Architecture Decision Record prompt template', displayName: 'create-adr', tier: 'standard' },
  // Installed — skill
  { dest: '.github/skills/pr-writing/SKILL.md', category: 'skills', installed: true, installedVersion: '0.5.0', bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'Pull request writing guidance', displayName: 'pr-writing', tier: 'standard' },
  // Available
  { dest: '.github/instructions/python.instructions.md', category: 'instructions', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'Python conventions and best practices', displayName: 'python', tier: 'standard' },
  { dest: '.github/prompts/write-user-story.prompt.md', category: 'prompts', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'User story creation prompt', displayName: 'write-user-story', tier: 'standard' },
  { dest: '.github/agents/codebase-documenter.agent.md', category: 'agents', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: '', skipped: false, description: 'Autonomous codebase documentation agent', displayName: 'codebase-documenter', tier: 'standard' },
  // Outside tier
  { dest: '.github/skills/ato-compliant-infrastructure/SKILL.md', category: 'skills', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: false, override: '', skipped: false, description: 'ATO-compliant infrastructure guidance', displayName: 'ato-compliant-infrastructure', tier: 'advanced' },
  { dest: '.github/agents/knowledge-guide.agent.md', category: 'agents', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: false, override: '', skipped: false, description: 'Proactive methodology guidance agent', displayName: 'knowledge-guide', tier: 'advanced' },
  // Excluded
  { dest: '.github/instructions/java.instructions.md', category: 'instructions', installed: false, installedVersion: null, bundledVersion: '0.5.0', updateAvailable: false, inTier: true, override: 'excluded', skipped: false, description: 'Java language conventions', displayName: 'java', tier: 'standard' },
];

const MOCK_CONFIG: Config = {
  manifest: 'activate-framework',
  tier: 'standard',
  repo: 'peregrine-digital/activate-framework',
  branch: 'main',
  fileOverrides: {
    '.github/prompts/create-adr.prompt.md': 'pinned',
    '.github/instructions/java.instructions.md': 'excluded',
  },
  skippedVersions: {},
  telemetryEnabled: true,
};

const MOCK_TELEMETRY: TelemetryEntry[] = Array.from({ length: 14 }, (_, i) => {
  const d = new Date();
  d.setDate(d.getDate() - (13 - i));
  const used = Math.floor(Math.random() * 180) + 20;
  return {
    date: d.toISOString().split('T')[0],
    timestamp: d.toISOString(),
    premium_entitlement: 300,
    premium_remaining: 300 - used,
    premium_used: used,
    quota_reset_date_utc: null,
  };
});

function mockState(): AppState {
  return {
    config: { ...MOCK_CONFIG },
    tiers: [
      { id: 'core', label: 'Core', description: 'Essential files only' },
      { id: 'standard', label: 'Standard', description: 'Recommended for most teams' },
      { id: 'advanced', label: 'Advanced', description: 'All available files' },
    ],
    manifests: [
      { id: 'activate-framework', name: 'Activate Framework', description: 'Main plugin', basePath: 'plugins/activate-framework', tiers: [], files: [] },
    ],
    files: [...MOCK_FILES],
    categories: [
      { id: 'instructions', label: 'Instructions' },
      { id: 'prompts', label: 'Prompts' },
      { id: 'skills', label: 'Skills' },
      { id: 'agents', label: 'Agents' },
      { id: 'mcp-servers', label: 'MCP Servers' },
      { id: 'other', label: 'Other' },
    ],
    state: { hasInstallMarker: true },
    projectDir: '/Users/demo/my-project',
    telemetryLogPath: '/Users/demo/.activate/telemetry.log',
  };
}

/**
 * Mock adapter for standalone development.
 * Returns realistic static data without requiring a running daemon.
 */
export function createMockAPI(): ActivateAPI {
  const listeners = new Set<() => void>();

  const notify = () => listeners.forEach((cb) => cb());

  const delay = <T>(value: T, ms = 200): Promise<T> =>
    new Promise((resolve) => setTimeout(() => resolve(value), ms));

  return {
    getState: () => delay(mockState()),
    getConfig: () => delay({ ...MOCK_CONFIG }),

    setConfig: async () => {
      await delay(undefined, 100);
      notify();
    },
    refreshConfig: async () => {
      await delay(undefined, 100);
      notify();
    },

    installFile: async () => {
      await delay(undefined, 300);
      notify();
    },
    uninstallFile: async () => {
      await delay(undefined, 200);
      notify();
    },
    diffFile: async (file) =>
      delay({
        file: file.dest,
        diff: `--- installed\n+++ bundled\n@@ -1,3 +1,3 @@\n-version: ${file.installedVersion}\n+version: ${file.bundledVersion}\n`,
      }, 300),
    skipUpdate: async () => {
      await delay(undefined, 100);
      notify();
    },
    setFileOverride: async () => {
      await delay(undefined, 100);
      notify();
    },

    updateAll: async () => {
      await delay(undefined, 500);
      notify();
    },
    addToWorkspace: async () => {
      await delay(undefined, 300);
      notify();
    },
    removeFromWorkspace: async () => {
      await delay(undefined, 300);
      notify();
    },

    listManifests: () => delay(mockState().manifests),
    listBranches: () => delay(['main', 'develop', 'feat/shared-ui-svelte']),

    runTelemetry: async () => delay(undefined, 400),
    readTelemetryLog: () => delay([...MOCK_TELEMETRY]),

    openFile: async () => {},
    changeTier: async () => notify(),
    changeManifest: async () => notify(),
    installCLI: async () => {},
    checkForUpdates: async () => {},

    onStateChanged: (callback) => {
      listeners.add(callback);
      return () => listeners.delete(callback);
    },
  };
}
