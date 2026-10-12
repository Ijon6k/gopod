import type { Project, Service, Container, Pod, Domain, Volume, Network, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { createCurvedBezierPath, computeBounds } from './path';

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
	const projContainers = containers.filter((c) => c.projectId === project.id);
	const projPods = pods.filter((p) => p.projectId === project.id);
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

	// ── 1. Map Workload Groups & Child Containers ──
	let currentWorkloadY = startY;
	const containerIdMap = new Map<string, string>(); // name/id -> flowNodeId
	const serviceToFirstContainerMap = new Map<string, string>(); // serviceId -> primaryContainerId
	const serviceGroupMap = new Map<string, TopologyNode>();
	const serviceYMap = new Map<string, { y: number; height: number; centerY: number }>();

	// Roomy & balanced card dimensions (not cramped, not oversized)
	const groupWidth = 360;
	const childWidth = 328;
	const childHeight = 62;
	const childGap = 10;
	const groupPaddingTop = 50;
	const groupPaddingBottom = 16;

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

		type WorkloadItem = {
			id: string;
			name: string;
			image: string;
			status: string;
			cpu?: number;
			memory?: number;
			raw: any;
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
			// Single container fallback
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

		// Calculate Workload Dimensions
		const childCount = workloadItems.length;
		const groupHeight = Math.max(
			105,
			groupPaddingTop + childCount * (childHeight + childGap) - childGap + groupPaddingBottom
		);

		const isPod =
			svc.runtimeTarget === 'pod' ||
			svc.type === 'pod' ||
			projPods.some((p) => p.name.includes(svc.name) || p.id === svc.id);

		const groupType = isPod
			? 'pod'
			: svc.type === 'compose'
				? 'compose'
				: svc.type === 'quadlet'
					? 'quadlet'
					: 'service';

		const totalCpu = workloadItems.reduce((acc, curr) => acc + (curr.cpu || 0), 0);

		// 1A. Create Native Svelte Flow Parent Group Node (High contrast & draggable)
		const groupNode: TopologyNode = {
			id: `group-${svc.id}`,
			type: 'group',
			isGroup: true,
			groupType,
			title: svc.name,
			subtitle: `${childCount} container${childCount > 1 ? 's' : ''} · ${totalCpu.toFixed(1)}% CPU`,
			status: svc.status,
			x: colWorkloadsX,
			y: currentWorkloadY,
			width: groupWidth,
			height: groupHeight,
			monoDetail: svc.port ? `:${svc.port}` : '',
			badge:
				groupType === 'compose'
					? 'Compose Stack'
					: groupType === 'quadlet'
						? 'Quadlet Unit'
						: groupType === 'pod'
							? 'Podman Pod'
							: 'Service',
			projectId: project.id,
			serviceId: svc.id,
			raw: svc
		};
		nodes.push(groupNode);
		serviceGroupMap.set(svc.id, groupNode);

		const svcCenterY = currentWorkloadY + Math.min(groupHeight / 2, 45);
		serviceYMap.set(svc.id, {
			y: currentWorkloadY,
			height: groupHeight,
			centerY: svcCenterY
		});

		// 1B. Create Child Container Nodes (Relative positioning within parent group)
		workloadItems.forEach((w, idx) => {
			const childNode: TopologyNode = {
				id: w.id,
				type: 'container',
				parentId: groupNode.id,
				title: w.name,
				subtitle: w.image,
				status: w.status,
				x: 16,
				y: groupPaddingTop + idx * (childHeight + childGap),
				width: childWidth,
				height: childHeight,
				monoDetail: `${(w.cpu || 0).toFixed(1)}% · ${w.memory || 0}MB`,
				badge:
					groupType === 'compose'
						? 'Compose'
						: groupType === 'quadlet'
							? 'Quadlet'
							: groupType === 'pod'
								? 'Pod Member'
								: 'Container',
				projectId: project.id,
				serviceId: svc.id,
				raw: w.raw
			};
			nodes.push(childNode);
			containerIdMap.set(w.name, childNode.id);
			containerIdMap.set(w.id, childNode.id);

			if (idx === 0) {
				serviceToFirstContainerMap.set(svc.id, childNode.id);
			}
		});

		// 22px spacing between service groups (comfortable, not spread out)
		currentWorkloadY += groupHeight + 24;
	});

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

	// Inspect containers for published ports
	projContainers.forEach((c) => {
		const targetNodeId = containerIdMap.get(c.id) || containerIdMap.get(c.name);
		if (!targetNodeId) return;

		if (c.ports) {
			const portDefs = Array.isArray(c.ports) ? c.ports : [c.ports];
			portDefs.forEach((pStr: string) => {
				const match = pStr.match(/(?:([0-9.]+):)?([0-9]+)(?:->|:)([0-9]+)/);
				if (match) {
					const hostIp = match[1] || '0.0.0.0';
					const hostPort = parseInt(match[2], 10);
					const containerPort = parseInt(match[3], 10);
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
							id: `port-${hostIp.replace(/\./g, '_')}-${hostPort}`,
							hostPort,
							containerPort,
							hostIp,
							targetContainerId: targetNodeId,
							serviceId: c.serviceId || ''
						});
					}
				}
			});
		}
	});

	// Also check services with explicit hostPort
	projServices.forEach((svc) => {
		if (svc.hostPort && svc.hostPort > 0) {
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
			const domNode: TopologyNode = {
				id: `dom-${dom.id}`,
				type: 'domain',
				title: dom.hostname,
				subtitle: dom.tls ? 'Let’s Encrypt TLS' : 'HTTP Only',
				status: dom.status === 'active' ? 'running' : 'degraded',
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

			// Connect Caddy Ingress (Col 2) -> Target Service Workload (Col 3)
			const targetContainerId = serviceToFirstContainerMap.get(dom.serviceId);
			const targetServiceMetrics = serviceYMap.get(dom.serviceId);
			const targetConnectY = targetServiceMetrics ? targetServiceMetrics.centerY : caddyNode.y + 28;

			if (targetContainerId) {
				edges.push({
					id: `edge-caddy-target-${dom.id}-${targetContainerId}`,
					sourceId: caddyNode.id,
					targetId: targetContainerId,
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

			// Connect containers using this volume
			projContainers.forEach((c) => {
				const hasVol =
					c.serviceId === v.serviceId ||
					(c as any).mounts?.some((m: any) => m?.name === v.name || m?.source?.includes(v.name));
				const contNodeId = containerIdMap.get(c.id);
				if (hasVol && contNodeId) {
					edges.push({
						id: `edge-${contNodeId}-${volNode.id}`,
						sourceId: contNodeId,
						targetId: volNode.id,
						sourceX: colWorkloadsX + groupWidth,
						sourceY: targetVolY + 26,
						targetX: volNode.x,
						targetY: targetVolY + 26,
						path: createCurvedBezierPath(colWorkloadsX + groupWidth, targetVolY + 26, volNode.x, targetVolY + 26),
						kind: 'storage',
						dashed: true
					});
				}
			});

			volY += 52 + 14;
		});
	}

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}
