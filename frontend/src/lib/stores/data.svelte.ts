// ──────────────────────────────────────────────
// GOPOD — Reactive Data Store (Svelte 5)
// ──────────────────────────────────────────────

import type {
	Project,
	Service,
	Deployment,
	Container,
	Pod,
	Image,
	Volume,
	Network,
	Domain,
	Server,
	PodmanSecret,
	SSHKey,
	ContainerRegistry,
	PortMapping,
	CaddyAccessLog,
	VolumeSnapshot,
	VolumeBackupSchedule
} from '$lib/types';

import { api } from '$lib/api';


import projectsJson from '$lib/data/mock/dummy_projects.json';
import servicesJson from '$lib/data/mock/dummy_services.json';
import deploymentsJson from '$lib/data/mock/dummy_deployments.json';
import containersJson from '$lib/data/mock/dummy_containers.json';
import podsJson from '$lib/data/mock/dummy_pods.json';
import imagesJson from '$lib/data/mock/dummy_images.json';
import volumesJson from '$lib/data/mock/dummy_volumes.json';
import networksJson from '$lib/data/mock/dummy_networks.json';
import domainsJson from '$lib/data/mock/dummy_domains.json';
import serverJson from '$lib/data/mock/dummy_server.json';
import portsJson from '$lib/data/mock/dummy_ports.json';
import accessLogsJson from '$lib/data/mock/dummy_accessLogs.json';
import volumeSnapshotsJson from '$lib/data/mock/dummy_volumeSnapshots.json';
import volumeSchedulesJson from '$lib/data/mock/dummy_volumeSchedules.json';

export const initialPodmanSecrets: PodmanSecret[] = [
	{ id: 'sec-1', name: 'db_password', createdAt: '2025-01-15T08:00:00Z', driver: 'file' },
	{ id: 'sec-2', name: 'jwt_secret', createdAt: '2025-01-15T08:05:00Z', driver: 'file' },
	{ id: 'sec-3', name: 'session_key', createdAt: '2025-02-01T12:00:00Z', driver: 'file' },
	{ id: 'sec-4', name: 'redis_auth', createdAt: '2025-02-10T14:30:00Z', driver: 'file' }
];

export const initialSSHKeys: SSHKey[] = [
	{
		id: 'key-1',
		name: 'Default Deployment Key',
		publicKey: 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIInJ8tO01n9eW4M8pY2jV7qB5c0z6X1aF3gT7hU9kL2m gopod-deploy',
		fingerprint: 'SHA256:d8a2f1b0c9e8d7c6b5a4938271605f4e',
		type: 'ed25519',
		createdAt: '2025-01-10T10:00:00Z'
	},
	{
		id: 'key-2',
		name: 'Personal GitHub (ed25519)',
		publicKey: 'ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKmP5oQ2rS4tU6vW8xY0zA1bC3dE5fG7hI9jK1lM3nO5 pixy@workstation',
		fingerprint: 'SHA256:4a3b2c1d0e9f8a7b6c5d4e3f2a1b0c9d',
		type: 'ed25519',
		createdAt: '2025-02-14T08:30:00Z'
	}
];

export const initialRegistries: ContainerRegistry[] = [
	{
		id: 'reg-1',
		name: 'Docker Hub (Default)',
		url: 'docker.io',
		username: 'ijon6k',
		createdAt: '2025-01-12T12:00:00Z'
	},
	{
		id: 'reg-2',
		name: 'GitHub Packages (ghcr.io)',
		url: 'ghcr.io',
		username: 'ijon6k',
		createdAt: '2025-02-01T09:15:00Z'
	}
];

