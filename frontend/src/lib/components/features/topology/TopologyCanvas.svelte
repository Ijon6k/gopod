<script lang="ts">
	import type { TopologyGraph, TopologyNode, TopologyRegion, SelectedItem } from './types';
	import TopologyNodeComp from './TopologyNode.svelte';
	import TopologyRegionComp from './TopologyRegion.svelte';
	import TopologyEdgeComp from './TopologyEdge.svelte';
	import { ui } from '$lib/stores/ui.svelte';

	interface Props {
		graph: TopologyGraph;
		selectedItem: SelectedItem;
		onselect: (item: SelectedItem) => void;
		zoom: number;
		panX: number;
		panY: number;
		onViewportChange: (viewport: { zoom: number; panX: number; panY: number }) => void;
	}

	let {
		graph,
		selectedItem,
		onselect,
		zoom = $bindable(1),
		panX = $bindable(60),
		panY = $bindable(40),
		onViewportChange
	}: Props = $props();

	let containerEl: HTMLDivElement | null = $state(null);
	let isPanning = $state(false);
	let startMouseX = $state(0);
	let startMouseY = $state(0);
	let startPanX = $state(0);
	let startPanY = $state(0);

	let hoveredNodeId: string | null = $state(null);
	let hoveredRegionId: string | null = $state(null);

	// Find connected edge IDs for hovered/selected node
	let activeNodeId = $derived(
		hoveredNodeId || (selectedItem?.kind === 'node' ? selectedItem.item.id : null)
	);

	let activeEdgeIds = $derived(
		activeNodeId
			? new Set(
					graph.edges
						.filter((e) => e.sourceId === activeNodeId || e.targetId === activeNodeId)
						.map((e) => e.id)
				)
			: new Set<string>()
	);

	// Check if we have domains or runtime pods (Project View active)
	let isProjectView = $derived(graph.nodes.some((n) => n.type === 'domain'));

	// Pan handlers
	function handleMouseDown(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (target.closest('.pointer-events-auto') && !target.classList.contains('canvas-bg')) {
			return;
		}

		isPanning = true;
		startMouseX = e.clientX;
		startMouseY = e.clientY;
		startPanX = panX;
		startPanY = panY;

		window.addEventListener('mousemove', handleMouseMove);
		window.addEventListener('mouseup', handleMouseUp);
	}

	function handleMouseMove(e: MouseEvent) {
		if (!isPanning) return;
		const dx = e.clientX - startMouseX;
		const dy = e.clientY - startMouseY;
		panX = startPanX + dx;
		panY = startPanY + dy;
		onViewportChange?.({ zoom, panX, panY });
	}

	function handleMouseUp() {
		if (isPanning) {
			isPanning = false;
			window.removeEventListener('mousemove', handleMouseMove);
			window.removeEventListener('mouseup', handleMouseUp);
		}
	}

	// Wheel zoom centered on pointer
	function handleWheel(e: WheelEvent) {
		e.preventDefault();
		if (!containerEl) return;

		const rect = containerEl.getBoundingClientRect();
		const mouseX = e.clientX - rect.left;
		const mouseY = e.clientY - rect.top;

		const zoomFactor = e.deltaY < 0 ? 1.1 : 0.9;
		const newZoom = Math.max(0.3, Math.min(2.2, zoom * zoomFactor));

		if (newZoom === zoom) return;

		panX = mouseX - (mouseX - panX) * (newZoom / zoom);
		panY = mouseY - (mouseY - panY) * (newZoom / zoom);
		zoom = newZoom;

		onViewportChange?.({ zoom, panX, panY });
	}

	function handleBackgroundClick(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (target.classList.contains('canvas-bg')) {
			onselect(null);
		}
	}
</script>

<!-- Canvas Container -->
<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<div
	bind:this={containerEl}
	onmousedown={handleMouseDown}
	onwheel={handleWheel}
	onclick={handleBackgroundClick}
	class="relative h-full min-h-[680px] w-full flex-1 overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-canvas)] transition-colors duration-200 select-none"
	style="cursor: {isPanning ? 'grabbing' : 'grab'};"
