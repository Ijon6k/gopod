import type { Project, Service, Container, Pod, Domain, Volume, Network, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { NODE_HEIGHT, COMPACT_NODE_HEIGHT } from './constants';
import { createCurvedBezierPath, computeBounds } from './path';

// ──────────────────────────────────────────────
// PROJECT FOCUS TOPOLOGY — 4-TIER ARCHITECTURE PIPELINE
// Tier 1: Public WAN & Domains (Entrypoint)
// Tier 2: Caddy Ingress Gateway (Reverse Proxy / TLS)
// Tier 3: Services & Stacks (Logical Management)
// Tier 4: Workloads & Containers (Physical Processes, Aligned Sub-trees)
// Tier 5: Storage & Volumes (Internal Storage)
// ──────────────────────────────────────────────
export function computeProjectFocusLayout({
	projectId,
	projects,
	services,
	containers,
	pods,
	domains,
	volumes,
	networks,
	server
}: {
	projectId: string;
	projects: Project[];
	services: Service[];
	containers: Container[];
	pods: Pod[];
	domains: Domain[];
	volumes: Volume[];
	networks: Network[];
	server?: Server;
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

	// Column X coordinates for the 5-zone architecture pipeline
	const colIngressX = 60;
	const colGatewayX = 360;
	const colServiceX = 660;
	const colRuntimeX = 980;
	const colStorageX = 1300;

	const startY = 110;
	const itemGap = 28;

	// ── 1. Layout Tier 3 (Services) & Tier 4 (Containers) with Sub-Tree Alignment ──
	let currentY = startY;
	const serviceNodeMap = new Map<string, TopologyNode>();
	const containerNodeMap = new Map<string, TopologyNode>();
	const handledContainerIds = new Set<string>();

	projServices.forEach((svc) => {
		// Find physical containers belonging to this service
		const matchedContainers = projContainers.filter(
			(c) =>
				c.serviceId === svc.id ||
				c.name === svc.name ||
				c.name.startsWith(`${svc.id}-`) ||
				c.name.startsWith(`${svc.id}_`) ||
				(c.labels &&
					(c.labels['io.gopod.service'] === svc.id ||
						c.labels['io.gopod.name'] === svc.name ||
						c.labels['com.docker.compose.service'] === svc.name ||
						c.labels['io.podman.compose.service'] === svc.name))
		);

		// If matched, mark them as handled
		matchedContainers.forEach((c) => handledContainerIds.add(c.id));

		// Determine container workloads for this service
		type WorkloadItem = {
			id: string;
			name: string;
			image: string;
			status: string;
			cpu?: number;
			memory?: number;
			raw: any;
			isPod?: boolean;
		};

		const workloadItems: WorkloadItem[] = [];

		if (matchedContainers.length > 0) {
			matchedContainers.forEach((c) => {
				workloadItems.push({
					id: `container-${c.id}`,
					name: c.name,
					image: c.image.split('/').pop() || c.image,
					status: c.status,
					cpu: c.cpu,
					memory: c.memory,
					raw: c
				});
			});
		} else if (svc.workloads && svc.workloads.length > 0) {
			svc.workloads.forEach((wl: any) => {
				workloadItems.push({
					id: `workload-${svc.id}-${wl.name}`,
					name: wl.name,
					image: wl.image || 'container:latest',
					status: wl.status || svc.status,
					cpu: wl.cpu,
					memory: wl.memory,
					raw: wl
				});
			});
		} else {
			// Single container representation for quadlet or standalone service
			workloadItems.push({
				id: `container-${svc.id}`,
				name: svc.type === 'quadlet' ? `${svc.name}.service` : svc.name,
				image: svc.image ? svc.image.split('/').pop() || svc.image : 'container:latest',
				status: svc.status,
				cpu: svc.cpu,
				memory: svc.memory,
				raw: svc
			});
		}

		// Calculate vertical height of container cluster
		const containerCount = workloadItems.length;
		const containerCardHeight = COMPACT_NODE_HEIGHT;
		const containerGap = 12;
		const clusterHeight =
			containerCount * containerCardHeight + Math.max(0, containerCount - 1) * containerGap;
		const blockHeight = Math.max(NODE_HEIGHT, clusterHeight);

		// Position Service Node (Vertically centered against its container cluster)
		const svcY = currentY + (blockHeight - NODE_HEIGHT) / 2;
		let subtitle = 'Application';
		if (svc.type === 'quadlet') subtitle = 'Quadlet (Systemd)';
		else if (svc.type === 'compose') subtitle = 'Compose Stack';
		else if (svc.type === 'kubernetes') subtitle = 'Kubernetes Pod';
		else if (svc.type === 'pod') subtitle = 'Podman Pod';
		else if (svc.type === 'database') subtitle = 'Database Engine';
		else if (svc.type === 'image') {
			const sname = svc.name.toLowerCase();
			subtitle =
				sname.includes('redis') ||
				sname.includes('postgres') ||
				sname.includes('mysql') ||
				sname.includes('mongo') ||
				sname.includes('db')
					? 'Database'
					: 'Container Image';
		}

		const svcNode: TopologyNode = {
			id: `svc-${svc.id}`,
			type: 'service',
			title: svc.name,
			subtitle,
			status: svc.status,
			x: colServiceX,
			y: Math.round(svcY),
			width: 240,
			height: NODE_HEIGHT,
			monoDetail: svc.port ? `:${svc.port}` : '',
			badge: svc.type,
			projectId: project.id,
			serviceId: svc.id,
			raw: svc
		};
		nodes.push(svcNode);
		serviceNodeMap.set(svc.id, svcNode);

		// Check if this service is in a Pod, or represents a Pod
		const matchingPod =
			projPods.find((p) => p.projectId === project.id && p.name.includes(svc.name)) ||
			(svc.runtimeTarget === 'pod'
				? { id: `pod-${svc.id}`, name: `${svc.name}-pod`, status: svc.status }
				: null);

		if (matchingPod) {
			const podRegion: TopologyRegion = {
				id: `region-pod-${matchingPod.id}`,
				type: 'pod',
				label: matchingPod.name,
				sublabel: `${containerCount} containers · localhost`,
				x: colRuntimeX - 10,
				y: Math.round(currentY - 8),
				width: 270,
				height: Math.round(blockHeight + 16),
				projectId: project.id,
				status: matchingPod.status,
				raw: matchingPod
			};
			regions.push(podRegion);
		}

		// Position Container Nodes directly alongside the Service Node
		const containerStartY = currentY + (blockHeight - clusterHeight) / 2;
		workloadItems.forEach((w, idx) => {
			const contY = Math.round(containerStartY + idx * (containerCardHeight + containerGap));
			const contNode: TopologyNode = {
				id: w.id,
				type: 'container',
				title: w.name,
				subtitle: w.image,
				status: w.status,
				x: colRuntimeX,
				y: contY,
				width: 250,
				height: containerCardHeight,
				monoDetail: `${(w.cpu || 0).toFixed(1)}% · ${w.memory || 0}MB`,
				badge: svc.type === 'compose' ? 'Compose' : svc.type === 'quadlet' ? 'Quadlet' : 'Container',
				projectId: project.id,
				serviceId: svc.id,
				raw: w.raw
			};
			nodes.push(contNode);
			containerNodeMap.set(w.name, contNode);
			containerNodeMap.set(w.id, contNode);

			// Connect Service -> Container with smooth Bézier curve
			edges.push({
				id: `edge-${svcNode.id}-${contNode.id}`,
				sourceId: svcNode.id,
				targetId: contNode.id,
				sourceX: svcNode.x + svcNode.width,
				sourceY: svcNode.y + svcNode.height / 2,
				targetX: contNode.x,
				targetY: contNode.y + contNode.height / 2,
				path: createCurvedBezierPath(
					svcNode.x + svcNode.width,
					svcNode.y + svcNode.height / 2,
					contNode.x,
					contNode.y + contNode.height / 2
				),
				label: svc.type === 'quadlet' ? 'systemd' : undefined,
				kind: svc.type === 'database' ? 'database' : 'workload'
			});
		});

		// Advance Y cleanly for the next service block
		currentY += blockHeight + itemGap;
	});

	// If project has standalone containers not bound to any service
	const standaloneContainers = projContainers.filter((c) => !handledContainerIds.has(c.id));
	if (standaloneContainers.length > 0) {
		standaloneContainers.forEach((c) => {
			const contNode: TopologyNode = {
				id: `container-${c.id}`,
				type: 'container',
				title: c.name,
				subtitle: c.image.split('/').pop() || c.image,
				status: c.status,
				x: colRuntimeX,
				y: Math.round(currentY),
				width: 250,
				height: COMPACT_NODE_HEIGHT,
				monoDetail: `${c.cpu.toFixed(1)}% · ${c.memory}MB`,
				badge: 'Standalone',
				projectId: project.id,
				raw: c
			};
			nodes.push(contNode);
			containerNodeMap.set(c.name, contNode);
			currentY += COMPACT_NODE_HEIGHT + itemGap;
		});
	}

	// Calculate overall vertical center of services to balance Ingress & Gateway
	const totalServicesHeight = Math.max(NODE_HEIGHT, currentY - startY);
	const gatewayCenterY = Math.round(startY + (totalServicesHeight - NODE_HEIGHT) / 2);

	// ── 2. Layout Tier 2: Caddy Ingress Gateway ──
	const caddyNode: TopologyNode = {
		id: 'caddy-gateway',
		type: 'proxy',
		title: 'Caddy Ingress',
		subtitle: 'Reverse Proxy & TLS Termination',
		status: 'active',
		x: colGatewayX,
		y: gatewayCenterY,
		width: 220,
		height: NODE_HEIGHT,
		monoDetail: 'ports 80, 443',
		badge: 'Caddy Gateway',
		raw: { server, domainsCount: projDomains.length }
	};
	nodes.push(caddyNode);

	// Connect Caddy Gateway -> All Services (Reverse Proxy Upstream Routing)
	projServices.forEach((svc) => {
		const svcNode = serviceNodeMap.get(svc.id);
		if (!svcNode) return;

		edges.push({
			id: `edge-caddy-${svcNode.id}`,
			sourceId: caddyNode.id,
			targetId: svcNode.id,
			sourceX: caddyNode.x + caddyNode.width,
			sourceY: caddyNode.y + caddyNode.height / 2,
			targetX: svcNode.x,
			targetY: svcNode.y + svcNode.height / 2,
			path: createCurvedBezierPath(
				caddyNode.x + caddyNode.width,
				caddyNode.y + caddyNode.height / 2,
				svcNode.x,
				svcNode.y + svcNode.height / 2
			),
			label: svc.port ? `:${svc.port}` : ':80',
			kind: 'ingress'
		});
	});

	// ── 3. Layout Tier 1: Ingress & WAN Entrypoints ──
	if (projDomains.length > 0) {
		// Stack Domains vertically around the Gateway Center
		const domainClusterHeight =
			projDomains.length * NODE_HEIGHT + Math.max(0, projDomains.length - 1) * 16;
		let domainStartY = Math.max(startY, Math.round(gatewayCenterY + (NODE_HEIGHT - domainClusterHeight) / 2));

		projDomains.forEach((dom) => {
			const domNode: TopologyNode = {
				id: `dom-${dom.id}`,
				type: 'domain',
				title: dom.hostname,
				subtitle: dom.tls ? 'Let’s Encrypt TLS' : 'HTTP Only',
				status: dom.status === 'active' ? 'running' : 'degraded',
				x: colIngressX,
				y: domainStartY,
				width: 220,
				height: NODE_HEIGHT,
				monoDetail: `proxy :${dom.proxyPort || dom.containerPort}`,
				badge: 'Domain',
				projectId: project.id,
				serviceId: dom.serviceId,
				raw: dom
			};
			nodes.push(domNode);

			// Connect Domain -> Caddy Gateway
			edges.push({
				id: `edge-dom-${dom.id}-caddy`,
				sourceId: domNode.id,
				targetId: caddyNode.id,
				sourceX: domNode.x + domNode.width,
				sourceY: domNode.y + domNode.height / 2,
				targetX: caddyNode.x,
				targetY: caddyNode.y + caddyNode.height / 2,
				path: createCurvedBezierPath(
					domNode.x + domNode.width,
					domNode.y + domNode.height / 2,
					caddyNode.x,
					caddyNode.y + caddyNode.height / 2
				),
				label: dom.tls ? 'TLS' : 'HTTP',
				kind: 'ingress'
			});

			domainStartY += NODE_HEIGHT + 16;
		});
	} else {
		// Default Public WAN Entrypoint if no custom domains are yet assigned
		const internetNode: TopologyNode = {
			id: 'ingress-internet-project',
			type: 'internet',
			title: 'Public Internet',
			subtitle: 'IPv4 / IPv6 Client Ingress',
			status: 'active',
			x: colIngressX,
			y: gatewayCenterY,
			width: 220,
			height: NODE_HEIGHT,
			monoDetail: server?.ip ? `${server.ip}` : '0.0.0.0/0',
			badge: 'WAN Ingress',
			raw: { ip: server?.ip || '0.0.0.0/0' }
		};
		nodes.push(internetNode);

		// Connect Internet -> Caddy Gateway
		edges.push({
			id: 'edge-internet-caddy-proj',
			sourceId: internetNode.id,
			targetId: caddyNode.id,
			sourceX: internetNode.x + internetNode.width,
			sourceY: internetNode.y + internetNode.height / 2,
			targetX: caddyNode.x,
			targetY: caddyNode.y + caddyNode.height / 2,
			path: createCurvedBezierPath(
				internetNode.x + internetNode.width,
				internetNode.y + internetNode.height / 2,
				caddyNode.x,
				caddyNode.y + caddyNode.height / 2
			),
			label: 'HTTPS',
			kind: 'ingress'
		});
	}

	// ── 4. Layout Tier 5: Storage & Volumes ──
	let storageY = startY;
	projVolumes.forEach((vol) => {
		const volNode: TopologyNode = {
			id: `vol-${vol.id}`,
			type: 'volume',
			title: vol.name,
			subtitle: `Mount: ${vol.mount || 'local'}`,
			status: 'active',
			x: colStorageX,
			y: Math.round(storageY),
			width: 210,
			height: COMPACT_NODE_HEIGHT,
			monoDetail: vol.size || 'Persistent',
			badge: 'Volume',
			projectId: project.id,
			serviceId: vol.serviceId,
			raw: vol
		};
		nodes.push(volNode);

		// Find related container/workload to connect
		let relatedSource: TopologyNode | undefined;
		if (vol.serviceId) {
			relatedSource =
				Array.from(containerNodeMap.values()).find((n) => n.serviceId === vol.serviceId) ||
				serviceNodeMap.get(vol.serviceId);
		} else {
			relatedSource = Array.from(containerNodeMap.values()).find((n) =>
				n.title.toLowerCase().includes(vol.name.toLowerCase())
			);
		}

		if (relatedSource) {
			edges.push({
				id: `edge-${relatedSource.id}-${volNode.id}`,
				sourceId: relatedSource.id,
				targetId: volNode.id,
				sourceX: relatedSource.x + relatedSource.width,
				sourceY: relatedSource.y + relatedSource.height / 2,
				targetX: volNode.x,
				targetY: volNode.y + volNode.height / 2,
				path: createCurvedBezierPath(
					relatedSource.x + relatedSource.width,
					relatedSource.y + relatedSource.height / 2,
					volNode.x,
					volNode.y + volNode.height / 2
				),
				dashed: true,
				label: 'mount',
				kind: 'storage'
			});
		}

		storageY += COMPACT_NODE_HEIGHT + itemGap;
	});

	if (projNetwork) {
		const netNode: TopologyNode = {
			id: `net-${projNetwork.id}`,
			type: 'network',
			title: projNetwork.name,
			subtitle: `Driver: ${projNetwork.driver}`,
			status: 'active',
			x: colStorageX,
			y: Math.round(storageY),
			width: 210,
			height: COMPACT_NODE_HEIGHT,
			monoDetail: projNetwork.subnet,
			badge: 'Network',
			projectId: project.id,
			raw: projNetwork
		};
		nodes.push(netNode);
	}

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}
