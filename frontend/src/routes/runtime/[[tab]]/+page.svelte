<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ConfirmDialog, PageHeader, Tabs } from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/stores/data.svelte';
	import { engineLabel } from '$lib/utils/engine';
	import ContainersTab from '$lib/components/features/runtime/ContainersTab.svelte';
	import QuadletsTab from '$lib/components/features/runtime/QuadletsTab.svelte';
	import PodsTab from '$lib/components/features/runtime/PodsTab.svelte';
	import ImagesTab from '$lib/components/features/runtime/ImagesTab.svelte';
	import VolumesTab from '$lib/components/features/runtime/VolumesTab.svelte';
	import NetworksTab from '$lib/components/features/runtime/NetworksTab.svelte';
	import ContainerLogsModal from '$lib/components/features/runtime/ContainerLogsModal.svelte';
	import QuadletSourceModal from '$lib/components/features/runtime/QuadletSourceModal.svelte';
	import { RuntimeRowActions } from '$lib/components/features/runtime/runtime-actions.svelte';
	import type { QuadletUnit, Container } from '$lib/types';

	const toast = useKumoToastManager();
	let actions = new RuntimeRowActions(toast);

	let activeTab = $derived(page.params.tab ?? 'containers');
	let engineName = $derived(engineLabel(dataStore.server));

	// Layout boot fills the arrays once; here only correct staleness beyond the
	// TTL window. Live per-container samples then arrive from the shared
	// refcounted SSE stream (also owned by monitoring while it is open).
	$effect(() => {
		dataStore.fetchRuntimeData(15_000);
	});
	$effect(() => {
		dataStore.startStreamingStats();
		return () => dataStore.stopStreamingStats();
	});

	let containers = $derived(dataStore.containers);
	let pods = $derived(dataStore.pods);
	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let networks = $derived(dataStore.networks);
	let quadlets = $derived(dataStore.quadlets);
	let isRuntimeLoading = $derived(dataStore.isRuntimeLoading);

	let viewingQuadlet = $state<QuadletUnit | null>(null);
	let logsContainer = $state<Container | null>(null);

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
</script>

<svelte:head>
	<title>Runtime — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title={engineName}
		subtitle="Containers, pods, images, volumes, and networks, straight from the host engine."
	/>
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	{#if activeTab === 'containers'}
		<ContainersTab rows={containers} loading={isRuntimeLoading} {actions} onViewLogs={(c) => (logsContainer = c)} />
	{:else if activeTab === 'quadlets'}
		<QuadletsTab
			rows={quadlets}
			loading={isRuntimeLoading}
			{actions}
			onViewSource={(unit) => (viewingQuadlet = unit)}
		/>
	{:else if activeTab === 'pods'}
		<PodsTab rows={pods} loading={isRuntimeLoading} {actions} />
	{:else if activeTab === 'images'}
		<ImagesTab rows={images} loading={isRuntimeLoading} {actions} />
	{:else if activeTab === 'volumes'}
		<VolumesTab rows={volumes} loading={isRuntimeLoading} {actions} />
	{:else if activeTab === 'networks'}
		<NetworksTab rows={networks} loading={isRuntimeLoading} {actions} />
	{/if}
</div>

<ContainerLogsModal container={logsContainer} onclose={() => (logsContainer = null)} />
<QuadletSourceModal unit={viewingQuadlet} onclose={() => (viewingQuadlet = null)} />

<ConfirmDialog
	open={!!actions.requestedConfirm}
	title={actions.requestedConfirm?.title ?? ''}
	description={actions.requestedConfirm?.description}
	variant="danger"
	confirmText="Delete"
	isConfirming={actions.isConfirmBusy}
	matchValue={actions.requestedConfirm?.matchValue}
	matchLabel={actions.requestedConfirm?.matchLabel}
	oncancel={() => actions.cancelConfirm()}
	onconfirm={async () => {
		if (!actions.requestedConfirm) return;
		actions.isConfirmBusy = true;
		try {
			await actions.requestedConfirm.run();
			actions.cancelConfirm();
		} finally {
			actions.isConfirmBusy = false;
		}
	}}
/>