class DataStore {
	projects = $state<Project[]>([]);
	services = $state<Service[]>([]);
	deployments = $state<Deployment[]>([]);
	containers = $state<Container[]>([]);
	pods = $state<Pod[]>([]);
	images = $state<Image[]>([]);
	volumes = $state<Volume[]>([]);
	networks = $state<Network[]>([]);
	domains = $state<Domain[]>([]);
	server = $state<Server>({
		hostname: 'localhost',
		ip: '127.0.0.1',
		os: 'Linux',
		kernel: '—',
		vcpu: 4,
		memory: 16,
		storage: 100,
		storageUsed: 20,
		memoryUsed: 2,
		cpuUsage: 5,
		status: 'online',
		podmanVersion: '5.x',
		rootless: true,
		systemd: true,
		quadlet: true,
		caddy: '2.x',
		podmanHealth: 'healthy',
		networkHealth: 'healthy',
		storageHealth: 'healthy',
		uptime: 'Active'
	});
	ports = $state<PortMapping[]>([]);
	accessLogs = $state<CaddyAccessLog[]>([]);
	volumeSnapshots = $state<VolumeSnapshot[]>([]);
	volumeSchedules = $state<VolumeBackupSchedule[]>([]);
	podmanSecrets = $state<PodmanSecret[]>(initialPodmanSecrets);
	sshKeys = $state<SSHKey[]>(initialSSHKeys);
	registries = $state<ContainerRegistry[]>(initialRegistries);

	addSSHKey(key: Omit<SSHKey, 'id' | 'createdAt'>) {
		const newKey: SSHKey = {
			...key,
			id: `key-${Date.now()}`,
			createdAt: new Date().toISOString()
		};
		this.sshKeys.push(newKey);
		return newKey;
	}

	deleteSSHKey(id: string) {
		this.sshKeys = this.sshKeys.filter((k) => k.id !== id);
	}

	addRegistry(reg: Omit<ContainerRegistry, 'id' | 'createdAt'>) {
		const newReg: ContainerRegistry = {
			...reg,
			id: `reg-${Date.now()}`,
			createdAt: new Date().toISOString()
		};
		this.registries.push(newReg);
		return newReg;
	}

	deleteRegistry(id: string) {
		this.registries = this.registries.filter((r) => r.id !== id);
	}

	getProjectServices(projectId: string): Service[] {
		return this.services.filter((s) => s.projectId === projectId);
	}

	getProjectDomains(projectId: string): Domain[] {
		return this.domains.filter((d) => d.projectId === projectId);
	}

	getProjectDeployments(projectId: string): Deployment[] {
		return this.deployments.filter((d) => d.projectId === projectId);
	}

	getServiceDeployments(serviceId: string): Deployment[] {
		return this.deployments.filter((d) => d.serviceId === serviceId);
	}

	deleteDeployment(deploymentId: string) {
		this.deployments = this.deployments.filter((d) => d.id !== deploymentId);
	}

	clearServiceDeployments(serviceId: string, keepActive = true) {
		const serviceDeps = this.getServiceDeployments(serviceId);
		if (serviceDeps.length === 0) return;

		if (keepActive) {
			// Keep the most recent deployment (or running one)
			const activeId = serviceDeps[0]?.id;
			this.deployments = this.deployments.filter(
				(d) => d.serviceId !== serviceId || d.id === activeId
			);
		} else {
			this.deployments = this.deployments.filter((d) => d.serviceId !== serviceId);
		}
	}

	cancelDeployment(deploymentId: string) {
		const dep = this.deployments.find((d) => d.id === deploymentId);
		if (dep && (dep.status === 'deploying' || dep.status === 'building')) {
			dep.status = 'cancelled';
			dep.duration = 'Cancelled';
			dep.finishedAt = new Date().toISOString();
		}
	}

	getServiceContainers(serviceId: string): Container[] {
		return this.containers.filter((c) => c.serviceId === serviceId);
	}

	getProjectById(projectId: string): Project | undefined {
		return this.projects.find((p) => p.id === projectId);
	}

	async createProject(data: { name: string; description?: string }): Promise<Project> {
		const res = await api.projects.create(data);
		const newProject: Project = {
			id: res.id,
			name: res.name,
			description: res.description || '',
			status: 'healthy',
			services: [],
			domains: [],
			cpu: 0,
			memory: 0,
			memoryTotal: 0,
			createdAt: res.createdAt || new Date().toISOString()
		};
		const idx = this.projects.findIndex((p) => p.id === newProject.id);
		if (idx >= 0) {
			this.projects[idx] = newProject;
		} else {
			this.projects.push(newProject);
		}
		return newProject;
	}

