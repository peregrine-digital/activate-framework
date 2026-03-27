<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, Config } from '../types.js';
  import SelectModal from './SelectModal.svelte';
  import type { SelectOption } from './SelectModal.svelte';

  interface Props {
    appState: AppState;
    api: ActivateAPI;
    onBack: () => void;
  }

  let { appState, api, onBack }: Props = $props();

  let globalCfg = $state<Config | null>(null);
  let projectCfg = $state<Config | null>(null);

  async function loadConfigs() {
    globalCfg = await api.getConfig('global');
    projectCfg = await api.getConfig('project');
  }

  $effect(() => { loadConfigs(); });

  let resolved = $derived(appState.config);
  let tiers = $derived(appState.tiers);
  let tierLabel = $derived(tiers.find((t) => t.id === resolved.tier)?.label || resolved.tier || '—');
  let hasPresets = $derived((appState.presets?.length ?? 0) > 0);
  let presetLabel = $derived(appState.presets?.find((p) => p.id === resolved.preset)?.name || resolved.preset || '—');
  let overrideFields = $derived(hasPresets ? ['manifest', 'tier', 'repo', 'branch', 'preset'] : ['manifest', 'tier', 'repo', 'branch']);

  // Inline editing state
  let editingRepo = $state(false);
  let repoInput = $state('');
  let editingBranch = $state(false);
  let branchInput = $state('');
  let selectModal = $state<{ title: string; options: SelectOption[]; onSelect: (id: string) => void } | null>(null);

  function configSource(field: keyof Config): 'project' | 'global' | 'default' {
    if (projectCfg && projectCfg[field] != null && projectCfg[field] !== '') return 'project';
    if (globalCfg && globalCfg[field] != null && globalCfg[field] !== '') return 'global';
    return 'default';
  }

  async function clearOverride(updates: Record<string, string>) {
    await api.setConfig({ ...updates, scope: 'project' } as any);
    await loadConfigs();
  }

  function startEditRepo() {
    repoInput = resolved.repo || '';
    editingRepo = true;
  }

  async function saveRepo() {
    const value = repoInput.trim();
    await api.setConfig({ repo: value || '__clear__', scope: 'project' });
    editingRepo = false;
    await loadConfigs();
  }

  async function openBranchModal() {
    const branches = await api.listBranches();
    const options: SelectOption[] = [
      { id: '__clear__', label: '(reset to default)', description: 'Use default branch' },
      { id: '__custom__', label: 'Custom branch…', description: 'Enter a branch name manually' },
      ...branches.map((b) => ({ id: b, label: b, active: b === resolved.branch })),
    ];
    selectModal = {
      title: 'Select Branch',
      options,
      onSelect: async (id) => {
        selectModal = null;
        if (id === '__custom__') {
          branchInput = resolved.branch || '';
          editingBranch = true;
          return;
        }
        await api.setConfig({ branch: id, scope: 'project' });
        await loadConfigs();
      },
    };
  }

  async function saveBranch() {
    const value = branchInput.trim();
    await api.setConfig({ branch: value || '__clear__', scope: 'project' });
    editingBranch = false;
    await loadConfigs();
  }

  function openPresetModal() {
    const presets = appState.presets ?? [];
    const options: SelectOption[] = [
      { id: '__clear__', label: '(reset to default)', description: 'Remove preset override' },
      ...presets.map((p) => ({
        id: p.id,
        label: p.name,
        description: p.description,
        active: p.id === resolved.preset,
      })),
    ];
    selectModal = {
      title: 'Select Preset',
      options,
      onSelect: async (id) => {
        selectModal = null;
        await api.setConfig({ preset: id, scope: 'project' } as any);
        await loadConfigs();
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
  <h2 class="text-sm font-semibold flex-1 my-0">Workspace Settings</h2>
</div>

<hr class="divider" />

<div class="section-label">Resolved Configuration</div>

<div class="setting-row">
  <span class="font-semibold text-xs">Manifest</span>
  <span class="text-xs flex items-center gap-1.5">
    {resolved.manifest || '—'}
    <span class="source-badge {configSource('manifest')}">{configSource('manifest')}</span>
  </span>
</div>

<div class="setting-row">
  <span class="font-semibold text-xs">Tier</span>
  <span class="text-xs flex items-center gap-1.5">
    {tierLabel}
    <span class="source-badge {configSource('tier')}">{configSource('tier')}</span>
  </span>
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
      {resolved.repo || 'peregrine-digital/activate-framework'}
      <span class="source-badge {configSource('repo')}">{configSource('repo')}</span>
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
      {resolved.branch || 'main'}
      <span class="source-badge {configSource('branch')}">{configSource('branch')}</span>
      <button class="edit-btn" onclick={openBranchModal} title="Change branch">✎</button>
    </span>
  {/if}
</div>

{#if hasPresets}
  <div class="setting-row">
    <span class="font-semibold text-xs">Preset</span>
    <span class="text-xs flex items-center gap-1.5">
      {presetLabel}
      <span class="source-badge {configSource('preset')}">{configSource('preset')}</span>
      <button class="edit-btn" onclick={openPresetModal} title="Change preset">✎</button>
    </span>
  </div>
{/if}

<hr class="divider" />

<div class="section-label">Project Overrides</div>

{#if projectCfg}
  {#each overrideFields as field}
    <div class="setting-row">
      <span class="font-semibold text-xs capitalize">{field}</span>
      <span class="text-xs flex items-center gap-1.5">
        {(projectCfg as any)[field] || '(not set)'}
        {#if (projectCfg as any)[field]}
          <button class="toggle-btn" onclick={() => clearOverride({ [field]: '__clear__' })}>✕</button>
        {/if}
      </span>
    </div>
  {/each}
{:else}
  <div class="text-xs opacity-50 py-3 pl-2">Loading…</div>
{/if}

{#if resolved.fileOverrides && Object.keys(resolved.fileOverrides).length > 0}
  <hr class="divider" />

  <div class="section-label">File Overrides</div>
  {#each Object.entries(resolved.fileOverrides) as [dest, override]}
    <div class="setting-row">
      <span class="text-xs truncate flex-1 min-w-0">{dest.split('/').pop()}</span>
      <span class="text-xs flex items-center gap-1.5">
        <span class="source-badge project">{override}</span>
        <button class="toggle-btn" onclick={() => api.setFileOverride(dest, '')}>✕</button>
      </span>
    </div>
  {/each}
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
  .source-badge {
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 0.25rem;
    font-weight: 500;
  }
  .source-badge.project {
    background: var(--color-activate-badge-bg);
    color: var(--color-activate-badge-fg);
  }
  .source-badge.global {
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
  }
  .source-badge.default {
    opacity: 0.4;
    font-style: italic;
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
  .toggle-btn.active {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
    box-shadow: 0 0 8px var(--color-activate-glow);
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
</style>
