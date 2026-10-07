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
	PortMapping,
	CaddyAccessLog,
	VolumeSnapshot,
	VolumeBackupSchedule
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

	get monitoringData(): Record<string, TimeSeriesPoint[]> {
		return this.telemetryStore.monitoringData;
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
		this.projectsStore.cancelDeployment(id);
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
	deleteService(id: string) {
		return this.projectsStore.deleteService(id);
	}
	deployService(serviceId: string, trigger = 'manual') {
		return this.projectsStore.deployService(serviceId, trigger);
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
	addDomain(domain: Domain) {
		this.projectsStore.addDomain(domain);
	}
	updateDomain(domain: Domain) {
		this.projectsStore.updateDomain(domain);
	}
	deleteDomain(id: string) {
		this.projectsStore.deleteDomain(id);
	}

	// ── Runtime Domain Actions ──
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
	pruneImages(all = false) {
		return this.runtimeStore.pruneImages(all);
	}
	pruneVolumes() {
		return this.runtimeStore.pruneVolumes();
	}
	pruneSystem() {
		return this.runtimeStore.pruneSystem();
	}
	fetchRuntimeData() {
		return this.runtimeStore.fetchRuntimeData();
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
	toggleVolumeSchedule(id: string) {
		return this.storageStore.toggleVolumeSchedule(id);
	}

	// ── Telemetry & Live Polling Actions ──
	startStreamingStats() {
		this.telemetryStore.startStreamingStats((stats) => {
			this.applyStatsUpdate(stats);
		});
	}
	stopStreamingStats() {
		this.telemetryStore.stopStreamingStats();
	}
	applySystemUpdate(sys: any) {
		this.telemetryStore.applySystemUpdate(sys);
	}

	private applyStatsUpdate(stats: any[]) {
		for (const s of stats) {
			const found = this.runtimeStore.containers.find((c) => c.name === s.name || c.id === s.id);
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

					let projName = 'System Host';
					let projId = 'system';
					let servName = name;
					let servId = name;

					// Label-first association, then compose project label, then prefix / name matching
					if (rc.labels && rc.labels['io.gopod.project']) {
						projId = rc.labels['io.gopod.project'];
						servId = rc.labels['io.gopod.service'] || name;
						servName = rc.labels['io.gopod.name'] || name;
						projName = this.projectsStore.getProjectById(projId)?.name || projId;
					} else if (rc.labels && rc.labels['com.docker.compose.project']) {
						const composeProject = rc.labels['com.docker.compose.project'];
						const matchedService = this.services.find(
							(s: Service) => s.id === composeProject || s.name === composeProject
						);
						if (matchedService) {
							servId = matchedService.id;
							servName = rc.labels['com.docker.compose.service'] || matchedService.name;
							projId = matchedService.projectId;
							projName = this.projectsStore.getProjectById(projId)?.name || projId;
						} else {
							servId = composeProject;
							servName = rc.labels['com.docker.compose.service'] || name;
						}
					} else {
						// Service ID / name prefix matching
						const matchedService = this.services.find(
							(s: Service) => (s.id && (name.startsWith(s.id) || name.includes(s.id))) || (s.name && (name.startsWith(s.name) || name.includes(s.name)))
						);
						if (matchedService) {
							servId = matchedService.id;
							servName = matchedService.name;
							projId = matchedService.projectId;
							projName = this.projectsStore.getProjectById(projId)?.name || projId;
						} else {
							const parts = name.split('-');
							if (parts.length >= 3) {
								const potentialProj = this.projectsStore.projects.find(
									(p) => p.id === parts[0] || p.name.toLowerCase() === parts[0]
								);
								if (potentialProj) {
									projId = potentialProj.id;
									projName = potentialProj.name;
									servName = parts.slice(1, parts.length - 1).join('-');
									servId = `${projId}-${servName}`;
								}
							}
						}
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

	async fetchInitialData() {
		await Promise.all([
			this.projectsStore.fetchProjectsData(),
			this.runtimeStore.fetchRuntimeData(),
			this.credentialsStore.fetchCredentials(),
			this.storageStore.fetchStorageData(),
			this.telemetryStore.fetchTrafficLogs(),
			this.fetchLiveStats()
		]);
	}
}

export const dataStore = new DataStore();
