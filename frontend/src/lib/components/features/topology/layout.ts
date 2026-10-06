// ──────────────────────────────────────────────
// GOPOD Topology — Deterministic Layout Engine Facade
// ──────────────────────────────────────────────

import type { Project, Service, Container, Pod, Domain, Volume, Network, Server } from '$lib/types';
import type { TopologyGraph } from './types';
import { computeGlobalLayout } from './layout/global';
import { computeProjectFocusLayout } from './layout/project';

export { NODE_WIDTH, NODE_HEIGHT, COMPACT_NODE_HEIGHT } from './layout/constants';
export { createOrthogonalPath, computeBounds } from './layout/path';
export { computeGlobalLayout } from './layout/global';
export { computeProjectFocusLayout } from './layout/project';

export interface LayoutInput {
	projects: Project[];
	services: Service[];
	containers: Container[];
	pods: Pod[];
	domains: Domain[];
	volumes: Volume[];
	networks: Network[];
	server: Server;
	viewMode: string; // 'global' or projectId
}

export function computeTopologyGraph({
	projects,
	services,
	containers,
	pods,
	domains,
	volumes,
	networks,
	server,
	viewMode
}: LayoutInput): TopologyGraph {
	if (viewMode === 'global') {
		return computeGlobalLayout({ projects, services, domains, server });
	} else {
		return computeProjectFocusLayout({
			projectId: viewMode,
			projects,
			services,
			containers,
			pods,
			domains,
			volumes,
			networks
		});
	}
}
