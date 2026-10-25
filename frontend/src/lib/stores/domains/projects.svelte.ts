import type { Project, Service, Deployment, Domain } from '$lib/types';
import { api } from '$lib/api';
import { formatTimeAgo } from '$lib/utils/format';

export class ProjectsDomainStore {
	projects = $state<Project[]>([]);
	services = $state<Service[]>([]);
	deployments = $state<Deployment[]>([]);
	domains = $state<Domain[]>([]);

	// Deployment watchers keyed by service id. A new deploy replaces the
	// previous watcher; cleanup() stops every one of them.
	private deploymentWatchers = new Map<string, () => void>();

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

	async cancelDeployment(deploymentId: string): Promise<void> {
		const dep = this.deployments.find((d) => d.id === deploymentId);
		if (dep) {
			dep.status = 'cancelled';
			dep.duration = 'Cancelled by operator';
			dep.finishedAt = new Date().toISOString();
			this.deployments = [...this.deployments];
		}
		try {
			await api.services.cancelDeployment(deploymentId);
			if (dep?.serviceId) {
				const svc = await api.services.get(dep.serviceId);
				const idx = this.services.findIndex((s) => s.id === dep.serviceId);
				if (idx >= 0) {
					this.services[idx] = { ...this.services[idx], ...svc };
				}
			}
		} catch (err) {
			console.warn('Failed to sync deployment cancellation to backend:', err);
		}
	}

