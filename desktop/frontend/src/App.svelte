<script lang="ts">
  import '../../../ui/src/app.css';
  import { createWailsAPI } from '$lib/adapters/wails';
  import { createNavigation } from '$lib/navigation.svelte';
  import type { AppState } from '$lib/types';
  import WelcomePage from '$lib/components/WelcomePage.svelte';
  import WorkspaceView from '$lib/components/WorkspaceView.svelte';

  // Wails Go bindings
  const wailsApp = (window as any).go?.main?.App;

  const api = createWailsAPI();
  const nav = createNavigation();

  interface WorkspaceInfo {
    path: string;
    name: string;
    manifest?: string;
    tier?: string;
    fileCount: number;
    exists: boolean;
  }

  let view = $state<'welcome' | 'workspace'>('welcome');
  let appState = $state<AppState | null>(null);
  let workspaces = $state<WorkspaceInfo[]>([]);
  let loading = $state(true);

  // Listen for native menu events from Wails
  if (typeof window !== 'undefined') {
    (window as any).runtime?.EventsOn('navigate', (target: string) => {
      if (target === 'settings') {
        nav.navigateTo('settings');
      } else if (target === 'workspace-settings' && view === 'workspace') {
        nav.navigateTo('workspace-settings');
      } else if (target === 'usage' && view === 'workspace') {
        nav.navigateTo('usage');
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
    nav.reset();
    wailsApp?.SetWorkspaceMenuVisible(true);
  }

  async function browseWorkspace() {
    if (wailsApp?.SelectWorkspace) {
      const state = await wailsApp.SelectWorkspace();
      if (state?.projectDir) {
        appState = await api.getState();
        view = 'workspace';
        nav.reset();
        wailsApp?.SetWorkspaceMenuVisible(true);
        loadWorkspaces();
        return;
      }
    }
  }

  function backToWelcome() {
    view = 'welcome';
    nav.reset();
    appState = null;
    wailsApp?.CloseWorkspace();
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
  {#if nav.page === 'settings' && view !== 'workspace'}
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
  {#if nav.page === 'settings' && view !== 'workspace'}
    <!-- Global settings accessible from welcome screen -->
    {#await api.getState() then mockState}
      <WorkspaceView
        appState={mockState}
        {api}
        page={nav.page}
        onNavigate={nav.navigateTo}
        onBack={nav.navigateBack}
      />
    {/await}
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
  {:else}
    <WorkspaceView
      {appState}
      {api}
      page={nav.page}
      onNavigate={nav.navigateTo}
      onBack={nav.navigateBack}
    />
  {/if}
  </main>
</div>
