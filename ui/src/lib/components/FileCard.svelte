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

<div class="file-row group" class:file-row--outdated={outdated}>
  <!-- Status icon — fixed 20px column -->
  <span class="file-status" class:text-activate-warning={installed && outdated} class:text-activate-success={installed && !outdated} class:opacity-30={!installed}>
    {installed ? (outdated ? '⬆' : '✓') : '○'}
  </span>

  <!-- File info — flex fill -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div
    class="file-info"
    class:cursor-pointer={installed}
    onclick={() => installed && onOpen(file)}
    role={installed ? 'button' : undefined}
    tabindex={installed ? 0 : -1}
    onkeydown={(e) => installed && e.key === 'Enter' && onOpen(file)}
  >
    <span class="file-name">
      {name}
      {#if installed}
        {#if outdated}
          <span class="file-version file-version--outdated" title="Installed: {iv} → Available: {bv}">v{iv} → v{bv}</span>
        {:else}
          <span class="file-version" title="Version {iv}">v{iv}</span>
        {/if}
      {/if}
      {#if file.override === 'pinned'}
        <span class="file-badge" title="Pinned — always included">📌</span>
      {:else if file.override === 'excluded'}
        <span class="file-badge" title="Excluded — never installed">🚫</span>
      {/if}
    </span>
    {#if desc}
      <span class="file-desc">{desc}</span>
    {/if}
  </div>

  <!-- Actions — fixed right column, consistent width -->
  <div class="file-actions">
    {#if installed && outdated}
      <button class="icon-btn" title="Show diff" onclick={(e) => { e.stopPropagation(); onDiff(file); }}>⇔</button>
      <button class="icon-btn" title="Skip update" onclick={(e) => { e.stopPropagation(); onSkipUpdate(file); }}>✓</button>
      <button class="icon-btn" title="Update" onclick={(e) => { e.stopPropagation(); onInstall(file); }}>↑</button>
    {:else if installed}
      <span class="icon-btn-spacer"></span>
      <span class="icon-btn-spacer"></span>
      <span class="icon-btn-spacer"></span>
    {:else}
      <span class="icon-btn-spacer"></span>
      <span class="icon-btn-spacer"></span>
      {#if file.override !== 'excluded'}
        <button class="icon-btn" title="Install" onclick={(e) => { e.stopPropagation(); onInstall(file); }}>↓</button>
      {:else}
        <span class="icon-btn-spacer"></span>
      {/if}
    {/if}

    {#if file.override === 'pinned'}
      <button class="icon-btn" title="Unpin" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, ''); }}>✕</button>
    {:else if file.override === 'excluded'}
      <button class="icon-btn" title="Include" onclick={(e) => { e.stopPropagation(); onSetOverride(file.dest, ''); }}>✕</button>
    {:else if installed}
      <button class="icon-btn icon-btn-danger" title="Uninstall" onclick={(e) => { e.stopPropagation(); onUninstall(file); }}>✕</button>
    {:else}
      <span class="icon-btn-spacer"></span>
    {/if}
  </div>
</div>

<style>
  .file-row {
    display: grid;
    grid-template-columns: 20px 1fr auto;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;
    border-radius: 0.5rem;
    min-height: 42px;
    transition: background 0.12s ease;
  }
  .file-row:hover {
    background: var(--color-activate-bg-hover);
  }
  .file-row--outdated {
    border-left: 2px solid var(--color-activate-warning);
    padding-left: 8px;
  }

  .file-status {
    font-size: 14px;
    text-align: center;
    line-height: 1;
  }

  .file-info {
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .file-name {
    font-size: 13px;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .file-version {
    font-size: 11px;
    opacity: 0.4;
    font-weight: 400;
    margin-left: 4px;
  }
  .file-version--outdated {
    color: var(--color-activate-warning);
    opacity: 0.85;
    font-weight: 500;
  }
  .file-badge {
    font-size: 11px;
    margin-left: 3px;
  }
  .file-desc {
    font-size: 11px;
    opacity: 0.5;
    line-height: 1.3;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 1;
    -webkit-box-orient: vertical;
  }

  .file-actions {
    display: flex;
    align-items: center;
    gap: 3px;
  }

  .icon-btn, .icon-btn-spacer {
    width: 28px;
    height: 28px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }
  .icon-btn {
    background: none;
    border: 1px solid transparent;
    color: var(--color-activate-fg);
    cursor: pointer;
    border-radius: 0.375rem;
    font-size: 14px;
    opacity: 0;
    transition: all 0.12s ease;
  }
  :global(.group):hover .icon-btn {
    opacity: 0.6;
  }
  .icon-btn:hover {
    opacity: 1 !important;
    background: var(--color-activate-bg-hover);
    border-color: var(--color-activate-border);
  }
  .icon-btn-danger:hover {
    color: var(--color-activate-error);
  }
</style>
