<script lang="ts">
  import '../../../ui/src/app.css';
  import { createMockAPI } from '$lib/adapters/mock';
  import type { AppState, Page } from '$lib/types';
  import WelcomePage from '$lib/components/WelcomePage.svelte';
  import MainPage from '$lib/components/MainPage.svelte';
  import UsagePage from '$lib/components/UsagePage.svelte';
  import SettingsPage from '$lib/components/SettingsPage.svelte';
  import NoCliPage from '$lib/components/NoCliPage.svelte';

  // Wails Go bindings
  const wailsApp = (window as any).go?.main?.App;

  // TODO: Replace mock with createWailsAPI() once fully wired
  const api = createMockAPI();

  interface WorkspaceInfo {
    path: string;
    name: string;
    manifest?: string;
    tier?: string;
    fileCount: number;
    exists: boolean;
  }

  let view = $state<'welcome' | 'workspace'>('welcome');
  let page = $state<Page>('main');
  let appState = $state<AppState | null>(null);
  let workspaces = $state<WorkspaceInfo[]>([]);
  let loading = $state(true);

  async function loadWorkspaces() {
    if (wailsApp?.ListWorkspaces) {
      workspaces = (await wailsApp.ListWorkspaces()) ?? [];
    }
    loading = false;
  }

  async function selectWorkspace(path: string) {
    if (wailsApp?.InitWorkspace) {
      await wailsApp.InitWorkspace(path);
    }
    appState = await api.getState();
    view = 'workspace';
    page = 'main';
  }

  async function browseWorkspace() {
    if (wailsApp?.SelectWorkspace) {
      const state = await wailsApp.SelectWorkspace();
      if (state?.projectDir) {
        appState = await api.getState();
        view = 'workspace';
        page = 'main';
        loadWorkspaces();
        return;
      }
    }
  }

  function backToWelcome() {
    view = 'welcome';
    appState = null;
    loadWorkspaces();
  }

  api.onStateChanged(async () => {
    if (view === 'workspace') {
      appState = await api.getState();
    }
  });

  loadWorkspaces();
</script>

<div class="bg-activate-bg text-activate-fg min-h-screen font-sans text-sm p-4">
  <header class="mb-4 flex items-center gap-2">
    {#if view === 'workspace'}
      <button
        class="opacity-60 hover:opacity-100 cursor-pointer text-xs"
        onclick={backToWelcome}
        title="Back to workspaces"
      >
        ←
      </button>
    {/if}
    <h1 class="text-lg font-bold">Activate Framework</h1>
  </header>

  {#if loading}
    <div class="py-8 text-center opacity-50">Loading…</div>
  {:else if view === 'welcome'}
    <WelcomePage
      {workspaces}
      onSelect={selectWorkspace}
      onBrowse={browseWorkspace}
    />
  {:else if !appState}
    <div class="py-8 text-center opacity-50">Loading workspace…</div>
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
