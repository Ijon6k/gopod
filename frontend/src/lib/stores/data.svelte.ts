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
	BackupDestination,
	QuadletUnit
} from '$lib/types';

import { api } from '$lib/api';
import { ProjectsDomainStore } from './domains/projects.svelte';
import { RuntimeDomainStore } from './domains/runtime.svelte';
import { CredentialsDomainStore } from './domains/credentials.svelte';
import { StorageDomainStore } from './domains/storage.svelte';
import { TelemetryDomainStore } from './domains/telemetry.svelte';
import {
	extractContainerHealth,
	parseContainerPortMappings,
	resolveContainerMetadata as resolveContainerMeta,
	resolveVolumeMetadata as resolveVolumeMeta
} from './container-meta';

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
		// Ownership is resolved at read time so a volume list taken before the
		// services snapshot still re-attaches to the right project/service the
		// moment both datasets exist (mirrors relinkContainers for containers).
		return this.runtimeStore.volumes.map((v) => {
			const meta = this.resolveVolumeMetadata(v.name, v.labels);
			return {
				...v,
				projectId: meta.projId,
				projectName: meta.projName,
				serviceId: meta.servId,
				serviceName: meta.servName
			};
		});
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

	get backupDestinations() {
		return this.storageStore.backupDestinations;
	}

	// ── Storage Domain Actions ──
	getProjectVolumeSnapshots(projectId: string) {
		return this.storageStore.getProjectVolumeSnapshots(projectId);
	}
	getProjectVolumeSchedules(projectId: string) {
		return this.storageStore.getProjectVolumeSchedules(projectId);
	}
	getServiceVolumeSnapshots(serviceId: string) {
		return this.storageStore.getServiceVolumeSnapshots(serviceId);
	}
	getServiceVolumeSchedules(serviceId: string) {
		return this.storageStore.getServiceVolumeSchedules(serviceId);
	}
	createVolumeSnapshot(volumeName: string, projectId: string, serviceId: string) {
		return this.storageStore.createVolumeSnapshot(volumeName, projectId, serviceId);
	}
	createDatabaseDump(req: import('$lib/types').DatabaseDumpRequest) {
		return this.storageStore.createDatabaseDump(req);
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
	createVolumeSchedule(input: {
		projectId: string;
		serviceId?: string;
		volumeName: string;
		cron: string;
		label?: string;
		retentionCount: number;
		destinationId?: string;
	}) {
		return this.storageStore.createVolumeSchedule(input);
	}
	fetchBackupDestinations() {
		return this.storageStore.fetchBackupDestinations();
	}
	createBackupDestination(input: Omit<BackupDestination, 'id'>) {
		return this.storageStore.createBackupDestination(input);
	}
	deleteBackupDestination(id: string) {
		return this.storageStore.deleteBackupDestination(id);
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
					const statusText = (rc.status || '').trim();
					const health = extractContainerHealth(statusText, rc.state);

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
						startedAt: rc.startedAt || '',
						netRx: stat?.netDisplay ? stat.netDisplay.split('/')[0]?.trim() : '—',
						netTx: stat?.netDisplay ? stat.netDisplay.split('/')[1]?.trim() : '—',
						blockRead: '—',
						blockWrite: '—',
						pids: stat?.pids || (isRunning ? 1 : 0),
						restarts: 0,
						uptime: statusText || (isRunning ? 'Up' : 'Stopped'),
						health,
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

	// Pure resolution lives in ./container-meta; this facade feeds it the
	// live snapshot (services, projects, per-project service lookup).
	resolveContainerMetadata(
		name: string,
		labels?: Record<string, string>
	): { projId: string; projName: string; servId: string; servName: string } {
		return resolveContainerMeta(name, labels, {
			services: this.services,
			projects: this.projectsStore.projects,
			getProjectServices: (projectId: string) => this.getProjectServices(projectId)
		});
	}

	resolveVolumeMetadata(
		name: string,
		labels?: Record<string, string>
	): { projId: string; projName: string; servId: string; servName: string } {
		return resolveVolumeMeta(name, labels, {
			services: this.services,
			projects: this.projectsStore.projects,
			getProjectServices: (projectId: string) => this.getProjectServices(projectId)
		});
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
