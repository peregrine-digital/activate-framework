<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, FileStatus, Category } from '../types.js';
  import StatusBar from './StatusBar.svelte';
  import ButtonRow from './ButtonRow.svelte';
  import CategoryList from './CategoryList.svelte';
  import SelectModal from './SelectModal.svelte';
  import type { SelectOption } from './SelectModal.svelte';

  interface Props {
    state: AppState;
    api: ActivateAPI;
    onNavigate: (page: 'usage' | 'settings' | 'workspace-settings') => void;
  }

  let { state, api, onNavigate }: Props = $props();

  let config = $derived(state.config);
  let tiers = $derived(state.tiers);
  let files = $derived(state.files);
  let categories = $derived(state.categories);
  let isActive = $derived(state.state.hasInstallMarker);
  let tierLabel = $derived(tiers.find((t) => t.id === config.tier)?.label || config.tier || '—');
  let skippedVersions = $derived(config.skippedVersions || {});

  let installedFiles = $derived(files.filter((f) => f.installed && f.override !== 'excluded'));
  let availableFiles = $derived(files.filter((f) => !f.installed && f.inTier && f.override !== 'excluded'));
  let outsideTierFiles = $derived(files.filter((f) => !f.installed && !f.inTier && f.override !== 'excluded'));
  let excludedFiles = $derived(files.filter((f) => f.override === 'excluded'));

  // Select modal state
  let selectModal = $state<{ title: string; options: SelectOption[]; onSelect: (id: string) => void } | null>(null);

  function handleInstall(file: FileStatus) { api.installFile(file); }
  function handleUninstall(file: FileStatus) { api.uninstallFile(file); }
  function handleDiff(file: FileStatus) { api.diffFile(file); }
  function handleSkipUpdate(file: FileStatus) { api.skipUpdate(file); }
  function handleOpen(file: FileStatus) { api.openFile(file); }
  function handleSetOverride(dest: string, override: '' | 'pinned' | 'excluded') { api.setFileOverride(dest, override); }

  function handleChangeTier() {
    if (api.platform === 'vscode') {
      api.changeTier();
      return;
    }
    const options: SelectOption[] = tiers.map((t) => ({
      id: t.id,
      label: t.label,
      description: t.description,
      active: t.id === config.tier,
    }));
    selectModal = {
      title: 'Select Tier',
      options,
      onSelect: async (id) => {
        selectModal = null;
        await api.setConfig({ scope: 'project', tier: id });
      },
    };
  }

  async function handleChangeManifest() {
    if (api.platform === 'vscode') {
      api.changeManifest();
      return;
    }
    const manifests = await api.listManifests();
    const options: SelectOption[] = manifests.map((m) => ({
      id: m.id,
      label: m.name,
      description: m.description,
      active: m.id === config.manifest,
    }));
    selectModal = {
      title: 'Select Manifest',
      options,
      onSelect: async (id) => {
        selectModal = null;
        await api.setConfig({ scope: 'project', manifest: id });
      },
    };
  }
</script>

{#if selectModal}
  <SelectModal
    title={selectModal.title}
    options={selectModal.options}
    onSelect={selectModal.onSelect}
    onClose={() => (selectModal = null)}
  />
{/if}

<StatusBar
  tier={config.tier}
  {tierLabel}
  manifestName={config.manifest}
  {isActive}
  manifestCount={state.manifests.length}
  platform={api.platform}
  onShowSettings={() => onNavigate('workspace-settings')}
/>

<ButtonRow
  {isActive}
  manifestCount={state.manifests.length}
  platform={api.platform}
  onChangeTier={handleChangeTier}
  onChangeManifest={handleChangeManifest}
  onToggleWorkspace={() => isActive ? api.removeFromWorkspace() : api.addToWorkspace()}
  onUpdateAll={() => api.updateAll()}
  onShowUsage={() => onNavigate('usage')}
/>

<hr class="divider" />

<div class="section-label">
  Installed · {installedFiles.length}
</div>
{#if installedFiles.length > 0}
  <CategoryList
    files={installedFiles}
    {categories}
    installed={true}
    sectionPrefix="installed"
    {skippedVersions}
    onInstall={handleInstall}
    onUninstall={handleUninstall}
    onDiff={handleDiff}
    onSkipUpdate={handleSkipUpdate}
    onOpen={handleOpen}
    onSetOverride={handleSetOverride}
  />
{:else}
  <div class="text-activate-fg-muted italic py-3 pl-8 text-xs">No files installed</div>
{/if}

<div class="section-label">
  Available · {availableFiles.length}
</div>
{#if availableFiles.length > 0}
  <CategoryList
    files={availableFiles}
    {categories}
    installed={false}
    sectionPrefix="available"
    {skippedVersions}
    onInstall={handleInstall}
    onUninstall={handleUninstall}
    onDiff={handleDiff}
    onSkipUpdate={handleSkipUpdate}
    onOpen={handleOpen}
    onSetOverride={handleSetOverride}
  />
{:else}
  <div class="text-activate-fg-muted italic py-3 pl-8 text-xs">All tier files installed</div>
{/if}

{#if outsideTierFiles.length > 0}
  <div class="section-label">
    Outside Tier · {outsideTierFiles.length}
  </div>
  <p class="text-[10px] text-activate-fg-muted italic pb-1 pl-3 opacity-60">Switch to a higher tier to access these files</p>
  <CategoryList
    files={outsideTierFiles}
    {categories}
    installed={false}
    sectionPrefix="outside"
    {skippedVersions}
    onInstall={handleInstall}
    onUninstall={handleUninstall}
    onDiff={handleDiff}
    onSkipUpdate={handleSkipUpdate}
    onOpen={handleOpen}
    onSetOverride={handleSetOverride}
  />
{/if}

{#if excludedFiles.length > 0}
  <div class="section-label">
    Excluded · {excludedFiles.length}
  </div>
  <p class="text-[10px] text-activate-fg-muted italic pb-1 pl-3 opacity-60">These files are excluded and will not be installed</p>
  <CategoryList
    files={excludedFiles}
    {categories}
    installed={false}
    sectionPrefix="excluded"
    {skippedVersions}
    onInstall={handleInstall}
    onUninstall={handleUninstall}
    onDiff={handleDiff}
    onSkipUpdate={handleSkipUpdate}
    onOpen={handleOpen}
    onSetOverride={handleSetOverride}
  />
{/if}
