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

<div class="flex items-center gap-2 py-2 text-xs opacity-85 flex-wrap">
  <span class="bg-activate-badge-bg text-activate-badge-fg rounded-full px-2 py-0.5 text-[11px]">
    {tierLabel}
  </span>
  <span class="opacity-40">·</span>
  <span class="bg-activate-badge-bg text-activate-badge-fg rounded-full px-2 py-0.5 text-[11px]">
    {manifestName}
  </span>
  <span class="opacity-40">·</span>
  <span class="inline-flex items-center gap-1">
    {isActive ? '✓' : '○'} Installed
  </span>
  <span class="grow"></span>
  <button
    class="cursor-pointer opacity-60 text-xl px-1 py-0.5 rounded transition-opacity hover:opacity-100 hover:bg-activate-bg-hover"
    onclick={onShowSettings}
    title="Settings"
  >⚙</button>
</div>
