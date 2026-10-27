<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import type { SystemDiskUsage, SystemInfoResponse } from '$lib/api/system/types';
	import {
		Button,
		LayerCard,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		Badge,
		Empty,
		ConfirmDialog
	} from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Cpu, Broom, ArrowClockwise, HardDrives, CircleNotch } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let info = $state<SystemInfoResponse | null>(null);
	let dfList = $state<SystemDiskUsage[]>([]);
	let isLoading = $state(true);
	let isRefreshing = $state(false);
	let isPruning = $state(false);
	let isReloadingQuadlets = $state(false);
	let showPruneConfirm = $state(false);

	async function loadEngineData() {
		// The first load renders skeletons (isLoading); every later call shows
		// a busy refresh button instead of tearing down the table.
		if (!isLoading) isRefreshing = true;
		try {
			const [sysInfo, dfRes] = await Promise.all([
				api.system.info().catch(() => null),
				api.system.df().catch(() => [])
			]);
			if (sysInfo) info = sysInfo;
			dfList = dfRes;
		} catch (err) {
			console.error('Failed to load engine data:', err);
		} finally {
			isLoading = false;
			isRefreshing = false;
		}
	}

	onMount(() => {
		void loadEngineData();
	});

	async function handlePrune() {
		isPruning = true;
		try {
			const res = await api.system.prune();
			toast.add({
				title: 'System pruned',
				description: res.message || 'Dangling images and unused containers cleaned.',
				variant: 'success'
			});
			await loadEngineData();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Prune operation failed';
			toast.add(errorToastOptions('Prune failed', msg));
		} finally {
			isPruning = false;
			showPruneConfirm = false;
		}
	}

	async function handleReloadQuadlets() {
		isReloadingQuadlets = true;
		try {
			await api.system.reloadQuadlets();
			toast.add({
				title: 'Daemon reloaded',
				description: 'systemd user generator and Quadlet units reloaded successfully.',
				variant: 'success'
			});
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Quadlet reload failed';
			toast.add(errorToastOptions('Reload failed', msg));
		} finally {
			isReloadingQuadlets = false;
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- Podman Engine Telemetry Card -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
			<div class="flex items-center gap-2">
				<Cpu size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Podman engine telemetry
				</h3>
			</div>
			<div class="flex items-center gap-2">
				{#if info?.rootless ?? true}
					<Badge variant="success">Rootless socket</Badge>
				{:else}
					<Badge variant="warning">Root socket</Badge>
				{/if}
				<Badge variant="neutral">Cgroups {info?.cgroupVersion || 'v2'}</Badge>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
			<div
				class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3"
			>
				<span class="text-[11px] text-[var(--text-tertiary)]">Engine version</span>
				<span class="font-mono text-sm font-semibold text-[var(--text-primary)]">
					{info?.podmanVersion || 'v5.x'}
				</span>
			</div>
			<div
				class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3"
			>
				<span class="text-[11px] text-[var(--text-tertiary)]">Running containers</span>
				<span class="font-mono text-sm font-semibold text-[var(--status-green)]">
					{info?.runningCount ?? 0}
				</span>
			</div>
			<div
				class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3"
			>
				<span class="text-[11px] text-[var(--text-tertiary)]">Stopped containers</span>
				<span class="font-mono text-sm font-semibold text-[var(--text-secondary)]">
					{info?.stoppedCount ?? 0}
				</span>
			</div>
			<div
				class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3"
			>
				<span class="text-[11px] text-[var(--text-tertiary)]">Total workloads</span>
				<span class="font-mono text-sm font-semibold text-[var(--text-primary)]">
					{info?.totalCount ?? 0}
				</span>
			</div>
		</div>
	</LayerCard>

	<!-- Storage Breakdown (df) Table Card -->
	<LayerCard class="flex flex-col gap-0 overflow-hidden p-0">
		<div class="flex items-center justify-between border-b border-[var(--border-subtle)] px-5 py-4">
			<div class="flex items-center gap-2">
				<HardDrives size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
					Container storage usage breakdown
				</h3>
			</div>
			<Button
				variant="ghost"
				size="xs"
				onclick={loadEngineData}
				disabled={isRefreshing}
				aria-label="Refresh storage metrics"
			>
				{#if isRefreshing}
					<CircleNotch size={13} class="mr-1 animate-spin" /> Refreshing...
				{:else}
					<ArrowClockwise size={13} class="mr-1" /> Refresh metrics
				{/if}
			</Button>
		</div>

		{#if isLoading}
			<div class="flex flex-col gap-2 p-5">
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
				<div class="h-9 animate-pulse rounded-[var(--radius-sm)] bg-[var(--bg-surface)]"></div>
			</div>
		{:else if dfList.length === 0}
			<div class="p-8">
				<Empty
					size="sm"
					title="Storage metrics unavailable"
					description="Unable to query Podman storage disk usage at this time."
				/>
			</div>
		{:else}
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full border-collapse text-left text-xs">
					<TableHeader
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<TableRow>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Resource type
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Total items
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Active items
							</TableHead>
							<TableHead class="px-4 py-3 text-[12px] font-semibold text-[var(--text-tertiary)]">
								Disk size
							</TableHead>
							<TableHead
								class="px-4 py-3 text-right text-[12px] font-semibold text-[var(--text-tertiary)]"
							>
								Reclaimable
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#each dfList as item (item.type)}
							<TableRow class="hover:bg-[var(--bg-table-row-alt)]">
								<TableCell class="px-4 py-3 font-medium text-[var(--text-primary)] capitalize">
									{item.type}
								</TableCell>
								<TableCell class="px-4 py-3 font-mono text-[var(--text-secondary)]">
									{item.total}
								</TableCell>
								<TableCell class="px-4 py-3 font-mono text-[var(--text-secondary)]">
									{item.active}
								</TableCell>
								<TableCell class="px-4 py-3 font-mono text-[var(--text-primary)]">
									{item.size}
								</TableCell>
								<TableCell class="px-4 py-3 text-right font-mono text-[var(--status-amber)]">
									{item.reclaimable}
								</TableCell>
							</TableRow>
						{/each}
					</TableBody>
				</Table>
			</div>
		{/if}
	</LayerCard>

	<!-- Maintenance Operations Card -->
	<LayerCard class="flex flex-col gap-4 p-5">
		<div class="border-b border-[var(--border-subtle)] pb-3">
			<h3 class="m-0 text-xs font-semibold text-[var(--text-primary)]">
				Maintenance & system operations
			</h3>
			<p class="m-0 mt-0.5 text-[11px] text-[var(--text-tertiary)]">
				Perform garbage collection and reload declarative unit generators.
			</p>
		</div>

		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">System prune</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Cleans dangling images, stopped containers, and unused build caches from local storage.
				</span>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={isPruning}
				onclick={() => (showPruneConfirm = true)}
			>
				{#if isPruning}
					<CircleNotch size={13} class="animate-spin" /> Pruning...
				{:else}
					<Broom size={13} /> Prune unused resources
				{/if}
			</Button>
		</div>

		<div
			class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-3 sm:flex-row sm:items-center sm:justify-between"
		>
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">Reload Quadlet units</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Executes <code class="font-mono text-[var(--text-primary)]"
						>systemctl --user daemon-reload</code
					> to resynchronize systemd Quadlets.
				</span>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={isReloadingQuadlets}
				onclick={handleReloadQuadlets}
			>
				{#if isReloadingQuadlets}
					<CircleNotch size={13} class="animate-spin" /> Reloading...
				{:else}
					<ArrowClockwise size={13} /> Reload systemd daemon
				{/if}
			</Button>
		</div>
	</LayerCard>
</div>

<!-- Confirm Dialog for System Prune -->
<ConfirmDialog
	open={showPruneConfirm}
	title="Prune container resources"
	description="This operation will remove all stopped containers, unused networks, and dangling image layers. Active volumes and running containers are not modified."
	confirmText="Prune now"
	variant="danger"
	onconfirm={handlePrune}
	oncancel={() => (showPruneConfirm = false)}
/>
