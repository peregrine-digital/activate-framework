<script lang="ts">
  interface Props {
    isActive: boolean;
    manifestCount: number;
    onChangeTier: () => void;
    onChangeManifest: () => void;
    onToggleWorkspace: () => void;
    onUpdateAll: () => void;
    onShowUsage: () => void;
  }

  let { isActive, manifestCount, onChangeTier, onChangeManifest, onToggleWorkspace, onUpdateAll, onShowUsage }: Props = $props();
</script>

<div class="flex gap-2.5 pb-4 flex-wrap animate-in" style="animation-delay: 50ms">
  <button class="btn btn-secondary" onclick={onChangeTier}>◆ Tier</button>
  {#if manifestCount > 1}
    <button class="btn btn-secondary" onclick={onChangeManifest}>⇋ Manifest</button>
  {/if}
  <button class="btn btn-secondary" onclick={onToggleWorkspace}>
    {isActive ? '− Remove' : '+ Install'}
  </button>
  <button class="btn btn-primary" onclick={onUpdateAll}>↻ Update</button>
  <button class="btn btn-secondary" onclick={onShowUsage}>📊 Usage</button>
</div>
