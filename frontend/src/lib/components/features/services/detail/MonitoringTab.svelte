<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { AreaChart, StatusBadge } from '$lib/components/ui';
	import { monitoringData } from '$lib/data';
	import { MicroWorkloadTable } from '$lib/components/features/monitoring';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let isPod = $derived(service.type === 'pod' || (service.workloads && service.workloads.length > 1));
	let workloads = $derived<Workload[]>(service.workloads ?? [{ name: service.name, image: service.image ?? '—', status: service.status }]);

	let selectedContainer = $state('all');
	let timeRange = $state<'1h' | '6h' | '24h' | '7d'>('24h');

	// Compute metrics based on selected container
	let currentWorkload = $derived(
		selectedContainer === 'all'
			? null
			: workloads.find((w) => w.name === selectedContainer)
	);

	let currentCpu = $derived(
		currentWorkload?.cpu ?? service.cpu ?? 1.2
	);

	let currentMem = $derived(
		currentWorkload?.memory ?? service.memory ?? 180
	);

	function formatMemory(mb: number): string {
		return mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
	}
</script>

<div class="w-full flex flex-col gap-6">
	<!-- Filter bar: Container selector (for Pod) + Time Range -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-[var(--border-subtle)]">
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
				<span class="text-xs font-[var(--font-mono)] text-[var(--text-primary)]">{service.name}</span>
			</div>
		{/if}

		<!-- Time range selector -->
		<div class="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
			{#each ['1h', '6h', '24h', '7d'] as r}
				<button
					type="button"
					onclick={() => (timeRange = r as any)}
					class="px-2.5 py-1 rounded text-xs transition-colors cursor-pointer border-0 {timeRange === r
						? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					{r}
				</button>
			{/each}
		</div>
	</div>

	<!-- Current Metrics Summary Bar -->
	<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]">
		<div class="flex flex-col gap-1">
			<span class="text-[10px] uppercase font-medium text-[var(--text-tertiary)] tracking-[0.08em]">CPU Usage</span>
			<strong class="text-lg font-medium text-[var(--text-primary)] tabular-nums">{currentCpu.toFixed(1)}%</strong>
		</div>
		<div class="flex flex-col gap-1">
			<span class="text-[10px] uppercase font-medium text-[var(--text-tertiary)] tracking-[0.08em]">Memory Usage</span>
			<strong class="text-lg font-medium text-[var(--text-primary)] tabular-nums">{formatMemory(currentMem)}</strong>
		</div>
		<div class="flex flex-col gap-1">
			<span class="text-[10px] uppercase font-medium text-[var(--text-tertiary)] tracking-[0.08em]">Restarts</span>
			<strong class="text-lg font-medium text-[var(--text-primary)] tabular-nums">0</strong>
		</div>
		<div class="flex flex-col gap-1">
			<span class="text-[10px] uppercase font-medium text-[var(--text-tertiary)] tracking-[0.08em]">Health Check</span>
			<div>
				<StatusBadge status={service.health === 'unhealthy' ? 'unhealthy' : 'healthy'} size="sm" />
			</div>
		</div>
	</div>

	<!-- 2x2 Metric Charts Grid -->
	<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
		<!-- CPU Chart -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">CPU Utilization</span>
				<span class="text-xs font-[var(--font-mono)] text-[var(--text-primary)]">{currentCpu.toFixed(1)}%</span>
			</div>
			<AreaChart data={monitoringData.cpu} height={140} />
		</div>

		<!-- Memory Chart -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Memory Utilization</span>
				<span class="text-xs font-[var(--font-mono)] text-[var(--text-primary)]">{formatMemory(currentMem)}</span>
			</div>
			<AreaChart data={monitoringData.memory} height={140} strokeColor="#4C9A72" fillColor="#4C9A72" />
		</div>

		<!-- Storage Chart -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Disk I/O</span>
				<span class="text-xs font-[var(--font-mono)] text-[var(--text-primary)]">30.6 MB/s</span>
			</div>
			<AreaChart data={monitoringData.storage} height={140} strokeColor="#B8893B" fillColor="#B8893B" />
		</div>

		<!-- Network Chart -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Network Traffic</span>
				<span class="text-xs font-[var(--font-mono)] text-[var(--text-primary)]">12.4 MB/s</span>
			</div>
			<AreaChart data={monitoringData.network} height={140} strokeColor="#7680B5" fillColor="#7680B5" />
		</div>
	</div>

	<!-- Micro-Workloads & Container Cgroups Breakdown -->
	<MicroWorkloadTable {service} />
</div>
