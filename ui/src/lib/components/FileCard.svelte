<script lang="ts">
  import type { FileStatus } from '../types.js';

  interface Props {
    file: FileStatus;
    installed: boolean;
    skippedVersion?: string;
    onInstall: (file: FileStatus) => void;
    onUninstall: (file: FileStatus) => void;
    onDiff: (file: FileStatus) => void;
    onSkipUpdate: (file: FileStatus) => void;
    onOpen: (file: FileStatus) => void;
    onSetOverride: (dest: string, override: '' | 'pinned' | 'excluded') => void;
  }

  let {
    file,
    installed,
    skippedVersion,
    onInstall,
    onUninstall,
    onDiff,
    onSkipUpdate,
    onOpen,
    onSetOverride,
  }: Props = $props();

  let name = $derived(file.displayName || file.dest.split('/').pop()?.replace(/\.md$/, '') || file.dest);
  let desc = $derived(file.description || '');
  let iv = $derived(file.installedVersion || '?');
  let bv = $derived(file.bundledVersion || '?');
  let outdated = $derived(
    installed &&
    file.installedVersion &&
    file.bundledVersion &&
    file.installedVersion !== file.bundledVersion &&
    skippedVersion !== file.bundledVersion
  );
</script>

<div
  class="group flex items-center gap-1.5 px-2 py-1.5 pl-5 rounded-lg min-h-[34px] transition-all duration-150
    hover:bg-activate-bg-hover
    {outdated ? 'border-l-2 border-activate-warning' : ''}"
>
  <!-- Main content -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div
    class="flex items-start gap-1.5 flex-1 min-w-0 {installed ? 'cursor-pointer' : ''}"
    onclick={() => installed && onOpen(file)}
    role={installed ? 'button' : undefined}
    tabindex={installed ? 0 : -1}
    onkeydown={(e) => installed && e.key === 'Enter' && onOpen(file)}
  >
    <!-- Status icon -->
    <span class="shrink-0 w-3.5 text-center text-xs leading-[18px]
      {installed ? (outdated ? 'text-activate-warning' : 'text-activate-success') : 'opacity-40'}">
      {installed ? (outdated ? '⬆' : '✓') : '○'}
    </span>

    <!-- File info -->
    <div class="flex flex-col min-w-0">
      <span class="text-xs font-medium whitespace-nowrap overflow-hidden text-ellipsis">
        {name}

        {#if installed}
          {#if outdated}
            <span class="text-[10px] text-activate-warning opacity-90 font-medium ml-1" title="Installed: {iv} → Available: {bv}">
              v{iv} → v{bv}
            </span>
          {:else}
            <span class="text-[10px] opacity-45 font-normal ml-1" title="Version {iv}">v{iv}</span>
          {/if}
        {/if}

        {#if file.override === 'pinned'}
          <span class="text-[10px] ml-1" title="Pinned — always included">📌</span>
        {:else if file.override === 'excluded'}
          <span class="text-[10px] ml-1" title="Excluded — never installed">🚫</span>
        {/if}
      </span>
      <span class="text-[11px] opacity-65 leading-tight line-clamp-2">{desc}</span>
    </div>
  </div>

  <!-- Actions -->
  <div class="flex items-center gap-1 shrink-0">
    <span class="text-[10px] opacity-40 whitespace-nowrap">{file.tier}</span>

    <!-- File action buttons (visible on hover) -->
    {#if installed && outdated}
      <button class="icon-btn" title="Show diff" onclick={(e) => { e.stopPropagation(); onDiff(file); }}>⇔</button>
      <button class="icon-btn" title="Skip this update" onclick={(e) => { e.stopPropagation(); onSkipUpdate(file); }}>✓</button>
      <button class="icon-btn" title="Update to latest" onclick={(e) => { e.stopPropagation(); onInstall(file); }}>↑</button>
      <button class="icon-btn icon-btn-danger" title="Uninstall" onclick={(e) => { e.stopPropagation(); onUninstall(file); }}>✕</button>
    {:else if installed}
      <button class="icon-btn icon-btn-danger" title="Uninstall" onclick={(e) => { e.stopPropagation(); onUninstall(file); }}>✕</button>
    {:else if file.override !== 'excluded'}
      <button class="icon-btn" title="Install" onclick={(e) => { e.stopPropagation(); onInstall(file); }}>↓</button>
    {/if}

    <!-- Override buttons -->
    {#if file.override === 'pinned'}
      <button class="icon-btn" title="Remove pin" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, ''); }}>📌✕</button>
    {:else if file.override === 'excluded'}
      <button class="icon-btn" title="Remove exclusion" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, ''); }}>🚫✕</button>
    {:else}
      <button class="icon-btn" title="Pin (always include)" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, 'pinned'); }}>📌</button>
      <button class="icon-btn" title="Exclude (never install)" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, 'excluded'); }}>🚫</button>
    {/if}
  </div>
</div>

<style>
  .icon-btn {
    background: none;
    border: 1px solid transparent;
    color: var(--color-activate-fg);
    cursor: pointer;
    padding: 2px 5px;
    border-radius: 0.375rem;
    font-size: 13px;
    opacity: 0;
    transition: all 0.15s ease;
  }
  :global(.group):hover .icon-btn {
    opacity: 0.5;
  }
  .icon-btn:hover {
    opacity: 1 !important;
    background: var(--color-activate-bg-hover);
    transform: scale(1.1);
  }
  .icon-btn-danger:hover {
    color: var(--color-activate-error);
  }
</style>
