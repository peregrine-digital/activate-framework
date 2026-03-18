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

<div class="flex flex-col h-full">
  <div class="flex items-center justify-between mb-6">
    <h2 class="text-base font-semibold">Workspaces</h2>
    <button
      class="bg-activate-btn-bg text-activate-btn-fg px-3 py-1.5 rounded text-xs font-medium hover:opacity-90 cursor-pointer"
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
        class="bg-activate-btn-bg text-activate-btn-fg px-4 py-2 rounded text-sm font-medium hover:opacity-90 cursor-pointer"
        onclick={onBrowse}
      >
        Open Directory…
      </button>
    </div>
  {:else}
    <div class="flex flex-col gap-1">
      {#each workspaces as ws}
        <button
          class="flex items-start gap-3 p-3 rounded text-left hover:bg-activate-bg-hover transition-colors w-full cursor-pointer"
          class:opacity-40={!ws.exists}
          onclick={() => ws.exists && onSelect(ws.path)}
          disabled={!ws.exists}
        >
          <div class="text-lg mt-0.5">{ws.exists ? '📁' : '⚠️'}</div>
          <div class="flex-1 min-w-0">
            <div class="font-medium text-sm truncate">{ws.name}</div>
            <div class="text-xs opacity-60 truncate">{ws.path}</div>
            {#if ws.manifest || ws.fileCount > 0}
              <div class="flex items-center gap-2 mt-1">
                {#if ws.manifest}
                  <span class="text-[10px] px-1.5 py-0.5 rounded bg-activate-badge-bg text-activate-badge-fg">
                    {ws.manifest}
                  </span>
                {/if}
                {#if ws.tier}
                  <span class="text-[10px] px-1.5 py-0.5 rounded bg-activate-badge-bg text-activate-badge-fg">
                    {ws.tier}
                  </span>
                {/if}
                {#if ws.fileCount > 0}
                  <span class="text-[10px] opacity-50">
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
