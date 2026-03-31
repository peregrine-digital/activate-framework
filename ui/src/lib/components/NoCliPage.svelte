<script lang="ts">
  interface Props {
    onInstallCLI: () => void;
  }

  let { onInstallCLI }: Props = $props();
</script>

<div class="text-center py-12 px-4 animate-in">
  <div class="text-5xl mb-5 opacity-80">⚠️</div>
  <h2 class="text-base font-semibold mb-2">Activate CLI not found</h2>
  <p class="text-activate-fg-muted text-[13px] mb-6 max-w-[260px] mx-auto">
    Install the CLI to manage your workspace files and configurations.
  </p>
  <button class="btn btn-primary !px-6 !py-2.5 !text-sm" onclick={onInstallCLI}>Install Activate CLI</button>
  <p class="text-activate-fg-muted text-[11px] mt-5">
    Or run: <code class="glass !rounded px-2 py-0.5 text-xs font-mono">curl -fsSL https://... | bash</code>
  </p>
</div>
