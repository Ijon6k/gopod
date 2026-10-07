<script lang="ts">
	import { onMount } from 'svelte';
	import type { Service, Workload, TimeSeriesPoint, Container } from '$lib/types';
	import { StatusBadge } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { MicroWorkloadTable } from '$lib/components/features/monitoring';
	import { formatMemory, parseNetToMb } from '$lib/utils/format';
	import TelemetryMetricCards from './TelemetryMetricCards.svelte';
	import TelemetryChartsGrid from './TelemetryChartsGrid.svelte';
	import {
		ArrowClockwise,
		Warning,
		CheckCircle
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onNavigateTab?: (tab: string) => void;
	}

	let { service, onNavigateTab }: Props = $props();

	// Match actual containers from Podman runtime store (Dokploy multi-container parity)
	let matchingContainers = $derived.by<Container[]>(() => {
		return dataStore.containers.filter(
			(c) =>
				c.serviceId === service.id ||
				c.name === service.name ||
				c.name.startsWith(service.id) ||
				(c.serviceName && c.serviceName === service.name) ||
				(c.labels &&
					(c.labels['com.docker.compose.project'] === service.name ||
						c.labels['io.podman.compose.project'] === service.name ||
						c.labels['io.gopod.service'] === service.id))
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
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-[var(--radius-card)] bg-[var(--status-red-muted)] border border-[var(--status-red)] text-xs shadow-xs">
			<div class="flex items-center gap-2.5 text-[var(--status-red)]">
				<Warning size={16} class="shrink-0" />
				<div>
					<span class="font-bold">Service inactive or degraded:</span>
					<span class="text-[var(--text-secondary)] ml-1">
						{matchingContainers.length === 0 ? 'No container running for this workload. Click Deploy or Start to initialize.' : 'Container process exited or healthcheck is degraded.'}
					</span>
				</div>
			</div>
			<button
				type="button"
				onclick={() => onNavigateTab?.('logs')}
				class="px-2.5 py-1 rounded bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--status-red)] font-mono text-[11px] border border-[var(--status-red)] cursor-pointer self-start sm:self-auto shrink-0 transition-colors"
			>
				Inspect Logs →
			</button>
		</div>
	{:else if isElevatedMem || isElevatedCpu}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-3 rounded-[var(--radius-card)] bg-[var(--status-amber-muted)] border border-[var(--status-amber)] text-xs shadow-xs">
			<div class="flex items-center gap-2.5 text-[var(--status-amber)]">
				<Warning size={16} class="shrink-0" />
				<div>
					<span class="font-bold">Elevated Resource Consumption:</span>
					<span class="text-[var(--text-secondary)] ml-1">
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
				class="px-2.5 py-1 rounded bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] text-[var(--status-amber)] font-mono text-[11px] border border-[var(--status-amber)] cursor-pointer self-start sm:self-auto shrink-0 transition-colors"
			>
				Adjust Cgroups Limit →
			</button>
		</div>
	{:else}
		<div class="flex items-center justify-between px-3.5 py-2.5 rounded-[var(--radius-card)] bg-[var(--status-green-muted)] border border-[var(--status-green)] text-xs text-[var(--text-secondary)] shadow-xs">
			<div class="flex items-center gap-2.5 flex-wrap">
				<CheckCircle size={15} class="text-[var(--status-green)] shrink-0" />
				<span class="font-semibold text-[var(--text-primary)]">Workload operating nominally</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span>{formatMemory(currentMem)} / {memLimit} MB allocated</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span>{matchingContainers.length} {matchingContainers.length > 1 ? 'containers active' : 'container active'}</span>
				<span class="text-[var(--text-tertiary)]">•</span>
				<span class="text-[var(--status-green)] font-mono">cgroups v2 live</span>
			</div>
			<span class="text-[11px] font-mono text-[var(--status-green)] shrink-0 hidden sm:inline">Podman 5.x OK</span>
		</div>
	{/if}

	<!-- Contextual Metric Cards with Progress Bars -->
	<TelemetryMetricCards
		{currentCpu}
		{currentMem}
		{memLimit}
		{memPercent}
		{currentNetRx}
		{currentNetTx}
		hasContainers={matchingContainers.length > 0}
	/>

	<!-- 2x2 Metric Charts Grid with Live Per-Container Time Series (Dokploy Parity) -->
	<TelemetryChartsGrid
		{timeRange}
		{cpuHistory}
		{currentCpu}
		{memoryHistory}
		{currentMem}
		{memLimit}
		{networkHistory}
		{currentNetRx}
		{currentNetTx}
		storageHistory={dataStore.monitoringData.storage}
		storageUsed={dataStore.server.storageUsed}
		storageTotal={dataStore.server.storage}
	/>

	<!-- Micro-Workloads & Container Cgroups Breakdown -->
	<MicroWorkloadTable
		{service}
		onOpenTerminal={() => onNavigateTab?.('terminal')}
		onOpenLogs={() => onNavigateTab?.('logs')}
	/>
</div>
