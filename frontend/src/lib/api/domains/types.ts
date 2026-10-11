export type DomainStatus = 'active' | 'pending' | 'error';

export interface Domain {
	id: string;
	hostname: string;
	projectId: string;
	serviceId: string;
	serviceName?: string;
	tls: boolean;
	status: DomainStatus;
	proxyPort?: number;
	containerPort?: number;
	publishedPort?: number;
	targetPort?: number;
	dnsStatus?: 'valid' | 'pending' | 'error';
	resolvedIp?: string;
	httpsRedirect?: boolean;
	autoHttps?: boolean;
	pathPrefix?: string;
	stripPathPrefix?: boolean;
	stripPath?: boolean;
	websocket?: boolean;
	cors?: boolean;
	enableCors?: boolean;
	corsOrigins?: string[];
	hsts?: boolean;
	basicAuth?: boolean;
	basicAuthUser?: string;
	basicAuthUsers?: { username: string; hash: string }[];
	customHeaders?: Record<string, string>;
	rateLimitRps?: number;
	rateLimitBurst?: number;
	ipAllowlist?: string[];
	caddyRouteId?: string;
	createdAt?: string;
}
