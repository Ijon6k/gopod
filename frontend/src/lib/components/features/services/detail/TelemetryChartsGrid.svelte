<script lang="ts">
	import type { TimeSeriesPoint } from '$lib/types';
	import { AreaChart } from '$lib/components/ui';
	import { formatMemory } from '$lib/utils/format';

	interface Props {
		timeRange: string;
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
		timeRange,
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
</script>

<div class="flex flex-col gap-4 w-full">
	<!-- 1. CPU Chart (Full Width, 0-100% Normalized Scale) -->
	<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">CPU Utilization</span>
				<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				<span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[var(--accent)] border border-[var(--border)]">
					0% – 100% Scale
				</span>
			</div>
			<div class="flex items-center gap-3 text-[11px] font-mono text-[var(--text-tertiary)]">
				<span>Current: <strong class="text-[var(--text-primary)]">{currentCpu.toFixed(1)}%</strong></span>
				<span>•</span>
				<span>Peak: <strong class="text-[var(--text-secondary)]">{Math.max(...cpuHistory.map((p) => p.value), currentCpu).toFixed(1)}%</strong></span>
				<span>•</span>
				<span>Limit: <strong class="text-[var(--text-tertiary)]">100.0%</strong></span>
			</div>
		</div>

		<div class="relative w-full">
			<AreaChart data={cpuHistory} height={180} maxValue={100} showGrid={true} />
			<div class="absolute right-1 top-1 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">100%</div>
			<div class="absolute right-1 top-1/2 -translate-y-1/2 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">50%</div>
			<div class="absolute right-1 bottom-1 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">0%</div>
		</div>
	</div>

	<!-- 2. Memory Chart (Full Width, 0 to memLimit MB Normalized Scale) -->
	<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">Memory Allocation</span>
				<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				<span class="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-surface)] text-[var(--status-green)] border border-[var(--border)]">
					0 MB – {memLimit} MB Scale
				</span>
			</div>
			<div class="flex items-center gap-3 text-[11px] font-mono text-[var(--text-tertiary)]">
				<span>Current: <strong class="text-[var(--text-primary)]">{formatMemory(currentMem)}</strong></span>
				<span>•</span>
				<span>Peak: <strong class="text-[var(--text-secondary)]">{formatMemory(Math.max(...memoryHistory.map((p) => p.value), currentMem))}</strong></span>
				<span>•</span>
				<span>Limit: <strong class="text-[var(--text-secondary)]">{memLimit} MB</strong></span>
			</div>
		</div>

		<div class="relative w-full">
			<AreaChart data={memoryHistory} height={180} maxValue={memLimit} showGrid={true} strokeColor="#4C9A72" fillColor="#4C9A72" />
			<div class="absolute right-1 top-1 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">{memLimit} MB (100%)</div>
			<div class="absolute right-1 top-1/2 -translate-y-1/2 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">{Math.round(memLimit / 2)} MB (50%)</div>
			<div class="absolute right-1 bottom-1 text-[9px] font-mono text-[var(--text-tertiary)] select-none pointer-events-none">0 MB</div>
		</div>
	</div>

	<!-- 3. Network Chart (Full Width) -->
	<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">Network Traffic</span>
				<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
			</div>
			<div class="flex items-center gap-3 text-[11px] font-mono text-[var(--text-tertiary)]">
				<span>Rx: <strong class="text-[var(--text-secondary)]">{currentNetRx}</strong></span>
				<span>•</span>
				<span>Tx: <strong class="text-[var(--text-secondary)]">{currentNetTx}</strong></span>
			</div>
		</div>
		<AreaChart data={networkHistory} height={170} strokeColor="#7680B5" fillColor="#7680B5" />
	</div>

	<!-- 4. Storage / Host I/O Chart (Full Width) -->
	<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-[var(--text-primary)]">System Host Storage</span>
				<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
			</div>
			<div class="flex items-center gap-3 text-[11px] font-mono text-[var(--text-tertiary)]">
				<span>Usage: <strong class="text-[var(--text-secondary)]">{storageUsed} GB</strong></span>
				<span>•</span>
				<span>Total: <strong class="text-[var(--text-secondary)]">{storageTotal} GB</strong></span>
			</div>
		</div>
		<AreaChart data={storageHistory} height={170} maxValue={storageTotal} showGrid={true} strokeColor="#B8893B" fillColor="#B8893B" />
	</div>
</div>
