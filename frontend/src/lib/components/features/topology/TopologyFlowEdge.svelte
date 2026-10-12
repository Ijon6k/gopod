<script lang="ts">
	import { BaseEdge, EdgeLabel, getSmoothStepPath, Position, type EdgeProps } from '@xyflow/svelte';

	let {
		id,
		sourceX,
		sourceY,
		targetX,
		targetY,
		sourcePosition = Position.Right,
		targetPosition = Position.Left,
		label,
		selected,
		markerEnd,
		style,
		data
	}: EdgeProps = $props();

	// Orthogonal step path with smooth rounded corners & robust fallbacks so lines never disappear
	let pathData = $derived.by(() => {
		if (
			typeof sourceX !== 'number' ||
			typeof sourceY !== 'number' ||
			typeof targetX !== 'number' ||
			typeof targetY !== 'number' ||
			isNaN(sourceX) ||
			isNaN(sourceY) ||
			isNaN(targetX) ||
			isNaN(targetY)
		) {
			return { path: '', labelX: 0, labelY: 0 };
		}
		try {
			const [p, lx, ly] = getSmoothStepPath({
				sourceX,
				sourceY,
				targetX,
				targetY,
				sourcePosition: sourcePosition || Position.Right,
				targetPosition: targetPosition || Position.Left,
				borderRadius: 10
			});
			const fallbackPath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`;
			return {
				path: p || fallbackPath,
				labelX: lx || (sourceX + targetX) / 2,
				labelY: ly || (sourceY + targetY) / 2
			};
		} catch {
			const fallbackPath = `M ${sourceX} ${sourceY} L ${targetX} ${targetY}`;
			return {
				path: fallbackPath,
				labelX: (sourceX + targetX) / 2,
				labelY: (sourceY + targetY) / 2
			};
		}
	});

	let path = $derived(pathData.path);
	let labelX = $derived(pathData.labelX);
	let labelY = $derived(pathData.labelY);

	let kind = $derived((data as any)?.kind || 'workload');
	let isDashed = $derived(
		(data as any)?.dashed || kind === 'localhost-port' || false
	);
	let isStopped = $derived((data as any)?.isStopped || false);

	// Semantic stroke color based on traffic kind:
	// - Direct public host port: Amber
	// - Localhost loopback host port: Blue
	// - Caddy Domain Ingress: Lavender Accent
	let strokeColor = $derived.by(() => {
		if (selected) return 'var(--accent)';
		if (isStopped) return 'var(--text-tertiary)';
		if (kind === 'localhost-port') return 'var(--status-blue)';
		if (kind === 'direct-port') return 'var(--status-amber)';
		if (kind === 'ingress') return 'var(--accent)';
		return 'var(--accent)';
	});

	let strokeWidth = $derived(selected ? 2.25 : 1.5);
</script>

<g class="transition-all duration-150">
	<!-- Subtle ambient glow underlay (refined, not tacky) -->
	{#if !isStopped}
		<path
			d={path}
			fill="none"
			stroke={strokeColor}
			stroke-width={strokeWidth + 2.5}
			stroke-opacity={selected ? 0.35 : 0.15}
			class="pointer-events-none"
		/>
	{/if}

	<!-- Interactive Hover Hit Area -->
	<path
		d={path}
		fill="none"
		stroke="transparent"
		stroke-width={14}
		class="pointer-events-auto cursor-pointer"
	/>

	<!-- Main Visible Rounded Step Edge (Zero animation, dashed only when workload is stopped) -->
	<BaseEdge
		{id}
		{path}
		{markerEnd}
		style="stroke: {strokeColor}; stroke-width: {strokeWidth}px; {isDashed ? 'stroke-dasharray: 5 4; stroke-opacity: 0.6;' : ''} {style || ''}"
		class="transition-colors"
	/>

	<!-- Kumo Badge Label Pill Centered on Edge -->
	{#if label}
		<EdgeLabel x={labelX} y={labelY} selectEdgeOnClick transparent>
			<div
				class="inline-flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-base)] px-2 py-0.5 font-mono text-[9.5px] font-medium text-[var(--text-secondary)] shadow-xs select-none"
			>
				<span class="h-1.5 w-1.5 rounded-full {isStopped ? 'bg-[var(--text-tertiary)]' : 'bg-[var(--accent)]'}"></span>
				<span>{label}</span>
			</div>
		</EdgeLabel>
	{/if}
</g>
