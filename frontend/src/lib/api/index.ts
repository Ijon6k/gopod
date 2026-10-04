// ──────────────────────────────────────────────
// GOPOD — Modular API Client Index
// Fully typed Axios integration for backend endpoints
// ──────────────────────────────────────────────

export { apiClient } from './client';
export { projectsApi } from './projects';
export { servicesApi } from './services';
export { domainsApi } from './domains';
export { runtimeApi } from './runtime';
export { systemApi } from './system';
export { volumesApi } from './volumes';
export { trafficApi } from './traffic';

// Default aggregated API service
import projectsApi from './projects';
import servicesApi from './services';
import domainsApi from './domains';
import runtimeApi from './runtime';
import systemApi from './system';
import volumesApi from './volumes';
import trafficApi from './traffic';

export const api = {
	projects: projectsApi,
	services: servicesApi,
	domains: domainsApi,
	runtime: runtimeApi,
	system: systemApi,
	volumes: volumesApi,
	traffic: trafficApi
};

export default api;
