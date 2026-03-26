<script lang="ts">
  import './app.css';
  import './vscode-theme.css';
  import { createVSCodeAPI } from './lib/adapters/vscode.js';
  import { createNavigation } from './lib/navigation.svelte.js';
  import type { AppState } from './lib/types.js';
  import WorkspaceView from './lib/components/WorkspaceView.svelte';

  const api = createVSCodeAPI();
  const nav = createNavigation();

  let appState = $state<AppState | null>(null);
  let loading = $state(true);
  let hasCli = $state(true);

  async function load() {
    try {
      appState = await api.getState();
      loading = false;
      hasCli = true;
    } catch {
      hasCli = false;
      loading = false;
      nav.page = 'no-cli';
    }
  }

  api.onStateChanged(() => load());

  // Listen for init message from extension host
  if (typeof window !== 'undefined') {
    window.addEventListener('message', (event) => {
      const msg = event.data;
      if (msg?.type === 'init') {
        hasCli = msg.hasCli !== false;
        if (!hasCli) {
          nav.page = 'no-cli';
          loading = false;
        } else {
          load();
        }
      }
    });
  }

  load();
</script>

<div class="bg-activate-bg text-activate-fg min-h-screen min-w-[280px] font-sans text-sm px-2.5">
  {#if loading || (!appState && hasCli)}
    <div class="py-8 text-center opacity-50">Loading…</div>
  {:else if appState}
    <WorkspaceView
      {appState}
      {api}
      page={nav.page}
      onNavigate={nav.navigateTo}
      onBack={nav.navigateBack}
    />
  {/if}
</div>
