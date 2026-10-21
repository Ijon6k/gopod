// ──────────────────────────────────────────────
// GOPOD — Unified Reactive Data Coordinator (Svelte 5)
// Coordinates domain-driven stores with modular architecture
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
	TimeSeriesPoint,
	PodmanSecret,
	SSHKey,
	ContainerRegistry,
	GitToken,
	PortMapping,
	CaddyAccessLog,
	VolumeSnapshot,
	VolumeBackupSchedule,
	QuadletUnit
} from '$lib/types';

import { api } from '$lib/api';
import { ProjectsDomainStore } from './domains/projects.svelte';
import { RuntimeDomainStore } from './domains/runtime.svelte';
import { CredentialsDomainStore } from './domains/credentials.svelte';
import { StorageDomainStore } from './domains/storage.svelte';
import { TelemetryDomainStore } from './domains/telemetry.svelte';

export class DataStore {
	private projectsStore = new ProjectsDomainStore();
	private runtimeStore = new RuntimeDomainStore();
	private credentialsStore = new CredentialsDomainStore();
	private storageStore = new StorageDomainStore();
	private telemetryStore = new TelemetryDomainStore();

	// ── Reactive Getters ──
	get projects(): Project[] {
		return this.projectsStore.projects;
	}
	set projects(val: Project[]) {
		this.projectsStore.projects = val;
	}

	get services(): Service[] {
		return this.projectsStore.services;
	}
	set services(val: Service[]) {
		this.projectsStore.services = val;
	}

	get deployments(): Deployment[] {
		return this.projectsStore.deployments;
	}
	set deployments(val: Deployment[]) {
		this.projectsStore.deployments = val;
	}

	get domains(): Domain[] {
		return this.projectsStore.domains;
	}
	set domains(val: Domain[]) {
		this.projectsStore.domains = val;
	}

	get containers(): Container[] {
		return this.runtimeStore.containers;
	}
	set containers(val: Container[]) {
		this.runtimeStore.containers = val;
	}

	get pods(): Pod[] {
		return this.runtimeStore.pods;
	}
	set pods(val: Pod[]) {
		this.runtimeStore.pods = val;
	}

	get images(): Image[] {
		return this.runtimeStore.images;
	}
	set images(val: Image[]) {
		this.runtimeStore.images = val;
	}

	get volumes(): Volume[] {
		return this.runtimeStore.volumes;
	}
	set volumes(val: Volume[]) {
		this.runtimeStore.volumes = val;
	}

	get networks(): Network[] {
		return this.runtimeStore.networks;
	}
	set networks(val: Network[]) {
		this.runtimeStore.networks = val;
	}

	get ports(): PortMapping[] {
		return this.runtimeStore.ports;
	}
	set ports(val: PortMapping[]) {
		this.runtimeStore.ports = val;
	}

	get quadlets(): QuadletUnit[] {
		return this.runtimeStore.quadlets;
	}
	set quadlets(val: QuadletUnit[]) {
		this.runtimeStore.quadlets = val;
	}

	get sshKeys(): SSHKey[] {
		return this.credentialsStore.sshKeys;
	}
	set sshKeys(val: SSHKey[]) {
		this.credentialsStore.sshKeys = val;
	}

	get registries(): ContainerRegistry[] {
		return this.credentialsStore.registries;
	}
	set registries(val: ContainerRegistry[]) {
		this.credentialsStore.registries = val;
	}

	get podmanSecrets(): PodmanSecret[] {
		return this.credentialsStore.podmanSecrets;
	}
	set podmanSecrets(val: PodmanSecret[]) {
		this.credentialsStore.podmanSecrets = val;
	}

	get gitTokens(): GitToken[] {
		return this.credentialsStore.gitTokens;
	}
	set gitTokens(val: GitToken[]) {
		this.credentialsStore.gitTokens = val;
	}

	get volumeSnapshots(): VolumeSnapshot[] {
		return this.storageStore.volumeSnapshots;
	}
	set volumeSnapshots(val: VolumeSnapshot[]) {
		this.storageStore.volumeSnapshots = val;
	}

	get volumeSchedules(): VolumeBackupSchedule[] {
		return this.storageStore.volumeSchedules;
	}
	set volumeSchedules(val: VolumeBackupSchedule[]) {
		this.storageStore.volumeSchedules = val;
	}

