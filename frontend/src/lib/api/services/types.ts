import type { Status } from '$lib/types/common';

export type ServiceType =
	'application' | 'image' | 'compose' | 'pod' | 'kubernetes' | 'quadlet' | 'database';

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
		apparmorProfile?: string;
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

export interface DeploymentStep {
	id: string;
	name: string;
	status: 'success' | 'running' | 'failed' | 'pending';
	duration?: string;
}

export interface Deployment {
	id: string;
	projectId: string;
	projectName?: string;
	serviceId: string;
	serviceName?: string;
	number?: number;
	version?: string;
	commit?: string;
	commitHash?: string;
	commitMessage: string;
	branch?: string;
	status: Status;
	duration: string;
	timeAgo?: string;
	startedAt: string;
	finishedAt?: string;
	trigger?: 'git-push' | 'manual' | 'rollback' | 'webhook' | 'cli' | 'compose' | 'quadlet' | string;
	author?: string;
	isCurrent?: boolean;
	image?: string;
	logs?: string[];
	steps?: DeploymentStep[];
}

export interface Service {
	id: string;
	projectId: string;
	name: string;
	type: ServiceType;
	status: Status;
	source?: string;
	sourceType?: ServiceType | 'git' | 'image' | 'drop';
	branch?: string;
	sshKeyId?: string;
	image?: string;
	registryId?: string;
	buildType?: 'dockerfile' | 'nixpacks' | 'static';
	buildPath?: string;
	dockerfilePath?: string;
	runtimeTarget?: 'standalone' | 'pod' | 'quadlet';
	inPod?: boolean;
	podId?: string;
	command?: string;
	entrypoint?: string;
	domain?: string;
	port?: number;
	hostPort?: number;
	/** Bind address for the published host port: '0.0.0.0' (all interfaces) or '127.0.0.1' (loopback only). Undefined = 0.0.0.0. */
	hostIp?: string;
	cpu: number;
	memory: number;
	cpuLimit?: number;
	memoryLimit?: number;
	restartPolicy?: string;
	health?: string;
	replicas: number;
	description?: string;
	workloads?: Workload[];
	envVars: EnvVar[];
	deployments?: Deployment[];
	createdAt: string;
	updatedAt?: string;
	composeYaml?: string;
	k8sYaml?: string;
	quadletConfig?: string;
	quadletUnit?: string;
	quadletFile?: string;
	quadletActive?: boolean;
	secretMounts?: ServiceSecretMount[];
	advanced?: ServiceAdvancedConfig;
	autoDeploy?: boolean;
	watchPaths?: string;
	/** True when the backend holds a webhook token for this service. The token itself is never serialized; fetch it via /services/{id}/webhook. */
	hasWebhook?: boolean;
	/** True when the encrypted env blob could not be decrypted with the current master key. */
	envDecryptionError?: boolean;
	publishDir?: string;
}

export interface DeployResponse {
	status: string;
	serviceId: string;
	containerName: string;
	deployedAt: string;
}

export interface JournalResponse {
	serviceId: string;
	journal: string;
}

export interface QuadletValidationResponse {
	valid: boolean;
	output: string;
	error?: string;
}

export interface WebhookTokenResponse {
	webhookToken: string;
	hasWebhook: boolean;
}

export interface RevealEnvResponse {
	values: Record<string, string>;
}

export interface GroupPodRequest {
	podName: string;
	createPod?: boolean;
	ports?: string[];
}
