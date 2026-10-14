<script lang="ts">
	import { formatMemory } from '$lib/utils/format';
	import { Cpu, Gauge, ArrowsDownUp } from 'phosphor-svelte';
	import { LayerCard } from 'kumo-svelte';

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

<div class="grid grid-cols-1 gap-4 md:grid-cols-3">
	<!-- 1. CPU Card -->
	<LayerCard class="flex flex-col justify-between gap-3 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-secondary)]">CPU utilization</span>
			<div class="rounded bg-[var(--accent-muted)] p-1 text-[var(--accent)]">
				<Cpu size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong
				class="text-2xl font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
			>
				{currentCpu.toFixed(1)}%
			</strong>
			<span class="font-mono text-xs text-[var(--text-secondary)]">cgroup v2</span>
		</div>

		<!-- Capacity Bar -->
		<div class="h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-surface)]">
			<div
				class="h-full rounded-full transition-all duration-500 {currentCpu > 5
					? 'bg-[var(--status-red)]'
					: currentCpu > 2
						? 'bg-[var(--status-amber)]'
						: 'bg-[var(--accent)]'}"
				style="width: {Math.min(currentCpu, 100)}%;"
			></div>
		</div>

		<div
			class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-1 text-[11px] text-[var(--text-tertiary)]"
		>
			<span>Current: {currentCpu.toFixed(1)}%</span>
			<span
				class={currentCpu > 5
					? 'font-medium text-[var(--status-amber)]'
					: 'font-medium text-[var(--status-green)]'}
			>
				{currentCpu > 5 ? 'Elevated' : 'Optimal'}
			</span>
		</div>
	</LayerCard>

	<!-- 2. Memory Card -->
	<LayerCard class="flex flex-col justify-between gap-3 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-secondary)]">Memory allocation</span>
			<div class="rounded bg-[var(--status-green-muted)] p-1 text-[var(--status-green)]">
				<Gauge size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong
				class="text-2xl font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
			>
				{formatMemory(currentMem)}
			</strong>
			<span class="font-mono text-xs text-[var(--text-secondary)]">{memPercent}% used</span>
		</div>

		<!-- Capacity Bar -->
		<div class="h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-surface)]">
			<div
				class="h-full rounded-full transition-all duration-500 {memPercent > 85
					? 'bg-[var(--status-red)]'
					: memPercent > 70
						? 'bg-[var(--status-amber)]'
						: 'bg-[var(--status-green)]'}"
				style="width: {memPercent}%;"
			></div>
		</div>

		<div
			class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-1 text-[11px] text-[var(--text-tertiary)]"
		>
			<span>Budget: {memLimit} MB</span>
			<span>Available: {Math.max(0, memLimit - currentMem)} MB</span>
		</div>
	</LayerCard>

	<!-- 3. Network Throughput Card -->
	<LayerCard class="flex flex-col justify-between gap-3 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-secondary)]">Network I/O</span>
			<div class="rounded bg-[var(--status-amber-muted)] p-1 text-[var(--status-amber)]">
				<ArrowsDownUp size={14} />
			</div>
		</div>

		<div class="flex items-baseline justify-between gap-2">
			<strong
				class="truncate text-lg font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
			>
				{currentNetRx}
			</strong>
			<span class="font-mono text-xs text-[var(--text-secondary)]">Rx</span>
		</div>

		<div class="h-1.5 w-full overflow-hidden rounded-full bg-[var(--bg-surface)]">
			<div
				class="h-full rounded-full bg-[var(--status-amber)]"
				style="width: {hasContainers ? '45%' : '0%'};"
			></div>
		</div>

		<div
			class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-1 text-[11px] text-[var(--text-tertiary)]"
		>
			<span class="truncate">Tx: {currentNetTx}</span>
			<span class="font-medium text-[var(--status-green)]">Socket IO</span>
		</div>
	</LayerCard>
</div>
