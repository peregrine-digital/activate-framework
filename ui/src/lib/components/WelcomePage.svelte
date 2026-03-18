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
</script>

<div class="flex flex-col h-full min-w-0">
  <div class="flex items-center justify-between mb-4 gap-2">
    <h2 class="text-base font-semibold shrink-0">Workspaces</h2>
    <button
      class="bg-activate-btn-primary-bg text-activate-btn-primary-fg px-3 py-1.5 rounded text-xs font-medium hover:bg-activate-btn-primary-hover cursor-pointer shrink-0"
      onclick={onBrowse}
    >
      Open Directory…
    </button>
  </div>

  {#if workspaces.length === 0}
    <div class="flex-1 flex flex-col items-center justify-center text-center opacity-60 gap-4 py-12">
      <div class="text-3xl">📂</div>
      <div>
        <p class="font-medium mb-1">No workspaces found</p>
        <p class="text-xs">Open a directory to get started with Activate.</p>
      </div>
      <button
        class="bg-activate-btn-primary-bg text-activate-btn-primary-fg px-4 py-2 rounded text-sm font-medium hover:bg-activate-btn-primary-hover cursor-pointer"
        onclick={onBrowse}
      >
        Open Directory…
      </button>
    </div>
  {:else}
    <div class="flex flex-col gap-1">
      {#each workspaces as ws}
        <button
          class="glass flex items-start gap-3 p-3.5 text-left transition-all duration-150 w-full cursor-pointer min-w-0
            hover:border-activate-btn-primary-bg/30 hover:shadow-[0_0_15px_var(--color-activate-glow)]"
          class:opacity-40={!ws.exists}
          onclick={() => ws.exists && onSelect(ws.path)}
          disabled={!ws.exists}
        >
          <div class="text-base mt-0.5 shrink-0">{ws.exists ? '📁' : '⚠️'}</div>
          <div class="flex-1 min-w-0 overflow-hidden">
            <div class="font-medium text-sm truncate">{ws.name}</div>
            <div class="text-xs opacity-50 truncate">{ws.path}</div>
            {#if ws.manifest || ws.fileCount > 0}
              <div class="flex flex-wrap items-center gap-1.5 mt-1.5">
                {#if ws.manifest}
                  <span class="text-[10px] px-1.5 py-0.5 rounded bg-activate-badge-bg text-activate-badge-fg whitespace-nowrap">
                    {ws.manifest}
                  </span>
                {/if}
                {#if ws.tier}
                  <span class="text-[10px] px-1.5 py-0.5 rounded bg-activate-badge-bg text-activate-badge-fg whitespace-nowrap">
                    {ws.tier}
                  </span>
                {/if}
                {#if ws.fileCount > 0}
                  <span class="text-[10px] opacity-50 whitespace-nowrap">
                    {ws.fileCount} file{ws.fileCount === 1 ? '' : 's'}
                  </span>
                {/if}
              </div>
            {/if}
            {#if !ws.exists}
              <div class="text-[10px] text-activate-error mt-1">Directory not found</div>
            {/if}
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>
