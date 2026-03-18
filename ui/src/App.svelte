<script lang="ts">
  import './app.css';
  import { createMockAPI } from './lib/adapters/mock.js';
  import { createNavigation } from './lib/navigation.svelte.js';
  import type { AppState } from './lib/types.js';
  import WorkspaceView from './lib/components/WorkspaceView.svelte';

  const api = createMockAPI();
  const nav = createNavigation();

  let state = $state<AppState | null>(null);
  let loading = $state(true);

  async function load() {
    state = await api.getState();
    loading = false;
  }

  api.onStateChanged(() => load());
  load();
</script>

<div class="bg-activate-bg text-activate-fg min-h-screen font-sans text-sm px-2.5">
  {#if loading || !state}
    <div class="py-8 text-center opacity-50">Loading…</div>
  {:else}
    <WorkspaceView
      appState={state}
      {api}
      page={nav.page}
      onNavigate={nav.navigateTo}
      onBack={nav.navigateBack}
    />
  {/if}

  <!-- Dev nav (standalone only) -->
  <div class="fixed bottom-0 left-0 right-0 bg-activate-bg-surface border-t border-activate-border py-2 px-4 flex gap-2 text-xs">
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={nav.page === 'main'} onclick={() => nav.reset()}>Main</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={nav.page === 'usage'} onclick={() => nav.navigateTo('usage')}>Usage</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={nav.page === 'settings'} onclick={() => nav.navigateTo('settings')}>Settings</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={nav.page === 'no-cli'} onclick={() => nav.navigateTo('no-cli')}>No CLI</button>
  </div>
</div>
