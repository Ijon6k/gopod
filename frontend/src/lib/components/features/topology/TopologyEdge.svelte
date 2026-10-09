<script lang="ts">
	import type { TopologyEdge } from './types';
	import { ui } from '$lib/stores/ui.svelte';

	interface Props {
		edge: TopologyEdge;
		isHighlighted?: boolean;
		isSelected?: boolean;
	}

	let { edge, isHighlighted = false, isSelected = false }: Props = $props();

	// Midpoint for label
	let labelX = $derived(Math.round((edge.sourceX + edge.targetX) / 2));
	let labelY = $derived(Math.round((edge.sourceY + edge.targetY) / 2));
</script>

<g class="transition-all duration-200">
	<!-- Hit-test / glow path -->
	<path
		d={edge.path}
		fill="none"
		stroke={isSelected || isHighlighted ? 'var(--border-canvas-edge-glow)' : 'transparent'}
		stroke-width={isSelected || isHighlighted ? 5 : 8}
		stroke-linecap="round"
		stroke-linejoin="round"
	/>

	<!-- Main edge path -->
	<path
		d={edge.path}
		fill="none"
		stroke={isSelected
			? 'var(--accent)'
			: isHighlighted
				? ui.theme === 'light'
					? '#4F46E5'
					: '#818CF8'
				: 'var(--border-canvas-edge)'}
		stroke-width={isSelected ? 2 : isHighlighted ? 1.75 : 1.25}
		stroke-dasharray={edge.dashed ? '4 3' : undefined}
		stroke-linecap="round"
		stroke-linejoin="round"
	/>

	<!-- Optional Edge Port/Protocol Label -->
	{#if edge.label && (isHighlighted || isSelected)}
		<rect
			x={labelX - 18}
			y={labelY - 9}
			width={36}
			height={18}
			rx={4}
			fill="var(--bg-canvas-node)"
			stroke="var(--border-canvas-node)"
			stroke-width={1}
		/>
		<text
			x={labelX}
			y={labelY + 3.5}
			text-anchor="middle"
			class="fill-[var(--text-secondary)] font-mono text-[9.5px] font-medium select-none"
		>
			{edge.label}
		</text>
	{/if}
</g>
