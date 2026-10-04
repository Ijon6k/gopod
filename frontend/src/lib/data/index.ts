// ──────────────────────────────────────────────
// GOPOD — Mock Data Index
// Re-exports all JSON data with proper typing + helpers
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
	AuditLog
} from '$lib/types';

// Reference Dummy Mock Data (Preserved for documentation, testing, and offline fallback)
import dummyProjectsJson from './mock/dummy_projects.json';
import dummyServicesJson from './mock/dummy_services.json';
import dummyDeploymentsJson from './mock/dummy_deployments.json';
import dummyContainersJson from './mock/dummy_containers.json';
import dummyPodsJson from './mock/dummy_pods.json';
import dummyImagesJson from './mock/dummy_images.json';
import dummyVolumesJson from './mock/dummy_volumes.json';
import dummyNetworksJson from './mock/dummy_networks.json';
import dummyDomainsJson from './mock/dummy_domains.json';
import dummyServerJson from './mock/dummy_server.json';
import dummyAuditLogsJson from './mock/dummy_auditLogs.json';

export const dummyMockData = {
	projects: dummyProjectsJson,
	services: dummyServicesJson,
	deployments: dummyDeploymentsJson,
	containers: dummyContainersJson,
	pods: dummyPodsJson,
	images: dummyImagesJson,
	volumes: dummyVolumesJson,
	networks: dummyNetworksJson,
	domains: dummyDomainsJson,
	server: dummyServerJson,
	auditLogs: dummyAuditLogsJson
};

import { dataStore } from '$lib/stores/data.svelte';

// Typed exports
export { dataStore };
export const projects = dataStore.projects;
export const services = dataStore.services;
export const deployments = dataStore.deployments;
export const containers = dataStore.containers;
export const pods = dataStore.pods;
export const images = dummyImagesJson as Image[];
export const volumes = dummyVolumesJson as Volume[];
export const networks = dummyNetworksJson as Network[];
export const domains = dataStore.domains;
export const server = dummyServerJson as Server;
export const auditLogs = dummyAuditLogsJson as AuditLog[];
export const podmanSecrets = dataStore.podmanSecrets;
export const sshKeys = dataStore.sshKeys;
export const registries = dataStore.registries;
export const ports = dataStore.ports;
export const accessLogs = dataStore.accessLogs;
export const volumeSnapshots = dataStore.volumeSnapshots;
export const volumeSchedules = dataStore.volumeSchedules;

// ── Helper functions ──

export function getProjectServices(projectId: string): Service[] {
	return dataStore.getProjectServices(projectId);
}

export function getProjectDomains(projectId: string): Domain[] {
	return dataStore.getProjectDomains(projectId);
}

export function getProjectVolumeSnapshots(projectId: string) {
	return dataStore.getProjectVolumeSnapshots(projectId);
}

export function getProjectVolumeSchedules(projectId: string) {
	return dataStore.getProjectVolumeSchedules(projectId);
}

export function getProjectDeployments(projectId: string): Deployment[] {
	return dataStore.getProjectDeployments(projectId);
}

export function getServiceDeployments(serviceId: string): Deployment[] {
	return dataStore.getServiceDeployments(serviceId);
}

export function getServiceContainers(serviceId: string): Container[] {
	return dataStore.getServiceContainers(serviceId);
}

export function getProjectById(projectId: string): Project | undefined {
	return dataStore.getProjectById(projectId);
}

export function getServiceById(serviceId: string): Service | undefined {
	return dataStore.getServiceById(serviceId);
}

// ── Monitoring time series (generated) ──

function generateSeries(base: number, variance: number, points: number = 48): TimeSeriesPoint[] {
	return Array.from({ length: points }, (_, i) => {
		const t = new Date(Date.now() - (points - i) * 30 * 60 * 1000);
		return {
			time: t.toISOString(),
			label: `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}`,
			value: Math.max(0, base + (Math.random() - 0.5) * variance)
		};
	});
}

export const monitoringData: Record<string, TimeSeriesPoint[]> = {
	cpu: generateSeries(14, 12),
	memory: generateSeries(38, 8),
	storage: generateSeries(30.6, 1),
	network: generateSeries(12, 18)
};
