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
	ContainerRegistry
} from '$lib/types';

import projectsJson from '$lib/data/projects.json';
import servicesJson from '$lib/data/services.json';
import deploymentsJson from '$lib/data/deployments.json';
import containersJson from '$lib/data/containers.json';
import podsJson from '$lib/data/pods.json';
import imagesJson from '$lib/data/images.json';
import volumesJson from '$lib/data/volumes.json';
import networksJson from '$lib/data/networks.json';
import domainsJson from '$lib/data/domains.json';
import serverJson from '$lib/data/server.json';

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
	projects = $state<Project[]>(projectsJson as Project[]);
	services = $state<Service[]>([
		// Ensure AeroChat has a rich Pod service for testing Pod-specific operations
		{
			id: 'aerochat-realtime',
			projectId: 'aerochat',
			name: 'realtime',
			type: 'pod',
			status: 'running',
			source: '',
			domain: 'ws.example.com',
			port: 8080,
			cpu: 2.5,
			memory: 328,
			restartPolicy: 'always',
			health: 'healthy',
			replicas: 1,
			description: 'Podman Pod grouping WebSocket listener, cache worker, and telemetry',
			workloads: [
				{ name: 'ws-gateway', image: 'quay.io/joko/aerochat-ws:v3.2', status: 'running', cpu: 1.4, memory: 180, ports: '8080' },
				{ name: 'redis-cache', image: 'redis:7-alpine', status: 'running', cpu: 0.3, memory: 48, ports: '6379' },
				{ name: 'metrics-exporter', image: 'prom/statsd-exporter:latest', status: 'running', cpu: 0.1, memory: 32 }
			],
			envVars: [
				{ key: 'PORT', value: '8080', secret: false },
				{ key: 'REDIS_HOST', value: '127.0.0.1', secret: false },
				{ key: 'LOG_LEVEL', value: 'info', secret: false }
			],
			secretMounts: [
				{ secretId: 'sec-2', secretName: 'jwt_secret', type: 'env', envVar: 'JWT_SECRET' }
			],
			deployments: [],
			createdAt: '2024-10-12',
			advanced: {
				runtime: {
					mode: 'rootless',
					userNamespace: 'keep-id',
					devices: []
				},
				lifecycle: {
					quadletEnabled: true,
					systemdUnitName: 'aerochat-realtime.pod',
					restartPolicy: 'always'
				},
				security: {
					privileged: false,
					selinuxLabel: 'container_file_t',
					capAdd: ['NET_BIND_SERVICE'],
					capDrop: ['ALL'],
					noNewPrivileges: true
				},
				storage: {
					volumes: [{ source: 'redis_data', target: '/data', options: 'Z' }]
				},
				resources: {
					cpuLimit: '2.0',
					memoryLimit: '512MB',
					pidsLimit: 2048,
					swapLimit: '0'
				}
			}
		},
		...(servicesJson as Service[])
	]);
	deployments = $state<Deployment[]>(deploymentsJson as Deployment[]);
	containers = $state<Container[]>(containersJson as Container[]);
	pods = $state<Pod[]>(podsJson as Pod[]);
	images = $state<Image[]>(imagesJson as Image[]);
	volumes = $state<Volume[]>(volumesJson as Volume[]);
	networks = $state<Network[]>(networksJson as Network[]);
	domains = $state<Domain[]>(domainsJson as Domain[]);
	server = $state<Server>(serverJson as Server);
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

	getServiceById(serviceId: string): Service | undefined {
		return this.services.find((s) => s.id === serviceId);
	}

	addService(service: Service) {
		this.services.unshift(service);
		// If it has domain, add it to domains
		if (service.domain) {
			this.addDomain({
				id: `d-${Date.now()}`,
				hostname: service.domain,
				projectId: service.projectId,
				serviceId: service.id,
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
				id: `pod-${service.id}`,
				name: service.name,
				projectId: service.projectId,
				projectName: this.getProjectById(service.projectId)?.name ?? service.projectId,
				containers: service.workloads?.map((w) => `${service.id}-${w.name}`) ?? [service.name],
				status: service.status,
				network: `${service.projectId}-network`,
				createdAt: service.createdAt
			});
		}
	}

	updateService(updated: Service) {
		const idx = this.services.findIndex((s) => s.id === updated.id);
		if (idx !== -1) {
			this.services[idx] = { ...updated };
		}
	}

	addDomain(domain: Domain) {
		this.domains.unshift(domain);
	}

	deleteDomain(domainId: string) {
		this.domains = this.domains.filter((d) => d.id !== domainId);
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

	async fetchLiveStats() {
		try {
			const [sysRes, statsRes] = await Promise.all([
				fetch('/api/system').catch(() => null),
				fetch('/api/stats').catch(() => null)
			]);

			if (sysRes && sysRes.ok) {
				const sys = await sysRes.json();
				this.server = {
					...this.server,
					hostname: sys.hostname || this.server.hostname,
					os: sys.os || this.server.os,
					kernel: sys.kernel || this.server.kernel,
					vcpu: sys.vcpu || this.server.vcpu,
					memory: parseFloat(sys.memoryTotalGB?.toFixed(1)) || this.server.memory,
					memoryUsed: parseFloat(sys.memoryUsedGB?.toFixed(1)) || this.server.memoryUsed,
					cpuUsage: parseFloat(sys.cpuUsage?.toFixed(1)) || this.server.cpuUsage,
					podmanVersion: sys.podmanVersion || this.server.podmanVersion,
					rootless: sys.rootless ?? this.server.rootless,
					uptime: sys.uptime || this.server.uptime,
					status: 'online'
				};
			}

			if (statsRes && statsRes.ok) {
				const stats = await statsRes.json();
				if (Array.isArray(stats) && stats.length > 0) {
					for (const s of stats) {
						const found = this.containers.find((c) => c.name === s.name || c.id === s.id);
						if (found) {
							found.cpu = s.cpuPercent ?? found.cpu;
							found.pids = s.pids ?? found.pids;
						} else {
							this.containers.unshift({
								id: s.id,
								name: s.name,
								projectId: 'system',
								projectName: 'System Host',
								serviceId: s.name,
								serviceName: s.name,
								image: s.name,
								status: 'running',
								cpu: s.cpuPercent || 0.1,
								memory: Math.round((s.memUsage || 0) / (1024 * 1024)) || 32,
								memoryLimit: Math.round((s.memLimit || 0) / (1024 * 1024)) || 512,
								ports: '—',
								startedAt: 'Active',
								netRx: s.netDisplay?.split('/')[0]?.trim() || '—',
								netTx: s.netDisplay?.split('/')[1]?.trim() || '—',
								blockRead: '—',
								blockWrite: '—',
								pids: s.pids || 1,
								restarts: 0,
								uptime: 'Active'
							});
						}
					}
				}
			}
		} catch (e) {
			// Silently fallback to mock data
		}
	}
}

export const dataStore = new DataStore();
