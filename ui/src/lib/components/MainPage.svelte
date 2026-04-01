<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { AppState, FileStatus, Category } from '../types.js';
  import StatusBar from './StatusBar.svelte';
  import ButtonRow from './ButtonRow.svelte';
  import CategoryList from './CategoryList.svelte';
  import SelectModal from './SelectModal.svelte';
  import type { SelectOption } from './SelectModal.svelte';

  interface Props {
    appState: AppState;
    api: ActivateAPI;
    onNavigate: (page: 'usage' | 'settings' | 'workspace-settings') => void;
  }

  let { appState, api, onNavigate }: Props = $props();

  let config = $derived(appState.config);
  let tiers = $derived(appState.tiers);
  let files = $derived(appState.files);
  let categories = $derived(appState.categories);
  let isActive = $derived(appState.state.hasInstallMarker);
  let hasPresets = $derived((appState.presets?.length ?? 0) > 0);
  let tierLabel = $derived(tiers.find((t) => t.id === config.tier)?.label || config.tier || '—');
  let presetLabel = $derived(appState.presets?.find((p) => p.id === config.preset)?.name || config.preset || '—');
  let skippedVersions = $derived(config.skippedVersions || {});

  // Use inPreset for filtering when presets are active, fall back to inTier
  function isIncluded(f: FileStatus): boolean {
    return hasPresets ? (f.inPreset ?? f.inTier) : f.inTier;
  }

  let installedFiles = $derived(files.filter((f) => f.installed && isIncluded(f) && f.override !== 'excluded'));
  let availableFiles = $derived(files.filter((f) => !f.installed && isIncluded(f) && f.override !== 'excluded'));
  let outsideFiles = $derived(files.filter((f) => !isIncluded(f) && f.override !== 'excluded'));
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

  function handleChangePreset() {
    if (api.platform === 'vscode') {
      api.changePreset();
      return;
    }
    const presets = appState.presets ?? [];
    const options: SelectOption[] = presets.map((p) => ({
      id: p.id,
      label: p.name,
      description: p.description,
      active: p.id === config.preset,
    }));
    selectModal = {
      title: 'Select Preset',
      options,
      onSelect: async (id) => {
        selectModal = null;
        await api.setConfig({ scope: 'project', preset: id });
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
  manifestCount={appState.manifests.length}
  platform={api.platform}
  {hasPresets}
  {presetLabel}
  onShowSettings={() => onNavigate('workspace-settings')}
/>

<ButtonRow
  {isActive}
  manifestCount={appState.manifests.length}
  platform={api.platform}
  {hasPresets}
  onChangeTier={handleChangeTier}
  onChangeManifest={handleChangeManifest}
  onChangePreset={handleChangePreset}
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
  <div class="text-activate-fg-muted italic py-3 pl-8 text-xs">{hasPresets ? 'All preset files installed' : 'All tier files installed'}</div>
{/if}

{#if outsideFiles.length > 0}
  <div class="section-label">
    {hasPresets ? 'Outside Preset' : 'Outside Tier'} · {outsideFiles.length}
  </div>
  <p class="text-[10px] text-activate-fg-muted italic pb-1 pl-3 opacity-60">{hasPresets ? 'Switch to a higher preset to access these files' : 'Switch to a higher tier to access these files'}</p>
  <CategoryList
    files={outsideFiles}
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
