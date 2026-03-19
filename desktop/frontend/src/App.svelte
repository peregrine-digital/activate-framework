<script lang="ts">
  import '../../../ui/src/app.css';
  import { flushSync } from 'svelte';
  import { createWailsAPI } from '$lib/adapters/wails';
  import { createNavigation } from '$lib/navigation.svelte';
  import type { AppState } from '$lib/types';
  import WelcomePage from '$lib/components/WelcomePage.svelte';
  import WorkspaceView from '$lib/components/WorkspaceView.svelte';
  import NoCliPage from '$lib/components/NoCliPage.svelte';

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

  let view = $state<'welcome' | 'loading' | 'workspace' | 'no-cli'>('welcome');
  let appState = $state<AppState | null>(null);
  let stateVersion = $state(0);
  let workspaces = $state<WorkspaceInfo[]>([]);
  let loading = $state(true);
  let loadingName = $state('');
  let loadingError = $state('');

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
    (window as any).runtime?.EventsOn('checkForUpdates', async () => {
      try {
        const result = await wailsApp?.CheckForUpdates?.();
        if (!result) return;
        const msgs: string[] = [];
        if (result.updateAvailable) {
          msgs.push(`CLI: v${result.currentVersion} → v${result.latestVersion}`);
        }
        if (result.desktop?.available) {
          msgs.push(`Desktop: v${version} → v${result.desktop.version}`);
        }
        if (msgs.length === 0) {
          (window as any).runtime?.MessageDialog?.({
            Type: 'InfoDialog',
            Title: 'Up to Date',
            Message: 'Everything is up to date.',
          });
        } else {
          const yes = await (window as any).runtime?.MessageDialog?.({
            Type: 'QuestionDialog',
            Title: 'Updates Available',
            Message: msgs.join('\n') + '\n\nUpdate now?',
          });
          if (yes === 'Yes') {
            if (result.updateAvailable) {
              await wailsApp?.UpdateCLI?.();
              await wailsApp?.RestartDaemon?.();
            }
            if (result.desktop?.available && result.desktop.downloadUrl) {
              (window as any).runtime?.BrowserOpenURL?.(result.desktop.downloadUrl);
            }
          }
        }
      } catch (e) {
        console.error('[activate] checkForUpdates error:', e);
      }
    });
  }

  let desktopVersion = '';
  if (wailsApp?.Version) {
    wailsApp.Version().then((v: string) => { desktopVersion = v; });
  }

  async function loadWorkspaces() {
    try {
      // Check if CLI binary is available
      if (wailsApp?.CLIFound) {
        const found = await wailsApp.CLIFound();
        if (!found) {
          flushSync(() => { view = 'no-cli'; loading = false; });
          return;
        }
      }
      if (wailsApp?.ListWorkspaces) {
        const result = (await wailsApp.ListWorkspaces()) ?? [];
        flushSync(() => {
          workspaces = result;
          loading = false;
        });
        return;
      }
    } catch (e) {
      console.error('[activate] loadWorkspaces failed:', e);
    }
    flushSync(() => { loading = false; });
  }

  async function selectWorkspace(path: string) {
    flushSync(() => {
      loadingName = path.split('/').pop() || path;
      loadingError = '';
      view = 'loading';
    });
    try {
      if (wailsApp?.InitWorkspace) {
        await wailsApp.InitWorkspace(path);
      }
      const state = await api.getState();
      flushSync(() => {
        appState = state;
        view = 'workspace';
      });
      nav.reset();
      wailsApp?.SetWorkspaceMenuVisible(true);
    } catch (e: any) {
      flushSync(() => {
        loadingError = e?.message || String(e);
      });
    }
  }

  async function browseWorkspace() {
    if (!wailsApp?.SelectWorkspace) return;
    try {
      const state = await wailsApp.SelectWorkspace();
      if (!state?.projectDir) return; // cancelled
      flushSync(() => {
        loadingName = state.projectDir.split('/').pop() || state.projectDir;
        loadingError = '';
        view = 'loading';
      });
      const appData = await api.getState();
      flushSync(() => {
        appState = appData;
        view = 'workspace';
      });
      nav.reset();
      wailsApp?.SetWorkspaceMenuVisible(true);
      loadWorkspaces();
    } catch (e: any) {
      flushSync(() => {
        loadingError = e?.message || String(e);
        if (view !== 'loading') view = 'loading';
      });
    }
  }

  function backToWelcome() {
    flushSync(() => {
      view = 'welcome';
      appState = null;
    });
    nav.reset();
    wailsApp?.CloseWorkspace();
    wailsApp?.SetWorkspaceMenuVisible(false);
    loadWorkspaces();
  }

  api.onStateChanged(async () => {
    if (view === 'workspace') {
      const newState = await api.getState();
      flushSync(() => {
        appState = newState;
        stateVersion++;
      });
    }
  });

  loadWorkspaces();
</script>

<div class="bg-activate-bg text-activate-fg h-screen w-screen overflow-hidden font-sans text-sm flex flex-col">
  {#if view === 'loading'}
    <!-- No header during loading -->
  {:else if nav.page === 'settings' && view !== 'workspace'}
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
  {#if view === 'loading'}
    <div class="flex flex-col items-center justify-center h-full gap-4 animate-in">
      {#if loadingError}
        <div class="text-2xl">⚠️</div>
        <div class="text-center max-w-xs">
          <p class="text-sm font-medium mb-1">Failed to open workspace</p>
          <p class="text-xs opacity-50 break-words">{loadingError}</p>
        </div>
        <button class="btn btn-primary text-xs" onclick={() => { loadingError = ''; view = 'welcome'; }}>
          ← Back
        </button>
      {:else}
        <div class="loading-spinner"></div>
        <div class="text-center">
          <p class="text-sm font-medium opacity-80">Opening workspace</p>
          <p class="text-xs opacity-40 mt-1">{loadingName}</p>
        </div>
      {/if}
    </div>
  {:else if nav.page === 'settings' && view !== 'workspace'}
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
  {:else if view === 'no-cli'}
    <NoCliPage onInstallCLI={async () => {
      try {
        await wailsApp?.InstallCLI?.();
        // CLI installed — reload workspaces
        flushSync(() => { view = 'welcome'; loading = true; });
        loadWorkspaces();
      } catch {
        (window as any).runtime?.BrowserOpenURL?.('https://github.com/peregrine-digital/activate-framework#installation');
      }
    }} />
  {:else if view === 'welcome'}
    <WelcomePage
      {workspaces}
      onSelect={selectWorkspace}
      onBrowse={browseWorkspace}
    />
  {:else if !appState}
    <div class="py-8 text-center opacity-50">Loading workspace…</div>
  {:else}
    {#key stateVersion}
    <WorkspaceView
      {appState}
      {api}
      page={nav.page}
      onNavigate={nav.navigateTo}
      onBack={nav.navigateBack}
    />
    {/key}
  {/if}
  </main>
</div>

<style>
  .loading-spinner {
    width: 32px;
    height: 32px;
    border-radius: 50%;
    border: 2.5px solid rgba(63, 63, 70, 0.4);
    border-top-color: var(--color-activate-btn-primary-bg);
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
