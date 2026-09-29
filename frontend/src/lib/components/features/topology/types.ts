// ──────────────────────────────────────────────
// GOPOD Topology — Type Definitions
// ──────────────────────────────────────────────

import type { Status } from '$lib/types';

export type NodeType =
	| 'internet'
	| 'proxy'
	| 'domain'
	| 'service'
	| 'container'
	| 'volume'
	| 'network';

export type RegionType = 'project' | 'pod';

export interface TopologyNode {
	id: string;
	type: NodeType;
	title: string;
	subtitle: string;
	status?: Status | 'active' | 'healthy' | 'unhealthy' | string;
	x: number;
	y: number;
	width: number;
	height: number;
	badge?: string;
	monoDetail?: string;
	projectId?: string;
	serviceId?: string;
	podId?: string;
	icon?: string;
	raw?: any;
}

export interface TopologyRegion {
	id: string;
	type: RegionType;
	label: string;
	sublabel?: string;
	x: number;
	y: number;
	width: number;
	height: number;
	projectId?: string;
	status?: string;
	raw?: any;
}

export interface TopologyEdge {
	id: string;
	sourceId: string;
	targetId: string;
	sourceX: number;
	sourceY: number;
	targetX: number;
	targetY: number;
	path: string; // SVG orthogonal 90-degree path "M ... L ..."
	label?: string;
	dashed?: boolean;
}

export interface TopologyGraph {
	nodes: TopologyNode[];
	regions: TopologyRegion[];
	edges: TopologyEdge[];
	bounds: { minX: number; minY: number; maxX: number; maxY: number; width: number; height: number };
}

export type SelectedItem =
	| { kind: 'node'; item: TopologyNode }
	| { kind: 'region'; item: TopologyRegion }
	| null;

export type ViewMode = 'global' | string; // 'global' or projectId
