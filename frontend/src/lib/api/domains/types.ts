export type DomainStatus = 'active' | 'pending' | 'error';
export type DNSCheckStatus = 'local' | 'valid' | 'cloudflare' | 'mismatch' | 'pending';

export interface DNSCheckResult {
	hostname: string;
	status: DNSCheckStatus;
	valid: boolean;
	behindCloudflare: boolean;
	resolvedIps: string[];
	resolvedIp?: string;
	serverIp?: string;
	serverIpPublic: boolean;
	cname?: string;
	message: string;
}

export interface Domain {
	id: string;
	hostname: string;
	projectId: string;
	serviceId: string;
	serviceName?: string;
	targetContainer?: string;
	tls: boolean;
	status: DomainStatus;
	proxyPort?: number;
	containerPort?: number;
	publishedPort?: number;
	targetPort?: number;
	dnsStatus?: 'valid' | 'pending' | 'mismatch' | 'error';
	resolvedIp?: string;
	resolvedIps?: string[];
	behindCloudflare?: boolean;
	dnsCheck?: DNSCheckResult;
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
	/** Write-only: accepted on create/update, never returned by the API. */
	basicAuthPass?: string;
	/** Computed by the backend: a password hash is stored for this domain. */
	hasBasicAuth?: boolean;
	basicAuthUsers?: { username: string; hash: string }[];
	customHeaders?: Record<string, string>;
	rateLimitRps?: number;
	rateLimitBurst?: number;
	ipAllowlist?: string[];
	caddyRouteId?: string;
	createdAt?: string;
}
