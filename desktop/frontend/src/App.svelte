<script lang="ts">
  import '../../../ui/src/app.css';
  import { createMockAPI } from '$lib/adapters/mock';
  import type { AppState, Page } from '$lib/types';
  import WelcomePage from '$lib/components/WelcomePage.svelte';
  import MainPage from '$lib/components/MainPage.svelte';
  import UsagePage from '$lib/components/UsagePage.svelte';
  import SettingsPage from '$lib/components/SettingsPage.svelte';
  import GlobalSettingsPage from '$lib/components/GlobalSettingsPage.svelte';
  import NoCliPage from '$lib/components/NoCliPage.svelte';

  // Wails Go bindings
  const wailsApp = (window as any).go?.main?.App;

  // TODO: Replace mock with createWailsAPI() once fully wired
  const api = createMockAPI('desktop');

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
  let pageHistory = $state<Page[]>([]);
  let appState = $state<AppState | null>(null);
  let workspaces = $state<WorkspaceInfo[]>([]);
  let loading = $state(true);

  function navigateTo(target: Page) {
    pageHistory.push(page);
    page = target;
  }

  function navigateBack() {
    page = pageHistory.pop() ?? 'main';
  }

  // Listen for native menu events from Wails
  if (typeof window !== 'undefined') {
    (window as any).runtime?.EventsOn('navigate', (target: string) => {
      if (target === 'settings') {
        navigateTo('settings');
      } else if (target === 'workspace-settings' && view === 'workspace') {
        navigateTo('workspace-settings');
      } else if (target === 'usage' && view === 'workspace') {
        navigateTo('usage');
      } else if (target === 'browse') {
        browseWorkspace();
      }
    });
  }

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
    wailsApp?.SetWorkspaceMenuVisible(true);
  }

  async function browseWorkspace() {
    if (wailsApp?.SelectWorkspace) {
      const state = await wailsApp.SelectWorkspace();
      if (state?.projectDir) {
        appState = await api.getState();
        view = 'workspace';
        page = 'main';
        wailsApp?.SetWorkspaceMenuVisible(true);
        loadWorkspaces();
        return;
      }
    }
  }

  function backToWelcome() {
    view = 'welcome';
    page = 'main';
    pageHistory = [];
    appState = null;
    wailsApp?.SetWorkspaceMenuVisible(false);
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
  {#if page === 'settings'}
    <header class="shrink-0 px-4 pt-3 pb-1"></header>
  {:else if view === 'workspace'}
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

  {#if page === 'settings'}
    <GlobalSettingsPage {api} serverVersion="0.5.0" onBack={navigateBack} />
  {:else if loading}
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
    <UsagePage {api} telemetryEnabled={appState.config.telemetryEnabled === true} onBack={navigateBack} />
  {:else if page === 'workspace-settings'}
    <SettingsPage {appState} {api} onBack={navigateBack} />
  {:else}
    <MainPage state={appState} {api} onNavigate={navigateTo} />
  {/if}
  </main>
</div>
