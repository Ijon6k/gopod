<script lang="ts">
	import { onMount } from 'svelte';
	import type { Service, Workload } from '$lib/types';
	import { AreaChart, StatusBadge } from '$lib/components/ui';
	import { monitoringData, dataStore } from '$lib/data';
	import { MicroWorkloadTable } from '$lib/components/features/monitoring';
	import {
		Cpu,
		Gauge,
		HardDrive,
		Pulse,
		ArrowClockwise,
		Warning,
		CheckCircle,
		ArrowsDownUp
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onNavigateTab?: (tab: string) => void;
	}

	let { service, onNavigateTab }: Props = $props();

	let isPod = $derived(service.type === 'pod' || (service.workloads && service.workloads.length > 1));
	let workloads = $derived<Workload[]>(
		service.workloads ?? [{ name: service.name, image: service.image ?? '—', status: service.status }]
	);

	let selectedContainer = $state('all');
	let timeRange = $state<'1h' | '6h' | '24h' | '7d'>('24h');
	let isRefreshing = $state(false);
	let refreshRate = $state<'3s' | '5s' | '15s' | 'paused'>('3s');
	let lastUpdatedSec = $state(0);

	// Compute metrics based on selected container
	let currentWorkload = $derived(
		selectedContainer === 'all' ? null : workloads.find((w) => w.name === selectedContainer)
	);

	let currentCpu = $derived(currentWorkload?.cpu ?? service.cpu ?? 1.2);
	let currentMem = $derived(currentWorkload?.memory ?? service.memory ?? 180);
	let memLimit = $derived(512);
	let memPercent = $derived(Math.min(Math.round((currentMem / memLimit) * 100), 100));

	let isElevatedMem = $derived(memPercent >= 75);
	let isElevatedCpu = $derived(currentCpu >= 5.0);
	let isUnhealthy = $derived(service.health === 'unhealthy' || service.status === 'failed' || service.status === 'degraded');

	function formatMemory(mb: number): string {
		return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
	}

	function handleRefresh() {
		isRefreshing = true;
		dataStore.fetchLiveStats();
		lastUpdatedSec = 0;
		setTimeout(() => (isRefreshing = false), 500);
	}

	onMount(() => {
		dataStore.fetchLiveStats();
		let pollTimer: any;

		function setupTimer() {
			if (pollTimer) clearInterval(pollTimer);
			if (refreshRate === 'paused') return;

			const ms = refreshRate === '3s' ? 3000 : refreshRate === '5s' ? 5000 : 15000;
			pollTimer = setInterval(() => {
				dataStore.fetchLiveStats();
				lastUpdatedSec = 0;
			}, ms);
		}

		setupTimer();

		const secTimer = setInterval(() => {
			if (refreshRate !== 'paused') {
				lastUpdatedSec += 1;
			}
		}, 1000);

		return () => {
			if (pollTimer) clearInterval(pollTimer);
			clearInterval(secTimer);
		};
	});
</script>

