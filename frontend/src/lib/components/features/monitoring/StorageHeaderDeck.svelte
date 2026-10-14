<script lang="ts">
	import { Button } from '$lib/components/ui';
	import { Broom, ChartPieSlice, ChartBar, ArrowClockwise, CircleNotch } from 'phosphor-svelte';

	interface Props {
		totalPodmanDisplay: string;
		totalReclaimableDisplay: string;
		totalRawReclaimable: number;
		chartMode: 'donut' | 'bar';
		isLoadingDf: boolean;
		pruningTarget: string | null;
		ontoggleChartMode: (mode: 'donut' | 'bar') => void;
		onrefresh: () => void;
		onpruneAll: () => void;
		onopenModal: () => void;
	}

	let {
		totalPodmanDisplay,
		totalReclaimableDisplay,
		totalRawReclaimable,
		chartMode,
		isLoadingDf,
		pruningTarget,
		ontoggleChartMode,
		onrefresh,
		onpruneAll,
		onopenModal
	}: Props = $props();
</script>

<div
	class="flex flex-col justify-between gap-4 border-b border-[var(--border)] p-5 sm:flex-row sm:items-center"
>
	<div class="flex flex-col gap-1">
		<div class="flex flex-wrap items-center gap-2.5">
			<h3 class="text-sm font-semibold text-[var(--text-primary)]">Storage allocation</h3>
			<span
				class="rounded border border-[var(--status-green)] bg-[var(--status-green-muted)] px-2 py-0.5 font-mono text-[11px] font-medium text-[var(--status-green)]"
			>
				{totalReclaimableDisplay} reclaimable
			</span>
		</div>
		<p class="text-xs text-[var(--text-tertiary)]">
			<strong class="font-mono text-[var(--text-secondary)]">{totalPodmanDisplay}</strong>
			allocated in
			<code class="font-mono text-[11px]">~/.local/share/containers/storage</code> (driver: overlay).
		</p>
	</div>

	<div class="flex flex-wrap items-center gap-2">
		<!-- Donut / Bar Visualization Switcher -->
		<div
			class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-0.5"
		>
			<button
				type="button"
				onclick={() => ontoggleChartMode('donut')}
				class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 text-xs transition-colors {chartMode ===
				'donut'
					? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				title="Donut distribution view"
			>
				<ChartPieSlice size={13} />
				<span>Donut</span>
			</button>
			<button
				type="button"
				onclick={() => ontoggleChartMode('bar')}
				class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 text-xs transition-colors {chartMode ===
				'bar'
					? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				title="Comparative horizontal bar view"
			>
				<ChartBar size={13} />
				<span>Bar</span>
			</button>
		</div>

		<!-- Refresh Button -->
		<Button
			variant="secondary"
			size="sm"
			onclick={onrefresh}
			disabled={isLoadingDf}
			aria-label="Refresh disk usage"
		>
			<ArrowClockwise size={13} class={isLoadingDf ? 'animate-spin' : ''} />
		</Button>

		<!-- Direct 1-Click Prune All -->
		<Button
			variant="primary"
			size="sm"
			onclick={onpruneAll}
			disabled={pruningTarget !== null || totalRawReclaimable === 0}
			aria-label="Prune all unused storage"
		>
			{#if pruningTarget === 'all'}
				<CircleNotch size={14} class="animate-spin" />
				<span>Pruning...</span>
			{:else}
				<Broom size={14} />
				<span>Prune all unused ({totalReclaimableDisplay})</span>
			{/if}
		</Button>

		<!-- Granular Modal Trigger -->
		<Button
			variant="secondary"
			size="sm"
			onclick={onopenModal}
			aria-label="Select specific categories to prune"
		>
			<span>Customize...</span>
		</Button>
	</div>
</div>
