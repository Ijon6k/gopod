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

import projectsJson from './projects.json';
import servicesJson from './services.json';
import deploymentsJson from './deployments.json';
import containersJson from './containers.json';
import podsJson from './pods.json';
import imagesJson from './images.json';
import volumesJson from './volumes.json';
import networksJson from './networks.json';
import domainsJson from './domains.json';
import serverJson from './server.json';
import auditLogsJson from './auditLogs.json';

import { dataStore } from '$lib/stores/data.svelte';

// Typed exports
export { dataStore };
export const projects = dataStore.projects;
export const services = dataStore.services;
export const deployments = dataStore.deployments;
export const containers = dataStore.containers;
export const pods = dataStore.pods;
export const images = imagesJson as Image[];
export const volumes = volumesJson as Volume[];
export const networks = networksJson as Network[];
export const domains = dataStore.domains;
export const server = serverJson as Server;
export const auditLogs = auditLogsJson as AuditLog[];
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
