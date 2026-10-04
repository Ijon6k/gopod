<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, DataTable, StatusBadge } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';

	let tabParam = $derived(page.params.tab ?? 'containers');
	let activeTab = $state('containers');

	let containers = $derived(dataStore.containers);
	let pods = $derived(dataStore.pods);
	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let networks = $derived(dataStore.networks);

	$effect(() => {
		activeTab = tabParam;
	});

	onMount(() => {
		dataStore.fetchRuntimeData();
		dataStore.fetchLiveStats();
	});

	const tabs = [
		{ id: 'containers', label: 'Containers' },
		{ id: 'pods', label: 'Pods' },
		{ id: 'images', label: 'Images' },
		{ id: 'volumes', label: 'Volumes' },
		{ id: 'networks', label: 'Networks' }
	];

	function onTabChange(id: string) {
		goto(`/runtime/${id}`, { replaceState: true });
	}

	function parsePorts(ports: string | undefined | null): string[] {
		if (!ports || ports === '—') return [];
		return ports
			.split(/[\n,]+/)
			.map((p) => p.trim().replaceAll('→', ':').replaceAll('->', ':'))
			.filter(Boolean);
	}
</script>

<svelte:head>
	<title>Runtime — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-6">
	<PageHeader title="Runtime" subtitle="Containers, pods, images, volumes, and networks." />
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	{#if activeTab === 'containers'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name' },
				{ key: 'projectName', label: 'Project' },
				{ key: 'image', label: 'Image', mono: true },
				{ key: 'status', label: 'Status', render: containerStatusSnippet },
				{ key: 'ports', label: 'Ports', render: containerPortsSnippet },
				{ key: 'startedAt', label: 'Started' }
			]}
			rows={containers}
			keyExtractor={(c) => c.id}
		/>
	{:else if activeTab === 'pods'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name' },
				{ key: 'projectName', label: 'Project' },
				{ key: 'status', label: 'Status', render: podStatusSnippet },
				{ key: 'containers', label: 'Containers', render: podContainersSnippet },
				{ key: 'network', label: 'Network' }
			]}
			rows={pods}
			keyExtractor={(p) => p.id}
		/>
	{:else if activeTab === 'images'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Repository', mono: true },
				{ key: 'tag', label: 'Tag', mono: true },
				{ key: 'size', label: 'Size' },
				{ key: 'usedBy', label: 'Used by' },
				{ key: 'createdAt', label: 'Created' }
			]}
			rows={images}
			keyExtractor={(img) => img.id}
		/>
	{:else if activeTab === 'volumes'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name' },
				{ key: 'projectName', label: 'Project' },
				{ key: 'serviceName', label: 'Service' },
				{ key: 'mount', label: 'Mount', mono: true },
				{ key: 'size', label: 'Size' },
				{ key: 'status', label: 'Status', render: volumeStatusSnippet }
			]}
			rows={volumes}
			keyExtractor={(v) => v.id}
		/>
	{:else if activeTab === 'networks'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name' },
				{ key: 'driver', label: 'Driver' },
				{ key: 'containers', label: 'Containers' },
				{ key: 'subnet', label: 'Subnet', mono: true },
				{ key: 'gateway', label: 'Gateway', mono: true }
			]}
			rows={networks}
			keyExtractor={(n) => n.id}
		/>
	{/if}
</div>

{#snippet containerStatusSnippet(row: typeof containers[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet containerPortsSnippet(row: typeof containers[0])}
	{@const portList = parsePorts(row.ports)}
	{#if portList.length > 0}
		<div class="flex flex-col gap-1 py-0.5">
			{#each portList as port}
				<span class="font-[var(--font-mono)] text-base text-[var(--text-primary)] leading-tight whitespace-nowrap">
					{port}
				</span>
			{/each}
		</div>
	{:else}
		<span class="text-[var(--text-tertiary)] text-base">—</span>
	{/if}
{/snippet}
{#snippet podStatusSnippet(row: typeof pods[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet podContainersSnippet(row: typeof pods[0])}
	<span>{row.containers.length}</span>
{/snippet}
{#snippet volumeStatusSnippet(row: typeof volumes[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
