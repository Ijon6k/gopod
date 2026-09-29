// ──────────────────────────────────────────────
// GOPOD Topology — Deterministic Layout Engine
// ──────────────────────────────────────────────

import type { Project, Service, Container, Pod, Domain, Volume, Network, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from './types';

// Node dimension constants
export const NODE_WIDTH = 200;
export const NODE_HEIGHT = 66;
export const COMPACT_NODE_HEIGHT = 58;

// Create orthogonal 90-degree SVG path
export function createOrthogonalPath(
	x1: number,
	y1: number,
	x2: number,
	y2: number,
	midOffset?: number
): string {
	const midX = midOffset !== undefined ? midOffset : Math.round((x1 + x2) / 2);
	// M x1 y1 H midX V y2 H x2
	return `M ${x1} ${y1} L ${midX} ${y1} L ${midX} ${y2} L ${x2} ${y2}`;
}

interface LayoutInput {
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

// ──────────────────────────────────────────────
// 1. GLOBAL INFRASTRUCTURE TOPOLOGY
// Internet → Caddy Reverse Proxy → Projects (Territories)
// ──────────────────────────────────────────────
function computeGlobalLayout({
	projects,
	services,
	domains,
	server
}: {
	projects: Project[];
	services: Service[];
	domains: Domain[];
	server: Server;
}): TopologyGraph {
	const nodes: TopologyNode[] = [];
	const regions: TopologyRegion[] = [];
	const edges: TopologyEdge[] = [];

	// Internet Ingress Node
	const internetNode: TopologyNode = {
		id: 'ingress-internet',
		type: 'internet',
		title: 'Public Internet',
		subtitle: 'IPv4 / IPv6 Traffic',
		status: 'active',
		x: 60,
		y: 220,
		width: 170,
		height: 62,
		monoDetail: '0.0.0.0/0',
		badge: 'WAN',
		raw: { ip: server.ip, os: server.os }
	};
	nodes.push(internetNode);

	// Caddy Reverse Proxy Node
	const caddyNode: TopologyNode = {
		id: 'proxy-caddy',
		type: 'proxy',
		title: 'Caddy Proxy',
		subtitle: 'Reverse Proxy & TLS',
		status: server.status === 'online' ? 'running' : 'degraded',
		x: 300,
		y: 220,
		width: 180,
		height: 62,
		monoDetail: `ports 80, 443`,
		badge: `Caddy ${server.caddy || 'v2.8'}`,
		raw: { server, domainsCount: domains.length }
	};
	nodes.push(caddyNode);

	// Edge: Internet -> Caddy
	edges.push({
		id: 'edge-internet-caddy',
		sourceId: internetNode.id,
		targetId: caddyNode.id,
		sourceX: internetNode.x + internetNode.width,
		sourceY: internetNode.y + internetNode.height / 2,
		targetX: caddyNode.x,
		targetY: caddyNode.y + caddyNode.height / 2,
		path: createOrthogonalPath(
			internetNode.x + internetNode.width,
			internetNode.y + internetNode.height / 2,
			caddyNode.x,
			caddyNode.y + caddyNode.height / 2
		),
		label: 'HTTPS'
	});

	// Project Territories Layout
	const projectRegionX = 560;
	let currentY = 40;
	const regionGap = 32;

	projects.forEach((proj) => {
		const projServices = services.filter((s) => s.projectId === proj.id);
		const projDomains = domains.filter((d) => d.projectId === proj.id);

		// Height based on services
		const svcCount = Math.max(1, projServices.length);
		const regionHeight = Math.max(130, 44 + svcCount * (COMPACT_NODE_HEIGHT + 12) + 16);
		const regionWidth = 440;

		const region: TopologyRegion = {
			id: `region-project-${proj.id}`,
			type: 'project',
			label: proj.name,
			sublabel: `${projServices.length} services · ${proj.cpu.toFixed(1)}% CPU`,
			x: projectRegionX,
			y: currentY,
			width: regionWidth,
			height: regionHeight,
			projectId: proj.id,
			status: proj.status,
			raw: proj
		};
		regions.push(region);

		// Place service nodes inside project region
		let svcY = currentY + 44;
		projServices.forEach((svc, idx) => {
			const svcNode: TopologyNode = {
				id: `svc-${svc.id}`,
				type: 'service',
				title: svc.name,
				subtitle: svc.type === 'application' ? 'Application' : svc.type === 'compose' ? 'Compose Stack' : 'Container Image',
				status: svc.status,
				x: projectRegionX + 24,
				y: svcY,
				width: 220,
				height: COMPACT_NODE_HEIGHT,
				monoDetail: `port :${svc.port}`,
				badge: svc.type,
				projectId: proj.id,
				serviceId: svc.id,
				raw: svc
			};
			nodes.push(svcNode);

			// Edge from Caddy to public services or primary service
			const hasDomain = projDomains.some((d) => d.serviceId === svc.id);
			if (hasDomain || idx === 0) {
				edges.push({
					id: `edge-caddy-svc-${svc.id}`,
					sourceId: caddyNode.id,
					targetId: svcNode.id,
					sourceX: caddyNode.x + caddyNode.width,
					sourceY: caddyNode.y + caddyNode.height / 2,
					targetX: svcNode.x,
					targetY: svcNode.y + svcNode.height / 2,
					path: createOrthogonalPath(
						caddyNode.x + caddyNode.width,
						caddyNode.y + caddyNode.height / 2,
						svcNode.x,
						svcNode.y + svcNode.height / 2,
						520
					),
					label: svc.port ? `:${svc.port}` : undefined
				});
			}

			svcY += COMPACT_NODE_HEIGHT + 12;
		});

		currentY += regionHeight + regionGap;
	});

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}

// ──────────────────────────────────────────────
// 2. PROJECT FOCUS TOPOLOGY
// Domain → Service → Pod / Container (Territory) → Volume / Network
// ──────────────────────────────────────────────
function computeProjectFocusLayout({
	projectId,
	projects,
	services,
	containers,
	pods,
	domains,
	volumes,
	networks
}: {
	projectId: string;
	projects: Project[];
	services: Service[];
	containers: Container[];
	pods: Pod[];
	domains: Domain[];
	volumes: Volume[];
	networks: Network[];
}): TopologyGraph {
	const project = projects.find((p) => p.id === projectId) || projects[0];
	const projServices = services.filter((s) => s.projectId === project.id);
	const projContainers = containers.filter((c) => c.projectId === project.id);
	const projPods = pods.filter((p) => p.projectId === project.id);
	const projDomains = domains.filter((d) => d.projectId === project.id);
	const projVolumes = volumes.filter((v) => v.projectId === project.id);
	const projNetwork = networks.find(
		(n) => n.projects.includes(project.name) || n.projects.includes(project.id)
	);

	const nodes: TopologyNode[] = [];
	const regions: TopologyRegion[] = [];
	const edges: TopologyEdge[] = [];

	// Column coordinates with generous routing channels
	const colDomainX = 80;
	const colServiceX = 360;
	const colRuntimeX = 670;
	const colStorageX = 1010;

	const startY = 90;
	const itemGap = 20;

	// ── 1. Ingress / Domains ──
	let domainY = startY;
	projDomains.forEach((dom) => {
		const domNode: TopologyNode = {
			id: `dom-${dom.id}`,
			type: 'domain',
			title: dom.hostname,
			subtitle: dom.tls ? 'TLS Enabled' : 'HTTP',
			status: dom.status === 'active' ? 'running' : 'degraded',
			x: colDomainX,
			y: domainY,
			width: 210,
			height: NODE_HEIGHT,
			monoDetail: `proxy :${dom.proxyPort}`,
			badge: 'Domain',
			projectId: project.id,
			serviceId: dom.serviceId,
			raw: dom
		};
		nodes.push(domNode);
		domainY += NODE_HEIGHT + itemGap;
	});

	// ── 2. Services ──
	let serviceY = startY;
	projServices.forEach((svc) => {
		const svcNode: TopologyNode = {
			id: `svc-${svc.id}`,
			type: 'service',
			title: svc.name,
			subtitle: svc.type === 'application' ? 'Application' : svc.type === 'compose' ? 'Compose App' : 'Image Workload',
			status: svc.status,
			x: colServiceX,
			y: serviceY,
			width: 220,
			height: NODE_HEIGHT,
			monoDetail: `port :${svc.port}`,
			badge: svc.type,
			projectId: project.id,
			serviceId: svc.id,
			raw: svc
		};
		nodes.push(svcNode);

		// Connect Domain -> Service if matching
		const matchingDomain = projDomains.find((d) => d.serviceId === svc.id);
		if (matchingDomain) {
			const domNode = nodes.find((n) => n.id === `dom-${matchingDomain.id}`);
			if (domNode) {
				edges.push({
					id: `edge-${domNode.id}-${svcNode.id}`,
					sourceId: domNode.id,
					targetId: svcNode.id,
					sourceX: domNode.x + domNode.width,
					sourceY: domNode.y + domNode.height / 2,
					targetX: svcNode.x,
					targetY: svcNode.y + svcNode.height / 2,
					path: createOrthogonalPath(
						domNode.x + domNode.width,
						domNode.y + domNode.height / 2,
						svcNode.x,
						svcNode.y + svcNode.height / 2
					),
					label: `:${svc.port}`
				});
			}
		}

		serviceY += NODE_HEIGHT + itemGap;
	});

	// ── 3. Runtime Layer: Pod Territories + Standalone Containers ──
	let runtimeY = startY;
	const handledContainerIds = new Set<string>();

	// Process Pods
	projPods.forEach((pod) => {
		// Find containers matching this pod
		const podContainers = projContainers.filter(
			(c) => pod.containers.includes(c.name) || pod.containers.includes(c.id)
		);

		const podInternalPadding = 20;
		const podHeaderHeight = 52;
		const containerCardHeight = 62;
		const containerCardGap = 12;

		const podHeight =
			podHeaderHeight +
			Math.max(1, podContainers.length) * containerCardHeight +
			Math.max(0, podContainers.length - 1) * containerCardGap +
			podInternalPadding;
		const podWidth = 268;

		const podRegion: TopologyRegion = {
			id: `region-pod-${pod.id}`,
			type: 'pod',
			label: `Pod · ${pod.name}`,
			sublabel: `${podContainers.length} containers`,
			x: colRuntimeX,
			y: runtimeY,
			width: podWidth,
			height: podHeight,
			projectId: project.id,
			status: pod.status,
			raw: pod
		};
		regions.push(podRegion);

		// Place containers inside this Pod region with safe vertical clearance
		let containerInsideY = runtimeY + podHeaderHeight;
		podContainers.forEach((c) => {
			handledContainerIds.add(c.id);
			const containerNode: TopologyNode = {
				id: `container-${c.id}`,
				type: 'container',
				title: c.name,
				subtitle: c.image.split('/').pop() || c.image,
				status: c.status,
				x: colRuntimeX + 16,
				y: containerInsideY,
				width: podWidth - 32, // 236px width, perfectly centered with 16px margins
				height: containerCardHeight,
				monoDetail: `${c.cpu.toFixed(1)}% · ${c.memory}MB`,
				badge: 'Container',
				projectId: project.id,
				serviceId: c.serviceId,
				podId: pod.id,
				raw: c
			};
			nodes.push(containerNode);

			// Connect Service -> Container
			if (c.serviceId) {
				const parentServiceNode = nodes.find((n) => n.id === `svc-${c.serviceId}`);
				if (parentServiceNode) {
					edges.push({
						id: `edge-${parentServiceNode.id}-${containerNode.id}`,
						sourceId: parentServiceNode.id,
						targetId: containerNode.id,
						sourceX: parentServiceNode.x + parentServiceNode.width,
						sourceY: parentServiceNode.y + parentServiceNode.height / 2,
						targetX: containerNode.x,
						targetY: containerNode.y + containerNode.height / 2,
						path: createOrthogonalPath(
							parentServiceNode.x + parentServiceNode.width,
							parentServiceNode.y + parentServiceNode.height / 2,
							containerNode.x,
							containerNode.y + containerNode.height / 2
						)
					});
				}
			}

			containerInsideY += containerCardHeight + containerCardGap;
		});

		runtimeY += podHeight + itemGap + 10;
	});

	// Standalone Containers (Containers not in any Pod)
	const standaloneContainers = projContainers.filter((c) => !handledContainerIds.has(c.id));
	standaloneContainers.forEach((c) => {
		const containerNode: TopologyNode = {
			id: `container-${c.id}`,
			type: 'container',
			title: c.name,
			subtitle: c.image.split('/').pop() || c.image,
			status: c.status,
			x: colRuntimeX,
			y: runtimeY,
			width: 220,
			height: NODE_HEIGHT,
			monoDetail: `${c.cpu.toFixed(1)}% · ${c.memory}MB`,
			badge: 'Standalone',
			projectId: project.id,
			serviceId: c.serviceId,
			raw: c
		};
		nodes.push(containerNode);

		// Connect Service -> Standalone Container
		if (c.serviceId) {
			const parentServiceNode = nodes.find((n) => n.id === `svc-${c.serviceId}`);
			if (parentServiceNode) {
				edges.push({
					id: `edge-${parentServiceNode.id}-${containerNode.id}`,
					sourceId: parentServiceNode.id,
					targetId: containerNode.id,
					sourceX: parentServiceNode.x + parentServiceNode.width,
					sourceY: parentServiceNode.y + parentServiceNode.height / 2,
					targetX: containerNode.x,
					targetY: containerNode.y + containerNode.height / 2,
					path: createOrthogonalPath(
						parentServiceNode.x + parentServiceNode.width,
						parentServiceNode.y + parentServiceNode.height / 2,
						containerNode.x,
						containerNode.y + containerNode.height / 2
					)
				});
			}
		}

		runtimeY += NODE_HEIGHT + itemGap;
	});

	// If no containers exist (e.g. compose stack without explicit runtime containers listed),
	// check service workloads (like homelab-stack)
	if (projContainers.length === 0) {
		projServices.forEach((svc) => {
			if (svc.workloads && svc.workloads.length > 0) {
				svc.workloads.forEach((wl: any) => {
					const wlNode: TopologyNode = {
						id: `workload-${svc.id}-${wl.name}`,
						type: 'container',
						title: wl.name,
						subtitle: wl.image,
						status: wl.status || 'running',
						x: colRuntimeX,
						y: runtimeY,
						width: 220,
						height: COMPACT_NODE_HEIGHT,
						monoDetail: `${wl.cpu || 0}% · ${wl.memory || 0}MB`,
						badge: 'Workload',
						projectId: project.id,
						serviceId: svc.id,
						raw: wl
					};
					nodes.push(wlNode);

					const parentServiceNode = nodes.find((n) => n.id === `svc-${svc.id}`);
					if (parentServiceNode) {
						edges.push({
							id: `edge-${parentServiceNode.id}-${wlNode.id}`,
							sourceId: parentServiceNode.id,
							targetId: wlNode.id,
							sourceX: parentServiceNode.x + parentServiceNode.width,
							sourceY: parentServiceNode.y + parentServiceNode.height / 2,
							targetX: wlNode.x,
							targetY: wlNode.y + wlNode.height / 2,
							path: createOrthogonalPath(
								parentServiceNode.x + parentServiceNode.width,
								parentServiceNode.y + parentServiceNode.height / 2,
								wlNode.x,
								wlNode.y + wlNode.height / 2
							)
						});
					}
					runtimeY += COMPACT_NODE_HEIGHT + itemGap;
				});
			}
		});
	}

	// ── 4. Storage (Volumes) & Network ──
	let storageY = startY;

	// Volumes
	projVolumes.forEach((vol) => {
		const volNode: TopologyNode = {
			id: `vol-${vol.id}`,
			type: 'volume',
			title: vol.name,
			subtitle: `Mount: ${vol.mount}`,
			status: vol.status === 'mounted' ? 'running' : 'stopped',
			x: colStorageX,
			y: storageY,
			width: 210,
			height: NODE_HEIGHT,
			monoDetail: vol.size,
			badge: 'Volume',
			projectId: project.id,
			serviceId: vol.serviceId,
			raw: vol
		};
		nodes.push(volNode);

		// Connect Container / Service -> Volume
		const relatedContainer = nodes.find(
			(n) => n.type === 'container' && (n.serviceId === vol.serviceId || n.title.includes(vol.serviceName))
		);
		const relatedSource = relatedContainer || nodes.find((n) => n.id === `svc-${vol.serviceId}`);
		if (relatedSource) {
			edges.push({
				id: `edge-${relatedSource.id}-${volNode.id}`,
				sourceId: relatedSource.id,
				targetId: volNode.id,
				sourceX: relatedSource.x + relatedSource.width,
				sourceY: relatedSource.y + relatedSource.height / 2,
				targetX: volNode.x,
				targetY: volNode.y + volNode.height / 2,
				path: createOrthogonalPath(
					relatedSource.x + relatedSource.width,
					relatedSource.y + relatedSource.height / 2,
					volNode.x,
					volNode.y + volNode.height / 2
				),
				dashed: true
			});
		}

		storageY += NODE_HEIGHT + itemGap;
	});

	// Project Network
	if (projNetwork) {
		const netNode: TopologyNode = {
			id: `net-${projNetwork.id}`,
			type: 'network',
			title: projNetwork.name,
			subtitle: `Driver: ${projNetwork.driver}`,
			status: 'active',
			x: colStorageX,
			y: storageY,
			width: 210,
			height: NODE_HEIGHT,
			monoDetail: projNetwork.subnet,
			badge: 'Network',
			projectId: project.id,
			raw: projNetwork
		};
		nodes.push(netNode);
		storageY += NODE_HEIGHT + itemGap;
	}

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}

// ──────────────────────────────────────────────
// Helper: Calculate bounding box of graph
// ──────────────────────────────────────────────
function computeBounds(nodes: TopologyNode[], regions: TopologyRegion[]) {
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
