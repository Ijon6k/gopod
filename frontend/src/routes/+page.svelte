<script lang="ts">
	import { PageHeader } from '$lib/components/ui';
	import { server, deployments, projects, monitoringData } from '$lib/data';
	import ServerSurface from '$lib/components/features/home/ServerSurface.svelte';
	import ResourceUsage from '$lib/components/features/home/ResourceUsage.svelte';
	import AttentionPanel from '$lib/components/features/home/AttentionPanel.svelte';
	import ActivityList from '$lib/components/features/home/ActivityList.svelte';
	import ProjectList from '$lib/components/features/home/ProjectList.svelte';

	let failed = $derived(deployments.filter((d) => d.status === 'failed'));

	const chartMeta: Record<string, { current: string }> = {
		cpu: { current: `${server.cpuUsage.toFixed(1)}%` },
		memory: { current: `${server.memoryUsed.toFixed(1)} / ${server.memory} GB` },
		storage: { current: `${server.storageUsed.toFixed(1)} / ${server.storage} GB` }
	};
</script>

<svelte:head>
	<title>Home — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-8">
	<PageHeader title="Home" subtitle="Your infrastructure at a glance." />

	<!-- 1. Server surface -->
	<ServerSurface {server} />

	<!-- 2. Resource usage + Attention -->
	<div class="grid gap-7 grid-cols-[minmax(0,1.65fr)_minmax(300px,1fr)] max-[1000px]:grid-cols-1">
		<ResourceUsage data={monitoringData} {chartMeta} />
		<AttentionPanel {failed} />
	</div>

	<!-- 3. Recent activity + Projects -->
	<div class="grid gap-7 grid-cols-[minmax(0,1.55fr)_minmax(320px,1fr)] max-[1000px]:grid-cols-1">
		<ActivityList {deployments} />
		<ProjectList {projects} />
	</div>
</div>
