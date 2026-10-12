<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, DataTable, StatusBadge, Modal, CodeEditor } from '$lib/components/ui';
	import { Button, Badge } from 'kumo-svelte';
	import { Play, Stop, ArrowClockwise, FileCode } from 'phosphor-svelte';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { QuadletUnit } from '$lib/types';

	let activeTab = $derived(page.params.tab ?? 'containers');

	let containers = $derived(dataStore.containers);
	let pods = $derived(dataStore.pods);
	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let networks = $derived(dataStore.networks);
	let quadlets = $derived(dataStore.quadlets);

	let viewingQuadlet = $state<QuadletUnit | null>(null);
	let isReloading = $state(false);

	onMount(() => {
		dataStore.fetchRuntimeData();
		dataStore.fetchLiveStats();
	});

	const tabs = [
		{ id: 'containers', label: 'Containers' },
		{ id: 'quadlets', label: 'Quadlets' },
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

<div class="flex w-full flex-col gap-6">
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
			pageSize={10}
			itemName="containers"
		/>
	{:else if activeTab === 'quadlets'}
		<div class="flex flex-col gap-4">
			<div class="flex items-center justify-between">
				<p class="m-0 text-xs text-[var(--text-secondary)]">
					Systemd Quadlet units installed in declarative search paths (<code class="font-mono"
						>~/.config/containers/systemd/</code
					>).
				</p>
				<Button
					variant="secondary"
					size="sm"
					disabled={isReloading}
					onclick={async () => {
						isReloading = true;
						try {
							await dataStore.reloadQuadlets();
						} finally {
							isReloading = false;
						}
					}}
				>
					<ArrowClockwise size={14} class={isReloading ? 'animate-spin' : ''} />
					<span>Daemon Reload</span>
				</Button>
			</div>

			<DataTable
				columns={[
					{ key: 'name', label: 'Unit File', mono: true },
					{ key: 'type', label: 'Type', render: quadletTypeSnippet },
					{ key: 'unitName', label: 'Service Unit', mono: true },
					{ key: 'status', label: 'Status', render: quadletStatusSnippet },
					{ key: 'path', label: 'Path', mono: true }
				]}
				rows={quadlets}
				keyExtractor={(q) => q.name}
				actions={quadletActionsSnippet}
				pageSize={10}
				itemName="quadlets"
			/>
		</div>
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
			pageSize={10}
			itemName="pods"
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
			pageSize={10}
			itemName="images"
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
			pageSize={10}
			itemName="volumes"
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
			pageSize={10}
			itemName="networks"
		/>
	{/if}
</div>

{#snippet containerStatusSnippet(row: (typeof containers)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet containerPortsSnippet(row: (typeof containers)[0])}
	{@const portList = parsePorts(row.ports)}
	{#if portList.length > 0}
		<div class="flex flex-col gap-1 py-0.5">
			{#each portList as port (port)}
				<span
					class="text-base leading-tight font-[var(--font-mono)] whitespace-nowrap text-[var(--text-primary)]"
				>
					{port}
				</span>
			{/each}
		</div>
	{:else}
		<span class="text-base text-[var(--text-tertiary)]">—</span>
	{/if}
{/snippet}
{#snippet podStatusSnippet(row: (typeof pods)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet podContainersSnippet(row: (typeof pods)[0])}
	<span>{row.containers.length}</span>
{/snippet}
{#snippet volumeStatusSnippet(row: (typeof volumes)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}

{#snippet quadletTypeSnippet(row: (typeof quadlets)[0])}
	<Badge variant="secondary" class="font-mono uppercase text-[11px]">{row.type}</Badge>
{/snippet}

{#snippet quadletStatusSnippet(row: (typeof quadlets)[0])}
	<StatusBadge
		status={row.status === 'active' ? 'running' : row.status === 'failed' ? 'failed' : 'stopped'}
		size="md"
	/>
{/snippet}

{#snippet quadletActionsSnippet(row: (typeof quadlets)[0])}
	<div class="flex items-center justify-end gap-1">
		{#if row.status === 'active'}
			<button
				type="button"
				title="Stop Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--status-red)]"
				onclick={() => dataStore.stopQuadlet(row.name)}
			>
				<Stop size={14} />
			</button>
			<button
				type="button"
				title="Restart Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
				onclick={() => dataStore.restartQuadlet(row.name)}
			>
				<ArrowClockwise size={14} />
			</button>
		{:else}
			<button
				type="button"
				title="Start Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--status-green)]"
				onclick={() => dataStore.startQuadlet(row.name)}
			>
				<Play size={14} />
			</button>
		{/if}
		<button
			type="button"
			title="View Unit Source"
			class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
			onclick={() => (viewingQuadlet = row)}
		>
			<FileCode size={14} />
		</button>
	</div>
{/snippet}

<Modal
	open={!!viewingQuadlet}
	onclose={() => (viewingQuadlet = null)}
	title={viewingQuadlet?.name ?? 'Quadlet Unit'}
	subtitle={`Systemd unit source at ${viewingQuadlet?.path ?? ''}`}
	size="lg"
>
	{#if viewingQuadlet}
		<div class="flex flex-col gap-3">
			<CodeEditor
				value={viewingQuadlet.content}
				language="quadlet"
				readOnly={true}
				height="260px"
			/>
			<div class="flex justify-end">
				<Button variant="secondary" onclick={() => (viewingQuadlet = null)}>Close</Button>
			</div>
		</div>
	{/if}
</Modal>
