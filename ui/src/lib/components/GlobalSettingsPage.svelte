<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { Config } from '../types.js';

  interface Props {
    api: ActivateAPI;
    serverVersion: string;
    onBack: () => void;
  }

  let { api, serverVersion, onBack }: Props = $props();

  let globalCfg = $state<Config | null>(null);

  async function loadConfig() {
    globalCfg = await api.getConfig('global');
  }

  $effect(() => { loadConfig(); });

  let telemetryEnabled = $derived(globalCfg?.telemetryEnabled === true);
</script>

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
    <span class="text-xs">{globalCfg.repo || '(not set)'}</span>
  </div>
  <div class="setting-row">
    <span class="font-semibold text-xs">Branch</span>
    <span class="text-xs">{globalCfg.branch || '(not set)'}</span>
  </div>
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
  .toggle-btn.active {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
    box-shadow: 0 0 8px var(--color-activate-glow);
  }
</style>
