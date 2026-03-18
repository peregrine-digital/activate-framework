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

<div class="glass flex items-center gap-2.5 px-3.5 py-2.5 mb-3 text-xs animate-in">
  <span class="bg-activate-badge-bg text-activate-badge-fg rounded-full px-2.5 py-0.5 text-[11px] font-medium">
    {tierLabel}
  </span>
  <span class="w-px h-3 bg-activate-border"></span>
  <span class="bg-activate-badge-bg text-activate-badge-fg rounded-full px-2.5 py-0.5 text-[11px] font-medium">
    {manifestName}
  </span>
  <span class="w-px h-3 bg-activate-border"></span>
  <span class="inline-flex items-center gap-1 {isActive ? 'text-activate-success' : 'text-activate-fg-muted'}">
    {isActive ? '●' : '○'}
    <span class="text-activate-fg">{isActive ? 'Active' : 'Inactive'}</span>
  </span>
  <span class="grow"></span>
  <button
    class="cursor-pointer opacity-50 text-base px-1.5 py-1 rounded-lg transition-all duration-150 hover:opacity-100 hover:bg-activate-bg-hover hover:scale-110"
    onclick={onShowSettings}
    title="Settings"
  >⚙</button>
</div>
