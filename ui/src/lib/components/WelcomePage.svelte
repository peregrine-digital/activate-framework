<script lang="ts">
  interface WorkspaceInfo {
    path: string;
    name: string;
    manifest?: string;
    tier?: string;
    fileCount: number;
    exists: boolean;
  }

  interface Props {
    workspaces: WorkspaceInfo[];
    onSelect: (path: string) => void;
    onBrowse: () => void;
  }

  let { workspaces, onSelect, onBrowse }: Props = $props();

  let validWorkspaces = $derived(workspaces.filter((w) => w.exists));
  let missingWorkspaces = $derived(workspaces.filter((w) => !w.exists));
</script>

<div class="flex flex-col h-full min-w-0 gap-5">
  <!-- Header row -->
  <div class="flex items-center justify-between gap-3">
    <h2 class="text-sm font-semibold text-activate-fg-muted uppercase tracking-wide">Workspaces</h2>
    <button class="btn btn-primary text-xs" onclick={onBrowse}>
      + Open Directory
    </button>
  </div>

  {#if workspaces.length === 0}
    <!-- Empty state -->
    <div class="flex-1 flex flex-col items-center justify-center text-center gap-5 py-16 animate-in">
      <div class="text-4xl opacity-40">📂</div>
      <div>
        <p class="font-medium text-sm mb-1.5">No workspaces found</p>
        <p class="text-xs opacity-50">Open a project directory to get started.</p>
      </div>
      <button class="btn btn-primary" onclick={onBrowse}>Open Directory</button>
    </div>
  {:else}
    <!-- Workspace list -->
    <div class="flex flex-col gap-2">
      {#each validWorkspaces as ws, i}
        <button
          class="ws-card animate-in"
          style="animation-delay: {i * 30}ms"
          onclick={() => onSelect(ws.path)}
        >
          <div class="flex items-center gap-3 min-w-0">
            <span class="text-lg shrink-0 opacity-60">📁</span>
            <div class="flex-1 min-w-0">
              <div class="font-semibold text-[13px] text-activate-fg truncate">{ws.name}</div>
              <div class="text-[11px] opacity-40 truncate mt-0.5">{ws.path}</div>
            </div>
          </div>
          {#if ws.manifest || ws.fileCount > 0}
            <div class="flex items-center gap-2 mt-2 pl-8">
              {#if ws.manifest}
                <span class="ws-badge">{ws.manifest}</span>
              {/if}
              {#if ws.tier}
                <span class="ws-badge">{ws.tier}</span>
              {/if}
              {#if ws.fileCount > 0}
                <span class="text-[11px] opacity-40">{ws.fileCount} file{ws.fileCount === 1 ? '' : 's'}</span>
              {/if}
            </div>
          {/if}
        </button>
      {/each}
    </div>

    <!-- Missing workspaces (collapsed) -->
    {#if missingWorkspaces.length > 0}
      <details class="mt-2">
        <summary class="text-[11px] opacity-40 cursor-pointer hover:opacity-60 select-none transition-opacity">
          {missingWorkspaces.length} missing workspace{missingWorkspaces.length === 1 ? '' : 's'}
        </summary>
        <div class="flex flex-col gap-1.5 mt-2 pl-1">
          {#each missingWorkspaces as ws}
            <div class="flex items-center gap-2 opacity-30 text-[11px]">
              <span>⚠️</span>
              <span class="truncate">{ws.name}</span>
              <span class="opacity-50 truncate shrink">{ws.path}</span>
            </div>
          {/each}
        </div>
      </details>
    {/if}
  {/if}
</div>

<style>
  .ws-card {
    display: flex;
    flex-direction: column;
    width: 100%;
    padding: 12px 14px;
    border-radius: 0.625rem;
    border: 1px solid var(--color-activate-border);
    background: rgba(39, 39, 42, 0.4);
    cursor: pointer;
    text-align: left;
    transition: all 0.15s ease;
  }
  .ws-card:hover {
    border-color: color-mix(in srgb, var(--color-activate-btn-primary-bg), transparent 60%);
    background: rgba(39, 39, 42, 0.7);
    box-shadow: 0 0 20px rgba(59, 130, 246, 0.06);
  }
  .ws-badge {
    font-size: 10px;
    padding: 1px 8px;
    border-radius: 9999px;
    background: rgba(63, 63, 70, 0.8);
    color: #a1a1aa;
    border: 1px solid rgba(82, 82, 91, 0.4);
    white-space: nowrap;
  }
</style>
