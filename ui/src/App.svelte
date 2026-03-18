<script lang="ts">
  import './app.css';
  import { createMockAPI } from './lib/adapters/mock.js';
  import type { AppState, Page } from './lib/types.js';
  import MainPage from './lib/components/MainPage.svelte';
  import UsagePage from './lib/components/UsagePage.svelte';
  import SettingsPage from './lib/components/SettingsPage.svelte';
  import NoCliPage from './lib/components/NoCliPage.svelte';

  const api = createMockAPI();

  let page = $state<Page>('main');
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
  {:else if page === 'no-cli'}
    <NoCliPage onInstallCLI={() => api.installCLI()} />
  {:else if page === 'usage'}
    <UsagePage {api} telemetryEnabled={state.config.telemetryEnabled === true} onBack={() => page = 'main'} />
  {:else if page === 'settings'}
    <SettingsPage appState={state} {api} extensionVersion="0.2.5" serverVersion="0.5.0" onBack={() => page = 'main'} />
  {:else}
    <MainPage {state} {api} onNavigate={(p) => page = p} />
  {/if}

  <!-- Dev nav (standalone only) -->
  <div class="fixed bottom-0 left-0 right-0 bg-activate-bg-surface border-t border-activate-border py-2 px-4 flex gap-2 text-xs">
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={page === 'main'} onclick={() => page = 'main'}>Main</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={page === 'usage'} onclick={() => page = 'usage'}>Usage</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={page === 'settings'} onclick={() => page = 'settings'}>Settings</button>
    <button class="opacity-60 hover:opacity-100 cursor-pointer" class:font-bold={page === 'no-cli'} onclick={() => page = 'no-cli'}>No CLI</button>
  </div>
</div>
