<script lang="ts">
  interface Props {
    onInstallCLI: () => void;
  }

  let { onInstallCLI }: Props = $props();
</script>

<div class="text-center py-8 px-4">
  <div class="text-4xl mb-4">⚠️</div>
  <h2 class="text-sm font-semibold mb-1">Activate CLI not found</h2>
  <p class="text-activate-fg-muted text-[13px] mb-5">
    Install it to manage your workspace files.
  </p>
  <button class="btn btn-primary" onclick={onInstallCLI}>Install Activate CLI</button>
  <p class="text-activate-fg-muted text-[11px] mt-4">
    Or run: <code class="bg-activate-bg-surface px-1.5 py-0.5 rounded text-xs">curl -fsSL https://... | bash</code>
  </p>
</div>

<style>
  .btn {
    border: 1px solid transparent;
    border-radius: 3px;
    cursor: pointer;
    font-family: inherit;
    font-size: 12px;
    line-height: 20px;
    padding: 8px 16px;
    white-space: nowrap;
  }
  .btn-primary {
    background: var(--color-activate-btn-primary-bg);
    color: var(--color-activate-btn-primary-fg);
  }
  .btn-primary:hover { background: var(--color-activate-btn-primary-hover); }
</style>
