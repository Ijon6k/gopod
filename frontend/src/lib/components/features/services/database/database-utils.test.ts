import { describe, it, expect, mock, beforeEach } from 'bun:test';
import type { Service } from '$lib/types';

type RevealMode = 'ok' | 'throw';

const revealCalls: Array<{ id: string; keys: string[] }> = [];
let revealMode: RevealMode = 'ok';

// The module under test reaches the API through `$lib/api`; replace it with a
// recording stub so the reveal flow can be asserted without a server.
mock.module('$lib/api', () => ({
	api: {
		services: {
			revealEnv: async (id: string, keys: string[]) => {
				revealCalls.push({ id, keys });
				if (revealMode === 'throw') throw new Error('403 Forbidden');
				return Object.fromEntries(keys.map((k) => [k, `revealed-${k}`]));
			}
		}
	}
}));

const {
	ENV_SECRET_MASK,
	revealMaskedEnvValues,
	extractDatabaseCredentials,
	updateServiceWithDatabaseConfig
} = await import('./database-utils');

function service(overrides: Partial<Service> = {}): Service {
	return {
		id: 'p1-demo-db-123456',
		projectId: 'p1',
		name: 'demo-db',
		type: 'image',
		status: 'running',
		image: 'postgres:17-alpine',
		port: 5432,
		envVars: [],
		createdAt: '2026-01-01T00:00:00Z',
		updatedAt: '2026-01-01T00:00:00Z',
		...overrides
	} as unknown as Service;
}

function config(overrides: Record<string, unknown> = {}) {
	return {
		engine: 'redis' as const,
		name: 'demo-db',
		description: '',
		image: 'redis:7-alpine',
		databaseName: '0',
		user: 'default',
		password: '',
		internalPort: 6379,
		volumeName: 'demo-db-data',
		volumeMountPath: '/data',
		envVars: [],
		...overrides
	};
}

beforeEach(() => {
	revealCalls.length = 0;
	revealMode = 'ok';
});

describe('environment secret mask', () => {
	it('matches the backend sentinel exactly', () => {
		expect(ENV_SECRET_MASK).toBe('••••••');
	});
});

describe('revealMaskedEnvValues', () => {
	it('asks the API only for masked secret keys', async () => {
		const svc = service({
			envVars: [
				{ key: 'REDIS_PASSWORD', value: ENV_SECRET_MASK, secret: true },
				{ key: 'PUBLIC_FLAG', value: ENV_SECRET_MASK, secret: false },
				{ key: 'PLAIN', value: 'visible', secret: false },
				{ key: '', value: ENV_SECRET_MASK, secret: true }
			]
		});

		const out = await revealMaskedEnvValues(svc);

		expect(revealCalls).toEqual([{ id: 'p1-demo-db-123456', keys: ['REDIS_PASSWORD'] }]);
		expect(out).toEqual({ REDIS_PASSWORD: 'revealed-REDIS_PASSWORD' });
	});

	it('does not call the API when nothing is masked', async () => {
		const out = await revealMaskedEnvValues(
			service({ envVars: [{ key: 'PLAIN', value: 'visible', secret: false }] })
		);
		expect(revealCalls).toEqual([]);
		expect(out).toEqual({});
	});

	it('falls back to "no overrides" when the reveal fails', async () => {
		revealMode = 'throw';
		const out = await revealMaskedEnvValues(
			service({ envVars: [{ key: 'REDIS_PASSWORD', value: ENV_SECRET_MASK, secret: true }] })
		);
		expect(out).toEqual({});
	});
});

describe('extractDatabaseCredentials', () => {
	it('returns the mask verbatim when no override was revealed', () => {
		const creds = extractDatabaseCredentials(
			service({
				envVars: [{ key: 'POSTGRES_PASSWORD', value: ENV_SECRET_MASK, secret: true }]
			})
		);
		expect(creds.password).toBe(ENV_SECRET_MASK);
	});

	it('prefers revealed overrides over the masked value', () => {
		const creds = extractDatabaseCredentials(
			service({
				envVars: [
					{ key: 'POSTGRES_PASSWORD', value: ENV_SECRET_MASK, secret: true },
					{ key: 'POSTGRES_USER', value: 'app', secret: false }
				]
			}),
			{ POSTGRES_PASSWORD: 'real-secret' }
		);
		expect(creds.password).toBe('real-secret');
		expect(creds.user).toBe('app');
	});
});

describe('updateServiceWithDatabaseConfig (redis command guard)', () => {
	it('never rebuilds the redis command from the mask sentinel', () => {
		const svc = service({
			image: 'redis:7-alpine',
			command: 'redis-server --requirepass stored-secret',
			envVars: [{ key: 'REDIS_PASSWORD', value: ENV_SECRET_MASK, secret: true }]
		});

		const updated = updateServiceWithDatabaseConfig(svc, config({ password: ENV_SECRET_MASK }));

		expect(updated.command).toBe('redis-server --requirepass stored-secret');
		expect(updated.envVars).toContainEqual({
			key: 'REDIS_PASSWORD',
			value: ENV_SECRET_MASK,
			secret: true
		});
	});

	it('rebuilds the redis command when a real password is supplied', () => {
		const svc = service({
			image: 'redis:7-alpine',
			command: 'redis-server --requirepass stored-secret',
			envVars: [{ key: 'REDIS_PASSWORD', value: ENV_SECRET_MASK, secret: true }]
		});

		const updated = updateServiceWithDatabaseConfig(
			svc,
			config({ password: 'rotated-secret' })
		);

		expect(updated.command).toBe('redis-server --requirepass rotated-secret');
		expect(updated.envVars).toContainEqual({
			key: 'REDIS_PASSWORD',
			value: 'rotated-secret',
			secret: true
		});
	});

	it('replaces managed keys but preserves custom env vars', () => {
		const svc = service({
			envVars: [
				{ key: 'POSTGRES_PASSWORD', value: ENV_SECRET_MASK, secret: true },
				{ key: 'CUSTOM_KEY', value: 'keep-me', secret: false }
			]
		});

		const updated = updateServiceWithDatabaseConfig(
			svc,
			config({
				engine: 'postgres',
				image: 'postgres:17-alpine',
				databaseName: 'shop',
				user: 'app',
				password: 'fresh-secret',
				internalPort: 5432,
				volumeMountPath: '/var/lib/postgresql/data'
			}) as never
		);

		expect(updated.envVars).toContainEqual({
			key: 'POSTGRES_PASSWORD',
			value: 'fresh-secret',
			secret: true
		});
		expect(updated.envVars).toContainEqual({ key: 'CUSTOM_KEY', value: 'keep-me', secret: false });
		expect(updated.envVars.filter((v) => v.key === 'REDIS_PASSWORD')).toEqual([]);
	});
});
