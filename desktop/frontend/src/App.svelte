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

<div class="bg-activate-bg text-activate-fg h-screen w-screen overflow-hidden font-sans text-sm flex flex-col">
  {#if view === 'workspace'}
    <header class="shrink-0 px-4 pt-3 pb-1 flex items-center">
      <button
        class="opacity-50 hover:opacity-100 cursor-pointer text-xs flex items-center gap-1 transition-opacity duration-150"
        onclick={backToWelcome}
        title="Back to workspaces"
      >
        ← <span class="text-[11px]">Workspaces</span>
      </button>
    </header>
  {:else}
    <header class="shrink-0 px-4 pt-4 pb-2">
      <h1 class="text-lg font-bold">Activate Framework</h1>
    </header>
  {/if}

  <main class="flex-1 overflow-y-auto overflow-x-hidden px-4 pb-4">

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
  </main>
</div>
