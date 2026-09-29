<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import type { TimeSeriesPoint } from '$lib/types';

	interface Props {
		data: TimeSeriesPoint[];
		height?: number;
		strokeColor?: string;
		fillColor?: string;
		class?: string;
	}

	let {
		data,
		height = 168,
		strokeColor = 'var(--accent)',
		fillColor = 'var(--accent)',
		class: className = ''
	}: Props = $props();

	// Compute SVG path from data
	let viewBox = $derived(`0 0 ${data.length > 0 ? data.length - 1 : 1} ${height}`);

	let maxVal = $derived(Math.max(...data.map((d) => d.value), 1));

	let points = $derived(
		data.map((d, i) => ({
			x: data.length > 1 ? (i / (data.length - 1)) * (data.length - 1) : 0,
			y: height - (d.value / maxVal) * (height - 16) - 8
		}))
	);

	// Create smooth path using catmull-rom to bezier conversion
	let linePath = $derived.by(() => {
		if (points.length < 2) return '';
		let path = `M ${points[0].x} ${points[0].y}`;
		for (let i = 1; i < points.length; i++) {
			const p0 = points[Math.max(0, i - 2)];
			const p1 = points[i - 1];
			const p2 = points[i];
			const p3 = points[Math.min(points.length - 1, i + 1)];

			const cp1x = p1.x + (p2.x - p0.x) / 6;
			const cp1y = p1.y + (p2.y - p0.y) / 6;
			const cp2x = p2.x - (p3.x - p1.x) / 6;
			const cp2y = p2.y - (p3.y - p1.y) / 6;

			path += ` C ${cp1x} ${cp1y}, ${cp2x} ${cp2y}, ${p2.x} ${p2.y}`;
		}
		return path;
	});

	let areaPath = $derived.by(() => {
		if (!linePath) return '';
		const lastX = points[points.length - 1]?.x ?? 0;
		const firstX = points[0]?.x ?? 0;
		return `${linePath} L ${lastX} ${height} L ${firstX} ${height} Z`;
	});

	let gradientId = $derived(`area-grad-${Math.random().toString(36).slice(2, 8)}`);
</script>

<div class={cn('w-full', className)} style="height: {height}px">
	<svg
		width="100%"
		height="100%"
		viewBox={viewBox}
		preserveAspectRatio="none"
		class="overflow-visible"
	>
		<defs>
			<linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
				<stop offset="0%" stop-color={fillColor} stop-opacity="0.14" />
				<stop offset="100%" stop-color={fillColor} stop-opacity="0" />
			</linearGradient>
		</defs>
		{#if areaPath}
			<path d={areaPath} fill="url(#{gradientId})" />
		{/if}
		{#if linePath}
			<path
				d={linePath}
				fill="none"
				stroke={strokeColor}
				stroke-width="1.5"
				vector-effect="non-scaling-stroke"
			/>
		{/if}
	</svg>
</div>
