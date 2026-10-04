<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, AreaChart } from '$lib/components/ui';
	import { server, monitoringData, dataStore } from '$lib/data';
	import { ServerOverviewBar, ProcessTaskManager, PodmanStorageManager, ContainerTerminalModal } from '$lib/components/features/monitoring';
	import { TreeStructure, Stack, HardDrive, ChartLineUp, ArrowClockwise } from 'phosphor-svelte';

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
		// When monitoring page is opened, start real-time SSE stream
		if (refreshRate !== 'paused') {
			dataStore.startStreamingStats();
		}
		dataStore.fetchLiveStats();

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

<div class="w-full flex flex-col gap-6">
	<!-- Page Header -->
	<PageHeader
		title="Monitoring & Telemetry"
		subtitle="Real-time resource utilization, container workloads, and system telemetry."
	/>

	<!-- 1. Host Hardware Overview Bar -->
	<ServerOverviewBar activeTab={activeView} onSelectTab={(tab) => setView(tab)} />

	<!-- 2. Subnav: Workloads vs System Daemons vs Storage & Prune vs Historical Telemetry -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-[var(--border)] pb-0">
		<div class="flex items-center gap-1">
			<button
				type="button"
				onclick={() => setView('tree')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView ===
				'tree'
					? 'border-[var(--accent)] text-[var(--text-primary)] font-semibold'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<TreeStructure size={14} />
				<span>Workloads</span>
			</button>

			<button
				type="button"
				onclick={() => setView('system')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView ===
				'system'
					? 'border-[var(--accent)] text-[var(--text-primary)] font-semibold'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<Stack size={14} />
				<span>System Daemons</span>
			</button>

			<button
				type="button"
				onclick={() => setView('storage')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView ===
				'storage'
					? 'border-[var(--accent)] text-[var(--text-primary)] font-semibold'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<HardDrive size={14} />
				<span>Storage & Prune</span>
			</button>

			<button
				type="button"
				onclick={() => setView('charts')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView ===
				'charts'
					? 'border-[var(--accent)] text-[var(--text-primary)] font-semibold'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<ChartLineUp size={14} />
				<span>Historical Telemetry</span>
			</button>
		</div>

		<!-- Live Telemetry Controls (Interval Selector + Pulse + Manual Trigger) -->
		<div class="flex items-center gap-2 pb-2">
			<div class="flex items-center gap-1.5 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-xs">
				<span class="w-2 h-2 rounded-full {refreshRate === 'paused' ? 'bg-[var(--status-amber)]' : 'bg-[var(--status-green)] animate-pulse'}"></span>
				<span class="text-[11px] font-mono text-[var(--text-secondary)]">
					{refreshRate === 'paused' ? 'Paused' : dataStore.isStreaming ? 'Stream (SSE)' : 'Live'}
				</span>
				<select
					bind:value={refreshRate}
					class="bg-transparent border-0 outline-none text-[11px] font-mono text-[var(--text-tertiary)] hover:text-[var(--text-primary)] cursor-pointer ml-0.5"
					aria-label="Refresh interval"
				>
					<option value="3s">3s</option>
					<option value="5s">5s</option>
					<option value="15s">15s</option>
					<option value="paused">Pause</option>
				</select>
			</div>

			{#if refreshRate !== 'paused'}
				<span class="text-[10.5px] font-mono text-[var(--text-tertiary)] hidden sm:inline tabular-nums">
					{lastUpdatedSec}s ago
				</span>
			{/if}

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
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
			<!-- 1. CPU Utilization -->
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3 shadow-xs">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">CPU Utilization</span>
						<strong class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5 tabular-nums">
							{server.cpuUsage.toFixed(1)}% <span class="text-xs font-normal text-[var(--text-tertiary)]">of {server.vcpu} vCPUs</span>
						</strong>
					</div>
					<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
						<span>Peak: <strong class="text-[var(--text-secondary)]">{(server.cpuUsage * 1.5).toFixed(1)}%</strong></span>
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
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3 shadow-xs">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">Memory Allocation (RAM)</span>
						<strong class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5 tabular-nums">
							{server.memoryUsed.toFixed(1)} / {server.memory} GB <span class="text-xs font-normal text-[var(--text-tertiary)]">({((server.memoryUsed / server.memory) * 100).toFixed(0)}%)</span>
						</strong>
					</div>
					<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
						<span>Peak: <strong class="text-[var(--text-secondary)]">{(server.memoryUsed * 1.15).toFixed(1)} GB</strong></span>
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
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3 shadow-xs">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">Storage I/O Operations</span>
						<strong class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5 tabular-nums">
							{server.storageUsed.toFixed(1)} / {server.storage} GB <span class="text-xs font-normal text-[var(--text-tertiary)]">({((server.storageUsed / server.storage) * 100).toFixed(0)}%)</span>
						</strong>
					</div>
					<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
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
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3 shadow-xs">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-semibold text-[var(--text-primary)]">Network Traffic</span>
						<strong class="text-lg font-bold tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5 tabular-nums">
							Inbound / Outbound <span class="text-xs font-normal text-[var(--text-tertiary)]">(Aggregate)</span>
						</strong>
					</div>
					<div class="flex items-center gap-2 text-[11px] font-mono text-[var(--text-tertiary)]">
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
