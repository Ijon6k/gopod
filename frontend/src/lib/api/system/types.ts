export interface Server {
	id?: string;
	hostname: string;
	ip: string;
	os: string;
	kernel: string;
	vcpu: number;
	memory: number;
	storage: number;
	storageUsed: number;
	memoryUsed: number;
	memoryAvailable?: number;
	swapUsed?: number;
	swapTotal?: number;
	cpuUsage: number;
	status: 'online' | 'offline';
	podmanVersion: string;
	rootless: boolean;
	systemd: boolean;
	quadlet: boolean;
	caddy: string;
	podmanHealth: string;
	networkHealth: string;
	storageHealth: string;
	uptime: string;
	cgroupVersion?: string;
	containersRunning?: number;
	containersTotal?: number;
	containersStopped?: number;
}

export interface SystemInfoResponse {
	hostname: string;
	os: string;
	kernel: string;
	vcpu: number;
	cpuUsage: number;
	memoryTotal: number;
	memoryFree: number;
	memoryAvailable?: number;
	memoryUsed: number;
	memoryUsedGB: number;
	memoryTotalGB: number;
	memoryAvailableGB?: number;
	swapTotal?: number;
	swapFree?: number;
	swapUsed?: number;
	swapUsedMB?: number;
	swapTotalGB?: number;
	podmanVersion: string;
	rootless: boolean;
	runningCount: number;
	stoppedCount: number;
	totalCount: number;
	uptime: string;
	cgroupVersion: string;
}

export interface ContainerStatResponse {
	id: string;
	name: string;
	cpuPercent: number;
	memUsage: number;
	memLimit: number;
	memPercent: number;
	memDisplay: string;
	netRx: number;
	netTx: number;
	netDisplay: string;
	blockInput: number;
	blockOutput: number;
	pids: number;
}

export interface SystemDiskUsage {
	type: string;
	total: number;
	active: number;
	rawSize: number;
	rawReclaimable: number;
	size: string;
	reclaimable: string;
}
