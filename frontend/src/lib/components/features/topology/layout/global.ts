import type { Project, Service, Domain, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { COMPACT_NODE_HEIGHT } from './constants';
import { createCurvedBezierPath, computeBounds } from './path';

// ──────────────────────────────────────────────
// GLOBAL INFRASTRUCTURE TOPOLOGY
// Internet → Caddy Reverse Proxy → Projects (Native Draggable Groups)
// ──────────────────────────────────────────────
export function computeGlobalLayout({
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

	const groupPaddingTop = 50;
	const groupPaddingBottom = 16;
	const itemSpacing = 10;
	const regionWidth = 360;
	const regionGap = 32;
	const projectRegionX = 620;

	// Pre-calculate project territory heights to vertically center WAN Entrypoint & Caddy Gateway
	let calculatedTotalHeight = 0;
	projects.forEach((proj, idx) => {
		const projServices = services.filter((s) => s.projectId === proj.id);
		const svcCount = Math.max(1, projServices.length);
		const rHeight = Math.max(
			120,
			groupPaddingTop + svcCount * (COMPACT_NODE_HEIGHT + itemSpacing) - itemSpacing + groupPaddingBottom
		);
		calculatedTotalHeight += rHeight + (idx < projects.length - 1 ? regionGap : 0);
	});

	const gatewayCenterY = Math.max(120, Math.round(40 + (calculatedTotalHeight - 62) / 2));

	// 1. Internet Ingress Node
	const internetNode: TopologyNode = {
		id: 'ingress-internet',
		type: 'internet',
		title: 'Public Internet',
		subtitle: 'IPv4 / IPv6 Traffic',
		status: 'active',
		x: 60,
		y: gatewayCenterY,
		width: 170,
		height: 62,
		monoDetail: '0.0.0.0/0',
		badge: 'WAN',
		raw: { ip: server.ip, os: server.os }
	};
	nodes.push(internetNode);

	// 2. Caddy Reverse Proxy Node
	const caddyNode: TopologyNode = {
		id: 'proxy-caddy',
		type: 'proxy',
		title: 'Caddy Proxy',
		subtitle: 'Reverse Proxy & TLS',
		status: server.status === 'online' ? 'running' : 'degraded',
		x: 300,
		y: gatewayCenterY,
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
		path: createCurvedBezierPath(
			internetNode.x + internetNode.width,
			internetNode.y + internetNode.height / 2,
			caddyNode.x,
			caddyNode.y + caddyNode.height / 2
		),
		label: 'HTTPS',
		kind: 'ingress'
	});

	// 3. Project Territories Layout (Native Svelte Flow Parent Groups)
	let currentY = 40;

	projects.forEach((proj) => {
		const projServices = services.filter((s) => s.projectId === proj.id);
		const projDomains = domains.filter((d) => d.projectId === proj.id);

		// Height based on child service count
		const svcCount = Math.max(1, projServices.length);
		const regionHeight = Math.max(
			120,
			groupPaddingTop + svcCount * (COMPACT_NODE_HEIGHT + itemSpacing) - itemSpacing + groupPaddingBottom
		);

		// 3A. Native Svelte Flow Parent Group Node
		const projectGroupNode: TopologyNode = {
			id: `group-project-${proj.id}`,
			type: 'group',
			isGroup: true,
			groupType: 'project',
			title: proj.name,
			subtitle: `${projServices.length} service${projServices.length === 1 ? '' : 's'} · ${proj.cpu.toFixed(1)}% CPU`,
			status: proj.status,
			x: projectRegionX,
			y: currentY,
			width: regionWidth,
			height: regionHeight,
			badge: 'Project',
			projectId: proj.id,
			raw: proj
		};
		nodes.push(projectGroupNode);

		// 3B. Place child service nodes inside project group (Relative Coordinates)
		projServices.forEach((svc, idx) => {
			let svcSubtitle = 'Application';
			if (svc.type === 'quadlet') svcSubtitle = 'Quadlet Service';
			else if (svc.type === 'compose') svcSubtitle = 'Compose Stack';
			else if (svc.type === 'kubernetes') svcSubtitle = 'Kubernetes Pod';
			else if (svc.type === 'pod') svcSubtitle = 'Podman Pod';
			else if (svc.type === 'database') svcSubtitle = 'Database Engine';
			else if (svc.type === 'image') {
				const sname = svc.name.toLowerCase();
				svcSubtitle =
					sname.includes('redis') ||
					sname.includes('postgres') ||
					sname.includes('mysql') ||
					sname.includes('mongo') ||
					sname.includes('db')
						? 'Database'
						: 'Container Image';
			}

			const childRelativeX = 16;
			const childRelativeY = groupPaddingTop + idx * (COMPACT_NODE_HEIGHT + itemSpacing);
			const childWidth = regionWidth - 32;

			const svcNode: TopologyNode = {
				id: `svc-${svc.id}`,
				type: 'service',
				parentId: projectGroupNode.id,
				title: svc.name,
				subtitle: svcSubtitle,
				status: svc.status,
				x: childRelativeX,
				y: childRelativeY,
				width: childWidth,
				height: COMPACT_NODE_HEIGHT,
				monoDetail: svc.port ? `:${svc.port}` : '',
				badge: svc.type,
				projectId: proj.id,
				serviceId: svc.id,
				raw: svc
			};
			nodes.push(svcNode);

			const matchingDomains = projDomains.filter((d) => d.serviceId === svc.id);

			// Edge routing:
			// 1. Domain-managed ingress routes through Caddy
			if (matchingDomains.length > 0) {
				edges.push({
					id: `edge-caddy-svc-${svc.id}`,
					sourceId: caddyNode.id,
					targetId: svcNode.id,
					sourceX: caddyNode.x + caddyNode.width,
					sourceY: caddyNode.y + caddyNode.height / 2,
					targetX: projectRegionX + childRelativeX,
					targetY: currentY + childRelativeY + COMPACT_NODE_HEIGHT / 2,
					path: createCurvedBezierPath(
						caddyNode.x + caddyNode.width,
						caddyNode.y + caddyNode.height / 2,
						projectRegionX + childRelativeX,
						currentY + childRelativeY + COMPACT_NODE_HEIGHT / 2
					),
					label: matchingDomains[0]?.hostname || (svc.port ? `:${svc.port}` : undefined),
					kind: 'ingress'
				});
			} else if (svc.hostPort) {
				// 2. Direct host port routes directly from Internet (bypassing Caddy)
				edges.push({
					id: `edge-wan-svc-${svc.id}`,
					sourceId: internetNode.id,
					targetId: svcNode.id,
					sourceX: internetNode.x + internetNode.width,
					sourceY: internetNode.y + internetNode.height / 2,
					targetX: projectRegionX + childRelativeX,
					targetY: currentY + childRelativeY + COMPACT_NODE_HEIGHT / 2,
					path: createCurvedBezierPath(
						internetNode.x + internetNode.width,
						internetNode.y + internetNode.height / 2,
						projectRegionX + childRelativeX,
						currentY + childRelativeY + COMPACT_NODE_HEIGHT / 2
					),
					label: `:${svc.hostPort}`,
					kind: 'direct-port'
				});
			}
			// 3. Otherwise, internal workload with no incoming external edge
		});

		currentY += regionHeight + regionGap;
	});

	const bounds = computeBounds(nodes, regions);
	return { nodes, regions, edges, bounds };
}
