<script lang="ts">
  import './app.css';
  import './vscode-theme.css';
  import { createVSCodeAPI } from './lib/adapters/vscode.js';
  import type { AppState, Page } from './lib/types.js';
  import MainPage from './lib/components/MainPage.svelte';
  import UsagePage from './lib/components/UsagePage.svelte';
  import SettingsPage from './lib/components/SettingsPage.svelte';
  import NoCliPage from './lib/components/NoCliPage.svelte';

  const api = createVSCodeAPI();

  let page = $state<Page>('main');
  let pageHistory = $state<Page[]>([]);
  let appState = $state<AppState | null>(null);
  let loading = $state(true);

  // Receive initial config from extension host
  let extensionVersion = $state('');
  let serverVersion = $state('');
  let hasCli = $state(true);

  function navigateTo(target: Page) {
    pageHistory.push(page);
    page = target;
  }

  function navigateBack() {
    page = pageHistory.pop() ?? 'main';
  }

  async function load() {
    try {
      appState = await api.getState();
      loading = false;
      hasCli = true;
    } catch {
      hasCli = false;
      loading = false;
      page = 'no-cli';
    }
  }

  api.onStateChanged(() => load());

  // Listen for init message from extension host
  if (typeof window !== 'undefined') {
    window.addEventListener('message', (event) => {
      const msg = event.data;
      if (msg?.type === 'init') {
        extensionVersion = msg.extensionVersion || '';
        serverVersion = msg.serverVersion || '';
        hasCli = msg.hasCli !== false;
        if (!hasCli) {
          page = 'no-cli';
          loading = false;
        } else {
          load();
        }
      }
    });
  }

  load();
</script>

<div class="bg-activate-bg text-activate-fg min-h-screen font-sans text-sm px-2.5">
  {#if loading || (!appState && hasCli)}
    <div class="py-8 text-center opacity-50">Loading…</div>
  {:else if page === 'no-cli' || !hasCli}
    <NoCliPage onInstallCLI={() => api.installCLI()} />
  {:else if appState && page === 'usage'}
    <UsagePage {api} telemetryEnabled={appState.config.telemetryEnabled === true} onBack={navigateBack} />
  {:else if appState && (page === 'settings' || page === 'workspace-settings')}
    <SettingsPage {appState} {api} onBack={navigateBack} />
  {:else if appState}
    <MainPage state={appState} {api} onNavigate={navigateTo} />
  {/if}
</div>
