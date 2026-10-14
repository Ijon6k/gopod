<script lang="ts">
	import { SvelteFlowProvider } from '@xyflow/svelte';
	import type { TopologyGraph, SelectedItem } from './types';
	import TopologyFlowInner from './TopologyFlowInner.svelte';

	interface Props {
		graph: TopologyGraph;
		selectedItem: SelectedItem;
		onselect: (item: SelectedItem) => void;
	}

	let { graph, selectedItem, onselect }: Props = $props();

	let innerRef: TopologyFlowInner | null = $state(null);

	export function zoomIn() {
		innerRef?.zoomIn();
	}

	export function zoomOut() {
		innerRef?.zoomOut();
	}

	export function fitView() {
		innerRef?.fitView();
	}

	export function resetZoom() {
		innerRef?.resetZoom();
	}
</script>

<div
	class="relative h-full min-h-[680px] w-full flex-1 overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-canvas)] transition-colors duration-200"
>
	<SvelteFlowProvider>
		<TopologyFlowInner bind:this={innerRef} {graph} {selectedItem} {onselect} />
	</SvelteFlowProvider>
</div>
