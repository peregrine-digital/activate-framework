<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, Config } from '../types.js';
  import SelectModal from './SelectModal.svelte';
  import type { SelectOption } from './SelectModal.svelte';

  interface Props {
    appState: AppState;
    api: ActivateAPI;
    serverVersion: string;
    onBack: () => void;
  }

  let { appState, api, serverVersion, onBack }: Props = $props();

  let globalCfg = $state<Config | null>(null);

  async function loadConfig() {
    globalCfg = await api.getConfig('global');
  }

  $effect(() => { loadConfig(); });

  let telemetryEnabled = $derived(globalCfg?.telemetryEnabled === true);
  let hasPresets = $derived((appState.presets?.length ?? 0) > 0);
  let presetLabel = $derived(appState.presets?.find((p) => p.id === globalCfg?.preset)?.name || globalCfg?.preset || '(not set)');

  // Inline editing state
  let editingRepo = $state(false);
  let repoInput = $state('');
  let editingBranch = $state(false);
  let branchInput = $state('');
  let selectModal = $state<{ title: string; options: SelectOption[]; onSelect: (id: string) => void } | null>(null);

  function startEditRepo() {
    if (api.platform === 'vscode' && api.editRepo) {
      api.editRepo(globalCfg?.repo || '', 'global');
      return;
    }
    repoInput = globalCfg?.repo || '';
    editingRepo = true;
  }

  async function saveRepo() {
    const value = repoInput.trim();
    await api.setConfig({ repo: value || '__clear__', scope: 'global' });
    editingRepo = false;
    await loadConfig();
  }

  async function openBranchModal() {
    if (api.platform === 'vscode' && api.editBranch) {
      api.editBranch(globalCfg?.branch || '', 'global');
      return;
    }
    const branches = await api.listBranches();
    const options: SelectOption[] = [
      { id: '__clear__', label: '(reset to default)', description: 'Use default branch' },
      { id: '__custom__', label: 'Custom branch…', description: 'Enter a branch name manually' },
      ...branches.map((b) => ({ id: b, label: b, active: b === globalCfg?.branch })),
    ];
    selectModal = {
      title: 'Select Branch',
      options,
      onSelect: async (id) => {
        selectModal = null;
        if (id === '__custom__') {
          branchInput = globalCfg?.branch || '';
          editingBranch = true;
          return;
        }
        await api.setConfig({ branch: id, scope: 'global' });
        await loadConfig();
      },
    };
  }

  async function saveBranch() {
    const value = branchInput.trim();
    await api.setConfig({ branch: value || '__clear__', scope: 'global' });
    editingBranch = false;
    await loadConfig();
  }

  function openPresetModal() {
    if (api.platform === 'vscode') {
      api.changePreset();
      return;
    }
    const presets = appState.presets ?? [];
    const options: SelectOption[] = [
      { id: '__clear__', label: '(reset to default)', description: 'Remove preset override' },
      ...presets.map((p) => ({
        id: p.id,
        label: p.name,
        description: p.description,
        active: p.id === globalCfg?.preset,
      })),
    ];
    selectModal = {
      title: 'Select Preset',
      options,
      onSelect: async (id) => {
        selectModal = null;
        await api.setConfig({ preset: id, scope: 'global' } as any);
        await loadConfig();
      },
    };
  }
</script>

