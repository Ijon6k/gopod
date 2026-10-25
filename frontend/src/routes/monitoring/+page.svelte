<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, AreaChart, Select } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { formatRateBps } from '$lib/utils/format';
	import {
		ServerOverviewBar,
		ProcessTaskManager,
		PodmanStorageManager,
		ContainerTerminalModal
	} from '$lib/components/features/monitoring';
	import { TreeStructure, Stack, HardDrive, ChartLineUp, ArrowClockwise } from 'phosphor-svelte';

	let server = $derived(dataStore.server);
	let urlView = $derived(page.url.searchParams.get('view'));
	let activeView = $state<'tree' | 'system' | 'storage' | 'charts'>('tree');
	let isRefreshing = $state(false);
	let refreshRate = $state<'3s' | '5s' | '15s' | 'paused'>('3s');
	let lastUpdatedSec = $state(0);
	let terminalTarget = $state<string | null>(null);

	// Direct series getters: each view tracks only the history it draws.
	let cpuHistory = $derived(dataStore.cpuHistory);
	let memoryHistory = $derived(dataStore.memoryHistory);
	let storageHistory = $derived(dataStore.storageHistory);
	let networkHistory = $derived(dataStore.networkHistory);

	// Session-max values cover exactly the sampled buffer the charts draw, so the
	// summary chips agree with what is visible
	let maxCpuSession = $derived(Math.max(server.cpuUsage, ...cpuHistory.map((p) => p.value)));
	let maxMemorySession = $derived(
		Math.max(server.memoryUsed, ...memoryHistory.map((p) => p.value))
	);

	$effect(() => {
		if (urlView === 'charts') {
			activeView = 'charts';
		} else if (urlView === 'system') {
			activeView = 'system';
		} else if (urlView === 'storage') {
			activeView = 'storage';
		} else if (urlView === 'tree') {
			activeView = 'tree';
		}
	});

	function setView(view: 'tree' | 'system' | 'storage' | 'charts') {
		activeView = view;
		const targetUrl = view === 'tree' ? '/monitoring' : `/monitoring?view=${view}`;
		goto(targetUrl, { replaceState: true, noScroll: true, keepFocus: true });
	}

	function handleOpenTerminal(containerName: string) {
		terminalTarget = containerName;
	}

	function handleRefresh() {
		isRefreshing = true;
		dataStore.fetchLiveStats();
		lastUpdatedSec = 0;
		setTimeout(() => (isRefreshing = false), 500);
	}

	// Stream ownership: a single $effect keyed on refreshRate + tab visibility
	// owns the whole lifecycle. Every re-run (mount, rate change, visibility)
	// stops the previous stream before starting the current one, so the
	// consumer refcount stays balanced — earlier start/stop pairs scattered
	// across onMount / handlers leaked the SSE connection permanently: change
	// the rate three times and the connection outlived the page forever.
	let tabHidden = $state(false);
	$effect(() => {
		if (refreshRate === 'paused' || tabHidden) {
			// No ownership action needed: the teardown of the previous run
			// already released the stream seat.
			return undefined;
		}
		dataStore.startStreamingStats();
		return () => dataStore.stopStreamingStats();
	});

	onMount(() => {
		// When monitoring page is opened, ensure complete projects & service topology is loaded.
		// TTL-guarded: the layout already fetched everything on boot.
		dataStore.fetchInitialData(25_000);

		// Visibility is not a reactive source, so it feeds the stream effect
		// through a tracked $state flag (start/stop stay paired per re-run).
		const handleVisibilityChange = () => {
			tabHidden = document.hidden;
		};
		document.addEventListener('visibilitychange', handleVisibilityChange);
		tabHidden = document.hidden;

		const secTimer = setInterval(() => {
			if (refreshRate !== 'paused') {
				lastUpdatedSec = (lastUpdatedSec + 1) % 60;
			}
		}, 1000);

		return () => {
			// Clean up when user navigates away
			document.removeEventListener('visibilitychange', handleVisibilityChange);
			clearInterval(secTimer);
		};
	});

	function handleRefreshRateChange(val: string) {
		refreshRate = val as '3s' | '5s' | '15s' | 'paused';
	}
</script>

