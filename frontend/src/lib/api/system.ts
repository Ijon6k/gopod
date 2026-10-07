import apiClient from './client';

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

export const systemApi = {
	/**
	 * Health check endpoint
	 */
	async health(): Promise<{ status: string; timestamp: string; socket: string }> {
		const res = await apiClient.get('/health');
		return res.data;
	},

	/**
	 * Get host and engine system metrics
	 */
	async info(): Promise<SystemInfoResponse> {
		const res = await apiClient.get<SystemInfoResponse>('/system');
		return res.data;
	},

	/**
	 * Snapshot of current container resource stats
	 */
	async stats(): Promise<ContainerStatResponse[]> {
		const res = await apiClient.get<ContainerStatResponse[]>('/stats');
		return res.data;
	},

	/**
	 * Clean up stopped containers, unused networks, and dangling images
	 */
	async prune(): Promise<{ message: string }> {
		const res = await apiClient.post<{ message: string }>('/system/prune');
		return res.data;
	},

	/**
	 * Get Podman disk usage breakdown across images, containers, and volumes
	 */
	async df(): Promise<SystemDiskUsage[]> {
		const res = await apiClient.get<SystemDiskUsage[]>('/system/df');
		return res.data;
	}
};

export default systemApi;