	get server(): Server {
		return this.telemetryStore.server;
	}
	set server(val: Server) {
		this.telemetryStore.server = val;
	}

	get accessLogs(): CaddyAccessLog[] {
		return this.telemetryStore.accessLogs;
	}
	set accessLogs(val: CaddyAccessLog[]) {
		this.telemetryStore.accessLogs = val;
	}

	get isStreaming(): boolean {
		return this.telemetryStore.isStreaming;
	}

	/** True while the initial runtime batch (pods/images/volumes/networks/containers) is loading. */
	get isRuntimeLoading(): boolean {
		return this.runtimeStore.loading;
	}

	get lastNetworkRateBps(): number {
		return this.telemetryStore.lastNetworkRateBps;
	}

	// Direct per-series getters: each chart reads exactly the series it draws,
	// so a push into one history never re-derives the other three charts the
	// way the old `monitoringData` object-allocating getter did.
	get cpuHistory(): TimeSeriesPoint[] {
		return this.telemetryStore.cpuHistory;
	}
	get memoryHistory(): TimeSeriesPoint[] {
		return this.telemetryStore.memoryHistory;
	}
	get storageHistory(): TimeSeriesPoint[] {
		return this.telemetryStore.storageHistory;
	}
	get networkHistory(): TimeSeriesPoint[] {
		return this.telemetryStore.networkHistory;
	}

	// ── Projects Domain Actions ──
	getProjectServices(projectId: string) {
		return this.projectsStore.getProjectServices(projectId);
	}
	getProjectDomains(projectId: string) {
		return this.projectsStore.getProjectDomains(projectId);
	}
	getProjectDeployments(projectId: string) {
		return this.projectsStore.getProjectDeployments(projectId);
	}
	getServiceDeployments(serviceId: string) {
		return this.projectsStore.getServiceDeployments(serviceId);
	}
	deleteDeployment(id: string) {
		this.projectsStore.deleteDeployment(id);
	}
	clearServiceDeployments(serviceId: string, keepActive = true) {
		this.projectsStore.clearServiceDeployments(serviceId, keepActive);
	}
	cancelDeployment(id: string) {
		return this.projectsStore.cancelDeployment(id);
	}
	cancelServiceDeployment(serviceId: string) {
		return this.projectsStore.cancelServiceDeployment(serviceId);
	}
	getProjectById(id: string) {
		return this.projectsStore.getProjectById(id);
	}
	createProject(data: { name: string; description?: string }) {
		return this.projectsStore.createProject(data);
	}
	deleteProject(id: string) {
		return this.projectsStore.deleteProject(id);
	}
	getServiceById(id: string) {
		return this.projectsStore.getServiceById(id);
	}
	addService(service: Service) {
		return this.projectsStore.addService(service);
	}
	updateService(service: Service) {
		return this.projectsStore.updateService(service);
	}
	deleteService(id: string, deleteVolumes = false) {
		return this.projectsStore.deleteService(id, deleteVolumes);
	}
	deployService(serviceId: string, trigger = 'manual', commit?: string, image?: string) {
		return this.projectsStore.deployService(serviceId, trigger, commit, image);
	}
	upsertDeployment(dep: Partial<Deployment> & { id: string }) {
		this.projectsStore.upsertDeployment(dep);
	}
	startService(serviceId: string) {
		return this.projectsStore.startService(serviceId);
	}
	stopService(serviceId: string) {
		return this.projectsStore.stopService(serviceId);
	}
	restartService(serviceId: string) {
		return this.projectsStore.restartService(serviceId);
	}

	demoteQuadlet(serviceId: string) {
		return this.projectsStore.demoteQuadlet(serviceId);
	}

	groupIntoPod(serviceId: string, podName: string, createPod = true, ports?: string[]) {
		return this.projectsStore.groupIntoPod(serviceId, podName, createPod, ports);
	}
	ungroupFromPod(serviceId: string) {
		return this.projectsStore.ungroupFromPod(serviceId);
	}
	addDomain(domain: Domain) {
		this.projectsStore.addDomain(domain);
	}
	updateDomain(domain: Domain) {
		this.projectsStore.updateDomain(domain);
	}
	deleteDomain(id: string) {
		this.projectsStore.deleteDomain(id);
	}
	revalidateDomain(id: string) {
		return this.projectsStore.revalidateDomain(id);
	}

