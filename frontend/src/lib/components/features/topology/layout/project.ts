import type { Project, Service, Container, Pod, Domain, Volume, Network, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { createCurvedBezierPath, computeBounds } from './path';
import { podServiceNames } from '../pod-naming';
import {
	ownerServiceId,
	buildContainerPodMap,
	runtimePodService,
	type PodAssignment
} from './project/ownership';
import {
	buildPodGroups,
	buildOrphanPodGroups,
	buildServiceGroups,
	buildUnmanagedGroup,
	type GroupBuilderContext
} from './project/groups';

// ──────────────────────────────────────────────
// REALISTIC TOPOLOGY ENGINE — HIERARCHICAL SERVICE INGRESS & DRAGGABLE GROUPS
// Tier 1: Ingress Entrypoints (Domains & Direct Host Ports 0.0.0.0 / 127.0.0.1 aligned per Service)
// Tier 2: Caddy Ingress Gateway (Reverse Proxy / TLS — ONLY connected to domain targets)
// Tier 3: Workload Groups (Compose Stacks, Podman Pods, Quadlet Units — Native Svelte Flow Parent Nodes)
// Tier 4: Storage & Volumes (Internal Persistence)
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
	// Container pool: metadata project link OR labels pointing into this
	// project (metadata attribution is fuzzy, labels are truth).
	const projServiceIds = new Set(projServices.map((s) => s.id));
	const projContainers = containers.filter(
		(c) =>
			c.projectId === project.id ||
			(c.labels &&
				(c.labels['io.gopod.project'] === project.id ||
					projServiceIds.has(c.labels['io.gopod.service']) ||
					projServiceIds.has(c.labels['com.docker.compose.project']) ||
					projServiceIds.has(c.labels['io.podman.compose.project'])))
	);
	// Pods are global daemon objects; matched to services by canonical
	// unique name below, never by substring.
	const allPods = pods;
	const projDomains = domains.filter((d) => d.projectId === project.id);
	const projVolumes = volumes.filter((v) => v.projectId === project.id);

	const nodes: TopologyNode[] = [];
	const regions: TopologyRegion[] = [];
	const edges: TopologyEdge[] = [];

	// Balanced Column Coordinates (Neither too spaced out nor too tight)
	const colIngressX = 60;
	const colGatewayX = 330;
	const colWorkloadsX = 620;
	const colStorageX = 1060;

	const startY = 60;

	// ── 1. Assign every container to exactly one owner (no double rendering) ──
	const containerPodMap = buildContainerPodMap(allPods);
	const containerAssignment = new Map<string, PodAssignment>();
	for (const c of projContainers) {
		const podName = containerPodMap.get(c.name);
		if (podName) {
			const owner = runtimePodService(podName, projServices);
			containerAssignment.set(c.id, {
				kind: 'pod',
				podName,
				podServiceId: owner ? owner.id : null
			});
		}
	}

	const ctx: GroupBuilderContext = {
		projectId: project.id,
		nodes,
		containerIdMap: new Map<string, string>(),
		serviceToFirstContainerMap: new Map<string, string>(),
		serviceGroupMap: new Map<string, TopologyNode>(),
		serviceYMap: new Map<string, { y: number; height: number; centerY: number }>(),
		currentWorkloadY: startY,
		groupWidth: 360,
		childWidth: 328,
		childHeight: 62,
		childGap: 10,
		groupPaddingTop: 50,
		groupPaddingBottom: 16,
		colWorkloadsX
	};
	const assignedContainerIds = new Set<string>();

	// One group per runtime pod (daemon truth), never one per member.
	const podServices = projServices.filter((s) => s.type === 'pod' || s.type === 'kubernetes');
	const renderedPodNames = buildPodGroups(ctx, {
		podServices,
		allPods,
		projContainers,
		projServices,
		projServiceIds,
		containerAssignment,
		assignedContainerIds
	});
	buildOrphanPodGroups(ctx, {
		allPods,
		projContainers,
		projServices,
		projServiceIds,
		projectId: project.id,
		containerAssignment,
		assignedContainerIds,
		renderedPodNames
	});
	buildServiceGroups(ctx, {
		projServices,
		projContainers,
		projServiceIds,
		assignedContainerIds
	});
	buildUnmanagedGroup(ctx, projContainers, assignedContainerIds);

	const { containerIdMap, serviceToFirstContainerMap, serviceYMap } = ctx;
	let currentWorkloadY = ctx.currentWorkloadY;

	// ── 1C. Localhost edges: pod members <-> infra (in-namespace traffic) ──
	const podInfraNodeId = new Map<string, string>(); // pod group id -> infra child id
	for (const n of nodes) {
		if (n.type === 'container' && n.badge === 'Pod Infra' && n.parentId) {
			if (!podInfraNodeId.has(n.parentId)) podInfraNodeId.set(n.parentId, n.id);
		}
	}
	for (const n of nodes) {
		if (n.type === 'container' && n.badge === 'Pod Member' && n.parentId) {
			const infraId = podInfraNodeId.get(n.parentId);
			if (!infraId) continue;
			edges.push({
				id: `edge-localhost-${n.id}`,
				sourceId: n.id,
				targetId: infraId,
				sourceX: 0,
				sourceY: 0,
				targetX: 0,
				targetY: 0,
				path: '',
				label: 'localhost',
				kind: 'localhost',
				dashed: true
			});
		}
	}

	// ── 2. Discover Direct Host Ports (Non-Caddy / Standalone Ports) ──
	type DiscoveredPort = {
		id: string;
		hostPort: number;
		containerPort: number;
		hostIp: string;
		targetContainerId: string;
		serviceId: string;
	};

	const directPorts: DiscoveredPort[] = [];
	const seenPortKeys = new Set<string>();

	const validPort = (p: number) => Number.isInteger(p) && p >= 1 && p <= 65535;

	// Inspect containers for published ports. The runtime reports one
	// comma-joined string ("18401:80/tcp, 18403:6379/tcp"): split into
	// tokens first, otherwise only the first binding ever renders.
	projContainers.forEach((c) => {
		const targetNodeId = containerIdMap.get(c.id) || containerIdMap.get(c.name);
		if (!targetNodeId) return;

		if (c.ports) {
			const portDefs = (
				Array.isArray(c.ports) ? c.ports.flatMap((p) => String(p).split(',')) : [c.ports]
			).flatMap((p) => String(p).split(/[\s,]+/));
			portDefs.forEach((pStr: string) => {
				const token = pStr.trim();
				if (!token) return;
				const match = token.match(
					/(?:([0-9a-fA-F.:\[\]]+):)?([0-9]+)(?:->|:)([0-9]+)(?:\/(tcp|udp))?/i
				);
				if (!match) return;
				const hostIp = match[1] || '0.0.0.0';
				const hostPort = parseInt(match[2], 10);
				const containerPort = parseInt(match[3], 10);
				if (!validPort(hostPort) || !validPort(containerPort)) return;
				const key = `${hostIp}:${hostPort}`;

				// Check if this port is exclusively managed by a Domain in Caddy
				const isDomainPort = projDomains.some(
					(d) =>
						d.serviceId === c.serviceId &&
						(d.containerPort === containerPort || d.containerPort === hostPort)
				);

				if (!seenPortKeys.has(key) && !isDomainPort && hostPort > 0) {
					seenPortKeys.add(key);
					directPorts.push({
						id: `port-${hostIp.replace(/[^a-zA-Z0-9]/g, '_')}-${hostPort}`,
						hostPort,
						containerPort,
						hostIp,
						targetContainerId: targetNodeId,
						serviceId: c.serviceId || ''
					});
				}
			});
		}
	});

	// Service-level hostPort fallback: only when the runtime reported NO
	// ports for this service. Otherwise stale service.port values (e.g. an
	// 8080 default) would render phantom port nodes.
	projServices.forEach((svc) => {
		if (svc.hostPort && svc.hostPort > 0 && validPort(svc.hostPort)) {
			if (directPorts.some((p) => p.serviceId === svc.id)) return;
			const key = `0.0.0.0:${svc.hostPort}`;
			const isDomainPort = projDomains.some((d) => d.serviceId === svc.id);
			if (!seenPortKeys.has(key) && !isDomainPort) {
				seenPortKeys.add(key);
				const targetId = serviceToFirstContainerMap.get(svc.id);
				if (targetId) {
					directPorts.push({
						id: `port-0_0_0_0-${svc.hostPort}`,
						hostPort: svc.hostPort,
						containerPort: svc.port || svc.hostPort,
						hostIp: '0.0.0.0',
						targetContainerId: targetId,
						serviceId: svc.id
					});
				}
			}
		}
	});

	// ── 3. Render Ingress Architecture — Caddy Centered as Reverse Proxy ──
	// Caddy sits in the vertical center of the topology (prominent reverse proxy vibe)
	const totalWorkloadsSpan = Math.max(220, currentWorkloadY - startY);
	const caddyCenterY = Math.max(startY + 20, Math.round(startY + (totalWorkloadsSpan - 56) / 2));

	const caddyNode: TopologyNode = {
		id: 'caddy-gateway',
		type: 'proxy',
		title: 'Caddy Ingress',
		subtitle:
			projDomains.length > 0
				? 'Reverse Proxy & TLS Termination'
				: 'Reverse Proxy (Awaiting Ingress)',
		status: 'active',
		x: colGatewayX,
		y: caddyCenterY,
		width: 200,
		height: 56,
		monoDetail: 'ports 80, 443',
		badge: 'Caddy Gateway',
		raw: { server, domainsCount: projDomains.length }
	};
	nodes.push(caddyNode);

	// 3A. Layout Tier 1: Public Domains in Column 1 (colIngressX = 60)
	// All domains cluster neatly opposite Caddy in the center; ALL traffic funnels through Caddy
	const domainNodes: TopologyNode[] = [];
	if (projDomains.length > 0) {
		const domItemH = 54;
		const domGap = 12;
		const totalDomSpan = projDomains.length * domItemH + (projDomains.length - 1) * domGap;
		let curDomY = Math.max(startY, Math.round(caddyCenterY + 28 - totalDomSpan / 2));

		projDomains.forEach((dom) => {
			// A domain whose target service/container does not exist is
			// degraded, not silently unconnected.
			const targetContainerId = serviceToFirstContainerMap.get(dom.serviceId);
			const targetExists =
				!!targetContainerId || serviceYMap.has(dom.serviceId);
			const domNode: TopologyNode = {
				id: `dom-${dom.id}`,
				type: 'domain',
				title: dom.hostname,
				subtitle: dom.tls ? 'Let’s Encrypt TLS' : 'HTTP Only',
				status: dom.status === 'active' && targetExists ? 'running' : 'degraded',
				x: colIngressX,
				y: curDomY,
				width: 200,
				height: domItemH,
				monoDetail: `:${dom.containerPort || dom.proxyPort || 80}`,
				badge: 'Domain',
				projectId: project.id,
				serviceId: dom.serviceId,
				raw: dom
			};
			nodes.push(domNode);
			domainNodes.push(domNode);

			// Connect Domain (Col 1) -> Caddy Ingress (Col 2)
			edges.push({
				id: `edge-${domNode.id}-caddy`,
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

			// Connect Caddy Ingress (Col 2) -> Target Service Workload (Col 3).
			// Prefer the live container; fall back to the service/pod group
			// node so the cable always exists (a member with no live
			// container still shows a cable to its pod group).
			let caddyTargetId = targetContainerId;
			if (!caddyTargetId) {
				const fallbackGroup = ctx.serviceGroupMap.get(dom.serviceId);
				if (fallbackGroup) caddyTargetId = fallbackGroup.id;
			}
			const targetServiceMetrics = serviceYMap.get(dom.serviceId);
			const targetConnectY = targetServiceMetrics ? targetServiceMetrics.centerY : caddyNode.y + 28;

			if (caddyTargetId) {
				edges.push({
					id: `edge-caddy-target-${dom.id}-${caddyTargetId}`,
					sourceId: caddyNode.id,
					targetId: caddyTargetId,
					sourceX: caddyNode.x + caddyNode.width,
					sourceY: caddyNode.y + caddyNode.height / 2,
					targetX: colWorkloadsX,
					targetY: targetConnectY,
					path: createCurvedBezierPath(
						caddyNode.x + caddyNode.width,
						caddyNode.y + caddyNode.height / 2,
						colWorkloadsX,
						targetConnectY
					),
					label: dom.hostname,
					kind: 'ingress'
				});
			}

			curDomY += domItemH + domGap;
		});
	}

	// 3B. Layout Standalone Host Ports in Column 2 ("tetep kedepan tapi dibelakangnya caddy")
	// Positioned along the gateway column avoiding Caddy, routing directly into their target service
	const occupiedYRanges: { start: number; end: number }[] = [
		{ start: caddyNode.y - 14, end: caddyNode.y + caddyNode.height + 14 }
	];

	projServices.forEach((svc) => {
		const svcMetrics = serviceYMap.get(svc.id) || { y: startY, height: 110, centerY: startY + 45 };
		const svcDirectPorts = directPorts.filter((p) => p.serviceId === svc.id);

		svcDirectPorts.forEach((dp) => {
			let portY = svcMetrics.centerY - 27;

			// Collision avoidance in Column 2 (do not overlap Caddy or earlier placed ports)
			for (let attempt = 0; attempt < 12; attempt++) {
				const hasCollision = occupiedYRanges.some(
					(range) => portY < range.end && portY + 54 > range.start
				);
				if (!hasCollision) break;
				// Shift above or below Caddy cleanly
				if (portY < caddyNode.y + 28) {
					portY -= 64;
				} else {
					portY += 64;
				}
			}

			occupiedYRanges.push({ start: portY, end: portY + 54 });

			const isPublic = dp.hostIp === '0.0.0.0' || dp.hostIp === '';
			const portNode: TopologyNode = {
				id: dp.id,
				type: 'port',
				title: `${isPublic ? '0.0.0.0' : '127.0.0.1'}:${dp.hostPort}`,
				subtitle: isPublic ? 'Public Host Port' : 'Localhost Loopback',
				status: 'active',
				x: colGatewayX,
				y: portY,
				width: 200,
				height: 54,
				monoDetail: `-> :${dp.containerPort}`,
				badge: isPublic ? 'Public WAN' : 'Host Only',
				projectId: project.id,
				serviceId: dp.serviceId,
				raw: dp
			};
			nodes.push(portNode);

			// Direct Host Port (Col 2) -> Target Container (Col 3)
			edges.push({
				id: `edge-${dp.id}-${dp.targetContainerId}`,
				sourceId: dp.id,
				targetId: dp.targetContainerId,
				sourceX: portNode.x + portNode.width,
				sourceY: portNode.y + portNode.height / 2,
				targetX: colWorkloadsX,
				targetY: portNode.y + portNode.height / 2,
				path: createCurvedBezierPath(
					portNode.x + portNode.width,
					portNode.y + portNode.height / 2,
					colWorkloadsX,
					portNode.y + portNode.height / 2
				),
				label: `:${dp.hostPort}->:${dp.containerPort}`,
				kind: isPublic ? 'direct-port' : 'localhost-port',
				dashed: !isPublic
			});
		});
	});

	// Orphan Direct Ports (unassigned to any service)
	const unassignedPorts = directPorts.filter((p) => !p.serviceId || !serviceYMap.has(p.serviceId));
	if (unassignedPorts.length > 0) {
		let orphanY = currentWorkloadY;
		unassignedPorts.forEach((dp) => {
			for (let attempt = 0; attempt < 10; attempt++) {
				const hasCollision = occupiedYRanges.some(
					(range) => orphanY < range.end && orphanY + 54 > range.start
				);
				if (!hasCollision) break;
				orphanY += 64;
			}
			occupiedYRanges.push({ start: orphanY, end: orphanY + 54 });

			const isPublic = dp.hostIp === '0.0.0.0' || dp.hostIp === '';
			const portNode: TopologyNode = {
				id: dp.id,
				type: 'port',
				title: `${isPublic ? '0.0.0.0' : '127.0.0.1'}:${dp.hostPort}`,
				subtitle: isPublic ? 'Public Host Port' : 'Localhost Loopback',
				status: 'active',
				x: colGatewayX,
				y: orphanY,
				width: 200,
				height: 54,
				monoDetail: `-> :${dp.containerPort}`,
				badge: isPublic ? 'Public WAN' : 'Host Only',
				projectId: project.id,
				serviceId: dp.serviceId,
				raw: dp
			};
			nodes.push(portNode);

			edges.push({
				id: `edge-${dp.id}-${dp.targetContainerId}`,
				sourceId: dp.id,
				targetId: dp.targetContainerId,
				sourceX: portNode.x + portNode.width,
				sourceY: portNode.y + portNode.height / 2,
				targetX: colWorkloadsX,
				targetY: portNode.y + portNode.height / 2,
				path: createCurvedBezierPath(
					portNode.x + portNode.width,
					portNode.y + portNode.height / 2,
					colWorkloadsX,
					portNode.y + portNode.height / 2
				),
				label: `:${dp.hostPort}->:${dp.containerPort}`,
				kind: isPublic ? 'direct-port' : 'localhost-port',
				dashed: !isPublic
			});

			orphanY += 54 + 10;
		});
	}

	// ── 4. Layout Tier 4: Storage & Volumes ──
	if (projVolumes.length > 0) {
		let volY = startY;
		projVolumes.forEach((v) => {
			// Align with matched service if possible
			const matchedServiceMetrics = v.serviceId ? serviceYMap.get(v.serviceId) : undefined;
			const targetVolY = matchedServiceMetrics ? matchedServiceMetrics.centerY - 26 : volY;

			const volNode: TopologyNode = {
				id: `vol-${v.id}`,
				type: 'volume',
				title: v.name,
				subtitle: v.driver || 'local volume',
				status: 'active',
				x: colStorageX,
				y: targetVolY,
				width: 190,
				height: 52,
				monoDetail: v.size || '',
				badge: 'Volume',
				projectId: project.id,
				raw: v
			};
			nodes.push(volNode);

			// Connect containers using this volume (strict owner match +
			// explicit mount reference; never bare-name guessing).
			projContainers.forEach((c) => {
				const ownerId = ownerServiceId(c, projServices, projServiceIds);
				const hasVol =
					(v.serviceId && ownerId === v.serviceId) ||
					(c as any).mounts?.some((m: any) => m?.name === v.name || m?.source?.includes(v.name));
				const contNodeId = containerIdMap.get(c.id);
				if (hasVol && contNodeId) {
					edges.push({
						id: `edge-${contNodeId}-${volNode.id}`,
						sourceId: contNodeId,
						targetId: volNode.id,
						sourceX: colWorkloadsX + 360,
						sourceY: targetVolY + 26,
						targetX: volNode.x,
						targetY: targetVolY + 26,
						path: createCurvedBezierPath(
							colWorkloadsX + 360,
							targetVolY + 26,
							volNode.x,
							targetVolY + 26
						),
						kind: 'storage',
						dashed: true
					});
				}
			});

			volY += 52 + 14;
		});
	}

	// Merge ingress cables sharing identical endpoints (e.g. two domains
	// targeting one pod): physically one cable, so render one edge labeled
	// with both hostnames instead of two perfectly overlapping lines that
	// look like a single missing cable.
	const ingressGroups = new Map<string, TopologyEdge[]>();
	for (const e of edges) {
		if (e.kind !== 'ingress') continue;
		const k = `${e.sourceId}->${e.targetId}`;
		const g = ingressGroups.get(k);
		if (g) g.push(e);
		else ingressGroups.set(k, [e]);
	}
	for (const g of ingressGroups.values()) {
		if (g.length < 2) continue;
		const [first, ...rest] = g;
		const labels = [first.label, ...rest.map((e) => e.label)].filter(Boolean);
		const uniqueLabels = [...new Set(labels)];
		first.label = uniqueLabels.join(' + ');
		for (const e of rest) {
			const idx = edges.indexOf(e);
			if (idx >= 0) edges.splice(idx, 1);
		}
	}

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}