>
	<!-- SVG Layer for Dot Grid and Orthogonal Edges -->
	<svg
		class="canvas-bg pointer-events-auto absolute inset-0 h-full w-full"
		xmlns="http://www.w3.org/2000/svg"
	>
		<defs>
			<!-- Dynamic dot grid adapting to theme -->
			<pattern
				id="topo-grid"
				width="24"
				height="24"
				patternUnits="userSpaceOnUse"
				patternTransform="translate({panX % 24}, {panY % 24})"
			>
				<circle cx="1.5" cy="1.5" r="1.2" fill="var(--bg-canvas-dot)" opacity="0.8" />
			</pattern>
		</defs>

		<!-- Canvas Background with Dot Pattern -->
		<rect class="canvas-bg" width="100%" height="100%" fill="url(#topo-grid)" />

		<!-- Zoom & Pan SVG Transformation Group -->
		<g transform="translate({panX}, {panY}) scale({zoom})">
			<!-- Orthogonal Edges Layer -->
			{#each graph.edges as edge (edge.id)}
				<TopologyEdgeComp
					{edge}
					isHighlighted={activeEdgeIds.has(edge.id)}
					isSelected={selectedItem?.kind === 'node' &&
						(edge.sourceId === selectedItem.item.id || edge.targetId === selectedItem.item.id)}
				/>
			{/each}
		</g>
	</svg>

	<!-- HTML Layer for Column Guides, Regions and Nodes -->
	<div
		class="pointer-events-none absolute top-0 left-0 h-0 w-0"
		style="transform: translate3d({panX}px, {panY}px, 0) scale({zoom}); transform-origin: 0 0;"
	>
		<!-- Aesthetic Architecture Column Guides (Project View) -->
		{#if isProjectView}
			<div
				class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-semibold whitespace-nowrap text-[var(--text-secondary)] select-none"
				style="transform: translate3d(80px, 48px, 0);"
			>
				<span
					class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-shell)]/85 px-2.5 py-1 shadow-xs"
				>
					<span class="h-2 w-2 rounded-full bg-[var(--text-tertiary)]"></span> Ingress & Domains
				</span>
			</div>
			<div
				class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-semibold whitespace-nowrap text-[var(--text-secondary)] select-none"
				style="transform: translate3d(390px, 48px, 0);"
			>
				<span
					class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-shell)]/85 px-2.5 py-1 shadow-xs"
				>
					<span class="h-2 w-2 rounded-full bg-[var(--accent)]"></span> Services & Workloads
				</span>
			</div>
			<div
				class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-semibold whitespace-nowrap text-[var(--text-secondary)] select-none"
				style="transform: translate3d(730px, 48px, 0);"
			>
				<span
					class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-shell)]/85 px-2.5 py-1 shadow-xs"
				>
					<span class="h-2 w-2 rounded-full bg-[var(--status-green)]"></span> Runtime Pods & Containers
				</span>
			</div>
			<div
				class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-semibold whitespace-nowrap text-[var(--text-secondary)] select-none"
				style="transform: translate3d(1120px, 48px, 0);"
			>
				<span
					class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-shell)]/85 px-2.5 py-1 shadow-xs"
				>
					<span class="h-2 w-2 rounded-full bg-[var(--status-amber)]"></span> Storage & Volumes
				</span>
			</div>
		{/if}

		<!-- 1. Territory Regions (Pod runtime groupings) -->
		{#each graph.regions as region (region.id)}
			<TopologyRegionComp
				{region}
				isSelected={selectedItem?.kind === 'region' && selectedItem.item.id === region.id}
				isHovered={hoveredRegionId === region.id}
				onclick={() => onselect({ kind: 'region', item: region })}
				onmouseenter={() => (hoveredRegionId = region.id)}
				onmouseleave={() => (hoveredRegionId = null)}
			/>
		{/each}

		<!-- 2. Actual Resource Nodes -->
		{#each graph.nodes as node (node.id)}
			<TopologyNodeComp
				{node}
				isSelected={selectedItem?.kind === 'node' && selectedItem.item.id === node.id}
				isHovered={hoveredNodeId === node.id}
				onclick={() => onselect({ kind: 'node', item: node })}
				onmouseenter={() => (hoveredNodeId = node.id)}
				onmouseleave={() => (hoveredNodeId = null)}
			/>
		{/each}
	</div>

	<!-- Canvas Helper Watermark Pill -->
	<div
		class="pointer-events-none absolute bottom-3 left-3 flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)]/90 px-3 py-1.5 text-[11px] text-[var(--text-tertiary)] backdrop-blur-sm transition-colors"
	>
		<span>Drag to pan</span>
		<span class="opacity-40">·</span>
		<span>Scroll to zoom</span>
		<span class="opacity-40">·</span>
		<span>Click node to inspect</span>
	</div>
</div>
