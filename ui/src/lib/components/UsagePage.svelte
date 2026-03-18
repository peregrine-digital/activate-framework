<script lang="ts">
  import type { ActivateAPI } from '../api.js';
  import type { TelemetryEntry } from '../types.js';

  interface Props {
    api: ActivateAPI;
    telemetryEnabled: boolean;
    onBack: () => void;
  }

  let { api, telemetryEnabled, onBack }: Props = $props();

  let entries = $state<TelemetryEntry[]>([]);
  let loading = $state(true);

  async function load() {
    loading = true;
    entries = await api.readTelemetryLog();
    loading = false;
  }

  async function refresh() {
    await api.runTelemetry();
    await load();
  }

  // Load on mount
  $effect(() => { load(); });

  // Deduplicate by date, most recent first
  let daily = $derived.by(() => {
    const byDate = new Map<string, TelemetryEntry>();
    for (const e of [...entries].sort((a, b) => (a.timestamp || '').localeCompare(b.timestamp || ''))) {
      byDate.set(e.date, e);
    }
    return [...byDate.values()].sort((a, b) => (b.date || '').localeCompare(a.date || ''));
  });

  let latest = $derived(daily[0]);
  let pctUsed = $derived(
    latest?.premium_entitlement && latest?.premium_used != null
      ? Math.round((latest.premium_used / latest.premium_entitlement) * 100)
      : null
  );

  let usageColor = $derived(
    pctUsed == null ? 'inherit'
      : pctUsed >= 90 ? 'var(--color-activate-error)'
      : pctUsed >= 70 ? 'var(--color-activate-warning)'
      : 'var(--color-activate-success)'
  );

  let sparkData = $derived(daily.slice(0, 30).reverse());
  let maxUsed = $derived(Math.max(...sparkData.map((e) => e.premium_used ?? 0), 1));
</script>

<div class="flex items-center gap-2 py-2 pb-1">
  <h2 class="text-sm font-semibold flex-1">📊 Copilot Usage</h2>
</div>

<div class="flex gap-1.5 pb-2.5 flex-wrap">
  <button class="btn btn-secondary" onclick={onBack}>← Back</button>
  <button class="btn btn-primary" onclick={refresh} disabled={!telemetryEnabled}>↻ Refresh</button>
  <button class="btn btn-secondary" onclick={() => api.runTelemetry()}>📄 Open Log</button>
  <button class="btn btn-secondary" onclick={() => api.setConfig({ telemetryEnabled: !telemetryEnabled, scope: 'global' })}>
    {telemetryEnabled ? '⏸ Disable' : '▶ Enable'} Telemetry
  </button>
</div>

{#if !telemetryEnabled}
  <div class="opacity-50 italic py-4 text-xs text-center">Telemetry is disabled. Click Enable to start tracking Copilot usage.</div>
{/if}

<hr class="border-none border-t border-activate-border my-0.5 mb-2" />

{#if loading}
  <div class="opacity-50 italic py-4 text-xs text-center">Loading…</div>
{:else if latest}
  <!-- Summary cards -->
  <div class="flex gap-2.5">
    <div class="flex-1 min-w-0 bg-activate-bg-surface border border-activate-border rounded-md p-3 mb-3">
      <div class="text-[11px] uppercase tracking-wider opacity-60 mb-1.5 font-semibold">Used Today</div>
      <div class="text-[28px] font-bold leading-tight" style:color={usageColor}>{latest.premium_used ?? '—'}</div>
      <div class="text-xs opacity-70 mt-1">of {latest.premium_entitlement ?? '?'} premium requests</div>
      {#if pctUsed != null}
        <div class="bg-activate-border rounded h-2 mt-2 overflow-hidden opacity-60">
          <div class="h-full rounded transition-[width] duration-300" style:width="{Math.min(pctUsed, 100)}%" style:background={usageColor}></div>
        </div>
      {/if}
    </div>
    <div class="flex-1 min-w-0 bg-activate-bg-surface border border-activate-border rounded-md p-3 mb-3">
      <div class="text-[11px] uppercase tracking-wider opacity-60 mb-1.5 font-semibold">Remaining</div>
      <div class="text-[28px] font-bold leading-tight">{latest.premium_remaining ?? '—'}</div>
      {#if latest.quota_reset_date_utc}
        <div class="text-xs opacity-70 mt-1">Resets {latest.quota_reset_date_utc.split('T')[0]}</div>
      {/if}
    </div>
  </div>

  <!-- Sparkline -->
  {#if sparkData.length > 1}
    <div class="text-[11px] uppercase tracking-wider opacity-60 mt-2.5 mb-1 font-semibold">Last {sparkData.length} days</div>
    <div class="flex items-end gap-0.5 h-10 my-2 mb-3">
      {#each sparkData as entry}
        {@const h = entry.premium_used != null ? Math.max(2, Math.round((entry.premium_used / maxUsed) * 40)) : 2}
        <div
          class="flex-1 min-w-[3px] max-w-3 bg-activate-btn-primary-bg rounded-t opacity-70 hover:opacity-100 relative group/bar"
          style:height="{h}px"
          title="{entry.date}: {entry.premium_used ?? 0} used"
        ></div>
      {/each}
    </div>
  {/if}

  <!-- Daily table -->
  {#if daily.length > 0}
    <div class="text-[11px] uppercase tracking-wider opacity-60 mt-2.5 mb-1 font-semibold">Daily Log · {daily.length} entries</div>
    <table class="w-full border-collapse text-xs">
      <thead>
        <tr>
          <th class="py-1 px-1.5 text-left font-semibold text-[11px] opacity-70 uppercase tracking-wider">Date</th>
          <th class="py-1 px-1.5 text-right font-semibold text-[11px] opacity-70 uppercase tracking-wider">Used</th>
          <th class="py-1 px-1.5 text-right font-semibold text-[11px] opacity-70 uppercase tracking-wider">Left</th>
          <th class="py-1 px-1.5 text-right font-semibold text-[11px] opacity-70 uppercase tracking-wider">Quota</th>
          <th class="py-1 px-1.5 text-right font-semibold text-[11px] opacity-70 uppercase tracking-wider">%</th>
        </tr>
      </thead>
      <tbody>
        {#each daily as entry}
          {@const pct = entry.premium_entitlement && entry.premium_used != null
            ? Math.round((entry.premium_used / entry.premium_entitlement) * 100)
            : null}
          <tr class="hover:bg-activate-bg-hover border-b border-activate-border">
            <td class="py-1 px-1.5">{entry.date || '—'}</td>
            <td class="py-1 px-1.5 text-right tabular-nums">{entry.premium_used ?? '—'}</td>
            <td class="py-1 px-1.5 text-right tabular-nums">{entry.premium_remaining ?? '—'}</td>
            <td class="py-1 px-1.5 text-right tabular-nums">{entry.premium_entitlement ?? '—'}</td>
            <td class="py-1 px-1.5 text-right tabular-nums">{pct != null ? `${pct}%` : '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
{:else}
  <div class="opacity-50 italic py-4 text-xs text-center">No telemetry data yet. Click Refresh to log now.</div>
{/if}

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
  .btn-primary:hover { background: var(--color-activate-btn-primary-hover); }
  .btn-secondary {
    background: var(--color-activate-btn-secondary-bg);
    color: var(--color-activate-btn-secondary-fg);
  }
  .btn-secondary:hover { background: var(--color-activate-btn-secondary-hover); }
</style>