	async cancelServiceDeployment(serviceId: string): Promise<void> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'stopped';
		}
		this.deployments = this.deployments.map((d) => {
			if (d.serviceId === serviceId && (d.status === 'building' || d.status === 'deploying')) {
				return {
					...d,
					status: 'cancelled',
					duration: 'Cancelled by operator',
					finishedAt: new Date().toISOString()
				};
			}
			return d;
		});

		try {
			const updated = await api.services.cancelServiceDeployment(serviceId);
			if (svc) {
				svc.status = updated.status || 'stopped';
			}
			const idx = this.services.findIndex((s) => s.id === serviceId);
			if (idx >= 0) {
				this.services[idx] = { ...this.services[idx], ...updated };
			}
		} catch (err) {
			console.warn('Failed to cancel service deployment on backend:', err);
		}
	}

	upsertDeployment(dep: Partial<Deployment> & { id: string }) {
		const idx = this.deployments.findIndex((d) => d.id === dep.id);
		if (idx >= 0) {
			// Field-level merge: no array identity churn, so consumers reading the
			// array re-run only for the fields that actually changed.
			Object.assign(this.deployments[idx], dep);
		} else {
			this.deployments = [dep as Deployment, ...this.deployments];
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
				id: crypto.randomUUID(),
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

	async deployService(
		serviceId: string,
		trigger = 'manual',
		commit?: string,
		image?: string
	): Promise<Deployment> {
		const svc = this.services.find((s) => s.id === serviceId);
		if (svc) {
			svc.status = 'deploying';
		}
		try {
			const res = await api.services.deploy(serviceId, trigger, commit, image);
			if (res) {
				const targetProjId = (res as any).projectId || svc?.projectId || '';
				const foundProj = this.projects.find((p) => p.id === targetProjId);
				const dep: Deployment = {
					id: (res as any).id,
					projectId: targetProjId,
					projectName:
						foundProj?.name ||
						(res as any).projectName ||
						(svc as any)?.projectName ||
						targetProjId ||
						'Default',
					serviceId: (res as any).serviceId || serviceId,
					serviceName: svc?.name || (res as any).serviceName || serviceId,
					number: (res as any).number || this.deployments.length + 1,
					version: (res as any).version || `#${this.deployments.length + 1}`,
					commit: (res as any).commitHash || '',
					commitMessage: (res as any).commitMessage || 'Deployment initiated',
					branch: (res as any).branch || svc?.branch || 'main',
					status: (res as any).status || 'building',
					duration: (res as any).duration || 'Running…',
					// Derived from the real backend timestamp instead of a
					// fabricated constant ('Just now' froze on posts from hours
					// ago when the table re-rendered without a refetch).
					timeAgo: formatTimeAgo((res as any).startedAt || Date.now()),
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

				// Active watcher: poll deployment & service status every 1.5s until
				// finished. One watcher per service; a new deploy replaces the old one.
				this.startDeploymentWatcher(serviceId, dep.id);

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

	startDeploymentWatcher(serviceId: string, depId: string) {
		this.deploymentWatchers.get(serviceId)?.();
		let attempts = 0;
		// Assigned exactly once below. The pre-initialized `undefined` records
		// that stop() may run before the interval exists: a replaced watcher's
		// stopper owns its own timer, so the early clearInterval is a no-op.
		let pollTimer: ReturnType<typeof setInterval> | undefined = undefined;
		let inFlight = false;
		// Change detection: only write state when the payload actually moved.
		// Every 1.5s otherwise re-derived every consumer of the arrays.
		let lastDepFingerprint = '';
		let lastServiceStatus = '';
		// Stopping is identity-guarded: a replaced watcher's late tick or attempt
		// cap must not remove the entry that now belongs to the newer watcher.
		const stop = () => {
			clearInterval(pollTimer);
			if (this.deploymentWatchers.get(serviceId) === stop) {
				this.deploymentWatchers.delete(serviceId);
			}
		};
		pollTimer = setInterval(async () => {
			// A slow tick never overlaps the next one.
			if (inFlight) return;
			attempts++;
			inFlight = true;
			try {
				const [updatedDep, updatedSvc] = await Promise.all([
					api.services.getDeployment(depId).catch(() => null),
					api.services.get(serviceId).catch(() => null)
				]);
				if (updatedDep) {
					const depFingerprint = [
						updatedDep.status,
						updatedDep.duration || '',
						updatedDep.finishedAt || '',
						updatedDep.version || '',
						updatedDep.image || '',
						updatedDep.commitMessage || ''
					].join('|');
					if (depFingerprint !== lastDepFingerprint) {
						lastDepFingerprint = depFingerprint;
						const dIdx = this.deployments.findIndex((d) => d.id === depId);
						if (dIdx >= 0) {
							const dep = this.deployments[dIdx];
							dep.status = updatedDep.status;
							dep.duration = updatedDep.duration || dep.duration;
							dep.finishedAt = updatedDep.finishedAt || dep.finishedAt;
							dep.commitMessage = updatedDep.commitMessage || dep.commitMessage;
							dep.version = updatedDep.version || dep.version;
							dep.image = updatedDep.image || dep.image;
							// Re-age the relative label at exactly the moments the row
							// content changes, so terminal states never render the
							// ingest-time "When" hours later.
							dep.timeAgo = formatTimeAgo(updatedDep.startedAt || dep.startedAt);
						}
					}
					if (updatedDep.status === 'failed' || updatedDep.status === 'success') {
						const sIdx = this.services.findIndex((s) => s.id === serviceId);
						if (sIdx >= 0) {
							const terminalStatus =
								updatedSvc?.status && updatedSvc.status !== 'deploying'
									? updatedSvc.status
									: updatedDep.status === 'failed'
										? 'failed'
										: 'running';
							if (terminalStatus !== lastServiceStatus) {
								lastServiceStatus = terminalStatus;
								this.services[sIdx].status = terminalStatus;
							}
						}
						stop();
					}
				}
			} catch (_) {} finally {
				inFlight = false;
			}

			if (attempts > 300) {
				stop();
			}
		}, 1500);
		this.deploymentWatchers.set(serviceId, stop);
	}

	// Stop every background watcher. Called on logout so timers never outlive
	// the authenticated session.
	cleanup() {
		for (const stop of [...this.deploymentWatchers.values()]) stop();
		this.deploymentWatchers.clear();
	}

	async demoteQuadlet(serviceId: string): Promise<Service> {
		const res = await api.services.demoteQuadlet(serviceId);
		const idx = this.services.findIndex((s) => s.id === serviceId);
		if (idx >= 0 && res) {
			this.services[idx] = res;
		}
		return res;
	}

	async groupIntoPod(
		serviceId: string,
		podName: string,
		createPod = true,
		ports?: string[]
	): Promise<Service> {
		const res = await api.services.groupIntoPod(serviceId, {
			podName,
			createPod,
			ports
		});
		const idx = this.services.findIndex((s) => s.id === serviceId);
		if (idx >= 0 && res) {
			this.services[idx] = res;
		}
		await this.fetchProjectsData();
		return res;
	}

	async ungroupFromPod(serviceId: string): Promise<Service> {
		const res = await api.services.ungroupFromPod(serviceId);
		const idx = this.services.findIndex((s) => s.id === serviceId);
		if (idx >= 0 && res) {
			this.services[idx] = res;
		}
		await this.fetchProjectsData();
		return res;
	}

	addDomain(domain: Domain) {
		// Optimistic insert; on API failure the added row is the only thing undone,
		// so concurrent edits to other rows are never reverted with it.
		this.domains.unshift(domain);
		api.domains
			.create(domain)
			.then((saved) => {
				this.domains = this.domains.map((d) => (d.id === domain.id ? { ...d, ...saved } : d));
			})
			.catch((err) => {
				this.domains = this.domains.filter((d) => d.id !== domain.id);
				console.warn('Failed to create domain:', err);
			});
	}

	updateDomain(updated: Domain) {
		const idx = this.domains.findIndex((d) => d.id === updated.id);
		if (idx === -1) return;
		const previous = { ...this.domains[idx] };
		this.domains[idx] = { ...updated };
		api.domains.update(updated.id, updated).catch((err) => {
			// Revert by id, not by index: the list may have shifted while the
			// request was in flight.
			const cur = this.domains.findIndex((d) => d.id === updated.id);
			if (cur !== -1) {
				this.domains[cur] = previous;
			}
			console.warn('Failed to sync domain update:', err);
		});
	}

	deleteDomain(domainId: string) {
		const idx = this.domains.findIndex((d) => d.id === domainId);
		if (idx === -1) return;
		const removed = this.domains[idx];
		const priorNeighborId = idx > 0 ? this.domains[idx - 1].id : null;
		this.domains.splice(idx, 1);
		api.domains.delete(domainId).catch((err) => {
			// Restore unless a newer server list already contains the row; anchor
			// after the former neighbor so the original order survives the re-add.
			if (!this.domains.some((d) => d.id === domainId)) {
				const anchor = priorNeighborId
					? this.domains.findIndex((d) => d.id === priorNeighborId) + 1
					: 0;
				this.domains.splice(Math.min(Math.max(anchor, 0), this.domains.length), 0, removed);
			}
			console.warn('Failed to delete domain:', err);
		});
	}

	async revalidateDomain(domainId: string): Promise<Domain> {
		const updated = await api.domains.revalidate(domainId);
		const idx = this.domains.findIndex((d) => d.id === domainId);
		if (idx !== -1) {
			this.domains[idx] = { ...this.domains[idx], ...updated };
		}
		return updated;
	}

	async fetchProjectsData() {
		try {
			const [projectsData, servicesData, domainsData] = await Promise.all([
				api.projects.list().catch(() => null),
				api.services.list().catch(() => null),
				api.domains.list().catch(() => null)
			]);

			const pList = projectsData as any[] | null | undefined;
			if (pList && Array.isArray(pList)) {
				this.projects = pList.map((p: any) => ({
					id: p.id,
					name: p.name,
					description: p.description || '',
					status: (p.status || 'healthy') as any,
					services: p.services || [],
					domains: p.domains || [],
					cpu: p.cpu || 0,
					memory: p.memory || 0,
					memoryTotal: p.memoryTotal || 16384,
					createdAt: p.createdAt || new Date().toISOString()
				}));
			}
			// Assign whenever the API answers successfully — an empty result is the
			// truth (e.g. after deleting the last row); a failed request keeps the
			// previous lists.
			if (Array.isArray(servicesData)) {
				this.services = servicesData;
				const depResults = await Promise.all(
					servicesData.map((s: any) => api.services.deployments(s.id).catch(() => []))
				);
				const allDeps: Deployment[] = [];
				for (let i = 0; i < servicesData.length; i++) {
					const svc: any = servicesData[i];
					const deps = depResults[i];
					if (Array.isArray(deps)) {
						for (const dep of deps) {
							const targetProjId = dep.projectId || svc?.projectId || '';
							const proj = this.projects.find((p: any) => p.id === targetProjId);
							allDeps.push({
								...dep,
								commit: dep.commit || dep.commitHash || '',
								branch: dep.branch || svc?.branch || 'main',
								// Derived from the real backend timestamp; the old
								// code stamped a constant 'Recently' on every row.
								timeAgo: dep.startedAt ? formatTimeAgo(dep.startedAt) : '',
								projectName:
									proj?.name || dep.projectName || svc?.projectName || targetProjId || 'Default',
								serviceName: svc?.name || dep.serviceName || dep.serviceId || 'Service'
							});
						}
					}
				}
				this.deployments = allDeps;
			}
			if (Array.isArray(domainsData)) {
				this.domains = domainsData;
			}
		} catch (err) {
			console.warn('[ProjectsDomainStore] fetchProjectsData error:', err);
		}
	}
}
