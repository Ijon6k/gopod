<script lang="ts">
	import {
		SvelteFlow,
		Background,
		BackgroundVariant,
		MiniMap,
		ViewportPortal,
		useSvelteFlow,
		MarkerType,
		type Node as FlowNode,
		type Edge as FlowEdge
	} from '@xyflow/svelte';
	import { untrack } from 'svelte';
	import type { TopologyGraph, TopologyNode, SelectedItem } from './types';
	import TopologyFlowNode from './TopologyFlowNode.svelte';
	import TopologyFlowEdge from './TopologyFlowEdge.svelte';
	import TopologyRegionComp from './TopologyRegion.svelte';
	import { ui } from '$lib/stores/ui.svelte';

	interface Props {
		graph: TopologyGraph;
		selectedItem: SelectedItem;
		onselect: (item: SelectedItem) => void;
	}

	let { graph, selectedItem, onselect }: Props = $props();

	// Svelte Flow Hook
	const { zoomIn: flowZoomIn, zoomOut: flowZoomOut, fitView: flowFitView, setZoom } = useSvelteFlow();

	export function zoomIn() {
		flowZoomIn({ duration: 250 });
	}

	export function zoomOut() {
		flowZoomOut({ duration: 250 });
	}

	export function fitView() {
		flowFitView({ padding: 0.15, duration: 300 });
	}

	export function resetZoom() {
		setZoom(1, { duration: 250 });
	}

	// Registered Custom Node & Edge Types
	const nodeTypes = {
		gopodNode: TopologyFlowNode
	};

	const edgeTypes = {
		gopodEdge: TopologyFlowEdge
	};

	let hoveredRegionId: string | null = $state(null);
	let isProjectView = $derived(graph.nodes.some((n) => n.type === 'service'));

	// Flow State
	let flowNodes = $state<FlowNode[]>([]);
	let flowEdges = $state<FlowEdge[]>([]);
	let lastGraphRef: TopologyGraph | null = null;

	// Synchronize GOPOD deterministic layout with Svelte Flow only when graph structure changes
	$effect(() => {
		const currentGraph = graph;

		if (currentGraph !== lastGraphRef) {
			lastGraphRef = currentGraph;
			
			// Compute nodes and edges on graph transition
			flowNodes = currentGraph.nodes.map((n) => ({
				id: n.id,
				type: 'gopodNode',
				position: { x: n.x, y: n.y },
				data: { node: n },
				selected: selectedItem?.kind === 'node' && selectedItem.item.id === n.id,
				width: n.width,
				height: n.height,
				draggable: true
			}));

			flowEdges = currentGraph.edges.map((e) => {
				const targetNode = currentGraph.nodes.find((n) => n.id === e.targetId);
				const sourceNode = currentGraph.nodes.find((n) => n.id === e.sourceId);
				const isStopped =
					targetNode?.status === 'stopped' ||
					targetNode?.status === 'exited' ||
					targetNode?.status === 'offline' ||
					targetNode?.status === 'failed' ||
					sourceNode?.status === 'stopped' ||
					sourceNode?.status === 'exited';

				return {
					id: e.id,
					source: e.sourceId,
					target: e.targetId,
					type: 'gopodEdge',
					label: e.label,
					animated: false,
					data: {
						kind: e.kind || 'workload',
						dashed: isStopped,
						isStopped
					},
					markerEnd: {
						type: MarkerType.ArrowClosed,
						width: 10,
						height: 10,
						color: isStopped ? 'var(--text-tertiary)' : 'var(--accent)'
					}
				};
			});
		}
	});

	// Safe selection sync for external triggers (Esc key, inspector close) with untrack to prevent infinite loops
	$effect(() => {
		const targetId = selectedItem?.kind === 'node' ? selectedItem.item.id : null;
		untrack(() => {
			let updated = false;
			for (const node of flowNodes) {
				const shouldBeSelected = node.id === targetId;
				if (node.selected !== shouldBeSelected) {
					node.selected = shouldBeSelected;
					updated = true;
				}
			}
			if (updated) {
				flowNodes = [...flowNodes];
			}
		});
	});

	// Node click handler
	function handleNodeClick({ node }: { node: FlowNode }) {
		const topoNode = node.data?.node as TopologyNode | undefined;
		if (topoNode) {
			onselect({ kind: 'node', item: topoNode });
		}
	}

	// Canvas pane click handler
	function handlePaneClick() {
		onselect(null);
	}

	// MiniMap color mapping based on infrastructure resource category
	function getMiniMapNodeColor(node: FlowNode): string {
		const n = node.data?.node as TopologyNode | undefined;
		if (!n) return '#71717a';
		switch (n.type) {
			case 'domain':
			case 'internet':
				return '#7c5ce6'; // Lavender accent
			case 'proxy':
				return '#10b981'; // Emerald gateway
			case 'service':
				return '#3b82f6'; // Blue stack
			case 'container':
				return '#22c55e'; // Green workload
			case 'volume':
				return '#f59e0b'; // Amber storage
			case 'network':
				return '#a855f7'; // Purple bridge
			default:
				return '#71717a';
		}
	}
