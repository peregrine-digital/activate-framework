<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, Config } from '../types.js';

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

  function configSource(field: keyof Config): 'project' | 'global' | 'default' {
    if (projectCfg && projectCfg[field] != null && projectCfg[field] !== '') return 'project';
    if (globalCfg && globalCfg[field] != null && globalCfg[field] !== '') return 'global';
    return 'default';
  }

  function clearOverride(updates: Record<string, string>) {
    api.setConfig({ ...updates, scope: 'project' } as any);
  }
</script>

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
  <span class="text-xs flex items-center gap-1.5">
    {resolved.repo || 'peregrine-digital/activate-framework'}
    <span class="source-badge {configSource('repo')}">{configSource('repo')}</span>
  </span>
</div>

<div class="setting-row">
  <span class="font-semibold text-xs">Branch</span>
  <span class="text-xs flex items-center gap-1.5">
    {resolved.branch || 'main'}
    <span class="source-badge {configSource('branch')}">{configSource('branch')}</span>
  </span>
</div>

<hr class="divider" />

<div class="section-label">Project Overrides</div>

{#if projectCfg}
  {#each ['manifest', 'tier', 'repo', 'branch'] as field}
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
</style>
