import { z } from 'zod';

// ── SSH Deploy Key Validation Schema ──
export const sshKeySchema = z.object({
	name: z
		.string()
		.min(2, 'Key name must be at least 2 characters')
		.max(64, 'Key name cannot exceed 64 characters')
		.regex(
			/^[a-zA-Z0-9_-]+$/,
			'Key name can only contain letters, numbers, underscores, and hyphens'
		),
	publicKey: z.string().min(1, 'Public key is required'),
	privateKey: z.string().optional(),
	type: z.enum(['ed25519', 'rsa']).default('ed25519')
});

export type SSHKeyInput = z.infer<typeof sshKeySchema>;

// ── Container Registry Validation Schema ──
export const registrySchema = z.object({
	name: z
		.string()
		.min(2, 'Registry label must be at least 2 characters')
		.max(64, 'Registry label cannot exceed 64 characters'),
	url: z.string().min(1, 'Registry URL or domain is required'),
	username: z.string().min(1, 'Username is required'),
	token: z.string().min(1, 'Access token / password is required')
});

export type RegistryInput = z.infer<typeof registrySchema>;

// ── Service Workload Schema ──
export const serviceSchema = z.object({
	name: z
		.string()
		.min(2, 'Service name must be at least 2 characters')
		.max(64, 'Service name cannot exceed 64 characters')
		.regex(
			/^[a-zA-Z0-9_-]+$/,
			'Service name must only contain alphanumeric characters and hyphens'
		),
	projectId: z.string().min(1, 'Project ID is required'),
	image: z.string().min(1, 'Container image is required'),
	port: z
		.number()
		.int('Port must be an integer')
		.min(1, 'Port must be between 1 and 65535')
		.max(65535, 'Port must be between 1 and 65535'),
	hostPort: z.number().int().min(0, 'Host port must be 0 or higher').max(65535).optional(),
	runtimeTarget: z.enum(['container', 'pod', 'compose', 'quadlet']).default('container')
});

export type ServiceInput = z.infer<typeof serviceSchema>;

// ── Project Validation Schema ──
export const projectSchema = z.object({
	name: z
		.string()
		.min(2, 'Project name must be at least 2 characters')
		.max(64, 'Project name cannot exceed 64 characters')
		.regex(
			/^[a-zA-Z0-9_-]+$/,
			'Project name must only contain alphanumeric characters and hyphens'
		),
	description: z.string().max(256, 'Description cannot exceed 256 characters').optional()
});

export type ProjectInput = z.infer<typeof projectSchema>;

// ── Ingress Domain Schema ──
export const domainSchema = z.object({
	hostname: z
		.string()
		.min(3, 'Hostname must be at least 3 characters')
		.regex(/^[a-zA-Z0-9.-]+$/, 'Invalid hostname format'),
	serviceId: z.string().min(1, 'Service ID is required'),
	containerPort: z.number().int().min(1).max(65535),
	tls: z.boolean().default(true),
	cors: z.boolean().default(false),
	hsts: z.boolean().default(true)
});

export type DomainInput = z.infer<typeof domainSchema>;
