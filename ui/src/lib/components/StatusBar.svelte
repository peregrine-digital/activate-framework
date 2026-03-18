<script lang="ts">
  import type { FileStatus } from '../types.js';

  interface Props {
    tier: string;
    tierLabel: string;
    manifestName: string;
    isActive: boolean;
    manifestCount: number;
    onShowSettings: () => void;
  }

  let { tier, tierLabel, manifestName, isActive, manifestCount, onShowSettings }: Props = $props();
</script>

<div class="glass flex items-center gap-3 px-3.5 py-2.5 mb-3 text-xs animate-in">
  <span class="status-badge">{tierLabel}</span>
  <span class="status-sep">·</span>
  <span class="status-badge">{manifestName}</span>
  <span class="status-sep">·</span>
  <span class="inline-flex items-center gap-1.5">
    <span class="status-dot" class:status-dot--active={isActive}></span>
    <span class="text-[11px] {isActive ? 'text-activate-fg' : 'text-activate-fg-muted'}">{isActive ? 'Active' : 'Inactive'}</span>
  </span>
  <span class="grow"></span>
  <button
    class="cursor-pointer opacity-50 text-base px-1.5 py-1 rounded-lg transition-all duration-150 hover:opacity-100 hover:bg-activate-bg-hover"
    onclick={onShowSettings}
    title="Settings"
  >⚙</button>
</div>

<style>
  .status-badge {
    background: rgba(63, 63, 70, 0.8);
    color: #d4d4d8;
    border: 1px solid rgba(82, 82, 91, 0.5);
    border-radius: 9999px;
    padding: 2px 10px;
    font-size: 11px;
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
