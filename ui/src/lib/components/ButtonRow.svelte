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

<div class="flex gap-1.5 pb-2.5 flex-wrap">
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

<style>
  .btn {
    border: 1px solid transparent;
    border-radius: 3px;
    cursor: pointer;
    font-family: inherit;
    font-size: 12px;
    line-height: 20px;
    padding: 4px 10px;
    white-space: nowrap;
  }
  .btn-primary {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
  }
  .btn-primary:hover {
    background: var(--color-activate-btn-primary-hover);
  }
  .btn-secondary {
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
  }
  .btn-secondary:hover {
    background: var(--color-activate-btn-secondary-hover);
  }
</style>