	// ── Runtime Domain Actions ──
	// Stop background watchers owned by the domain stores (deploy pollers).
	cleanup() {
		this.projectsStore.cleanup();
	}
	getServiceContainers(serviceId: string) {
		return this.runtimeStore.getServiceContainers(serviceId);
	}
	startContainer(id: string) {
		return this.runtimeStore.startContainer(id);
	}
	stopContainer(id: string) {
		return this.runtimeStore.stopContainer(id);
	}
	restartContainer(id: string) {
		return this.runtimeStore.restartContainer(id);
	}
	deleteContainer(id: string, force = false) {
		return this.runtimeStore.deleteContainer(id, force);
	}
	startPod(id: string) {
		return this.runtimeStore.startPod(id);
	}
	stopPod(id: string) {
		return this.runtimeStore.stopPod(id);
	}
	deletePod(id: string, force = false) {
		return this.runtimeStore.deletePod(id, force);
	}
	removeImage(id: string) {
		return this.runtimeStore.removeImage(id);
	}
	removeVolume(name: string) {
		return this.runtimeStore.removeVolume(name);
	}
	removeNetwork(name: string) {
		return this.runtimeStore.removeNetwork(name);
	}
	pruneImages(all = false) {
		return this.runtimeStore.pruneImages(all);
	}
	startQuadlet(name: string) {
		return this.runtimeStore.startQuadlet(name);
	}
	stopQuadlet(name: string) {
		return this.runtimeStore.stopQuadlet(name);
	}
	restartQuadlet(name: string) {
		return this.runtimeStore.restartQuadlet(name);
	}
	reloadQuadlets() {
		return this.runtimeStore.reloadQuadlets();
	}
	deleteQuadlet(name: string) {
		return this.runtimeStore.deleteQuadlet(name);
	}
	fetchQuadlets() {
		return this.runtimeStore.fetchQuadlets();
	}
	pruneVolumes() {
		return this.runtimeStore.pruneVolumes();
	}
	pruneSystem() {
		return this.runtimeStore.pruneSystem();
	}
	fetchRuntimeData(maxAgeMs = 0) {
		if (this.isFresh('runtime', maxAgeMs)) return Promise.resolve();
		return this.runtimeStore.fetchRuntimeData().then(() => {
			this.lastFetchedAt['runtime'] = Date.now();
		});
	}

	// ── Credentials Domain Actions ──
	addSSHKey(key: Omit<SSHKey, 'id' | 'createdAt'>) {
		return this.credentialsStore.addSSHKey(key);
	}
	deleteSSHKey(id: string) {
		return this.credentialsStore.deleteSSHKey(id);
	}
	addRegistry(reg: Omit<ContainerRegistry, 'id' | 'createdAt'>) {
		return this.credentialsStore.addRegistry(reg);
	}
	deleteRegistry(id: string) {
		return this.credentialsStore.deleteRegistry(id);
	}
	addSecret(name: string, value = '') {
		return this.credentialsStore.addSecret(name, value);
	}
	addGitToken(token: Omit<GitToken, 'id' | 'createdAt'>) {
		return this.credentialsStore.addGitToken(token);
	}
	deleteGitToken(id: string) {
		return this.credentialsStore.deleteGitToken(id);
	}

	// ── Storage Domain Actions ──
	getProjectVolumeSnapshots(projectId: string) {
		return this.storageStore.getProjectVolumeSnapshots(projectId);
	}
	getProjectVolumeSchedules(projectId: string) {
		return this.storageStore.getProjectVolumeSchedules(projectId);
	}
	createVolumeSnapshot(volumeName: string, projectId: string, serviceId: string) {
		return this.storageStore.createVolumeSnapshot(volumeName, projectId, serviceId);
	}
	deleteVolumeSnapshot(id: string) {
		return this.storageStore.deleteVolumeSnapshot(id);
	}
	restoreVolumeSnapshot(id: string) {
		return this.storageStore.restoreVolumeSnapshot(id);
	}
	downloadVolumeSnapshot(id: string, filename: string) {
		return this.storageStore.downloadVolumeSnapshot(id, filename);
	}
	toggleVolumeSchedule(id: string) {
		return this.storageStore.toggleVolumeSchedule(id);
	}

