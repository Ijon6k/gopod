<script lang="ts">
	import { onMount } from 'svelte';
	import type { Service, Workload, TimeSeriesPoint, Container } from '$lib/types';
	import { AreaChart, StatusBadge } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
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

	// Match actual containers from Podman runtime store
	let matchingContainers = $derived.by<Container[]>(() => {
		return dataStore.containers.filter(
			(c) => c.serviceId === service.id || c.name.startsWith(service.id) || (c.serviceName && c.serviceName === service.name)
		);
	});

	let selectedContainer = $state('all');
	let timeRange = $state<'1h' | '6h' | '24h' | '7d'>('24h');
	let isRefreshing = $state(false);
	let isRestartingContainer = $state(false);
	let refreshRate = $state<'3s' | '5s' | '15s' | 'paused'>('3s');
	let lastUpdatedSec = $state(0);

	// Currently targeted container (null if aggregate 'all')
	let activeContainer = $derived.by<Container | null>(() => {
		if (selectedContainer === 'all') {
			return matchingContainers.length === 1 ? matchingContainers[0] : null;
		}
		return matchingContainers.find((c) => c.name === selectedContainer || c.id === selectedContainer) ?? null;
	});

	// Compute real live metrics based on selected container or container aggregate
	let currentCpu = $derived.by<number>(() => {
		if (activeContainer) {
			return activeContainer.cpu ?? 0;
		}
		if (matchingContainers.length > 0) {
			return parseFloat(matchingContainers.reduce((acc, c) => acc + (c.cpu || 0), 0).toFixed(1));
		}
		return service.cpu ?? 0;
	});

	let currentMem = $derived.by<number>(() => {
		if (activeContainer) {
			return activeContainer.memory ?? 0;
		}
		if (matchingContainers.length > 0) {
			return matchingContainers.reduce((acc, c) => acc + (c.memory || 0), 0);
		}
		return service.memory ?? 0;
	});

	let memLimit = $derived.by<number>(() => {
		if (activeContainer && activeContainer.memoryLimit) {
			return activeContainer.memoryLimit;
		}
		if (matchingContainers.length > 0) {
			const maxLimit = Math.max(...matchingContainers.map((c) => c.memoryLimit || 0));
			if (maxLimit > 0) return maxLimit;
		}
		return 512;
	});

	let memPercent = $derived(
		memLimit > 0 ? Math.min(Math.round((currentMem / memLimit) * 100), 100) : 0
	);

	let currentNetRx = $derived.by<string>(() => {
		if (activeContainer) return activeContainer.netRx || '0 B';
		if (matchingContainers.length > 0) {
			const valid = matchingContainers.map((c) => c.netRx).filter((v) => v && v !== '—');
			return valid.length > 0 ? valid.join(' + ') : '0 B';
		}
		return '0 B';
	});

	let currentNetTx = $derived.by<string>(() => {
		if (activeContainer) return activeContainer.netTx || '0 B';
		if (matchingContainers.length > 0) {
			const valid = matchingContainers.map((c) => c.netTx).filter((v) => v && v !== '—');
			return valid.length > 0 ? valid.join(' + ') : '0 B';
		}
		return '0 B';
	});

	let currentPids = $derived.by<number>(() => {
		if (activeContainer) return activeContainer.pids || (activeContainer.status === 'running' ? 1 : 0);
		if (matchingContainers.length > 0) {
			return matchingContainers.reduce((acc, c) => acc + (c.pids || 0), 0);
		}
		return service.status === 'running' ? 1 : 0;
	});

	let currentStatus = $derived.by<string>(() => {
		if (activeContainer) return activeContainer.status;
		if (matchingContainers.length > 0) {
			return matchingContainers.some((c) => c.status === 'running') ? 'running' : 'stopped';
		}
		return service.status || 'stopped';
	});

	let isElevatedMem = $derived(memPercent >= 75);
	let isElevatedCpu = $derived(currentCpu >= 5.0);
	let isUnhealthy = $derived(
		service.health === 'unhealthy' ||
		service.status === 'failed' ||
		service.status === 'degraded' ||
		currentStatus === 'stopped'
	);

	function formatMemory(mb: number): string {
		return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
	}

	function parseNetToMb(netStr: string): number {
		if (!netStr || netStr === '—') return 0;
		const parts = netStr.match(/([0-9.]+)\s*([a-zA-Z]+)/);
		if (!parts) return 0;
		const val = parseFloat(parts[1]);
		const unit = parts[2].toLowerCase();
		if (unit.startsWith('g')) return val * 1024;
		if (unit.startsWith('m')) return val;
		if (unit.startsWith('k')) return val / 1024;
		return val / (1024 * 1024);
	}

	// Dynamic Rolling Telemetry Buffers for Live Container Charts (Dokploy Parity)
	function initSeries(baseVal: number, count = 20): TimeSeriesPoint[] {
		const now = Date.now();
		return Array.from({ length: count }, (_, i) => {
			const t = new Date(now - (count - 1 - i) * 2000);
			return {
				time: t.toISOString(),
				label: `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}:${t.getSeconds().toString().padStart(2, '0')}`,
				value: Math.max(0, parseFloat(baseVal.toFixed(2)))
			};
		});
	}

	let cpuHistory = $state<TimeSeriesPoint[]>(initSeries(0));
	let memoryHistory = $state<TimeSeriesPoint[]>(initSeries(0));
	let networkHistory = $state<TimeSeriesPoint[]>(initSeries(0));

	function recordPoint(series: TimeSeriesPoint[], val: number, timeStr: string, labelStr: string) {
		series.push({ time: timeStr, label: labelStr, value: Math.max(0, parseFloat(val.toFixed(2))) });
		if (series.length > 25) {
			series.shift();
		}
	}

	// Record live data point whenever container stats update
	$effect(() => {
		const cpu = currentCpu;
		const mem = currentMem;
		const netRxMb = parseNetToMb(currentNetRx);
		const netTxMb = parseNetToMb(currentNetTx);
		const netTotal = parseFloat((netRxMb + netTxMb).toFixed(2));

		const now = new Date();
		const timeStr = now.toISOString();
		const labelStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`;

		recordPoint(cpuHistory, cpu, timeStr, labelStr);
		recordPoint(memoryHistory, mem, timeStr, labelStr);
		recordPoint(networkHistory, netTotal, timeStr, labelStr);
	});

	function handleRefresh() {
		isRefreshing = true;
		dataStore.fetchLiveStats();
		lastUpdatedSec = 0;
		setTimeout(() => (isRefreshing = false), 500);
	}

	async function handleRestartContainer() {
		const targetId = activeContainer?.id || (matchingContainers.length > 0 ? matchingContainers[0].id : null);
		if (!targetId || isRestartingContainer) return;
		isRestartingContainer = true;
		try {
			await api.runtime.containers.restart(targetId);
			await dataStore.fetchLiveStats();
		} catch (err) {
			console.error('Failed to restart container:', err);
		} finally {
			isRestartingContainer = false;
		}
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
	<!-- Top Bar: Filter Bar + Dokploy Container Selector + Polling Controls -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-[var(--border-subtle)]">
		<div class="flex items-center flex-wrap gap-3">
			<!-- Dokploy Parity Container Selector -->
			<div class="flex items-center gap-2">
				<span class="text-xs text-[var(--text-tertiary)] font-medium">Container:</span>
				<div class="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex items-center gap-1.5">
					<select
						bind:value={selectedContainer}
						class="bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
					>
						{#if matchingContainers.length > 1}
							<option value="all">All Containers ({matchingContainers.length} Aggregate)</option>
						{/if}
						{#each matchingContainers as c}
							<option value={c.name}>{c.name} ({c.id.slice(0, 12)}) — {c.status}</option>
						{/each}
						{#if matchingContainers.length === 0}
							<option value="all">{service.name} (Waiting for container to start)</option>
						{/if}
					</select>
				</div>
			</div>

			<!-- Quick Restart Action (Dokploy Parity) -->
			{#if matchingContainers.length > 0}
				<button
					type="button"
					onclick={handleRestartContainer}
					disabled={isRestartingContainer}
					class="px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors flex items-center gap-1.5 disabled:opacity-50"
					title="Restart container via Podman socket"
				>
					<ArrowClockwise size={12} class={isRestartingContainer ? 'animate-spin' : ''} />
					<span>Restart Container</span>
				</button>
			{/if}

			<!-- Polling & Live Socket Indicator -->
			<div class="flex items-center gap-1.5 px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-xs">
				<span class="w-2 h-2 rounded-full {refreshRate === 'paused' ? 'bg-amber-400' : 'bg-emerald-400 animate-pulse'}"></span>
				<span class="text-[11px] font-mono text-[var(--text-secondary)]">
					{refreshRate === 'paused' ? 'Paused' : `Live Polling (${refreshRate})`}
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

	<!-- Real-Time Anomaly & Threshold Status Banner -->
	{#if isUnhealthy}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-[var(--radius-card)] bg-rose-500/10 border border-rose-500/30 text-xs shadow-xs">
			<div class="flex items-center gap-2.5 text-rose-300">
				<Warning size={16} class="text-rose-400 shrink-0" />
				<div>
					<span class="font-bold">Service inactive or degraded:</span>
					<span class="text-rose-200/90 ml-1">
						{matchingContainers.length === 0 ? 'No container running for this workload. Click Deploy or Start to initialize.' : 'Container process exited or healthcheck is degraded.'}
					</span>
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
							Memory allocation is at {memPercent}% of {memLimit} MB budget ({formatMemory(currentMem)}). Close to cgroup threshold.
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
				<span>{matchingContainers.length} {matchingContainers.length > 1 ? 'containers active' : 'container active'}</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span class="text-emerald-400 font-mono">cgroups v2 live</span>
			</div>
			<span class="text-[11px] font-mono text-emerald-400 shrink-0 hidden sm:inline">Podman 5.x OK</span>
		</div>
	{/if}

	<!-- 4 Contextual Metric Cards with Progress Bars (Dokploy Live Parity) -->
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
				<span class="text-xs font-mono text-[var(--text-secondary)]">cgroup</span>
			</div>

			<!-- Capacity Bar -->
			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div
					class="h-full rounded-full transition-all duration-500 {currentCpu > 5 ? 'bg-rose-500' : currentCpu > 2 ? 'bg-amber-400' : 'bg-[var(--accent)]'}"
					style="width: {Math.min(currentCpu * 10, 100)}%;"
				></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span>Current: {currentCpu.toFixed(1)}%</span>
				<span class={currentCpu > 5 ? 'text-amber-400 font-medium' : 'text-emerald-400 font-medium'}>
					{currentCpu > 5 ? 'Elevated' : 'Optimal'}
				</span>
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
				<span>Free: {Math.max(0, memLimit - currentMem)} MB</span>
			</div>
		</div>

		<!-- 3. Network Throughput Card -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Network I/O</span>
				<div class="p-1 rounded bg-amber-500/10 text-amber-400">
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
				<div class="h-full rounded-full bg-amber-400" style="width: {matchingContainers.length > 0 ? '45%' : '0%'};"></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span class="truncate">Tx: {currentNetTx}</span>
				<span class="text-emerald-400 font-medium">Socket IO</span>
			</div>
		</div>

		<!-- 4. Health & Lifecycle Card -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col justify-between gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<span class="text-[10px] uppercase font-semibold text-[var(--text-tertiary)] tracking-wider">Health & Lifecycle</span>
				<div class="p-1 rounded bg-[var(--accent-muted)] text-[var(--accent)]">
					<Pulse size={14} />
				</div>
			</div>

			<div class="flex items-center justify-between gap-2">
				<StatusBadge status={currentStatus === 'running' ? 'healthy' : 'stopped'} size="sm" />
				<span class="text-xs font-mono text-[var(--text-secondary)]">{currentPids} PIDs</span>
			</div>

			<div class="w-full h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden">
				<div class="h-full rounded-full {currentStatus === 'running' ? 'bg-emerald-400' : 'bg-zinc-600'}" style="width: {currentStatus === 'running' ? '100%' : '0%'};"></div>
			</div>

			<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1 border-t border-[var(--border-subtle)]">
				<span class="truncate">Status: {currentStatus}</span>
				<span class="text-[var(--text-tertiary)] font-mono">{service.restartPolicy || 'always'}</span>
			</div>
		</div>
	</div>

	<!-- 2x2 Metric Charts Grid with Live Per-Container Time Series (Dokploy Parity) -->
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
					<span>Peak: <strong class="text-[var(--text-secondary)]">{Math.max(...cpuHistory.map((p) => p.value), currentCpu).toFixed(1)}%</strong></span>
				</div>
			</div>
			<AreaChart data={cpuHistory} height={150} />
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
					<span>Limit: {memLimit} MB</span>
				</div>
			</div>
			<AreaChart data={memoryHistory} height={150} strokeColor="#4C9A72" fillColor="#4C9A72" />
		</div>

		<!-- Network Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Network Traffic</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>Rx: <strong class="text-[var(--text-secondary)]">{currentNetRx}</strong></span>
					<span>•</span>
					<span>Tx: <strong class="text-[var(--text-secondary)]">{currentNetTx}</strong></span>
				</div>
			</div>
			<AreaChart data={networkHistory} height={150} strokeColor="#7680B5" fillColor="#7680B5" />
		</div>

		<!-- Storage / Host I/O Chart -->
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3 shadow-xs">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-[var(--text-primary)]">System Host Storage</span>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">({timeRange})</span>
				</div>
				<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
					<span>Usage: <strong class="text-[var(--text-secondary)]">{dataStore.server.storageUsed} GB</strong></span>
					<span>•</span>
					<span>Total: <strong class="text-[var(--text-secondary)]">{dataStore.server.storage} GB</strong></span>
				</div>
			</div>
			<AreaChart data={dataStore.monitoringData.storage} height={150} strokeColor="#B8893B" fillColor="#B8893B" />
		</div>
	</div>

	<!-- Micro-Workloads & Container Cgroups Breakdown -->
	<MicroWorkloadTable
		{service}
		onOpenTerminal={() => onNavigateTab?.('terminal')}
		onOpenLogs={() => onNavigateTab?.('logs')}
	/>
</div>