{#if selectModal}
  <SelectModal
    title={selectModal.title}
    options={selectModal.options}
    onSelect={selectModal.onSelect}
    onClose={() => (selectModal = null)}
  />
{/if}

<div class="flex gap-2 my-2 flex-wrap items-center">
  <button class="btn btn-secondary" onclick={onBack}>← Back</button>
  <h2 class="text-sm font-semibold flex-1 my-0">Settings</h2>
</div>

<hr class="divider" />

<div class="section-label">Global Defaults</div>

{#if globalCfg}
  <div class="setting-row">
    <span class="font-semibold text-xs">Manifest</span>
    <span class="text-xs">{globalCfg.manifest || '(not set)'}</span>
  </div>
  <div class="setting-row">
    <span class="font-semibold text-xs">Tier</span>
    <span class="text-xs">{globalCfg.tier || '(not set)'}</span>
  </div>
  <div class="setting-row">
    <span class="font-semibold text-xs">Repository</span>
    {#if editingRepo}
      <div class="flex items-center gap-1">
        <input
          type="text"
          bind:value={repoInput}
          placeholder="owner/repo"
          class="inline-input"
          onkeydown={(e) => { if (e.key === 'Enter') saveRepo(); if (e.key === 'Escape') editingRepo = false; }}
          autofocus
        />
        <button class="edit-action" onclick={saveRepo}>✓</button>
        <button class="edit-action" onclick={() => editingRepo = false}>✕</button>
      </div>
    {:else}
      <span class="text-xs flex items-center gap-1.5">
        {globalCfg.repo || '(not set)'}
        <button class="edit-btn" onclick={startEditRepo} title="Change repository">✎</button>
      </span>
    {/if}
  </div>
  <div class="setting-row">
    <span class="font-semibold text-xs">Branch</span>
    {#if editingBranch}
      <div class="flex items-center gap-1">
        <input
          type="text"
          bind:value={branchInput}
          placeholder="main"
          class="inline-input"
          onkeydown={(e) => { if (e.key === 'Enter') saveBranch(); if (e.key === 'Escape') editingBranch = false; }}
          autofocus
        />
        <button class="edit-action" onclick={saveBranch}>✓</button>
        <button class="edit-action" onclick={() => editingBranch = false}>✕</button>
      </div>
    {:else}
      <span class="text-xs flex items-center gap-1.5">
        {globalCfg.branch || '(not set)'}
        <button class="edit-btn" onclick={openBranchModal} title="Change branch">✎</button>
      </span>
    {/if}
  </div>
  {#if hasPresets}
    <div class="setting-row">
      <span class="font-semibold text-xs">Preset</span>
      <span class="text-xs flex items-center gap-1.5">
        {presetLabel}
        <button class="edit-btn" onclick={openPresetModal} title="Change preset">✎</button>
      </span>
    </div>
  {/if}
{:else}
  <div class="text-xs opacity-50 py-3 pl-2">Loading…</div>
{/if}

<hr class="divider" />

<div class="section-label">Telemetry</div>
<div class="setting-row">
  <span class="font-semibold text-xs">Usage tracking</span>
  <button
    class="toggle-btn {telemetryEnabled ? 'active' : ''}"
    onclick={() => api.setConfig({ telemetryEnabled: !telemetryEnabled, scope: 'global' } as any)}
  >
    {telemetryEnabled ? '● Enabled' : '○ Disabled'}
  </button>
</div>

<hr class="divider" />

<div class="section-label">Updates</div>
<div class="setting-row">
  <span class="font-semibold text-xs">CLI Version</span>
  <span class="text-xs">{serverVersion || '—'}</span>
</div>
{#if api.platform !== 'desktop'}
<div class="py-2">
  <button class="btn btn-primary" onclick={() => api.checkForUpdates()}>🔄 Check for Updates</button>
</div>
{/if}

<style>
  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 0;
    border-bottom: 1px solid color-mix(in srgb, var(--color-activate-border), transparent 50%);
    transition: background 0.15s ease;
  }
  .setting-row:hover {
    background: var(--color-activate-bg-hover);
    margin: 0 -0.5rem;
    padding-left: 0.5rem;
    padding-right: 0.5rem;
    border-radius: 0.5rem;
  }
  .toggle-btn {
    font-size: 11px;
    padding: 2px 8px;
    border-radius: 0.375rem;
    cursor: pointer;
    border: 1px solid transparent;
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
    transition: all 0.15s ease;
  }
  .toggle-btn:hover {
    background: var(--color-activate-btn-secondary-hover);
  }
  .edit-btn {
    font-size: 12px;
    padding: 1px 4px;
    border-radius: 0.25rem;
    cursor: pointer;
    border: none;
    background: transparent;
    color: var(--color-activate-fg);
    opacity: 0.35;
    transition: opacity 0.15s ease;
  }
  .edit-btn:hover {
    opacity: 1;
  }
  .edit-action {
    font-size: 12px;
    padding: 2px 6px;
    border-radius: 0.25rem;
    cursor: pointer;
    border: 1px solid transparent;
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
    transition: all 0.15s ease;
  }
  .edit-action:hover {
    background: var(--color-activate-btn-secondary-hover);
  }
  .inline-input {
    font-size: 12px;
    padding: 2px 6px;
    border-radius: 0.25rem;
    border: 1px solid var(--color-activate-border);
    background: rgba(39, 39, 42, 0.6);
    color: var(--color-activate-fg);
    width: 12rem;
    outline: none;
  }
  .inline-input:focus {
    border-color: var(--color-activate-btn-primary-bg);
  }
  .toggle-btn.active {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
    box-shadow: 0 0 8px var(--color-activate-glow);
  }
</style>