	async deleteProject(id: string): Promise<void> {
		await api.projects.delete(id);
		this.projects = this.projects.filter((p) => p.id !== id);
	}

	getServiceById(serviceId: string): Service | undefined {
		return this.services.find((s) => s.id === serviceId);
	}

	async addService(service: Service): Promise<Service> {
		try {
			const res = await api.services.create(service);
			const fullService: Service = {
				...service,
				...res,
				quadletConfig: service.quadletConfig || (res as any).quadletConfig,
				composeYaml: service.composeYaml || (res as any).composeYaml,
				k8sYaml: service.k8sYaml || (res as any).k8sYaml,
				description: service.description || res.description || '',
				deployments: service.deployments || []
			};
			const idx = this.services.findIndex((s) => s.id === fullService.id);
			if (idx >= 0) {
				this.services[idx] = fullService;
			} else {
				this.services.unshift(fullService);
			}

			// If it has domain, add it to domains
			if (service.domain) {
				this.addDomain({
					id: `d-${Date.now()}`,
					hostname: service.domain,
					projectId: service.projectId,
					serviceId: fullService.id,
					serviceName: service.name,
					tls: true,
					status: 'active',
					proxyPort: 8000 + Math.floor(Math.random() * 900),
					containerPort: service.port || 3000,
					publishedPort: service.port || 3000
				});
			}
			// If it's a pod, add to pods
			if (service.type === 'pod') {
				this.pods.unshift({
					id: `pod-${fullService.id}`,
					name: service.name,
					projectId: service.projectId,
					projectName: this.getProjectById(service.projectId)?.name ?? service.projectId,
					containers: service.workloads?.map((w) => `${fullService.id}-${w.name}`) ?? [service.name],
					status: service.status,
					network: `${service.projectId}-network`,
					createdAt: service.createdAt
				});
			}
			return fullService;
		} catch (err) {
			const idx = this.services.findIndex((s) => s.id === service.id);
			if (idx < 0) {
				this.services.unshift(service);
			}
			throw err;
		}
	}

	async updateService(updated: Service): Promise<Service> {
		const idx = this.services.findIndex((s) => s.id === updated.id);
		if (idx !== -1) {
			this.services[idx] = { ...updated };
		}
		try {
			const res = await api.services.update(updated.id, updated);
			return res;
		} catch (err) {
			console.warn('Failed to sync service update to backend:', err);
			return updated;
		}
	}

	async deleteService(id: string): Promise<void> {
		try {
			await api.services.delete(id);
		} catch (err) {
			console.warn('Failed to delete service on backend:', err);
		}
		this.services = this.services.filter((s) => s.id !== id);
		this.domains = this.domains.filter((d) => d.serviceId !== id);
	}

