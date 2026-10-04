<script lang="ts">
	import { dataStore } from '$lib/stores/data.svelte';
	import { Cpu, HardDrive, Gauge, ShieldCheck, Pulse } from 'phosphor-svelte';

	interface Props {
		activeTab?: string;
		onSelectTab?: (tab: 'tree' | 'system' | 'storage' | 'charts') => void;
	}

	let { activeTab = '', onSelectTab }: Props = $props();

	let server = $derived(dataStore.server);
	let containers = $derived(dataStore.containers);

	let activeContainers = $derived(containers.filter((c) => c.status === 'running' || c.status === 'healthy').length);
	let failedContainers = $derived(containers.filter((c) => c.status === 'failed').length);

	const memPercent = $derived(((server.memoryUsed / (server.memory || 1)) * 100).toFixed(0));
	const diskPercent = $derived(((server.storageUsed / (server.storage || 1)) * 100).toFixed(0));
</script>

<div class="w-full flex flex-col gap-3">
	<!-- Top Host Overview Grid -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3.5">
		<!-- 1. vCPU Cores -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">CPU Utilization</span>
				<div class="p-1 rounded bg-[var(--accent-muted)] text-[var(--accent)]">
					<Cpu size={15} />
				</div>
			</div>

			<div class="flex items-baseline justify-between gap-2">
				<strong class="text-2xl font-medium tracking-tight text-[var(--text-primary)] tabular-nums font-[var(--font-mono)]">
					{server.cpuUsage.toFixed(1)}%
				</strong>
				<span class="text-[11px] text-[var(--text-tertiary)] tabular-nums">
					{server.vcpu} vCPUs
				</span>
			</div>

			<!-- Mini Progress bar -->
			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500 bg-[var(--accent)]"
					style="width: {server.cpuUsage}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[10.5px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Load: <span class="font-mono text-[var(--text-secondary)]">{server.cpuUsage.toFixed(1)}% ({((server.cpuUsage / (server.vcpu || 1))).toFixed(1)}% / core)</span></span>
				<span class="{server.cpuUsage > 80 ? 'text-[var(--status-red)]' : server.cpuUsage > 50 ? 'text-[var(--status-amber)]' : 'text-[var(--status-green)]'} font-medium">
					{server.cpuUsage > 80 ? 'High' : server.cpuUsage > 50 ? 'Moderate' : 'Optimal'}
				</span>
			</div>
		</div>

		<!-- 2. Memory RAM -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Memory (RAM)</span>
				<div class="p-1 rounded bg-[var(--status-green-muted)] text-[var(--status-green)]">
					<Gauge size={15} />
				</div>
			</div>

			<div class="flex items-baseline justify-between gap-2">
				<strong class="text-2xl font-medium tracking-tight text-[var(--text-primary)] tabular-nums font-[var(--font-mono)]">
					{server.memoryUsed.toFixed(1)} <span class="text-xs font-normal text-[var(--text-tertiary)]">/ {server.memory} GB</span>
				</strong>
				<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] tabular-nums">
					{memPercent}%
				</span>
			</div>

			<!-- Memory Progress bar -->
			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500 bg-[var(--status-green)]"
					style="width: {memPercent}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[10.5px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Available: {(server.memory - server.memoryUsed).toFixed(1)} GB</span>
				<span>Swap: 0 MB used</span>
			</div>
		</div>

		<!-- 3. Disk Storage -->
		<button
			type="button"
			onclick={() => onSelectTab?.('storage')}
			class="p-4 rounded-[var(--radius-card)] border bg-[var(--bg-panel)] transition-all flex flex-col justify-between gap-3 text-left cursor-pointer group {activeTab === 'storage' ? 'border-[var(--accent)] ring-1 ring-[var(--accent)]/30' : 'border-[var(--border)] hover:border-[var(--status-amber)]/60'}"
			title="Click to view detailed storage analysis & prune"
		>
			<div class="flex items-center justify-between w-full">
				<span class="text-xs font-medium text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]">Root Storage</span>
				<div class="p-1 rounded bg-[var(--status-amber-muted)] text-[var(--status-amber)]">
					<HardDrive size={15} />
				</div>
			</div>

			<div class="flex items-baseline justify-between gap-2 w-full">
				<strong class="text-2xl font-medium tracking-tight text-[var(--text-primary)] tabular-nums font-[var(--font-mono)]">
					{server.storageUsed.toFixed(1)} <span class="text-xs font-normal text-[var(--text-tertiary)]">/ {server.storage} GB</span>
				</strong>
				<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] tabular-nums">
					{diskPercent}%
				</span>
			</div>

			<!-- Storage Progress bar -->
			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500 bg-[var(--status-amber)]"
					style="width: {diskPercent}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[10.5px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)] w-full">
				<span>Free: {(server.storage - server.storageUsed).toFixed(1)} GB</span>
				<span class="text-[var(--accent)] group-hover:underline">Analyze & Prune →</span>
			</div>
		</button>

		<!-- 4. Workloads & Engine Health -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Workloads & Health</span>
				<div class="p-1 rounded bg-[var(--accent-muted)] text-[var(--accent)]">
					<Pulse size={15} />
				</div>
			</div>

			<div class="flex items-baseline justify-between gap-2">
				<strong class="text-2xl font-medium tracking-tight text-[var(--text-primary)] tabular-nums font-[var(--font-mono)]">
					{activeContainers} <span class="text-xs font-normal text-[var(--text-tertiary)]">active</span>
				</strong>
				{#if failedContainers > 0}
					<span class="text-[11px] text-[var(--status-red)] font-medium tabular-nums">
						{failedContainers} failed
					</span>
				{:else}
					<span class="text-[11px] text-[var(--status-green)] font-medium">
						All healthy
					</span>
				{/if}
			</div>

			<div class="flex items-center gap-1.5 text-[11px]">
				<span class="w-2 h-2 rounded-full bg-[var(--status-green)] shrink-0"></span>
				<span class="text-[var(--text-secondary)] truncate">Podman v{server.podmanVersion} ({server.rootless ? 'Rootless' : 'Root'})</span>
			</div>

			<div class="flex items-center justify-between text-[10.5px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Uptime: {server.uptime}</span>
				<span class="font-[var(--font-mono)]">{server.kernel.split('-')[0]}</span>
			</div>
		</div>
	</div>
</div>
