<script lang="ts">
  import '../../../ui/src/app.css';
  import { createMockAPI } from '$lib/adapters/mock';
  import type { AppState, Page } from '$lib/types';
  import MainPage from '$lib/components/MainPage.svelte';
  import UsagePage from '$lib/components/UsagePage.svelte';
  import SettingsPage from '$lib/components/SettingsPage.svelte';
  import NoCliPage from '$lib/components/NoCliPage.svelte';

  // TODO: Replace with createWailsAPI() once Go bindings are wired
  const api = createMockAPI();

  let page = $state<Page>('main');
  let appState = $state<AppState | null>(null);
  let loading = $state(true);

  async function load() {
    appState = await api.getState();
    loading = false;
  }

  api.onStateChanged(() => load());
  load();
</script>

<div class="bg-activate-bg text-activate-fg min-h-screen font-sans text-sm p-4">
  <header class="mb-4">
    <h1 class="text-lg font-bold">Activate Framework</h1>
  </header>

  {#if loading || !appState}
    <div class="py-8 text-center opacity-50">Loading…</div>
  {:else if page === 'no-cli'}
    <NoCliPage onInstallCLI={() => api.installCLI()} />
  {:else if page === 'usage'}
    <UsagePage {api} telemetryEnabled={appState.config.telemetryEnabled === true} onBack={() => page = 'main'} />
  {:else if page === 'settings'}
    <SettingsPage {appState} {api} extensionVersion="" serverVersion="0.5.0" onBack={() => page = 'main'} />
  {:else}
    <MainPage state={appState} {api} onNavigate={(p) => page = p} />
  {/if}
</div>