	async deployService(serviceId: string) {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'deploying';
		}
		try {
			const res = await api.services.deploy(serviceId);
			if (svc) {
				svc.status = 'running';
			}
			return res;
		} catch (err) {
			console.error('Service deployment error:', err);
			if (svc) {
				svc.status = 'failed';
			}
			throw err;
		}
	}

	addDomain(domain: Domain) {
		this.domains.unshift(domain);
		api.domains.create(domain).catch((err) => console.warn('Failed to sync domain:', err));
	}

	updateDomain(updated: Domain) {
		const idx = this.domains.findIndex((d) => d.id === updated.id);
		if (idx !== -1) {
			this.domains[idx] = { ...updated };
		}
		api.domains.update(updated.id, updated).catch((err) => console.warn('Failed to sync domain update:', err));
	}

	deleteDomain(domainId: string) {
		this.domains = this.domains.filter((d) => d.id !== domainId);
		api.domains.delete(domainId).catch((err) => console.warn('Failed to sync domain delete:', err));
	}

	getProjectVolumeSnapshots(projectId: string): VolumeSnapshot[] {
		return this.volumeSnapshots.filter((s) => s.projectId === projectId);
	}

	getProjectVolumeSchedules(projectId: string): VolumeBackupSchedule[] {
		return this.volumeSchedules.filter((s) => s.projectId === projectId);
	}

	createVolumeSnapshot(volumeName: string, projectId: string, serviceId: string): VolumeSnapshot {
		const dateStr = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 16);
		const newSnapshot: VolumeSnapshot = {
			id: `snap-${Date.now()}`,
			volumeName,
			projectId,
			serviceId,
			filename: `${volumeName}_${dateStr}.tar.zst`,
			size: 'Calculating...',
			sizeBytes: 1024 * 1024 * 50,
			createdAt: new Date().toISOString(),
			timeAgo: 'Just now',
			status: 'completed',
			compression: 'zstd'
		};
		this.volumeSnapshots.unshift(newSnapshot);
		api.volumes.createSnapshot({ volumeName, projectId, serviceId }).catch(() => null);
		return newSnapshot;
	}

	deleteVolumeSnapshot(id: string) {
		this.volumeSnapshots = this.volumeSnapshots.filter((s) => s.id !== id);
		api.volumes.deleteSnapshot(id).catch(() => null);
	}

	toggleVolumeSchedule(id: string) {
		const sched = this.volumeSchedules.find((s) => s.id === id);
		if (sched) {
			sched.enabled = !sched.enabled;
		}
		api.volumes.toggleSchedule(id).catch(() => null);
	}

	async startContainer(id: string) {
		const found = this.containers.find((c) => c.id === id);
		if (found) found.status = 'running';
		await api.runtime.containers.start(id);
	}

	async stopContainer(id: string) {
		const found = this.containers.find((c) => c.id === id);
		if (found) found.status = 'stopped';
		await api.runtime.containers.stop(id);
	}

	async restartContainer(id: string) {
		await api.runtime.containers.restart(id);
	}

	async deleteContainer(id: string, force = false) {
		this.containers = this.containers.filter((c) => c.id !== id);
		await api.runtime.containers.delete(id, force);
	}

	async pruneImages(all = false) {
		await api.runtime.images.prune(all);
	}

	async pruneVolumes() {
		await api.runtime.volumes.prune();
	}

	async pruneSystem() {
		await api.system.prune();
	}

	addSecret(name: string) {
		const newSecret: PodmanSecret = {
			id: `sec-${Date.now()}`,
			name,
			createdAt: new Date().toISOString(),
			driver: 'file'
		};
		this.podmanSecrets.push(newSecret);
		return newSecret;
	}

	async fetchInitialData() {
		try {
			const [projectsData, servicesData, domainsData] = await Promise.all([
				api.projects.list().catch(() => null),
				api.services.list().catch(() => null),
				api.domains.list().catch(() => null)
			]);

			if (projectsData && projectsData.length > 0) {
				this.projects = projectsData.map((p: any) => ({
					id: p.id,
					name: p.name,
					description: p.description || '',
					status: (p.status || 'healthy') as any,
					services: [],
					domains: [],
					cpu: p.cpu || 0,
					memory: p.memory || 0,
					memoryTotal: p.memoryTotal || 0,
					createdAt: p.createdAt || new Date().toISOString()
				}));
			}
			if (servicesData && servicesData.length > 0) {
				this.services = servicesData;
			}
			if (domainsData && domainsData.length > 0) {
				this.domains = domainsData;
			}
		} catch (err) {
			// Ignore
		}

		await Promise.all([
			this.fetchRuntimeData(),
			this.fetchLiveStats()
		]);
	}

	async fetchRuntimeData() {
		try {
			const [podsData, imagesData, volumesData, networksData] = await Promise.all([
				api.runtime.pods.list().catch(() => []),
				api.runtime.images.list().catch(() => []),
				api.runtime.volumes.list().catch(() => []),
				api.runtime.networks.list().catch(() => [])
			]);

			if (Array.isArray(podsData)) {
				this.pods = podsData.map((p: any) => ({
					id: p.id,
					name: p.name,
					projectId: 'system',
					projectName: 'Podman Pod',
					containers: p.containers || [],
					status: p.status?.toLowerCase() === 'running' ? 'running' : 'stopped',
					network: p.network || 'bridge',
					createdAt: p.created || 'Recent'
				}));
			}

			if (Array.isArray(imagesData)) {
				this.images = imagesData.map((img: any) => ({
					id: img.id,
					name: img.name || img.repository || 'image',
					tag: img.tag || 'latest',
					size: img.size || '—',
					usedBy: 'Active',
					createdAt: img.createdAt || 'Recent'
				}));
			}

			if (Array.isArray(volumesData)) {
				this.volumes = volumesData.map((v: any) => ({
					id: v.name,
					name: v.name,
					projectId: 'system',
					projectName: 'Host Storage',
					serviceId: 'volume',
					serviceName: v.driver || 'local',
					mount: v.mount || v.mountPoint || '—',
					size: v.size || 'Active',
					status: 'mounted'
				}));
			}

			if (Array.isArray(networksData)) {
				this.networks = networksData.map((n: any) => ({
					id: n.id,
					name: n.name,
					driver: n.driver || 'bridge',
					projects: [n.name],
					containers: 0,
					subnet: n.subnet || '—',
					gateway: n.gateway || '—'
				}));
			}
		} catch (e) {
			console.warn('Error fetching runtime data:', e);
		}
	}

	private sseSource: EventSource | null = null;
	isStreaming = $state<boolean>(false);

	startStreamingStats() {
		if (typeof window === 'undefined') return;
		if (this.sseSource) return;

		try {
			this.sseSource = new EventSource('/api/stats/stream');
			this.isStreaming = true;

			this.sseSource.onmessage = (event) => {
				try {
					const data = JSON.parse(event.data);
					if (data.system) {
						const sys = data.system;
						this.server = {
							...this.server,
							hostname: sys.hostname || this.server.hostname,
							os: sys.os || this.server.os,
							kernel: sys.kernel || this.server.kernel,
							vcpu: sys.vcpu || this.server.vcpu,
							memory: parseFloat(sys.memoryTotalGB?.toFixed(1)) || this.server.memory,
							memoryUsed: parseFloat(sys.memoryUsedGB?.toFixed(1)) || this.server.memoryUsed,
							memoryAvailable: sys.memoryAvailableGB != null ? parseFloat(sys.memoryAvailableGB.toFixed(1)) : (this.server.memory - this.server.memoryUsed),
							swapUsed: sys.swapUsedMB != null ? sys.swapUsedMB : this.server.swapUsed,
							swapTotal: sys.swapTotalGB != null ? sys.swapTotalGB : this.server.swapTotal,
							cpuUsage: parseFloat(sys.cpuUsage?.toFixed(1)) || this.server.cpuUsage,
							podmanVersion: sys.podmanVersion || this.server.podmanVersion,
							rootless: sys.rootless ?? this.server.rootless,
							uptime: sys.uptime || this.server.uptime,
							status: 'online'
						};
					}

					if (data.stats && Array.isArray(data.stats) && data.stats.length > 0) {
						this.applyStatsUpdate(data.stats);
					}
				} catch (e) {
					// Ignore parse error
				}
			};

			this.sseSource.onerror = () => {
				// EventSource reconnects automatically
			};
		} catch (e) {
			this.fetchLiveStats();
		}
	}

	stopStreamingStats() {
		if (this.sseSource) {
			this.sseSource.close();
			this.sseSource = null;
			this.isStreaming = false;
		}
	}

	private applyStatsUpdate(stats: any[]) {
		for (const s of stats) {
			const found = this.containers.find((c) => c.name === s.name || c.id === s.id);
			if (found) {
				found.cpu = s.cpuPercent ? parseFloat(s.cpuPercent.toFixed(1)) : found.cpu;
				found.pids = s.pids ?? found.pids;
				if (s.memUsage) {
					found.memory = Math.round(s.memUsage / (1024 * 1024));
				}
				if (s.memLimit) {
					found.memoryLimit = Math.round(s.memLimit / (1024 * 1024));
				}
				if (s.netDisplay) {
					found.netRx = s.netDisplay.split('/')[0]?.trim();
					found.netTx = s.netDisplay.split('/')[1]?.trim();
				}
			}
		}
	}

	async fetchLiveStats() {
		try {
			const [sys, rawContainers, stats] = await Promise.all([
				api.system.info().catch(() => null),
				api.runtime.containers.list().catch(() => null),
				api.system.stats().catch(() => null)
			]);

			if (sys) {
				this.server = {
					...this.server,
					hostname: sys.hostname || this.server.hostname,
					os: sys.os || this.server.os,
					kernel: sys.kernel || this.server.kernel,
					vcpu: sys.vcpu || this.server.vcpu,
					memory: parseFloat(sys.memoryTotalGB?.toFixed(1)) || this.server.memory,
					memoryUsed: parseFloat(sys.memoryUsedGB?.toFixed(1)) || this.server.memoryUsed,
					memoryAvailable: sys.memoryAvailableGB != null ? parseFloat(sys.memoryAvailableGB.toFixed(1)) : (this.server.memory - this.server.memoryUsed),
					swapUsed: sys.swapUsedMB != null ? sys.swapUsedMB : this.server.swapUsed,
					swapTotal: sys.swapTotalGB != null ? sys.swapTotalGB : this.server.swapTotal,
					cpuUsage: parseFloat(sys.cpuUsage?.toFixed(1)) || this.server.cpuUsage,
					podmanVersion: sys.podmanVersion || this.server.podmanVersion,
					rootless: sys.rootless ?? this.server.rootless,
					uptime: sys.uptime || this.server.uptime,
					status: 'online'
				};
			}

			if (rawContainers && Array.isArray(rawContainers)) {
				const statsMap = new Map<string, any>();
				if (stats && Array.isArray(stats)) {
					for (const s of stats) {
						if (s.id) statsMap.set(s.id, s);
						if (s.name) statsMap.set(s.name, s);
					}
				}

				this.containers = rawContainers.map((rc: any) => {
					const name = rc.names && rc.names.length > 0 ? rc.names[0].replace(/^\//, '') : rc.id;
					const stat = statsMap.get(rc.id) || statsMap.get(name);

					let projName = 'System Host';
					let projId = 'system';
					let servName = name;
					let servId = name;

					const parts = name.split('-');
					if (parts.length >= 3) {
						const potentialProj = this.projects.find((p) => p.id === parts[0] || p.name.toLowerCase() === parts[0]);
						if (potentialProj) {
							projId = potentialProj.id;
							projName = potentialProj.name;
							servName = parts.slice(1, parts.length - 1).join('-');
							servId = `${projId}-${servName}`;
						}
					} else if (name.startsWith('todo-') || name.startsWith('todo_')) {
						projId = 'todo';
						projName = 'Todo App';
						servName = name.replace(/^todo[-_]/, '');
						servId = name;
					}

					const isRunning = rc.state === 'running' || (rc.status && rc.status.toLowerCase().includes('up'));

					return {
						id: rc.id,
						name: name,
						projectId: projId,
						projectName: projName,
						serviceId: servId,
						serviceName: servName,
						image: rc.image || 'unknown',
						status: isRunning ? 'running' : 'stopped',
						cpu: stat?.cpuPercent ? parseFloat(stat.cpuPercent.toFixed(1)) : 0,
						memory: stat?.memUsage ? Math.round(stat.memUsage / (1024 * 1024)) : 0,
						memoryLimit: stat?.memLimit ? Math.round(stat.memLimit / (1024 * 1024)) : 512,
						ports: rc.ports || '—',
						startedAt: rc.status || (isRunning ? 'Active' : 'Stopped'),
						netRx: stat?.netDisplay ? stat.netDisplay.split('/')[0]?.trim() : '—',
						netTx: stat?.netDisplay ? stat.netDisplay.split('/')[1]?.trim() : '—',
						blockRead: '—',
						blockWrite: '—',
						pids: stat?.pids || (isRunning ? 1 : 0),
						restarts: 0,
						uptime: rc.status || (isRunning ? 'Active' : 'Stopped')
					};
				});
			} else if (stats && Array.isArray(stats) && stats.length > 0) {
				this.applyStatsUpdate(stats);
			}
		} catch (e) {
			// Silently ignore
		}
	}
}

export const dataStore = new DataStore();
