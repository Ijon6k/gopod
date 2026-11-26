export interface Server {
	id?: string;
	hostname: string;
	ip: string;
	localIp?: string;
	ipIsPublic?: boolean;
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
	localIp?: string;
	ipIsPublic?: boolean;
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
	storageUsedGB?: number;
	storageTotalGB?: number;
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

export interface PanelSettings {
	domain: string;
	lockCatchall: boolean;
	tls: boolean;
}

export interface AdminUserSummary {
	id: string;
	email: string;
	name: string;
	role: 'admin' | 'member';
	permissions?: import('../auth/types').UserPermissions;
	projectIds: string[];
	projectScopes?: import('../auth/types').ProjectScope[];
	createdAt: string;
}

export interface SelfInspection {
	mode: 'quadlet-pod' | 'quadlet-standalone' | 'compose' | 'container' | 'dev-local' | 'native';
	containerized: boolean;
	containerId?: string;
	containerName?: string;
	runningImage?: string;
	localDigest?: string;
	podName?: string;
	unitName?: string;
	quadletDir?: string;
	dbusReachable: boolean;
	podmanSocketOk: boolean;
	isDevBuild: boolean;
	canAutoUpdate: boolean;
	autoUpdateReason?: string;
	manualHint?: string;
	executablePath?: string;
}

export interface PodmanCompatibility {
	version: string;
	recommendedMinVersion: string;
	compatible: boolean;
	rootless: boolean;
	os: string;
	warning?: string;
	upgradeHint: string;
}

export interface UpdateStatus {
	state: 'idle' | 'updating' | 'succeeded' | 'rolled_back' | 'failed';
	targetTag?: string;
	targetRef?: string;
	message?: string;
	startedAt?: string;
	finishedAt?: string;
}

export interface UpdateCheckResponse {
	repository: string;
	imageRepo: string;
	currentVersion: string;
	buildCommit?: string;
	buildDate?: string;
	channel: 'latest' | 'dev';
	latestVersion?: string;
	latestTag?: string;
	releaseUrl?: string;
	releaseNotes?: string;
	publishedAt?: string;
	remoteDigest?: string;
	updateAvailable: boolean;
	checkedAt: string;
	checkError?: string;
	self: SelfInspection;
	podman: PodmanCompatibility;
	lastStatus: UpdateStatus;
}

export interface VersionInfoResponse {
	version: string;
	buildCommit?: string;
	buildDate?: string;
	channel: 'latest' | 'dev';
	mode: string;
	isDevBuild: boolean;
}

export interface NotificationChannel {
	id: string;
	name: string;
	provider: 'discord' | 'telegram';
	events: string[];
	enabled: boolean;
	webhookHint?: string;
	chatId?: string;
	hasSecret: boolean;
	createdAt: string;
}

export interface MetricSample {
	timestamp: string;
	label: string;
	scope: string;
	targetId: string;
	cpuPercent: number;
	memUsedBytes: number;
	memLimitBytes: number;
	storageUsedGB: number;
	storageTotalGB: number;
	netRxBps: number;
	netTxBps: number;
}
