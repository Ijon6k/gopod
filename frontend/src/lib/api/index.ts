// ──────────────────────────────────────────────
// GOPOD — Modular API Index
// Clean exports of domain modules & centralized api object
// ──────────────────────────────────────────────

export { apiClient, ApiError } from './client';

export * as projectsDomain from './projects';
export * as servicesDomain from './services';
export * as runtimeDomain from './runtime';
export * as systemDomain from './system';
export * as credentialsDomain from './credentials';
export * as domainsDomain from './domains';
export * as storageDomain from './storage';
export * as auditDomain from './audit';
export * as trafficDomain from './traffic';
export * as authDomain from './auth';

export { projectsApi } from './projects/api';
export { servicesApi } from './services/api';
export { runtimeApi } from './runtime/api';
export { systemApi } from './system/api';
export { credentialsApi } from './credentials/api';
export { domainsApi } from './domains/api';
export { storageApi as volumesApi } from './storage/api';
export { auditApi } from './audit/api';
export { trafficApi } from './traffic/api';
export { authApi } from './auth/api';

import { projectsApi } from './projects/api';
import { servicesApi } from './services/api';
import { runtimeApi } from './runtime/api';
import { systemApi } from './system/api';
import { credentialsApi } from './credentials/api';
import { domainsApi } from './domains/api';
import { storageApi } from './storage/api';
import { auditApi } from './audit/api';
import { trafficApi } from './traffic/api';
import { authApi } from './auth/api';

export const api = {
	projects: projectsApi,
	services: servicesApi,
	runtime: runtimeApi,
	system: systemApi,
	credentials: credentialsApi,
	domains: domainsApi,
	volumes: storageApi,
	audit: auditApi,
	traffic: trafficApi,
	auth: authApi
};

export default api;
