import type { Project, Service, Domain, Server } from '$lib/types';
import type { TopologyNode, TopologyRegion, TopologyEdge, TopologyGraph } from '../types';
import { COMPACT_NODE_HEIGHT } from './constants';
import { createOrthogonalPath, computeBounds } from './path';

// ──────────────────────────────────────────────
// GLOBAL INFRASTRUCTURE TOPOLOGY
// Internet → Caddy Reverse Proxy → Projects (Territories)
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
		const regionWidth = 480;

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
			let svcSubtitle = 'Application';
			if (svc.type === 'quadlet') svcSubtitle = 'Quadlet Service';
			else if (svc.type === 'compose') svcSubtitle = 'Compose Stack';
			else if (svc.type === 'kubernetes') svcSubtitle = 'Kubernetes Pod';
			else if (svc.type === 'pod') svcSubtitle = 'Podman Pod';
			else if (svc.type === 'database') svcSubtitle = 'Database Engine';
			else if (svc.type === 'image') {
				const sname = svc.name.toLowerCase();
				svcSubtitle = sname.includes('redis') || sname.includes('postgres') || sname.includes('mysql') || sname.includes('mongo') || sname.includes('db') ? 'Database' : 'Container Image';
			}

			const svcNode: TopologyNode = {
				id: `svc-${svc.id}`,
				type: 'service',
				title: svc.name,
				subtitle: svcSubtitle,
				status: svc.status,
				x: projectRegionX + 24,
				y: svcY,
				width: 240,
				height: COMPACT_NODE_HEIGHT,
				monoDetail: svc.port ? `:${svc.port}` : '',
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