	// ── Telemetry & Live Polling Actions ──
	// Refcounted SSE stream: multiple pages (monitoring, runtime) can share one
	// connection; the transport is opened for the first owner and closed when
	// the last one leaves.
	private streamOwners = 0;
	startStreamingStats() {
		this.streamOwners++;
		if (this.streamOwners === 1) {
			this.telemetryStore.startStreamingStats((stats) => {
				this.applyStatsUpdate(stats);
			});
		}
	}
	stopStreamingStats() {
		this.streamOwners = Math.max(0, this.streamOwners - 1);
		if (this.streamOwners === 0) {
			this.telemetryStore.stopStreamingStats();
		}
	}
	applySystemUpdate(sys: any) {
		this.telemetryStore.applySystemUpdate(sys);
	}

	// O(n) per tick: one index pass replaces the per-entry `.find()` scans that
	// made every stats event quadratic over the deep-proxied store.
	private applyStatsUpdate(stats: any[]) {
		const containers = this.runtimeStore.containers;
		if (!containers.length || !stats.length) return;
		const index = new Map<string, Container>();
		for (const c of containers) {
			if (c.id) index.set(c.id, c);
			if (c.name) index.set(c.name, c);
		}
		for (const s of stats) {
			const found = index.get(s.id) || index.get(s.name);
			if (!found) continue;
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

	// One live-stats refresh at a time; the runtime page, layout boot and SSE
	// fallback all funnel through this method and used to pile up parallel
	// GET bursts when they raced.
	private liveStatsInFlight = false;
	async fetchLiveStats() {
		if (this.liveStatsInFlight) return null;
		this.liveStatsInFlight = true;
		try {
			const [rawContainers, stats] = await Promise.all([
				api.runtime.containers.list().catch(() => null),
				this.telemetryStore.fetchLiveStats()
			]);

			if (rawContainers && Array.isArray(rawContainers)) {
				const statsMap = new Map<string, any>();
				if (stats && Array.isArray(stats)) {
					for (const s of stats) {
						if (s.id) statsMap.set(s.id, s);
						if (s.name) statsMap.set(s.name, s);
					}
				}

				this.runtimeStore.containers = rawContainers.map((rc: any) => {
					const name = rc.names && rc.names.length > 0 ? rc.names[0].replace(/^\//, '') : rc.id;
					const stat = statsMap.get(rc.id) || statsMap.get(name);
					const meta = this.resolveContainerMetadata(name, rc.labels);
					const isRunning =
						rc.state === 'running' || (rc.status && rc.status.toLowerCase().includes('up'));

					return {
						id: rc.id,
						name: name,
						projectId: meta.projId,
						projectName: meta.projName,
						serviceId: meta.servId,
						serviceName: meta.servName,
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
						uptime: rc.status || (isRunning ? 'Active' : 'Stopped'),
						labels: rc.labels || {}
					};
				});
				this.runtimeStore.ports = parseContainerPortMappings(this.runtimeStore.containers);
			} else if (stats && Array.isArray(stats) && stats.length > 0) {
				this.applyStatsUpdate(stats);
			}
		} catch (e) {
			// Silently ignore
		} finally {
			this.liveStatsInFlight = false;
		}
	}

	/** Refresh pods + containers from live runtime so drift banners reflect reality. */
	async refreshRuntime() {
		await Promise.allSettled([this.fetchLiveStats(), this.runtimeStore.fetchRuntimeData()]);
	}

	resolveContainerMetadata(
		name: string,
		labels?: Record<string, string>
	): { projId: string; projName: string; servId: string; servName: string } {
		let projName = 'System Host';
		let projId = 'system';
		let servName = name;
		let servId = name;

		// 1. Explicit GOPOD metadata labels
		if (labels && labels['io.gopod.project']) {
			projId = labels['io.gopod.project'];
			servId = labels['io.gopod.service'] || name;
			servName = labels['io.gopod.name'] || name;
			projName = this.projectsStore.getProjectById(projId)?.name || projId;
			return { projId, projName, servId, servName };
		}

		// 2. Compose Project labels (Docker & Podman Compose parity)
		const composeProj =
			labels?.['com.docker.compose.project'] || labels?.['io.podman.compose.project'];
		const composeServ =
			labels?.['com.docker.compose.service'] || labels?.['io.podman.compose.service'];

		if (composeProj) {
			const matched = this.services.find((s: Service) => {
				if (!s) return false;
				if (s.id === composeProj || s.name === composeProj) return true;
				if (composeProj === `${s.projectId}-${s.name}` || composeProj === `${s.projectId}-${s.id}`)
					return true;
				if (s.id && (composeProj.startsWith(`${s.id}-`) || composeProj.startsWith(`${s.id}_`)))
					return true;
				if (s.projectId && composeProj.startsWith(`${s.projectId}-`)) {
					const rest = composeProj.slice(s.projectId.length + 1);
					return (
						rest === s.id ||
						rest === s.name ||
						rest.startsWith(`${s.id}-`) ||
						rest.startsWith(`${s.name}-`)
					);
				}
				return false;
			});
			if (matched) {
				servId = matched.id;
				servName = composeServ ? `${matched.name} (${composeServ})` : matched.name;
				projId = matched.projectId;
				projName = this.projectsStore.getProjectById(projId)?.name || projId;
				return { projId, projName, servId, servName };
			}
		}

		// 3. Compose working directory or config file paths
		const composeWorkDir =
			labels?.['com.docker.compose.project.working_dir'] ||
			labels?.['com.docker.compose.project.config_files'];
		if (composeWorkDir) {
			for (const p of this.projectsStore.projects) {
				if (composeWorkDir.includes(`/${p.id}/`) || composeWorkDir.includes(`/${p.name}/`)) {
					projId = p.id;
					projName = p.name;
					const pServices = this.getProjectServices(p.id);
					for (const s of pServices) {
						if (
							composeWorkDir.includes(`/${s.id}/`) ||
							composeWorkDir.endsWith(`/${s.id}`) ||
							composeWorkDir.includes(`/${s.id}.`) ||
							composeWorkDir.includes(`/${s.name}/`) ||
							composeWorkDir.endsWith(`/${s.name}`) ||
							(composeProj &&
								(s.id === composeProj ||
									s.name === composeProj ||
									composeProj === `${s.projectId}-${s.name}`))
						) {
							servId = s.id;
							servName = composeServ ? `${s.name} (${composeServ})` : s.name;
							return { projId, projName, servId, servName };
						}
					}
					break;
				}
			}
		}

		// 4. Podman systemd unit label (e.g. podman-compose@self-hosted-redis.service)
		const systemdUnit = labels?.['PODMAN_SYSTEMD_UNIT'];
		if (systemdUnit) {
			const unitMatch = systemdUnit.match(/podman-compose@([^.]+)\.service/);
			if (unitMatch) {
				const unitName = unitMatch[1];
				const matched = this.services.find((s) => {
					if (s.id === unitName || s.name === unitName) return true;
					if (unitName === `${s.projectId}-${s.name}` || unitName === `${s.projectId}-${s.id}`)
						return true;
					if (s.projectId && unitName.startsWith(`${s.projectId}-`)) {
						const rest = unitName.slice(s.projectId.length + 1);
						return (
							rest === s.id ||
							rest === s.name ||
							rest.startsWith(`${s.id}-`) ||
							rest.startsWith(`${s.name}-`)
						);
					}
					return false;
				});
				if (matched) {
					servId = matched.id;
					servName = composeServ || matched.name;
					projId = matched.projectId;
					projName = this.projectsStore.getProjectById(projId)?.name || projId;
					return { projId, projName, servId, servName };
				}
			}
		}

		// 5. Container name prefixes / exact matching with services
		const matchedByPrefix = this.services.find(
			(s: Service) =>
				(s.id && (name === s.id || name.startsWith(`${s.id}-`) || name.startsWith(`${s.id}_`))) ||
				(s.name &&
					(name === s.name || name.startsWith(`${s.name}-`) || name.startsWith(`${s.name}_`)))
		);
		if (matchedByPrefix) {
			servId = matchedByPrefix.id;
			servName = matchedByPrefix.name;
			projId = matchedByPrefix.projectId;
			projName = this.projectsStore.getProjectById(projId)?.name || projId;
			return { projId, projName, servId, servName };
		}

		// 6. Project name prefix in container name (e.g., self-hosted-redis-...)
		const parts = name.split(/[-_]/);
		if (parts.length >= 2) {
			const potentialProj = this.projectsStore.projects.find(
				(p) => p.id === parts[0] || p.name.toLowerCase() === parts[0]
			);
			if (potentialProj) {
				projId = potentialProj.id;
				projName = potentialProj.name;
				const matchedServInProj = this.getProjectServices(potentialProj.id).find(
					(s) => name.includes(s.name) || name.includes(s.id)
				);
				if (matchedServInProj) {
					servId = matchedServInProj.id;
					servName = matchedServInProj.name;
				} else {
					servName = parts.slice(1).join('-');
					servId = `${projId}-${servName}`;
				}
			}
		}

		return { projId, projName, servId, servName };
	}

	relinkContainers() {
		if (!this.runtimeStore.containers.length || !this.services.length) return;
		this.runtimeStore.containers = this.runtimeStore.containers.map((c) => {
			const meta = this.resolveContainerMetadata(c.name, c.labels);
			return {
				...c,
				projectId: meta.projId,
				projectName: meta.projName,
				serviceId: meta.servId,
				serviceName: meta.servName
			};
		});
	}

	// How long ago each dataset was fetched, in ms since epoch; used by the
	// TTL-aware entry points so re-visiting pages does not re-download the
	// whole cluster state.
	private lastFetchedAt: Record<string, number> = {};
	private isFresh(key: string, maxAgeMs: number): boolean {
		const at = this.lastFetchedAt[key] || 0;
		return maxAgeMs > 0 && at > 0 && Date.now() - at < maxAgeMs;
	}

	/**
	 * Boot & re-entry fetch. With `maxAgeMs` set the call is idempotent for that
	 * TTL window: pages that call this on mount stop double-hitting every
	 * endpoint the layout already filled.
	 */
	async fetchInitialData(maxAgeMs = 0) {
		if (this.isFresh('initial', maxAgeMs)) return;

		// 1. Fetch projects and services first to avoid container matching race condition
		await this.projectsStore.fetchProjectsData();

		// 2. Fetch runtime data, credentials, storage, traffic, and live stats.
		//    fetchLiveStats resolves projects/services metadata inline now that
		//    projects are loaded — the former post-hoc relinkContainers pass
		//    re-did the same O(C·S) resolution for nothing.
		await Promise.all([
			this.fetchRuntimeData(),
			this.credentialsStore.fetchCredentials(),
			this.storageStore.fetchStorageData(),
			this.telemetryStore.fetchTrafficLogs(),
			this.fetchLiveStats()
		]);
		this.lastFetchedAt.initial = Date.now();
	}
}

export const dataStore = new DataStore();

export function parseContainerPortMappings(containers: Container[]): PortMapping[] {
	const result: PortMapping[] = [];
	for (const c of containers) {
		if (!c.ports || c.ports === '—') continue;
		const portItems = c.ports
			.split(/[\n,]+/)
			.map((s) => s.trim())
			.filter(Boolean);
		for (const item of portItems) {
			const arrowMatch = item.match(
				/^(?:([0-9a-fA-F.:*]+):)?(\d+)\s*(?:->|:)\s*(\d+)(?:\/(\w+))?$/
			);
			if (arrowMatch) {
				const bindIp = arrowMatch[1] || '0.0.0.0';
				const hostPort = parseInt(arrowMatch[2], 10);
				const containerPort = parseInt(arrowMatch[3], 10);
				const proto = (arrowMatch[4] || 'tcp').toLowerCase();
				const isPublic =
					bindIp === '0.0.0.0' || bindIp === '::' || bindIp === '*' || bindIp === '[::]';
				result.push({
					id: `${c.id}-${hostPort}-${proto}`,
					hostPort,
					containerPort,
					protocol: proto,
					serviceId: c.serviceId || c.id,
					serviceName: c.serviceName || c.projectName || c.name,
					containerName: c.name,
					bindAddress: bindIp,
					isPublic,
					status: c.status === 'running' ? 'listening' : 'idle'
				});
			} else {
				const exposeMatch = item.match(/^(\d+)(?:\/(\w+))?$/);
				if (exposeMatch) {
					const containerPort = parseInt(exposeMatch[1], 10);
					const proto = (exposeMatch[2] || 'tcp').toLowerCase();
					result.push({
						id: `${c.id}-${containerPort}-${proto}`,
						hostPort: containerPort,
						containerPort: containerPort,
						protocol: proto,
						serviceId: c.serviceId || c.id,
						serviceName: c.serviceName || c.projectName || c.name,
						containerName: c.name,
						bindAddress: 'Internal network',
						isPublic: false,
						status: c.status === 'running' ? 'listening' : 'idle'
					});
				}
			}
		}
	}
	return result;
}
