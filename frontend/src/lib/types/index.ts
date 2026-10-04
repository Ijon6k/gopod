// ──────────────────────────────────────────────
// GOPOD — Shared TypeScript Types
// ──────────────────────────────────────────────

export type Status =
	| 'running'
	| 'stopped'
	| 'degraded'
	| 'failed'
	| 'deploying'
	| 'healthy'
	| 'unhealthy'
	| 'building'
	| 'cancelled'
	| 'queued';

export type DomainStatus = 'active' | 'pending' | 'error';
export type VolumeStatus = 'mounted' | 'unmounted';
export type ServiceType = 'application' | 'image' | 'compose' | 'pod' | 'kubernetes' | 'quadlet' | 'database';

export interface SSHKey {
	id: string;
	name: string;
	publicKey: string;
	privateKey?: string;
	fingerprint: string;
	type: 'ed25519' | 'rsa';
	createdAt: string;
}

export interface ContainerRegistry {
	id: string;
	name: string;
	url: string;
	username: string;
	token?: string;
	createdAt: string;
}

export interface PodmanSecret {
	id: string;
	name: string;
	createdAt: string;
	driver?: string;
}

export interface ServiceSecretMount {
	secretId: string;
	secretName: string;
	type: 'env' | 'file';
	envVar?: string;
	mountPath?: string;
}

export interface ServiceAdvancedConfig {
	runtime: {
		mode: 'rootless' | 'rootful';
		userNamespace: string;
		devices: string[];
	};
	lifecycle: {
		quadletEnabled: boolean;
		systemdUnitName: string;
		restartPolicy: string;
	};
	security: {
		privileged: boolean;
		selinuxLabel: string;
		capAdd: string[];
		capDrop: string[];
		noNewPrivileges: boolean;
	};
	storage: {
		volumes: { source: string; target: string; options: string }[];
	};
	resources: {
		cpuLimit: string;
		memoryLimit: string;
		pidsLimit: number;
		swapLimit: string;
	};
}

export interface Project {
	id: string;
	name: string;
	description: string;
	status: Status;
	services: Service[];
	domains: Domain[];
	cpu: number;
	memory: number;
	memoryTotal: number;
	createdAt: string;
}

export interface Service {
	id: string;
	projectId: string;
	name: string;
	type: ServiceType;
	status: Status;
	source: string;
	sourceType?: 'git' | 'image' | 'drop';
	branch?: string;
	sshKeyId?: string;
	image?: string;
	registryId?: string;
	buildType?: 'dockerfile' | 'nixpacks' | 'static';
	buildPath?: string;
	dockerfilePath?: string;
	runtimeTarget?: 'standalone' | 'pod' | 'quadlet';
	podId?: string;
	command?: string;
	entrypoint?: string;
	domain?: string;
	port: number;
	cpu: number;
	memory: number;
	restartPolicy: string;
	health: string;
	replicas: number;
	description?: string;
	workloads?: Workload[];
	envVars: EnvVar[];
	deployments: Deployment[];
	createdAt: string;
	composeYaml?: string;
	k8sYaml?: string;
	quadletConfig?: string;
	secretMounts?: ServiceSecretMount[];
	advanced?: ServiceAdvancedConfig;
	autoDeploy?: boolean;
	watchPaths?: string;
	webhookToken?: string;
}

export interface Workload {
	name: string;
	image: string;
	status: Status;
	cpu?: number;
	memory?: number;
	ports?: string;
}

export interface EnvVar {
	key: string;
	value: string;
	secret: boolean;
}

export interface Domain {
	id: string;
	hostname: string;
	projectId: string;
	serviceId: string;
	serviceName: string;
	tls: boolean;
	status: DomainStatus;
	proxyPort: number;
	containerPort: number;
	publishedPort: number;
	dnsStatus?: 'valid' | 'pending' | 'error';
	resolvedIp?: string;
	httpsRedirect?: boolean;
	pathPrefix?: string;
	stripPathPrefix?: boolean;
	websocket?: boolean;
	cors?: boolean;
	hsts?: boolean;
	basicAuth?: boolean;
	basicAuthUser?: string;
}

