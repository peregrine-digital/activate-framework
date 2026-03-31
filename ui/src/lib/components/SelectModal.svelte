<script lang="ts">
  export interface SelectOption {
    id: string;
    label: string;
    description?: string;
    active?: boolean;
  }

  interface Props {
    title: string;
    options: SelectOption[];
    onSelect: (id: string) => void;
    onClose: () => void;
  }

  let { title, options, onSelect, onClose }: Props = $props();
  let search = $state('');

  let filtered = $derived(
    search
      ? options.filter(
          (o) =>
            o.label.toLowerCase().includes(search.toLowerCase()) ||
            o.description?.toLowerCase().includes(search.toLowerCase()),
        )
      : options,
  );

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Backdrop -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="modal-backdrop animate-in" onclick={onClose}>
  <!-- Modal -->
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="modal-panel" onclick={(e) => e.stopPropagation()}>
    <div class="modal-header">
      <h3 class="text-sm font-semibold">{title}</h3>
      <button
        class="opacity-40 hover:opacity-100 transition-opacity cursor-pointer text-lg leading-none"
        onclick={onClose}
      >×</button>
    </div>

    {#if options.length > 5}
      <div class="px-3 pb-2">
        <input
          class="modal-search"
          type="text"
          placeholder="Search…"
          bind:value={search}
          autofocus
        />
      </div>
    {/if}

    <div class="modal-list">
      {#each filtered as opt}
        <button
          class="modal-option"
          class:modal-option--active={opt.active}
          onclick={() => onSelect(opt.id)}
        >
          <span class="flex-1 min-w-0">
            <span class="text-[13px] font-medium truncate block">{opt.label}</span>
            {#if opt.description}
              <span class="text-[11px] opacity-40 truncate block mt-0.5">{opt.description}</span>
            {/if}
          </span>
          {#if opt.active}
            <span class="text-activate-success text-sm shrink-0">✓</span>
          {/if}
        </button>
      {:else}
        <div class="px-4 py-6 text-center text-xs opacity-40">No matches</div>
      {/each}
    </div>
  </div>
</div>

<style>
  .modal-backdrop {
    position: fixed;
    inset: 0;
    z-index: 50;
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 15vh;
    background: rgba(0, 0, 0, 0.5);
    backdrop-filter: blur(4px);
  }
  .modal-panel {
    width: 340px;
    max-height: 60vh;
    display: flex;
    flex-direction: column;
    border-radius: 0.75rem;
    border: 1px solid var(--color-activate-border);
    background: var(--color-activate-bg-elevated);
    box-shadow: 0 25px 50px rgba(0, 0, 0, 0.4);
    overflow: hidden;
  }
  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 14px;
    border-bottom: 1px solid var(--color-activate-border);
  }
  .modal-search {
    width: 100%;
    padding: 6px 10px;
    border-radius: 0.375rem;
    border: 1px solid var(--color-activate-border);
    background: rgba(39, 39, 42, 0.6);
    color: var(--color-activate-fg);
    font-size: 13px;
    outline: none;
  }
  .modal-search:focus {
    border-color: var(--color-activate-btn-primary-bg);
  }
  .modal-list {
    overflow-y: auto;
    padding: 4px;
  }
  .modal-option {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 10px;
    border-radius: 0.375rem;
    text-align: left;
    cursor: pointer;
    transition: background 0.1s ease;
    border: none;
    background: transparent;
    color: var(--color-activate-fg);
  }
  .modal-option:hover {
    background: rgba(63, 63, 70, 0.5);
  }
  .modal-option--active {
    background: rgba(59, 130, 246, 0.08);
    border: 1px solid rgba(59, 130, 246, 0.15);
  }
</style>
