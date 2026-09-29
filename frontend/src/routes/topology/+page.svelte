<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader } from '$lib/components/ui';
	import {
		projects,
		services,
		containers,
		pods,
		domains,
		volumes,
		networks,
		server
	} from '$lib/data';
	import {
		TopologyCanvas,
		TopologyToolbar,
		TopologyInspector,
		computeTopologyGraph,
		type SelectedItem
	} from '$lib/components/features/topology';
	import { onMount } from 'svelte';

	// Query param support (e.g., /topology?project=aerochat)
	let initialProject = page.url.searchParams.get('project');
	let viewMode = $state(
		initialProject && projects.some((p) => p.id === initialProject)
			? initialProject
			: 'global'
	);

	let selectedItem: SelectedItem = $state(null);
	let zoom = $state(1);
	let panX = $state(40);
	let panY = $state(40);

	// Compute graph deterministically whenever inputs or viewMode change
	let graph = $derived(
		computeTopologyGraph({
			projects,
			services,
			containers,
			pods,
			domains,
			volumes,
			networks,
			server,
			viewMode
		})
	);

	function setViewMode(mode: string) {
		viewMode = mode;
		selectedItem = null;
		if (mode === 'global') {
			goto('/topology', { replaceState: true, noScroll: true });
			panX = 40;
			panY = 40;
			zoom = 0.9;
		} else {
			goto(`/topology?project=${mode}`, { replaceState: true, noScroll: true });
			panX = 40;
			panY = 40;
			zoom = 1;
		}
	}

	function handleZoomIn() {
		zoom = Math.min(2.2, Math.round(zoom * 1.15 * 100) / 100);
	}

	function handleZoomOut() {
		zoom = Math.max(0.3, Math.round(zoom * 0.85 * 100) / 100);
	}

	function handleResetZoom() {
		zoom = 1;
		panX = 40;
		panY = 40;
	}

	function handleFitView() {
		const bounds = graph.bounds;
		const viewportWidth = 1000;
		const viewportHeight = 650;

		const scaleX = (viewportWidth - 100) / bounds.width;
		const scaleY = (viewportHeight - 100) / bounds.height;
		const fitZoom = Math.max(0.4, Math.min(1.1, Math.min(scaleX, scaleY)));

		zoom = Math.round(fitZoom * 100) / 100;
		panX = Math.round(50 - bounds.minX * zoom);
		panY = Math.round(50 - bounds.minY * zoom);
	}

	// Keyboard shortcut: Esc to close inspector
	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Escape' && selectedItem) {
			selectedItem = null;
		}
	}

	onMount(() => {
		window.addEventListener('keydown', handleKeyDown);
		return () => {
			window.removeEventListener('keydown', handleKeyDown);
		};
	});
</script>

<svelte:head>
	<title>Topology — GOPOD</title>
</svelte:head>

<div class="w-full h-full flex flex-col gap-4 flex-1 pb-4">
	<!-- Page Header -->
	<PageHeader
		title="Topology"
		subtitle="Infrastructure map showing logical project territories, pod groupings, and resource flow."
	/>

	<!-- Toolbar Controls -->
	<TopologyToolbar
		{projects}
		{viewMode}
		{zoom}
		onViewModeChange={setViewMode}
		onZoomIn={handleZoomIn}
		onZoomOut={handleZoomOut}
		onResetZoom={handleResetZoom}
		onFitView={handleFitView}
	/>

	<!-- Interactive Topology Viewport with Inspector Dock -->
	<div class="relative flex-1 w-full min-h-[660px] flex rounded-[var(--radius-card)] overflow-hidden border border-[var(--border)] bg-[var(--bg-canvas)] transition-colors duration-200">
		<!-- Main Canvas Area -->
		<div class="flex-1 h-full flex relative overflow-hidden">
			<TopologyCanvas
				{graph}
				{selectedItem}
				onselect={(item) => (selectedItem = item)}
				bind:zoom
				bind:panX
				bind:panY
				onViewportChange={(v) => {
					zoom = v.zoom;
					panX = v.panX;
					panY = v.panY;
				}}
			/>
		</div>

		<!-- Right-Side Inspector Drawer -->
		{#if selectedItem}
			<TopologyInspector
				selected={selectedItem}
				onclose={() => (selectedItem = null)}
				onFocusProject={(projId) => setViewMode(projId)}
			/>
		{/if}
	</div>
</div>