<svelte:head>
	<title>Monitoring & Telemetry — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<!-- Page Header -->
	<PageHeader
		title="Monitoring & Telemetry"
		subtitle="Real-time resource utilization, container workloads, and system telemetry."
	/>

	<!-- 1. Host Hardware Overview Bar -->
	<ServerOverviewBar activeTab={activeView} onSelectTab={(tab) => setView(tab)} />

	<!-- 2. Subnav: Workloads vs System Daemons vs Storage & Prune vs Telemetry -->
	<div
		class="flex flex-col justify-between gap-3 border-b border-[var(--border)] pb-0 sm:flex-row sm:items-center"
	>
		<div class="flex items-center gap-1">
			<button
				type="button"
				onclick={() => setView('tree')}
				class="flex cursor-pointer items-center gap-2 border-0 border-b-2 bg-transparent px-3.5 py-2 text-xs font-medium transition-all {activeView ===
				'tree'
					? 'border-[var(--accent)] font-semibold text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<TreeStructure size={14} />
				<span>Workloads</span>
			</button>

			<button
				type="button"
				onclick={() => setView('system')}
				class="flex cursor-pointer items-center gap-2 border-0 border-b-2 bg-transparent px-3.5 py-2 text-xs font-medium transition-all {activeView ===
				'system'
					? 'border-[var(--accent)] font-semibold text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<Stack size={14} />
				<span>System Daemons</span>
			</button>

			<button
				type="button"
				onclick={() => setView('storage')}
				class="flex cursor-pointer items-center gap-2 border-0 border-b-2 bg-transparent px-3.5 py-2 text-xs font-medium transition-all {activeView ===
				'storage'
					? 'border-[var(--accent)] font-semibold text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<HardDrive size={14} />
				<span>Storage & Prune</span>
			</button>

			<button
				type="button"
				onclick={() => setView('charts')}
				class="flex cursor-pointer items-center gap-2 border-0 border-b-2 bg-transparent px-3.5 py-2 text-xs font-medium transition-all {activeView ===
				'charts'
					? 'border-[var(--accent)] font-semibold text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<ChartLineUp size={14} />
				<span>Telemetry</span>
			</button>
		</div>

		<!-- Live Telemetry Controls (Interval Selector + Pulse + Manual Trigger) -->
		<div class="flex items-center gap-2 pb-2">
			<div
				class="flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-xs"
			>
				<span
					class="h-2 w-2 rounded-full {refreshRate === 'paused'
						? 'bg-[var(--status-amber)]'
						: 'bg-[var(--status-green)]'}"
				></span>
				<span class="font-mono text-[11px] text-[var(--text-secondary)]">
					{refreshRate === 'paused' ? 'Paused' : dataStore.isStreaming ? 'Stream (SSE)' : 'Live'}
				</span>
				<div class="w-[84px]">
					<Select
						value={refreshRate}
						onValueChange={(val: any) => {
							if (val) handleRefreshRateChange(val);
						}}
						options={[
							{ label: 'Live', value: '3s' },
							{ label: 'Paused', value: 'paused' }
						]}
						size="xs"
					/>
				</div>
			</div>

			{#if refreshRate !== 'paused'}
				<span
					class="hidden font-mono text-[10.5px] text-[var(--text-tertiary)] tabular-nums sm:inline"
				>
					{lastUpdatedSec}s ago
				</span>
			{/if}

			<button
				type="button"
				onclick={handleRefresh}
				class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-1.5 text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Refresh telemetry"
				aria-label="Refresh telemetry"
			>
				<ArrowClockwise size={13} class={isRefreshing ? 'animate-spin' : ''} />
			</button>
		</div>
	</div>

	<!-- 3. Active View Content -->
	{#if activeView === 'tree'}
		<!-- Advanced Hierarchical Task Manager with Quick Triage Deck -->
		<ProcessTaskManager onOpenTerminal={handleOpenTerminal} initialViewMode="tree" />
	{:else if activeView === 'system'}
		<ProcessTaskManager onOpenTerminal={handleOpenTerminal} initialViewMode="system" />
	{:else if activeView === 'storage'}
		<PodmanStorageManager />
	{:else}
		<!-- Live session time-series area charts with scannable summary markers -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
			<!-- 1. CPU Utilization -->
			<LayerCard class="flex flex-col gap-3 p-5">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-tertiary)]">CPU utilization</span>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
						>
							{server.cpuUsage.toFixed(1)}%
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>of {server.vcpu} vCPUs</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span
							>Max (session): <strong class="font-medium text-[var(--text-secondary)]"
								>{maxCpuSession.toFixed(1)}%</strong
							></span
						>
						<span>•</span>
						<span>Session buffer</span>
					</div>
				</div>
				<AreaChart
					data={cpuHistory}
					height={210}
					strokeColor="var(--accent)"
					fillColor="var(--accent)"
				/>
			</LayerCard>

			<!-- 2. Memory (RAM) -->
			<LayerCard class="flex flex-col gap-3 p-5">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-tertiary)]"
							>Memory allocation (RAM)</span
						>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
						>
							{server.memoryUsed.toFixed(1)} / {server.memory} GB
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>({((server.memoryUsed / server.memory) * 100).toFixed(0)}%)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span
							>Max (session): <strong class="font-medium text-[var(--text-secondary)]"
								>{maxMemorySession.toFixed(1)} GB</strong
							></span
						>
						<span>•</span>
						<span>Session buffer</span>
					</div>
				</div>
				<AreaChart
					data={memoryHistory}
					height={210}
					strokeColor="var(--status-green)"
					fillColor="var(--status-green)"
				/>
			</LayerCard>

			<!-- 3. Storage usage -->
			<LayerCard class="flex flex-col gap-3 p-5">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-tertiary)]">Storage usage</span>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
						>
							{server.storageUsed.toFixed(1)} / {server.storage} GB
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>({((server.storageUsed / server.storage) * 100).toFixed(0)}%)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span>Session buffer</span>
					</div>
				</div>
				<AreaChart
					data={storageHistory}
					height={210}
					strokeColor="var(--status-amber)"
					fillColor="var(--status-amber)"
				/>
			</LayerCard>

			<!-- 4. Network throughput -->
			<LayerCard class="flex flex-col gap-3 p-5">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-tertiary)]">Network throughput</span>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-semibold text-[var(--text-primary)] tabular-nums"
						>
							{formatRateBps(dataStore.lastNetworkRateBps)}
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>(all containers, aggregate)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span>Session buffer</span>
					</div>
				</div>
				<AreaChart
					data={networkHistory}
					height={210}
					strokeColor="var(--status-blue)"
					fillColor="var(--status-blue)"
				/>
			</LayerCard>
		</div>
	{/if}

	{#if terminalTarget}
		<ContainerTerminalModal
			containerName={terminalTarget}
			onclose={() => (terminalTarget = null)}
		/>
	{/if}
</div>
