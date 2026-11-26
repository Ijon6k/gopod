<script lang="ts">
	import type { TimeSeriesPoint } from '$lib/types';
	import { AreaChart } from '$lib/components/ui';
	import { formatMemory, formatRateBps } from '$lib/utils/format';
	import { server } from '$lib/data';

	interface Props {
		cpuHistory: TimeSeriesPoint[];
		currentCpu: number;
		memoryHistory: TimeSeriesPoint[];
		currentMem: number;
		memLimit: number;
		networkHistory: TimeSeriesPoint[];
		currentNetRx: string;
		currentNetTx: string;
		storageHistory: TimeSeriesPoint[];
		storageUsed: number;
		storageTotal: number;
	}

	let {
		cpuHistory,
		currentCpu,
		memoryHistory,
		currentMem,
		memLimit,
		networkHistory,
		currentNetRx,
		currentNetTx,
		storageHistory,
		storageUsed,
		storageTotal
	}: Props = $props();

	let hostMemMB = $derived(Math.max(Math.round((server.memory || 16) * 1024), currentMem, 512));
	let chartMemMax = $derived(memLimit > 0 ? memLimit : hostMemMB);
	let netMax = $derived(Math.max(1024, ...networkHistory.map((p) => p.value)) * 1.25);
</script>

<div class="grid w-full grid-cols-1 gap-4 lg:grid-cols-2">
	<!-- 1. CPU Chart (2-Column Grid, 0-100% Normalized Scale, 2x Height) -->
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
	>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">CPU Utilization</span>
				<span
					class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent)]"
				>
					0% – 100% Scale
				</span>
			</div>
			<div
				class="flex flex-wrap items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]"
			>
				<span
					>Current: <strong class="text-[var(--text-primary)]">{currentCpu.toFixed(1)}%</strong
					></span
				>
				<span>•</span>
				<span
					>Max (session): <strong class="text-[var(--text-secondary)]"
						>{Math.max(...cpuHistory.map((p) => p.value), currentCpu).toFixed(1)}%</strong
					></span
				>
			</div>
		</div>

		<div class="relative w-full">
			<AreaChart data={cpuHistory} height={350} maxValue={100} showGrid={true} />
			<div
				class="pointer-events-none absolute top-1 right-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				100%
			</div>
			<div
				class="pointer-events-none absolute top-1/2 right-1 -translate-y-1/2 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				50%
			</div>
			<div
				class="pointer-events-none absolute right-1 bottom-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				0%
			</div>
		</div>
	</div>

	<!-- 2. Memory Chart (2-Column Grid, 0 to memLimit MB Normalized Scale, 2x Height) -->
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
	>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">Memory Allocation</span>
				<span
					class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--status-green)]"
				>
					{memLimit > 0
						? `0 – ${formatMemory(memLimit)} Scale`
						: `0 – ${formatMemory(chartMemMax)} (Host)`}
				</span>
			</div>
			<div
				class="flex flex-wrap items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]"
			>
				<span
					>Current: <strong class="text-[var(--text-primary)]">{formatMemory(currentMem)}</strong
					></span
				>
				<span>•</span>
				<span
					>Max (session): <strong class="text-[var(--text-secondary)]"
						>{formatMemory(Math.max(...memoryHistory.map((p) => p.value), currentMem))}</strong
					></span
				>
			</div>
		</div>

		<div class="relative w-full">
			<AreaChart
				data={memoryHistory}
				height={350}
				maxValue={chartMemMax}
				showGrid={true}
				strokeColor="var(--status-green)"
				fillColor="var(--status-green)"
			/>
			<div
				class="pointer-events-none absolute top-1 right-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				{formatMemory(chartMemMax)} (100%)
			</div>
			<div
				class="pointer-events-none absolute top-1/2 right-1 -translate-y-1/2 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				{formatMemory(chartMemMax / 2)} (50%)
			</div>
			<div
				class="pointer-events-none absolute right-1 bottom-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				0 MB
			</div>
		</div>
	</div>

	<!-- 3. Network Chart (2-Column Grid, 2x Height) -->
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
	>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]"
					>Network Throughput (B/s)</span
				>
			</div>
			<div
				class="flex flex-wrap items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]"
			>
				<span>Rx: <strong class="text-[var(--text-secondary)]">{currentNetRx}</strong></span>
				<span>•</span>
				<span>Tx: <strong class="text-[var(--text-secondary)]">{currentNetTx}</strong></span>
			</div>
		</div>
		<div class="relative w-full">
			<AreaChart
				data={networkHistory}
				height={340}
				maxValue={netMax}
				showGrid={true}
				strokeColor="var(--status-blue)"
				fillColor="var(--status-blue)"
			/>
			<div
				class="pointer-events-none absolute top-1 right-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				{formatRateBps(netMax)}
			</div>
			<div
				class="pointer-events-none absolute right-1 bottom-1 font-mono text-[9px] text-[var(--text-tertiary)] select-none"
			>
				0 B/s
			</div>
		</div>
	</div>

	<!-- 4. Storage / Host I/O Chart (2-Column Grid, 2x Height) -->
	<div
		class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
	>
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">System Host Storage</span>
			</div>
			<div
				class="flex flex-wrap items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]"
			>
				<span>Usage: <strong class="text-[var(--text-secondary)]">{storageUsed} GB</strong></span>
				<span>•</span>
				<span>Total: <strong class="text-[var(--text-secondary)]">{storageTotal} GB</strong></span>
			</div>
		</div>
		<AreaChart
			data={storageHistory}
			height={340}
			maxValue={storageTotal}
			showGrid={true}
			strokeColor="var(--status-amber)"
			fillColor="var(--status-amber)"
		/>
	</div>
</div>
