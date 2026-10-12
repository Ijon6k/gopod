export interface CaddyAccessLog {
	id: string;
	timestamp: string;
	timeAgo: string;
	clientIp: string;
	method: 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH' | 'HEAD' | 'OPTIONS' | string;
	host: string;
	uri: string;
	status: number;
	durationMs: number;
	duration?: number;
	bytesSent?: string;
	size?: number;
	serviceName?: string;
	upstream?: string;
	userAgent?: string;
	serviceId?: string;
	projectId?: string;
}
