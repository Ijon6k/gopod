export interface AuditLog {
	id: string;
	action: string;
	category:
		| 'deployment'
		| 'runtime'
		| 'settings'
		| 'security'
		| 'network'
		| 'service'
		| 'auth'
		| 'system'
		| string;
	actor: string;
	target: string;
	status: 'success' | 'failed' | 'healthy' | 'running' | 'warning' | string;
	ip: string;
	timeAgo: string;
	timestamp?: string;
	createdAt?: string;
	userId?: string;
	userName?: string;
	targetId?: string;
	targetName?: string;
	details?: Record<string, any>;
}