<div class="w-full flex flex-col gap-6">
	<!-- Top Bar: Filter Bar + Polling Controls + Live Status Indicator -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-[var(--border-subtle)]">
		<div class="flex items-center flex-wrap gap-3">
			{#if isPod}
				<div class="flex items-center gap-2">
					<span class="text-xs text-[var(--text-tertiary)] font-medium">Container:</span>
					<div class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
						<select
							bind:value={selectedContainer}
							class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
						>
							<option value="all">All containers (Aggregate)</option>
							{#each workloads as w}
								<option value={w.name}>{w.name} ({w.image})</option>
							{/each}
						</select>
					</div>
				</div>
			{:else}
				<div class="flex items-center gap-2">
					<span class="text-xs text-[var(--text-tertiary)] font-medium">Workload:</span>
					<span class="text-xs font-[var(--font-mono)] font-semibold text-[var(--text-primary)]">
						{service.name}
					</span>
				</div>
			{/if}

			<!-- Polling & Live Socket Indicator (Nielsen #1: System Status & Nielsen #3: User Control) -->
			<div class="flex items-center gap-1.5 px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-xs">
				<span class="w-2 h-2 rounded-full {refreshRate === 'paused' ? 'bg-amber-400' : 'bg-emerald-400 animate-pulse'}"></span>
				<span class="text-[11px] font-mono text-[var(--text-secondary)]">
					{refreshRate === 'paused' ? 'Paused' : `Socket (${refreshRate})`}
				</span>
				<select
					bind:value={refreshRate}
					class="bg-transparent border-0 outline-none text-[11px] font-mono text-[var(--text-tertiary)] hover:text-[var(--text-primary)] cursor-pointer ml-0.5"
				>
					<option value="3s">3s</option>
					<option value="5s">5s</option>
					<option value="15s">15s</option>
					<option value="paused">Pause</option>
				</select>
			</div>

			{#if refreshRate !== 'paused'}
				<span class="text-[10.5px] font-mono text-[var(--text-tertiary)] hidden md:inline">
					{lastUpdatedSec}s ago
				</span>
			{/if}
		</div>

		<!-- Time range selector & Manual Refresh -->
		<div class="flex items-center gap-2">
			<div class="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
				{#each ['1h', '6h', '24h', '7d'] as r}
					<button
						type="button"
						onclick={() => (timeRange = r as any)}
						class="px-2.5 py-1 rounded text-xs transition-colors cursor-pointer border-0 {timeRange === r
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						{r}
					</button>
				{/each}
			</div>

			<button
				type="button"
				onclick={handleRefresh}
				class="p-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
				title="Refresh telemetry"
				aria-label="Refresh telemetry"
			>
				<ArrowClockwise size={13} class={isRefreshing ? 'animate-spin' : ''} />
			</button>
		</div>
	</div>

	<!-- Real-Time Anomaly & Threshold Status Banner (Krug #1 Don't Make Me Think + Nielsen #1) -->
	{#if isUnhealthy}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-[var(--radius-card)] bg-rose-500/10 border border-rose-500/30 text-xs shadow-xs">
			<div class="flex items-center gap-2.5 text-rose-300">
				<Warning size={16} class="text-rose-400 shrink-0" />
				<div>
					<span class="font-bold">Service degraded / unhealthy:</span>
					<span class="text-rose-200/90 ml-1">Podman cgroup healthcheck failing. Process exited or failed to respond within timeout.</span>
				</div>
			</div>
			<button
				type="button"
				onclick={() => onNavigateTab?.('logs')}
				class="px-2.5 py-1 rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 font-mono text-[11px] border border-rose-500/30 cursor-pointer self-start sm:self-auto shrink-0 transition-colors"
			>
				Inspect Logs →
			</button>
		</div>
	{:else if isElevatedMem || isElevatedCpu}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-[var(--radius-card)] bg-amber-500/10 border border-amber-500/30 text-xs shadow-xs">
			<div class="flex items-center gap-2.5 text-amber-200">
				<Warning size={16} class="text-amber-400 shrink-0" />
				<div>
					<span class="font-bold">Elevated Resource Consumption:</span>
					<span class="text-amber-100/90 ml-1">
						{#if isElevatedMem}
							Memory allocation is at {memPercent}% of {memLimit} MB budget ({formatMemory(currentMem)}). Close to cgroup OOM threshold.
						{:else}
							CPU is peaking at {currentCpu.toFixed(1)}% across allocated cores.
						{/if}
					</span>
				</div>
			</div>
			<button
				type="button"
				onclick={() => onNavigateTab?.('advanced')}
				class="px-2.5 py-1 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-200 font-mono text-[11px] border border-amber-500/30 cursor-pointer self-start sm:self-auto shrink-0 transition-colors"
			>
				Adjust Cgroups Limit →
			</button>
		</div>
	{:else}
		<div class="flex items-center justify-between px-3.5 py-2.5 rounded-[var(--radius-card)] bg-emerald-500/5 border border-emerald-500/20 text-xs text-[var(--text-secondary)] shadow-xs">
			<div class="flex items-center gap-2.5 flex-wrap">
				<CheckCircle size={15} class="text-emerald-400 shrink-0" />
				<span class="font-semibold text-[var(--text-primary)]">Workload operating nominally</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span>{formatMemory(currentMem)} / {memLimit} MB allocated</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span>0 restarts (24h)</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span class="text-emerald-400 font-mono">cgroups v2 active</span>
			</div>
			<span class="text-[11px] font-mono text-emerald-400 shrink-0 hidden sm:inline">HTTP 200 Passing</span>
		</div>
	{/if}

	<!-- 4 Contextual Metric Cards with Progress Bars (Refactoring UI: Values emphasized over labels) -->
	<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
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
				<span class="text-xs font-mono text-[var(--text-secondary)]">1.0 vCPU</span>
			</div>

			<!-- Capacity Bar -->
			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500 {currentCpu > 5 ? 'bg-rose-500' : currentCpu > 2 ? 'bg-amber-400' : 'bg-[var(--accent)]'}"
					style="width: {Math.min(currentCpu * 10, 100)}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Peak: {(currentCpu * 1.8).toFixed(1)}%</span>
				<span class="text-emerald-400 font-medium">Optimal</span>
			</div>
		</div>

		<!-- 2. Memory Card -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Memory Allocation</span>
				<div class="p-1 rounded bg-emerald-500/10 text-emerald-400">
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
					class="h-full rounded-full transition-all duration-500 {memPercent > 85 ? 'bg-rose-500' : memPercent > 70 ? 'bg-amber-400' : 'bg-emerald-400'}"
					style="width: {memPercent}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Limit: {memLimit} MB</span>
				<span>Free: {memLimit - currentMem} MB</span>
			</div>
		</div>

		<!-- 3. Disk I/O Card -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Disk Throughput</span>
				<div class="p-1 rounded bg-amber-500/10 text-amber-400">
					<HardDrive size={14} />
				</div>
			</div>

			<div class="flex items-baseline justify-between gap-2">
				<strong class="text-2xl font-bold text-[var(--text-primary)] font-[var(--font-mono)] tracking-tight tabular-nums">
					30.6 <span class="text-xs font-normal text-[var(--text-tertiary)]">MB/s</span>
				</strong>
				<span class="text-xs font-mono text-[var(--text-secondary)]">NVMe</span>
			</div>

			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div class="h-full rounded-full bg-amber-400" style="width: 24%;"></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Read: 12.4 MB/s</span>
				<span>Write: 18.2 MB/s</span>
			</div>
		</div>

		<!-- 4. Health & Restarts Card -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Health & Lifecycle</span>
				<div class="p-1 rounded bg-[var(--accent-muted)] text-[var(--accent)]">
					<Pulse size={14} />
				</div>
			</div>

			<div class="flex items-center justify-between gap-2">
				<StatusBadge status={service.health === 'unhealthy' ? 'unhealthy' : 'healthy'} size="sm" />
				<span class="text-xs font-mono text-[var(--text-secondary)]">0 restarts</span>
			</div>

			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div class="h-full rounded-full bg-emerald-400" style="width: 100%;"></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>HTTP 200 /health</span>
				<span class="text-emerald-400 font-medium">Passing</span>
			</div>
		</div>
	</div>

	<!-- 2x2 Metric Charts Grid with Live High/Low Badges (Scannable At A Glance) -->
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<!-- CPU Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">CPU Utilization</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>Cur: <strong class="text-[var(--text-primary)]">{currentCpu.toFixed(1)}%</strong></span>
					<span>•</span>
					<span>Peak: <strong class="text-[var(--text-secondary)]">{(currentCpu * 1.8).toFixed(1)}%</strong></span>
					<span>•</span>
					<span>Avg: {(currentCpu * 1.1).toFixed(1)}%</span>
				</div>
			</div>
			<AreaChart data={monitoringData.cpu} height={150} />
		</div>

		<!-- Memory Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Memory Utilization</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>Cur: <strong class="text-[var(--text-primary)]">{formatMemory(currentMem)}</strong></span>
					<span>•</span>
					<span>Peak: <strong class="text-[var(--text-secondary)]">{formatMemory(currentMem * 1.2)}</strong></span>
					<span>•</span>
					<span>Limit: {memLimit} MB</span>
				</div>
			</div>
			<AreaChart data={monitoringData.memory} height={150} strokeColor="#4C9A72" fillColor="#4C9A72" />
		</div>

		<!-- Disk I/O Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Storage I/O Operations</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>Read: <strong class="text-[var(--text-secondary)]">12.4 MB/s</strong></span>
					<span>•</span>
					<span>Write: <strong class="text-[var(--text-secondary)]">18.2 MB/s</strong></span>
				</div>
			</div>
			<AreaChart data={monitoringData.storage} height={150} strokeColor="#B8893B" fillColor="#B8893B" />
		</div>

		<!-- Network Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Network Traffic</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>In: <strong class="text-[var(--text-secondary)]">4.8 MB/s</strong></span>
					<span>•</span>
					<span>Out: <strong class="text-[var(--text-secondary)]">7.6 MB/s</strong></span>
				</div>
			</div>
			<AreaChart data={monitoringData.network} height={150} strokeColor="#7680B5" fillColor="#7680B5" />
		</div>
	</div>

	<!-- Micro-Workloads & Container Cgroups Breakdown -->
	<MicroWorkloadTable
		{service}
		onOpenTerminal={() => onNavigateTab?.('terminal')}
		onOpenLogs={() => onNavigateTab?.('logs')}
	/>
</div>
