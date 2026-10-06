import type { Project, Service, Container, Pod, Domain, Volume, Network } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { NODE_HEIGHT, COMPACT_NODE_HEIGHT } from './constants';
import { createOrthogonalPath, computeBounds } from './path';

// ──────────────────────────────────────────────
// PROJECT FOCUS TOPOLOGY
// Domain → Service → Pod / Container (Territory) → Volume / Network
// ──────────────────────────────────────────────
export function computeProjectFocusLayout({
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

	// Column coordinates with routing channels
	const colDomainX = 80;
	const colServiceX = 390;
	const colRuntimeX = 730;
	const colStorageX = 1120;

	const startY = 100;
	const itemGap = 24;

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
			width: 240,
			height: NODE_HEIGHT,
			monoDetail: `proxy :${dom.proxyPort || dom.containerPort}`,
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
			y: serviceY,
			width: 250,
			height: NODE_HEIGHT,
			monoDetail: svc.port ? `:${svc.port}` : '',
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

	// ── 3. Runtime Layer: Pod Territories + Quadlet Units + Containers ──
	let runtimeY = startY;
	const handledContainerIds = new Set<string>();
	const handledServiceIds = new Set<string>();

	// Process Pods
	projPods.forEach((pod) => {
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
		const podWidth = 290;

		const podRegion: TopologyRegion = {
			id: `region-pod-${pod.id}`,
			type: 'pod',
			label: pod.name,
			sublabel: `${podContainers.length} containers · localhost`,
			x: colRuntimeX,
			y: runtimeY,
			width: podWidth,
			height: podHeight,
			projectId: project.id,
			status: pod.status,
			raw: pod
		};
		regions.push(podRegion);

		let containerInsideY = runtimeY + podHeaderHeight;
		podContainers.forEach((c) => {
			handledContainerIds.add(c.id);
			if (c.serviceId) handledServiceIds.add(c.serviceId);

			const containerNode: TopologyNode = {
				id: `container-${c.id}`,
				type: 'container',
				title: c.name,
				subtitle: c.image.split('/').pop() || c.image,
				status: c.status,
				x: colRuntimeX + 16,
				y: containerInsideY,
				width: podWidth - 32,
				height: containerCardHeight,
				monoDetail: `${c.cpu.toFixed(1)}% · ${c.memory}MB`,
				badge: 'Container',
				projectId: project.id,
				serviceId: c.serviceId,
				podId: pod.id,
				raw: c
			};
			nodes.push(containerNode);

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

	// Standalone Containers in projContainers (not in any Pod)
	const standaloneContainers = projContainers.filter((c) => !handledContainerIds.has(c.id));
	standaloneContainers.forEach((c) => {
		if (c.serviceId) handledServiceIds.add(c.serviceId);

		const containerNode: TopologyNode = {
			id: `container-${c.id}`,
			type: 'container',
			title: c.name,
			subtitle: c.image.split('/').pop() || c.image,
			status: c.status,
			x: colRuntimeX,
			y: runtimeY,
			width: 250,
			height: NODE_HEIGHT,
			monoDetail: `${c.cpu.toFixed(1)}% · ${c.memory}MB`,
			badge: 'Standalone',
			projectId: project.id,
			serviceId: c.serviceId,
			raw: c
		};
		nodes.push(containerNode);

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

	// Remaining services (Quadlets, Compose stacks, apps)
	projServices.forEach((svc) => {
		if (handledServiceIds.has(svc.id)) return;

		if (svc.type === 'quadlet') {
			const quadletNode: TopologyNode = {
				id: `runtime-quadlet-${svc.id}`,
				type: 'container',
				title: `${svc.name}.service`,
				subtitle: `Systemd · ${svc.source || svc.name + '.container'}`,
				status: svc.status,
				x: colRuntimeX,
				y: runtimeY,
				width: 250,
				height: NODE_HEIGHT,
				monoDetail: `${svc.cpu ? svc.cpu.toFixed(1) + '%' : '0.4%'} · ${svc.memory ? svc.memory + 'MB' : '128MB'}`,
				badge: 'Quadlet',
				projectId: project.id,
				serviceId: svc.id,
				raw: { ...svc, isQuadlet: true }
			};
			nodes.push(quadletNode);

			const parentServiceNode = nodes.find((n) => n.id === `svc-${svc.id}`);
			if (parentServiceNode) {
				edges.push({
					id: `edge-${parentServiceNode.id}-${quadletNode.id}`,
					sourceId: parentServiceNode.id,
					targetId: quadletNode.id,
					sourceX: parentServiceNode.x + parentServiceNode.width,
					sourceY: parentServiceNode.y + parentServiceNode.height / 2,
					targetX: quadletNode.x,
					targetY: quadletNode.y + quadletNode.height / 2,
					path: createOrthogonalPath(
						parentServiceNode.x + parentServiceNode.width,
						parentServiceNode.y + parentServiceNode.height / 2,
						quadletNode.x,
						quadletNode.y + quadletNode.height / 2
					),
					label: 'systemd'
				});
			}

			runtimeY += NODE_HEIGHT + itemGap;
			handledServiceIds.add(svc.id);
		} else if (svc.workloads && svc.workloads.length > 0) {
			svc.workloads.forEach((wl: any) => {
				const wlNode: TopologyNode = {
					id: `workload-${svc.id}-${wl.name}`,
					type: 'container',
					title: wl.name,
					subtitle: wl.image,
					status: wl.status || svc.status,
					x: colRuntimeX,
					y: runtimeY,
					width: 250,
					height: COMPACT_NODE_HEIGHT,
					monoDetail: `${wl.cpu || 0}% · ${wl.memory || 0}MB`,
					badge: 'Compose',
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
			handledServiceIds.add(svc.id);
		} else {
			const autoNode: TopologyNode = {
				id: `runtime-auto-${svc.id}`,
				type: 'container',
				title: svc.name,
				subtitle: svc.image || (svc.type === 'application' ? 'app:latest' : 'container:latest'),
				status: svc.status,
				x: colRuntimeX,
				y: runtimeY,
				width: 250,
				height: NODE_HEIGHT,
				monoDetail: `${svc.cpu ? svc.cpu.toFixed(1) + '%' : '0.0%'} · ${svc.memory ? svc.memory + 'MB' : '0MB'}`,
				badge: svc.type === 'database' ? 'Database' : 'Container',
				projectId: project.id,
				serviceId: svc.id,
				raw: svc
			};
			nodes.push(autoNode);

			const parentServiceNode = nodes.find((n) => n.id === `svc-${svc.id}`);
			if (parentServiceNode) {
				edges.push({
					id: `edge-${parentServiceNode.id}-${autoNode.id}`,
					sourceId: parentServiceNode.id,
					targetId: autoNode.id,
					sourceX: parentServiceNode.x + parentServiceNode.width,
					sourceY: parentServiceNode.y + parentServiceNode.height / 2,
					targetX: autoNode.x,
					targetY: autoNode.y + autoNode.height / 2,
					path: createOrthogonalPath(
						parentServiceNode.x + parentServiceNode.width,
						parentServiceNode.y + parentServiceNode.height / 2,
						autoNode.x,
						autoNode.y + autoNode.height / 2
					)
				});
			}
			runtimeY += NODE_HEIGHT + itemGap;
			handledServiceIds.add(svc.id);
		}
	});

	// ── 4. Storage (Volumes) & Network ──
	let storageY = startY;

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
