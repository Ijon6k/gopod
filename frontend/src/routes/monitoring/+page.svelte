<script lang="ts">
	import { PageHeader, AreaChart } from '$lib/components/ui';
	import { server, monitoringData } from '$lib/data';
	import { ServerOverviewBar, ProcessTaskManager } from '$lib/components/features/monitoring';
	import { TreeStructure, ChartLineUp } from 'phosphor-svelte';

	let activeView = $state<'tree' | 'charts'>('tree');

	function handleOpenTerminal(containerName: string) {
		// Quick jump or modal
		console.log('Open terminal for', containerName);
	}
</script>

<svelte:head>
	<title>Monitoring & Task Manager — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-6">
	<!-- Page Header -->
	<PageHeader
		title="Monitoring & Task Manager"
		subtitle="Real-time process telemetry, hierarchical container resource tree, and host hardware metrics."
	/>

	<!-- 1. Host Hardware Overview Bar -->
	<ServerOverviewBar />

	<!-- 2. Subnav: Task Manager vs Historical Metrics Charts -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-[var(--border)] pb-0">
		<div class="flex items-center gap-1">
			<button
				type="button"
				onclick={() => (activeView = 'tree')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView === 'tree'
					? 'border-[var(--accent)] text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<TreeStructure size={14} />
				<span>Process Tree (Task Manager)</span>
			</button>

			<button
				type="button"
				onclick={() => (activeView = 'charts')}
				class="flex items-center gap-2 px-3.5 py-2 text-xs font-medium border-b-2 transition-all cursor-pointer bg-transparent border-0 {activeView === 'charts'
					? 'border-[var(--accent)] text-[var(--text-primary)]'
					: 'border-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<ChartLineUp size={14} />
				<span>Historical Telemetry</span>
			</button>
		</div>
	</div>

	<!-- 3. Active View Content -->
	{#if activeView === 'tree'}
		<!-- Advanced Hierarchical Task Manager (Easypanel & Dokploy Style) -->
		<ProcessTaskManager onOpenTerminal={handleOpenTerminal} />
	{:else}
		<!-- Historical Time Series Area Charts — All 4 Metrics Displayed Simultaneously -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
			<!-- 1. CPU Utilization -->
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-secondary)]">CPU Utilization</span>
						<strong class="text-lg font-medium tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5">
							{server.cpuUsage.toFixed(1)}% <span class="text-xs font-normal text-[var(--text-tertiary)]">of {server.vcpu} vCPUs</span>
						</strong>
					</div>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">24h telemetry</span>
				</div>
				<AreaChart
					data={monitoringData.cpu ?? []}
					height={210}
					strokeColor="#6973A8"
					fillColor="#6973A8"
				/>
			</div>

			<!-- 2. Memory (RAM) -->
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-secondary)]">Memory (RAM)</span>
						<strong class="text-lg font-medium tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5">
							{server.memoryUsed.toFixed(1)} / {server.memory} GB <span class="text-xs font-normal text-[var(--text-tertiary)]">({((server.memoryUsed / server.memory) * 100).toFixed(0)}%)</span>
						</strong>
					</div>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">24h telemetry</span>
				</div>
				<AreaChart
					data={monitoringData.memory ?? []}
					height={210}
					strokeColor="#4C9A72"
					fillColor="#4C9A72"
				/>
			</div>

			<!-- 3. Storage I/O -->
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-secondary)]">Storage I/O</span>
						<strong class="text-lg font-medium tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5">
							{server.storageUsed.toFixed(1)} / {server.storage} GB <span class="text-xs font-normal text-[var(--text-tertiary)]">({((server.storageUsed / server.storage) * 100).toFixed(0)}%)</span>
						</strong>
					</div>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">24h telemetry</span>
				</div>
				<AreaChart
					data={monitoringData.storage ?? []}
					height={210}
					strokeColor="#B8893B"
					fillColor="#B8893B"
				/>
			</div>

			<!-- 4. Network Traffic -->
			<div class="bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] p-5 flex flex-col gap-3">
				<div class="flex items-baseline justify-between">
					<div class="flex flex-col">
						<span class="text-xs font-medium text-[var(--text-secondary)]">Network Traffic</span>
						<strong class="text-lg font-medium tracking-tight text-[var(--text-primary)] font-[var(--font-mono)] mt-0.5">
							Inbound / Outbound <span class="text-xs font-normal text-[var(--text-tertiary)]">(Aggregate)</span>
						</strong>
					</div>
					<span class="text-[11px] font-mono text-[var(--text-tertiary)]">24h telemetry</span>
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
</div>
