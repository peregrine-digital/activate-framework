<script lang="ts">
  import type { FileStatus } from '../types.js';
  import type { Platform } from '../api.js';

  interface Props {
    tier: string;
    tierLabel: string;
    manifestName: string;
    isActive: boolean;
    manifestCount: number;
    platform: Platform;
    hasPresets?: boolean;
    presetLabel?: string;
    onShowSettings: () => void;
  }

  let { tier, tierLabel, manifestName, isActive, manifestCount, platform, hasPresets = false, presetLabel = '—', onShowSettings }: Props = $props();
</script>

<div class="glass flex items-center gap-3 px-4 py-3 mb-4 text-xs animate-in">
  {#if hasPresets}
    <span class="status-badge">{presetLabel}</span>
  {:else}
    <span class="status-badge">{tierLabel}</span>
    <span class="status-sep">·</span>
    <span class="status-badge">{manifestName}</span>
  {/if}
  <span class="status-sep">·</span>
  <span class="inline-flex items-center gap-1.5">
    <span class="status-dot" class:status-dot--active={isActive}></span>
    <span class="text-xs {isActive ? 'text-activate-fg' : 'text-activate-fg-muted'}">{isActive ? 'Active' : 'Inactive'}</span>
  </span>
  <span class="grow"></span>
  {#if platform !== 'desktop'}
    <button
      class="cursor-pointer text-lg leading-none px-2 py-1.5 rounded-lg transition-all duration-150 opacity-60 hover:opacity-100 hover:bg-activate-bg-hover"
      onclick={onShowSettings}
      title="Settings"
    >⚙</button>
  {/if}
</div>

<style>
  .status-badge {
    background: rgba(63, 63, 70, 0.8);
    color: #d4d4d8;
    border: 1px solid rgba(82, 82, 91, 0.5);
    border-radius: 9999px;
    padding: 3px 12px;
    font-size: 12px;
    font-weight: 500;
    white-space: nowrap;
  }
  .status-sep {
    color: var(--color-activate-fg-muted);
    font-size: 14px;
    line-height: 1;
    opacity: 0.5;
  }
  .status-dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--color-activate-fg-muted);
    flex-shrink: 0;
  }
  .status-dot--active {
    background: var(--color-activate-success);
    box-shadow: 0 0 6px var(--color-activate-success);
  }
</style>
