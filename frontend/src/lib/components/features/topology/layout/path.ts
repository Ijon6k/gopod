import type { TopologyNode, TopologyRegion } from '../types';

// Create orthogonal 90-degree SVG path
export function createOrthogonalPath(
	x1: number,
	y1: number,
	x2: number,
	y2: number,
	midOffset?: number
): string {
	const midX = midOffset !== undefined ? midOffset : Math.round((x1 + x2) / 2);
	return `M ${x1} ${y1} L ${midX} ${y1} L ${midX} ${y2} L ${x2} ${y2}`;
}

// Calculate bounding box of graph
export function computeBounds(nodes: TopologyNode[], regions: TopologyRegion[]) {
	let minX = Infinity;
	let minY = Infinity;
	let maxX = -Infinity;
	let maxY = -Infinity;

	nodes.forEach((n) => {
		minX = Math.min(minX, n.x);
		minY = Math.min(minY, n.y);
		maxX = Math.max(maxX, n.x + n.width);
		maxY = Math.max(maxY, n.y + n.height);
	});

	regions.forEach((r) => {
		minX = Math.min(minX, r.x);
		minY = Math.min(minY, r.y);
		maxX = Math.max(maxX, r.x + r.width);
		maxY = Math.max(maxY, r.y + r.height);
	});

	if (minX === Infinity) {
		minX = 0;
		minY = 0;
		maxX = 1200;
		maxY = 800;
	}

	return {
		minX,
		minY,
		maxX,
		maxY,
		width: Math.max(800, maxX - minX),
		height: Math.max(600, maxY - minY)
	};
}
