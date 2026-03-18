<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, Config } from '../types.js';

  interface Props {
    appState: AppState;
    api: ActivateAPI;
    extensionVersion: string;
    serverVersion: string;
    onBack: () => void;
  }

  let { appState, api, extensionVersion, serverVersion, onBack }: Props = $props();

  let globalCfg = $state<Config | null>(null);
  let projectCfg = $state<Config | null>(null);

  async function loadConfigs() {
    globalCfg = await api.getConfig('global');
    projectCfg = await api.getConfig('project');
  }

  $effect(() => { loadConfigs(); });

  let resolved = $derived(appState.config);
  let tiers = $derived(appState.tiers);
  let telemetryEnabled = $derived(resolved.telemetryEnabled === true);
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

<div class="flex gap-1.5 my-2 flex-wrap">
  <button class="btn btn-secondary" onclick={onBack}>← Back</button>
  <h2 class="text-sm font-semibold flex-1 my-0">⚙ Settings</h2>
</div>

<hr class="border-none border-t border-activate-border my-2.5" />

<div class="text-[11px] uppercase tracking-wider opacity-60 mt-3.5 mb-1">Configuration</div>

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

<div class="setting-row">
  <span class="font-semibold text-xs">Telemetry</span>
  <span class="text-xs flex items-center gap-1.5">
    <button
      class="toggle-btn {telemetryEnabled ? 'active' : ''}"
      onclick={() => api.setConfig({ telemetryEnabled: !telemetryEnabled, scope: 'global' } as any)}
    >
      {telemetryEnabled ? '● Enabled' : '○ Disabled'}
    </button>
  </span>
</div>

<hr class="border-none border-t border-activate-border my-2.5" />

<div class="text-[11px] uppercase tracking-wider opacity-60 mt-3.5 mb-1">Global Defaults</div>

{#if globalCfg}
  {#each ['manifest', 'tier', 'repo', 'branch'] as field}
    <div class="setting-row">
      <span class="font-semibold text-xs capitalize">{field}</span>
      <span class="text-xs">{(globalCfg as any)[field] || '(not set)'}</span>
    </div>
  {/each}
  <div class="setting-row">
    <span class="font-semibold text-xs">Telemetry</span>
    <span class="text-xs">
      {globalCfg.telemetryEnabled === true ? 'Enabled' : globalCfg.telemetryEnabled === false ? 'Disabled' : '(not set)'}
    </span>
  </div>
{/if}

<hr class="border-none border-t border-activate-border my-2.5" />

<div class="text-[11px] uppercase tracking-wider opacity-60 mt-3.5 mb-1">Project Overrides</div>

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
{/if}

<hr class="border-none border-t border-activate-border my-2.5" />

<div class="text-[11px] uppercase tracking-wider opacity-60 mt-3.5 mb-1">Updates</div>
<div class="setting-row">
  <span class="font-semibold text-xs">CLI Version</span>
  <span class="text-xs">{serverVersion || '—'}</span>
</div>
<div class="setting-row">
  <span class="font-semibold text-xs">Extension Version</span>
  <span class="text-xs">{extensionVersion || '—'}</span>
</div>
<div class="py-1">
  <button class="btn btn-primary" onclick={() => api.checkForUpdates()}>🔄 Check for Updates</button>
</div>

<style>
  .setting-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 0;
    border-bottom: 1px solid color-mix(in srgb, var(--color-activate-border), transparent 60%);
  }
  .source-badge {
    font-size: 10px;
    padding: 1px 5px;
    border-radius: 3px;
    opacity: 0.8;
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
    border-radius: 3px;
    cursor: pointer;
    border: 1px solid transparent;
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
  }
  .toggle-btn.active {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
  }
  .btn {
    border: 1px solid transparent;
    border-radius: 3px;
    cursor: pointer;
    font-family: inherit;
    font-size: 12px;
    line-height: 20px;
    padding: 4px 10px;
    white-space: nowrap;
  }
  .btn-primary {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
  }
  .btn-primary:hover { background: var(--color-activate-btn-primary-hover); }
  .btn-secondary {
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
  }
  .btn-secondary:hover { background: var(--color-activate-btn-secondary-hover); }
</style>