export interface PortMapping {
	id: string;
	hostPort: number;
	containerPort: number;
	protocol: 'tcp' | 'udp';
	serviceId: string;
	serviceName: string;
	containerName: string;
	bindAddress: '0.0.0.0' | '127.0.0.1';
	isPublic: boolean;
	status: 'listening' | 'idle';
}

export interface CaddyAccessLog {
	id: string;
	timestamp: string;
	timeAgo: string;
	clientIp: string;
	method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'HEAD' | 'OPTIONS';
	host: string;
	uri: string;
	status: number;
	durationMs: number;
	bytesSent: string;
	serviceName: string;
	upstream: string;
}

export interface VolumeSnapshot {
	id: string;
	volumeName: string;
	projectId: string;
	serviceId: string;
	filename: string;
	size: string;
	sizeBytes: number;
	createdAt: string;
	timeAgo: string;
	status: 'completed' | 'in_progress' | 'failed';
	compression: 'zstd' | 'gzip';
}

export interface VolumeBackupSchedule {
	id: string;
	projectId: string;
	volumeName: string;
	cron: string;
	label: string;
	retentionCount: number;
	enabled: boolean;
	lastRun?: string;
	nextRun?: string;
}

export interface DeploymentStep {
	id: string;
	name: string;
	status: 'success' | 'running' | 'failed' | 'pending';
	duration?: string;
}

export interface Deployment {
	id: string;
	projectId: string;
	projectName: string;
	serviceId: string;
	serviceName: string;
	number: number;
	version: string;
	commit: string;
	commitMessage: string;
	branch: string;
	status: Status;
	duration: string;
	timeAgo: string;
	startedAt: string;
	finishedAt: string;
	trigger?: 'git-push' | 'manual' | 'rollback' | 'webhook' | 'cli';
	author?: string;
	isCurrent?: boolean;
	image?: string;
	logs?: string[];
	steps?: DeploymentStep[];
}

export interface Container {
	id: string;
	name: string;
	projectId: string;
	projectName: string;
	serviceId: string;
	serviceName: string;
	image: string;
	status: Status;
	cpu: number;
	memory: number;
	ports: string;
	startedAt: string;
	memoryLimit?: number;
	netRx?: string;
	netTx?: string;
	blockRead?: string;
	blockWrite?: string;
	pids?: number;
	restarts?: number;
	uptime?: string;
}

export interface Pod {
	id: string;
	name: string;
	projectId: string;
	projectName: string;
	containers: string[];
	status: Status;
	network: string;
	createdAt: string;
}

export interface Image {
	id: string;
	name: string;
	tag: string;
	size: string;
	usedBy: string;
	createdAt: string;
}

export interface Volume {
	id: string;
	name: string;
	projectId: string;
	projectName: string;
	serviceId: string;
	serviceName: string;
	mount: string;
	size: string;
	status: VolumeStatus;
}

export interface Network {
	id: string;
	name: string;
	driver: string;
	projects: string[];
	containers: number;
	subnet: string;
	gateway: string;
}

export interface Server {
	hostname: string;
	ip: string;
	os: string;
	kernel: string;
	vcpu: number;
	memory: number;
	storage: number;
	storageUsed: number;
	memoryUsed: number;
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
}

export interface TimeSeriesPoint {
	time: string;
	label: string;
	value: number;
}

export interface NavItem {
	label: string;
	path?: string;
	icon?: string;
	children?: NavItem[];
}

export interface AuditLog {
	id: string;
	action: string;
	category: 'deployment' | 'runtime' | 'settings' | 'security' | 'network';
	actor: string;
	target: string;
	status: 'healthy' | 'failed' | 'running';
	ip: string;
	timeAgo: string;
	timestamp: string;
}

