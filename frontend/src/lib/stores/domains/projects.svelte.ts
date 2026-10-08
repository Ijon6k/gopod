import type { Project, Service, Deployment, Domain } from '$lib/types';
import { api } from '$lib/api';

export class ProjectsDomainStore {
	projects = $state<Project[]>([]);
	services = $state<Service[]>([]);
	deployments = $state<Deployment[]>([]);
	domains = $state<Domain[]>([]);

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

	getProjectById(projectId: string): Project | undefined {
		return this.projects.find((p) => p.id === projectId);
	}

	getServiceById(serviceId: string): Service | undefined {
		return this.services.find((s) => s.id === serviceId);
	}

	deleteDeployment(deploymentId: string) {
		this.deployments = this.deployments.filter((d) => d.id !== deploymentId);
	}

	clearServiceDeployments(serviceId: string, keepActive = true) {
		const serviceDeps = this.getServiceDeployments(serviceId);
		if (serviceDeps.length === 0) return;

		if (keepActive) {
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
		this.services = this.services.filter((s) => s.projectId !== id);
		this.domains = this.domains.filter((d) => d.projectId !== id);
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

			if (service.domain) {
				this.addDomain({
					id: `d-${Date.now()}`,
					hostname: service.domain,
					projectId: service.projectId,
					serviceId: fullService.id,
					serviceName: service.name,
					tls: true,
					status: 'active',
					proxyPort: service.port || 3000,
					containerPort: service.port || 3000,
					publishedPort: service.port || 3000
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

	async deleteService(id: string, deleteVolumes = false): Promise<void> {
		try {
			await api.services.delete(id, deleteVolumes);
		} catch (err) {
			console.warn('Failed to delete service on backend:', err);
		}
		this.services = this.services.filter((s) => s.id !== id);
		this.domains = this.domains.filter((d) => d.serviceId !== id);
	}

	async startService(serviceId: string): Promise<Service> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'starting' as any;
		}
		try {
			const res = await api.services.start(serviceId);
			if (svc) {
				svc.status = res.status || 'running';
			}
			return res;
		} catch (err) {
			if (svc) {
				svc.status = 'failed';
			}
			throw err;
		}
	}

	async stopService(serviceId: string): Promise<Service> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'stopping' as any;
		}
		try {
			const res = await api.services.stop(serviceId);
			if (svc) {
				svc.status = res.status || 'stopped';
			}
			return res;
		} catch (err) {
			if (svc) {
				svc.status = 'running';
			}
			throw err;
		}
	}

	async restartService(serviceId: string): Promise<Service> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'deploying';
		}
		try {
			const res = await api.services.restart(serviceId);
			if (svc) {
				svc.status = res.status || 'running';
			}
			return res;
		} catch (err) {
			if (svc) {
				svc.status = 'failed';
			}
			throw err;
		}
	}

	async deployService(serviceId: string, trigger = 'manual'): Promise<Deployment> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'deploying';
		}
		try {
			const res = await api.services.deploy(serviceId, trigger);
			if (res && (res as any).id) {
				const dep: Deployment = {
					id: (res as any).id,
					projectId: (res as any).projectId || svc?.projectId || '',
					projectName: (res as any).projectId || svc?.projectId || '',
					serviceId: (res as any).serviceId || serviceId,
					serviceName: svc?.name || serviceId,
					number: (res as any).number || this.deployments.length + 1,
					version: (res as any).version || `#${this.deployments.length + 1}`,
					commit: (res as any).commitHash || '',
					commitMessage: (res as any).commitMessage || 'Deployment initiated',
					branch: (res as any).branch || svc?.branch || 'main',
					status: (res as any).status || 'building',
					duration: (res as any).duration || 'Running…',
					timeAgo: 'Just now',
					startedAt: (res as any).startedAt || new Date().toISOString(),
					finishedAt: (res as any).finishedAt || '',
					trigger: (res as any).trigger || trigger,
					image: (res as any).image || svc?.image
				};
				const existingIdx = this.deployments.findIndex((d) => d.id === dep.id);
				if (existingIdx >= 0) {
					this.deployments[existingIdx] = dep;
				} else {
					this.deployments.unshift(dep);
				}

				// Active watcher: Poll deployment & service status every 1.5s until finished
				const depId = dep.id;
				let attempts = 0;
				const pollTimer = setInterval(async () => {
					attempts++;
					try {
						const [updatedDep, updatedSvc] = await Promise.all([
							api.services.getDeployment(depId).catch(() => null),
							api.services.get(serviceId).catch(() => null)
						]);
						if (updatedDep) {
							const dIdx = this.deployments.findIndex((d) => d.id === depId);
							if (dIdx >= 0) {
								this.deployments[dIdx].status = updatedDep.status;
								this.deployments[dIdx].duration = updatedDep.duration || this.deployments[dIdx].duration;
								this.deployments[dIdx].finishedAt = updatedDep.finishedAt || '';
							}
						}
						if (updatedSvc && updatedSvc.status !== 'deploying') {
							if (svc) {
								svc.status = updatedSvc.status;
							}
							clearInterval(pollTimer);
						}
					} catch (_) {}

					if (attempts > 300) {
						clearInterval(pollTimer);
					}
				}, 1500);

				return dep;
			}
			return res as any;
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

	async fetchProjectsData() {
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
				const depResults = await Promise.all(
					servicesData.map((s: any) => api.services.deployments(s.id).catch(() => []))
				);
				const allDeps: Deployment[] = [];
				for (const deps of depResults) {
					if (Array.isArray(deps)) {
						allDeps.push(...deps);
					}
				}
				this.deployments = allDeps;
			}
			if (domainsData && domainsData.length > 0) {
				this.domains = domainsData;
			}
		} catch (err) {
			console.warn('[ProjectsDomainStore] fetchProjectsData error:', err);
		}
	}
}
