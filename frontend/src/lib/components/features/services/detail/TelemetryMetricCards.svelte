<script lang="ts">
	import { formatMemory } from '$lib/utils/format';
	import { Cpu, Gauge, ArrowsDownUp } from 'phosphor-svelte';

	interface Props {
		currentCpu: number;
		currentMem: number;
		memLimit: number;
		memPercent: number;
		currentNetRx: string;
		currentNetTx: string;
		hasContainers: boolean;
	}

	let {
		currentCpu,
		currentMem,
		memLimit,
		memPercent,
		currentNetRx,
		currentNetTx,
		hasContainers
	}: Props = $props();
</script>

<div class="grid grid-cols-1 md:grid-cols-3 gap-4">
	<!-- 1. CPU Card -->
	<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
		<div class="flex items-center justify-between">
			<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">CPU Utilization</span>
			<div class="p-1 rounded bg-[var(--accent-muted)] text-[var(--accent)]">
				<Cpu size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong class="text-2xl font-bold text-[var(--text-primary)] font-[var(--font-mono)] tracking-tight tabular-nums">
				{currentCpu.toFixed(1)}%
			</strong>
			<span class="text-xs font-mono text-[var(--text-secondary)]">cgroup v2</span>
		</div>

		<!-- Capacity Bar -->
		<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
			<div
				class="h-full rounded-full transition-all duration-500 {currentCpu > 5 ? 'bg-[var(--status-red)]' : currentCpu > 2 ? 'bg-[var(--status-amber)]' : 'bg-[var(--accent)]'}"
				style="width: {Math.min(currentCpu, 100)}%;"
			></div>
		</div>

		<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
			<span>Current: {currentCpu.toFixed(1)}%</span>
			<span class={currentCpu > 5 ? 'text-[var(--status-amber)] font-medium' : 'text-[var(--status-green)] font-medium'}>
				{currentCpu > 5 ? 'Elevated' : 'Optimal'}
			</span>
		</div>
	</div>

	<!-- 2. Memory Card -->
	<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
		<div class="flex items-center justify-between">
			<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Memory Allocation</span>
			<div class="p-1 rounded bg-[var(--status-green-muted)] text-[var(--status-green)]">
				<Gauge size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong class="text-2xl font-bold text-[var(--text-primary)] font-[var(--font-mono)] tracking-tight tabular-nums">
				{formatMemory(currentMem)}
			</strong>
			<span class="text-xs font-mono text-[var(--text-secondary)]">{memPercent}% used</span>
		</div>

		<!-- Capacity Bar -->
		<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
			<div
				class="h-full rounded-full transition-all duration-500 {memPercent > 85 ? 'bg-[var(--status-red)]' : memPercent > 70 ? 'bg-[var(--status-amber)]' : 'bg-[var(--status-green)]'}"
				style="width: {memPercent}%;"
			></div>
		</div>

		<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
			<span>Budget: {memLimit} MB</span>
			<span>Available: {Math.max(0, memLimit - currentMem)} MB</span>
		</div>
	</div>

	<!-- 3. Network Throughput Card -->
	<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
		<div class="flex items-center justify-between">
			<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Network I/O</span>
			<div class="p-1 rounded bg-[var(--status-amber-muted)] text-[var(--status-amber)]">
				<ArrowsDownUp size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong class="text-lg font-bold text-[var(--text-primary)] font-[var(--font-mono)] tracking-tight tabular-nums truncate">
				{currentNetRx}
			</strong>
			<span class="text-xs font-mono text-[var(--text-secondary)]">Rx</span>
		</div>

		<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
			<div class="h-full rounded-full bg-[var(--status-amber)]" style="width: {hasContainers ? '45%' : '0%'};"></div>
		</div>

		<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
			<span class="truncate">Tx: {currentNetTx}</span>
			<span class="text-[var(--status-green)] font-medium">Socket IO</span>
		</div>
	</div>
</div>
