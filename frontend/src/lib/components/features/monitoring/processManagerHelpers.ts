import type { Container, Project, Service } from '$lib/types';

export interface ServiceNode {
	service: Service;
	containers: Container[];
	totalCpu: number;
	totalMem: number;
	totalPids: number;
}

export interface ProjectNode {
	project: Project;
	serviceNodes: ServiceNode[];
	totalCpu: number;
	totalMem: number;
	totalContainers: number;
}

export function isAnomaly(c: Container): boolean {
	return (c.cpu ?? 0) >= 2.0 || (c.memory ?? 0) >= 400 || (c.status !== 'running' && c.status !== 'healthy');
}

export function getCpuColor(cpu: number): string {
	if (cpu >= 5.0) return 'bg-[var(--status-red)]';
	if (cpu >= 2.0) return 'bg-[var(--status-amber)]';
	return 'bg-[var(--accent)]';
}

export function getMemPercent(used: number, limit: number): number {
	return Math.min(Math.round((used / (limit || 512)) * 100), 100);
}

export function getMemColor(percent: number): string {
	if (percent >= 85) return 'bg-[var(--status-red)]';
	if (percent >= 70) return 'bg-[var(--status-amber)]';
	return 'bg-[var(--status-green)]';
}

export function buildProjectTree(
	allProjects: Project[],
	allContainers: Container[],
	getProjectServices: (projectId: string) => Service[],
	searchQuery: string,
	filterAnomaliesOnly: boolean,
	sortBy: 'cpu' | 'memory' | 'name' | 'pids',
	sortDesc: boolean
): ProjectNode[] {
	const projects = [...allProjects];
	const query = searchQuery.toLowerCase().trim();

	const registeredProjectIds = new Set(projects.map((p) => p.id));
	const unassignedContainers = allContainers.filter(
		(c) => !c.projectId || c.projectId === 'system' || !registeredProjectIds.has(c.projectId)
	);

	if (unassignedContainers.length > 0) {
		const hostProject: Project = {
			id: 'system',
			name: 'System Host & Standalone',
			description: 'Host infrastructure, daemons, and standalone containers',
			status: 'running',
			services: [],
			domains: [],
			cpu: 0,
			memory: 0,
			memoryTotal: 0,
			createdAt: 'Host'
		};
		projects.unshift(hostProject);
	}

	let result = projects
		.map((project) => {
			let services: Service[] = [];
			if (project.id === 'system') {
				const serviceGroups = new Map<string, Container[]>();
				for (const c of unassignedContainers) {
					const groupKey = c.serviceName || c.name;
					if (!serviceGroups.has(groupKey)) {
						serviceGroups.set(groupKey, []);
					}
					serviceGroups.get(groupKey)!.push(c);
				}

				services = Array.from(serviceGroups.entries()).map(([name, conts]) => ({
					id: `sys-${name}`,
					projectId: 'system',
					name: name,
					type: (conts.length > 1 ? 'compose' : 'image') as import('$lib/types').ServiceType,
					status: conts.some((c) => c.status === 'running') ? 'running' : 'stopped',
					source: 'local',
					port: 0,
					cpu: 0,
					memory: 0,
					restartPolicy: 'unless-stopped',
					health: 'healthy',
					replicas: conts.length,
					envVars: [],
					deployments: [],
					createdAt: 'Host'
				}));
			} else {
				services = getProjectServices(project.id);
			}

			let serviceNodes: ServiceNode[] = services
				.map((service) => {
					let containers = project.id === 'system'
						? unassignedContainers.filter((c) => (c.serviceName || c.name) === service.name)
						: allContainers.filter((c) => c.serviceId === service.id);

					if (filterAnomaliesOnly) {
						containers = containers.filter(isAnomaly);
					}

					containers.sort((a, b) => {
						let valA: any = a[sortBy] ?? 0;
						let valB: any = b[sortBy] ?? 0;
						if (typeof valA === 'string') {
							return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
						}
						return sortDesc ? valB - valA : valA - valB;
					});

					const totalCpu = containers.reduce((acc, c) => acc + (c.cpu || 0), 0);
					const totalMem = containers.reduce((acc, c) => acc + (c.memory || 0), 0);
					const totalPids = containers.reduce((acc, c) => acc + (c.pids || 0), 0);

					const matchesQuery =
						!query ||
						service.name.toLowerCase().includes(query) ||
						containers.some(
							(c) =>
								c.name.toLowerCase().includes(query) ||
								c.image.toLowerCase().includes(query)
						);

					return {
						service,
						containers,
						totalCpu,
						totalMem,
						totalPids,
						matchesQuery
					};
				})
				.filter((sNode) => {
					if (filterAnomaliesOnly && sNode.containers.length === 0) return false;
					return sNode.matchesQuery || sNode.containers.length > 0;
				});

			const totalCpu = serviceNodes.reduce((acc, s) => acc + s.totalCpu, 0);
			const totalMem = serviceNodes.reduce((acc, s) => acc + s.totalMem, 0);
			const totalContainers = serviceNodes.reduce((acc, s) => acc + s.containers.length, 0);

			const projectMatchesQuery =
				!query ||
				project.name.toLowerCase().includes(query) ||
				serviceNodes.length > 0;

			return {
				project,
				serviceNodes,
				totalCpu,
				totalMem,
				totalContainers,
				matchesQuery: projectMatchesQuery
			};
		})
		.filter((pNode) => {
			if (filterAnomaliesOnly && pNode.totalContainers === 0) return false;
			return pNode.matchesQuery && (pNode.serviceNodes.length > 0 || !query);
		});

	result.sort((a, b) => {
		let valA: any = sortBy === 'cpu' ? a.totalCpu : sortBy === 'memory' ? a.totalMem : a.project.name;
		let valB: any = sortBy === 'cpu' ? b.totalCpu : sortBy === 'memory' ? b.totalMem : b.project.name;
		if (typeof valA === 'string') {
			return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
		}
		return sortDesc ? valB - valA : valA - valB;
	});

	return result;
}
