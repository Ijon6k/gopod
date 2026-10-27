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
	/** Container engine on the host: 'podman' | 'docker' | '' when unknown. */
	engine?: string;
	engineVersion?: string;
	uptime: string;
	cgroupVersion?: string;
	containersRunning?: number;
	containersTotal?: number;
	containersStopped?: number;
}

export interface SystemInfoResponse {
	ip?: string;
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
	/** Container engine detected by the backend: 'podman' | 'docker' | ''. */
	engine?: string;
	engineVersion?: string;
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

export interface SystemLingerResponse {
	linger: boolean;
	user: string;
}

export interface GCPreview {
	busy: boolean;
	danglingImages: number;
	danglingImageBytes: number;
	stoppedContainers: number;
	reclaimableBytes: number;
	staleUnitFiles: string[] | null;
	orphanVolumes: string[] | null;
}

export interface GCResult {
	scope: string;
	skipped: boolean;
	reason?: string;
	danglingImagesRemoved: number;
	reclaimedBytes: number;
	unitFilesRemoved?: string[] | null;
	volumesRemoved?: string[] | null;
	errors?: string[] | null;
}

export interface KeyStatusResponse {
	keyFilePath: string;
	keyExists: boolean;
	keyFromEnv: boolean;
	fingerprint: string;
	algorithm: string;
}

/** Offline integrity report for one disaster recovery archive (backend verify probe). */
export interface SystemBackupVerification {
	valid: boolean;
	checkedAt: string;
	archiveReadable: boolean;
	manifestFound: boolean;
	manifestVersion?: number;
	manifestCreatedAt?: string;
	manifestKeyFromEnv: boolean;
	keyBundleFound: boolean;
	databaseFound: boolean;
	/** "ok" or the integrity failure description; empty when not probed. */
	databaseIntegrity?: string;
	/** "match" | "mismatch" | "unavailable" */
	checksumMatch: string;
	entries?: string[];
	errors?: string[];
	warnings?: string[];
}
