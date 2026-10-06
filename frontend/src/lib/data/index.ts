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

// Typed exports — dynamic proxies delegating to live reactive dataStore
export { dataStore };

function createArrayProxy<T>(getArray: () => T[]): T[] {
	return new Proxy([] as T[], {
		get(_, prop) {
			const arr = getArray();
			const val = Reflect.get(arr, prop, arr);
			if (typeof val === 'function') {
				return val.bind(arr);
			}
			return val;
		},
		has(_, prop) {
			return Reflect.has(getArray(), prop);
		},
		ownKeys(_) {
			return Reflect.ownKeys(getArray());
		},
		getOwnPropertyDescriptor(_, prop) {
			return Reflect.getOwnPropertyDescriptor(getArray(), prop);
		}
	});
}

function createObjectProxy<T extends object>(getObject: () => T): T {
	return new Proxy({} as T, {
		get(_, prop) {
			const obj = getObject();
			const val = Reflect.get(obj, prop, obj);
			if (typeof val === 'function') {
				return val.bind(obj);
			}
			return val;
		},
		has(_, prop) {
			return Reflect.has(getObject(), prop);
		},
		ownKeys(_) {
			return Reflect.ownKeys(getObject());
		},
		getOwnPropertyDescriptor(_, prop) {
			return Reflect.getOwnPropertyDescriptor(getObject(), prop);
		}
	});
}

export const projects = createArrayProxy(() => dataStore.projects);
export const services = createArrayProxy(() => dataStore.services);
export const deployments = createArrayProxy(() => dataStore.deployments);
export const containers = createArrayProxy(() => dataStore.containers);
export const pods = createArrayProxy(() => dataStore.pods);
export const images = createArrayProxy(() => dataStore.images);
export const volumes = createArrayProxy(() => dataStore.volumes);
export const networks = createArrayProxy(() => dataStore.networks);
export const domains = createArrayProxy(() => dataStore.domains);
export const server = createObjectProxy(() => dataStore.server);

export const auditLogs: AuditLog[] = [];

export const podmanSecrets = createArrayProxy(() => dataStore.podmanSecrets);
export const sshKeys = createArrayProxy(() => dataStore.sshKeys);
export const registries = createArrayProxy(() => dataStore.registries);
export const ports = createArrayProxy(() => dataStore.ports);
export const accessLogs = createArrayProxy(() => dataStore.accessLogs);
export const volumeSnapshots = createArrayProxy(() => dataStore.volumeSnapshots);
export const volumeSchedules = createArrayProxy(() => dataStore.volumeSchedules);

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

export function createProject(data: { name: string; description?: string }): Promise<Project> {
	return dataStore.createProject(data);
}

export function deleteProject(id: string): Promise<void> {
	return dataStore.deleteProject(id);
}

export function getServiceById(serviceId: string): Service | undefined {
	return dataStore.getServiceById(serviceId);
}

export function deleteService(id: string): Promise<void> {
	return dataStore.deleteService(id);
}

export function updateService(service: Service): Promise<Service> {
	return dataStore.updateService(service);
}

// ── Monitoring live metrics series (proxied to live dataStore stream) ──

export const monitoringData: Record<string, TimeSeriesPoint[]> = createObjectProxy(
	() => dataStore.monitoringData
);
