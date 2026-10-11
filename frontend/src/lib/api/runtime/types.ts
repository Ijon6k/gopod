import type { Status } from '$lib/types/common';
import type { VolumeStatus } from '$lib/api/storage/types';

export interface Container {
	id: string;
	name: string;
	projectId: string;
	projectName?: string;
	serviceId: string;
	serviceName?: string;
	image: string;
	status: Status;
	cpu: number;
	memory: number;
	ports: string;
	startedAt?: string;
	memoryLimit?: number;
	netRx?: string;
	netTx?: string;
	blockRead?: string;
	blockWrite?: string;
	pids?: number;
	restarts?: number;
	uptime?: string;
	labels?: Record<string, string>;
}

export interface Pod {
	id: string;
	name: string;
	projectId?: string;
	projectName?: string;
	containers: string[];
	status: Status;
	network?: string;
	createdAt?: string;
	labels?: Record<string, string>;
}

export interface Image {
	id: string;
	name?: string;
	repository?: string;
	tag: string;
	size: string;
	usedBy?: string;
	createdAt?: string;
}

export interface Volume {
	id: string;
	name: string;
	projectId?: string;
	projectName?: string;
	serviceId: string;
	serviceName?: string;
	mount?: string;
	size: string;
	status: VolumeStatus;
	driver?: string;
	scope?: string;
	mountpoint?: string;
	inUse?: boolean;
	labels?: Record<string, string>;
	created?: string;
}

export interface Network {
	id: string;
	name: string;
	driver: string;
	projects: string[];
	containers?: number | string[];
	subnet: string;
	gateway: string;
	internal?: boolean;
	dnsEnabled?: boolean;
	created?: string;
}

export interface PortMapping {
	id: string;
	hostPort: number;
	containerPort: number;
	protocol: 'tcp' | 'udp' | string;
	serviceId?: string;
	serviceName?: string;
	containerName?: string;
	bindAddress?: '0.0.0.0' | '127.0.0.1' | string;
	isPublic?: boolean;
	status?: 'listening' | 'idle' | string;
}

export interface PodmanContainerItem {
	id: string;
	names: string[];
	image: string;
	status: string;
	state: string;
	created: string;
	ports: string;
	stats?: {
		id: string;
		name: string;
		cpuPercent: number;
		memPercent: number;
		memDisplay: string;
		netDisplay: string;
		pids: number;
	};
}

export interface ContainerLogsResponse {
	id: string;
	logs: string;
	tail: number;
}
