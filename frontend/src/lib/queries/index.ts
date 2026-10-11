// ──────────────────────────────────────────────
// GOPOD — Centralized Reactive Queries Facade
// Re-exports domain queries with standardized Query Keys
// ──────────────────────────────────────────────

import { projectKeys } from '$lib/api/projects/keys';
import { serviceKeys } from '$lib/api/services/keys';
import { runtimeKeys } from '$lib/api/runtime/keys';
import { systemKeys } from '$lib/api/system/keys';
import { domainKeys } from '$lib/api/domains/keys';
import { credentialKeys } from '$lib/api/credentials/keys';
import { storageKeys } from '$lib/api/storage/keys';
import { auditKeys } from '$lib/api/audit/keys';
import { trafficKeys } from '$lib/api/traffic/keys';
import { authKeys } from '$lib/api/auth/keys';

export const queryKeys = {
	projects: projectKeys,
	services: serviceKeys,
	runtime: runtimeKeys,
	system: systemKeys,
	domains: domainKeys,
	credentials: credentialKeys,
	volumes: storageKeys,
	audit: auditKeys,
	traffic: trafficKeys,
	auth: authKeys
};

// Re-export all domain queries & hooks
export * from '$lib/api/projects/queries';
export * from '$lib/api/projects/mutations';

export * from '$lib/api/services/queries';
export * from '$lib/api/services/mutations';

export * from '$lib/api/runtime/queries';
export * from '$lib/api/runtime/mutations';

export * from '$lib/api/system/queries';

export * from '$lib/api/credentials/queries';
export * from '$lib/api/credentials/mutations';

export * from '$lib/api/domains/queries';
export * from '$lib/api/domains/mutations';

export * from '$lib/api/storage/queries';
export * from '$lib/api/storage/mutations';

export * from '$lib/api/audit/queries';

export * from '$lib/api/traffic/queries';

export * from '$lib/api/auth/queries';
export * from '$lib/api/auth/mutations';