</script>

<div class="relative h-full w-full flex-1 overflow-hidden select-none">
	<SvelteFlow
		bind:nodes={flowNodes}
		bind:edges={flowEdges}
		{nodeTypes}
		{edgeTypes}
		colorMode={ui.theme}
		fitView
		fitViewOptions={{ padding: 0.15 }}
		minZoom={0.2}
		maxZoom={2.5}
		onnodeclick={handleNodeClick}
		onpaneclick={handlePaneClick}
		proOptions={{ hideAttribution: true }}
		class="w-full h-full bg-[var(--bg-canvas)]"
	>
		<!-- Svelte Flow Dot Grid Background -->
		<Background
			variant={BackgroundVariant.Dots}
			gap={24}
			size={1.2}
			bgColor="var(--bg-canvas)"
			patternColor="var(--bg-canvas-dot)"
		/>

		<!-- Svelte Flow Interactive MiniMap -->
		<MiniMap
			position="bottom-right"
			nodeColor={getMiniMapNodeColor}
			nodeStrokeColor="transparent"
			maskColor="rgba(0, 0, 0, 0.55)"
			class="!bg-[var(--bg-panel)]/95 !border !border-[var(--border)] !rounded-[var(--radius-card)] !shadow-xl !backdrop-blur-md"
		/>

		<!-- Viewport Backplane: Architecture Pipeline Column Guides & Region Boundaries -->
		<ViewportPortal target="back">
			<!-- Architecture Pipeline Column Guides (Project View) -->
			{#if isProjectView}
				<div
					class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-medium whitespace-nowrap select-none"
					style="transform: translate3d(60px, 48px, 0);"
				>
					<span
						class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)]/95 px-2.5 py-1 text-[var(--text-secondary)] shadow-sm backdrop-blur-xs"
					>
						<span class="h-2 w-2 rounded-full bg-[var(--text-tertiary)]"></span>
						<span>1. Ingress & entrypoint</span>
					</span>
				</div>
				<div
					class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-medium whitespace-nowrap select-none"
					style="transform: translate3d(360px, 48px, 0);"
				>
					<span
						class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)]/95 px-2.5 py-1 text-[var(--text-secondary)] shadow-sm backdrop-blur-xs"
					>
						<span class="h-2 w-2 rounded-full bg-[var(--accent)]"></span>
						<span>2. Caddy ingress gateway</span>
					</span>
				</div>
				<div
					class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-medium whitespace-nowrap select-none"
					style="transform: translate3d(660px, 48px, 0);"
				>
					<span
						class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)]/95 px-2.5 py-1 text-[var(--text-secondary)] shadow-sm backdrop-blur-xs"
					>
						<span class="h-2 w-2 rounded-full bg-[var(--status-blue)]"></span>
						<span>3. Services & stacks</span>
					</span>
				</div>
				<div
					class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-medium whitespace-nowrap select-none"
					style="transform: translate3d(980px, 48px, 0);"
				>
					<span
						class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)]/95 px-2.5 py-1 text-[var(--text-secondary)] shadow-sm backdrop-blur-xs"
					>
						<span class="h-2 w-2 rounded-full bg-[var(--status-green)]"></span>
						<span>4. Runtime workloads</span>
					</span>
				</div>
				{#if graph.nodes.some((n) => n.type === 'volume' || n.type === 'network')}
					<div
						class="pointer-events-none absolute top-0 left-0 text-xs font-[var(--font-sans)] font-medium whitespace-nowrap select-none"
						style="transform: translate3d(1300px, 48px, 0);"
					>
						<span
							class="inline-flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-panel)]/95 px-2.5 py-1 text-[var(--text-secondary)] shadow-sm backdrop-blur-xs"
						>
							<span class="h-2 w-2 rounded-full bg-[var(--status-amber)]"></span>
							<span>5. Storage & volumes</span>
						</span>
					</div>
				{/if}
			{/if}

			<!-- Territory Regions (Pod runtime groupings) -->
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
		</ViewportPortal>
	</SvelteFlow>
</div>
