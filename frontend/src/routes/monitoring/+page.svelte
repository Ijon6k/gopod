<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, AreaChart } from '$lib/components/ui';
	import { monitoringData, dataStore } from '$lib/data';
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

	onMount(() => {
		// When monitoring page is opened, ensure complete projects & service topology is loaded
		dataStore.fetchInitialData();

		if (refreshRate !== 'paused') {
			dataStore.startStreamingStats();
		}

		const secTimer = setInterval(() => {
			if (refreshRate !== 'paused') {
				lastUpdatedSec = (lastUpdatedSec + 1) % 60;
			}
		}, 1000);

		return () => {
			// Clean up when user navigates away
			dataStore.stopStreamingStats();
			clearInterval(secTimer);
		};
	});

	$effect(() => {
		if (refreshRate === 'paused') {
			dataStore.stopStreamingStats();
		} else {
			dataStore.startStreamingStats();
		}
	});
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

	<!-- 2. Subnav: Workloads vs System Daemons vs Storage & Prune vs Historical Telemetry -->
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
				<span>Historical Telemetry</span>
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
						: 'animate-pulse bg-[var(--status-green)]'}"
				></span>
				<span class="font-mono text-[11px] text-[var(--text-secondary)]">
					{refreshRate === 'paused' ? 'Paused' : dataStore.isStreaming ? 'Stream (SSE)' : 'Live'}
				</span>
				<select
					bind:value={refreshRate}
					class="ml-0.5 cursor-pointer border-0 bg-transparent font-mono text-[11px] text-[var(--text-tertiary)] outline-none hover:text-[var(--text-primary)]"
					aria-label="Refresh interval"
				>
					<option value="3s">3s</option>
					<option value="5s">5s</option>
					<option value="15s">15s</option>
					<option value="paused">Pause</option>
				</select>
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
		<!-- Historical Time Series Area Charts with Scannable Summary Markers -->
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-2">
			<!-- 1. CPU Utilization -->
			<div
				class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
			>
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">CPU Utilization</span>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-bold tracking-tight text-[var(--text-primary)] tabular-nums"
						>
							{server.cpuUsage.toFixed(1)}%
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>of {server.vcpu} vCPUs</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span
							>Peak: <strong class="text-[var(--text-secondary)]"
								>{(server.cpuUsage * 1.5).toFixed(1)}%</strong
							></span
						>
						<span>•</span>
						<span>24h window</span>
					</div>
				</div>
				<AreaChart
					data={monitoringData.cpu ?? []}
					height={210}
					strokeColor="#6973A8"
					fillColor="#6973A8"
				/>
			</div>

			<!-- 2. Memory (RAM) -->
			<div
				class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
			>
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]"
							>Memory Allocation (RAM)</span
						>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-bold tracking-tight text-[var(--text-primary)] tabular-nums"
						>
							{server.memoryUsed.toFixed(1)} / {server.memory} GB
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>({((server.memoryUsed / server.memory) * 100).toFixed(0)}%)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span
							>Peak: <strong class="text-[var(--text-secondary)]"
								>{(server.memoryUsed * 1.15).toFixed(1)} GB</strong
							></span
						>
						<span>•</span>
						<span>24h window</span>
					</div>
				</div>
				<AreaChart
					data={monitoringData.memory ?? []}
					height={210}
					strokeColor="#4C9A72"
					fillColor="#4C9A72"
				/>
			</div>

			<!-- 3. Storage I/O -->
			<div
				class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
			>
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]"
							>Storage I/O Operations</span
						>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-bold tracking-tight text-[var(--text-primary)] tabular-nums"
						>
							{server.storageUsed.toFixed(1)} / {server.storage} GB
							<span class="text-xs font-normal text-[var(--text-tertiary)]"
								>({((server.storageUsed / server.storage) * 100).toFixed(0)}%)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span>NVMe ext4</span>
						<span>•</span>
						<span>24h window</span>
					</div>
				</div>
				<AreaChart
					data={monitoringData.storage ?? []}
					height={210}
					strokeColor="#B8893B"
					fillColor="#B8893B"
				/>
			</div>

			<!-- 4. Network Traffic -->
			<div
				class="flex flex-col gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 shadow-xs"
			>
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">Network Traffic</span>
						<strong
							class="mt-0.5 text-lg font-[var(--font-mono)] font-bold tracking-tight text-[var(--text-primary)] tabular-nums"
						>
							Inbound / Outbound <span class="text-xs font-normal text-[var(--text-tertiary)]"
								>(Aggregate)</span
							>
						</strong>
					</div>
					<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
						<span>eth0</span>
						<span>•</span>
						<span>24h window</span>
					</div>
				</div>
				<AreaChart
					data={monitoringData.network ?? []}
					height={210}
					strokeColor="#7680B5"
					fillColor="#7680B5"
				/>
			</div>
		</div>
	{/if}

	{#if terminalTarget}
		<ContainerTerminalModal
			containerName={terminalTarget}
			onclose={() => (terminalTarget = null)}
		/>
	{/if}
</div>
