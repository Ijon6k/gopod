export interface UserPermissions {
	canCreateProjects: boolean;
	canDeleteProjects: boolean;
	canCreateServices: boolean;
	canDeployServices: boolean;
	canDeleteServices: boolean;
	canRevealSecrets: boolean;
	canManageDomains: boolean;
	canManageBackups: boolean;
	canAccessRuntime: boolean;
	canControlRuntime: boolean;
	canViewMonitoring: boolean;
	canViewActivity: boolean;
}

export type PermissionKey = keyof UserPermissions;

export interface ProjectScope {
	projectId: string;
	serviceIds: string[];
}

export interface User {
	id: string;
	email: string;
	name: string;
	role: string;
	permissions?: UserPermissions;
	createdAt: string;
}

export interface SessionSummary {
	id: string;
	userId: string;
	userName?: string;
	userEmail?: string;
	userRole?: string;
	ipAddress: string;
	userAgent: string;
	device: string;
	isCurrent: boolean;
	lastSeenAt: string;
	expiresAt: string;
	createdAt: string;
}

export interface AuthStatus {
	initialized: boolean;
	authenticated: boolean;
	user?: User;
}

export interface AuthResponse {
	user: User;
	token?: string;
}

export interface SetupPayload {
	name: string;
	email: string;
	password: string;
}

export interface LoginPayload {
	email: string;
	password: string;
}
