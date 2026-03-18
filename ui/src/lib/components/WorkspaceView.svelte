<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, Page } from '../types.js';
  import MainPage from './MainPage.svelte';
  import UsagePage from './UsagePage.svelte';
  import SettingsPage from './SettingsPage.svelte';
  import GlobalSettingsPage from './GlobalSettingsPage.svelte';
  import NoCliPage from './NoCliPage.svelte';

  interface Props {
    appState: AppState;
    api: ActivateAPI;
    page: Page;
    serverVersion?: string;
    onNavigate: (page: Page) => void;
    onBack: () => void;
  }

  let { appState, api, page, serverVersion = '', onNavigate, onBack }: Props = $props();
</script>

{#if page === 'no-cli'}
  <NoCliPage onInstallCLI={() => api.installCLI()} />
{:else if page === 'settings'}
  <GlobalSettingsPage {api} {serverVersion} onBack={onBack} />
{:else if page === 'usage'}
  <UsagePage {api} telemetryEnabled={appState.config.telemetryEnabled === true} onBack={onBack} />
{:else if page === 'workspace-settings'}
  <SettingsPage {appState} {api} onBack={onBack} />
{:else}
  <MainPage state={appState} {api} onNavigate={onNavigate} />
{/if}
