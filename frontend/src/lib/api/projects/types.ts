import type { Status } from '$lib/types/common';
import type { Service } from '$lib/api/services/types';
import type { Domain } from '$lib/api/domains/types';

export interface Project {
	id: string;
	name: string;
	description: string;
	status: Status;
	services: Service[];
	serviceCount?: number;
	domains: Domain[];
	domainCount?: number;
	cpu: number;
	memory: number;
	memoryTotal: number;
	createdAt: string;
}
