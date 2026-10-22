<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button } from 'kumo-svelte';
	import { PageHeader, SkeletonCard } from '$lib/components/ui';
	import {
		server,
		deployments,
		projects,
		containers,
		cpuHistory,
		memoryHistory,
		storageHistory
	} from '$lib/data';
	import StatusStrip from '$lib/components/features/home/StatusStrip.svelte';
	import NeedsAttention from '$lib/components/features/home/NeedsAttention.svelte';
	import ResourceUsage from '$lib/components/features/home/ResourceUsage.svelte';
	import HostFacts from '$lib/components/features/home/HostFacts.svelte';
	import RecentDeployments from '$lib/components/features/home/RecentDeployments.svelte';
	import ProjectList from '$lib/components/features/home/ProjectList.svelte';

	let booted = $derived(Boolean(server?.hostname));
	let failed = $derived(deployments.filter((d) => d.status === 'failed'));
	let inFlight = $derived(
		deployments.filter((d) => d.status === 'building' || d.status === 'deploying').length
	);

	// Real join from the runtime store — nothing invented, nothing hidden.
	let unhealthyNames = $derived(containers.filter((c) => c.health === 'unhealthy').map((c) => c.name));
	let restartingNames = $derived(containers.filter((c) => c.health === 'restarting').map((c) => c.name));

	// Only the tranche ResourceUsage draws; a plain object is rebuilt only when
	// one of the three underlying histories actually changed.
	const usageSeries = $derived({ cpu: cpuHistory, memory: memoryHistory, storage: storageHistory });

	// Derived (a plain const went stale during the session) — three small
	// strings recompute per sample, nothing else re-renders.
	let chartMeta = $derived({
		cpu: { current: `${server.cpuUsage.toFixed(1)}%` },
		memory: { current: `${server.memoryUsed.toFixed(1)} / ${server.memory} GB` },
		storage: { current: `${server.storageUsed.toFixed(1)} / ${server.storage} GB` }
	});
</script>

<svelte:head>
	<title>Overview — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader title="Overview">
		{#snippet meta()}
			<span class="text-xs text-[var(--text-tertiary)]">
				{#if typeof server.containersRunning === 'number'}
					{server.containersRunning} containers running
				{/if}
				{#if inFlight > 0}
					{#if typeof server.containersRunning === 'number'} · {/if}
					{inFlight} {inFlight === 1 ? 'deployment' : 'deployments'} in flight
				{/if}
			</span>
		{/snippet}
		{#snippet actions()}
			<Button variant="primary" size="sm" onclick={() => goto('/projects/new')}>New project</Button>
		{/snippet}
	</PageHeader>

	{#if booted}
		<StatusStrip {server} />

		{#if failed.length + unhealthyNames.length + restartingNames.length > 0}
			<NeedsAttention {failed} unhealthy={unhealthyNames} restarting={restartingNames} />
		{/if}

		<div class="grid grid-cols-12 gap-6">
			<ResourceUsage data={usageSeries} {chartMeta} class="col-span-12 lg:col-span-8" />
			<HostFacts {server} class="col-span-12 lg:col-span-4" />
		</div>

		<div class="grid grid-cols-12 gap-6">
			<RecentDeployments {deployments} class="col-span-12 lg:col-span-7" />
			<ProjectList {projects} class="col-span-12 lg:col-span-5" />
		</div>
	{:else}
		<!-- Boot skeleton: honest waiting state instead of zero-value panes -->
		<div class="flex flex-col gap-6">
			<div class="h-[104px]" aria-hidden="true"></div>
			<SkeletonCard lines={4} class="min-h-[190px]" />
			<div class="grid grid-cols-12 gap-6 max-sm:grid-cols-1">
				<SkeletonCard lines={5} class="col-span-12 min-h-[290px] md:col-span-8" />
				<SkeletonCard lines={4} class="col-span-12 min-h-[290px] md:col-span-4" />
			</div>
		</div>
	{/if}
</div>
