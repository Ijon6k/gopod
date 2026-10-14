import type { Service, EnvVar } from '$lib/types';

export type DatabaseEngine =
	'postgres' | 'mysql' | 'mariadb' | 'redis' | 'mongodb' | 'clickhouse' | 'libsql' | 'custom';

export interface DatabaseEngineDefinition {
	id: DatabaseEngine;
	name: string;
	defaultImage: string;
	popularTags: string[];
	defaultPort: number;
	defaultUser: string;
	defaultDbName: string;
	defaultStoragePath: string;
	desc: string;
	hasUser: boolean;
	hasDbName: boolean;
	hasRootPassword?: boolean;
}

export const DATABASE_ENGINES: DatabaseEngineDefinition[] = [
	{
		id: 'postgres',
		name: 'PostgreSQL',
		defaultImage: 'docker.io/library/postgres:17-alpine',
		popularTags: [
			'postgres:17-alpine',
			'postgres:16-alpine',
			'postgres:15-alpine',
			'postgres:17',
			'postgres:16'
		],
		defaultPort: 5432,
		defaultUser: 'postgres',
		defaultDbName: 'gopod',
		defaultStoragePath: '/var/lib/postgresql/data',
		desc: 'Relational SQL database',
		hasUser: true,
		hasDbName: true
	},
	{
		id: 'mysql',
		name: 'MySQL',
		defaultImage: 'docker.io/library/mysql:8.4',
		popularTags: ['mysql:8.4', 'mysql:8.0', 'mysql:latest'],
		defaultPort: 3306,
		defaultUser: 'app',
		defaultDbName: 'gopod',
		defaultStoragePath: '/var/lib/mysql',
		desc: 'Relational database',
		hasUser: true,
		hasDbName: true,
		hasRootPassword: true
	},
	{
		id: 'mariadb',
		name: 'MariaDB',
		defaultImage: 'docker.io/library/mariadb:11.4',
		popularTags: ['mariadb:11.4', 'mariadb:10.11', 'mariadb:latest'],
		defaultPort: 3306,
		defaultUser: 'app',
		defaultDbName: 'gopod',
		defaultStoragePath: '/var/lib/mysql',
		desc: 'Open-source relational database and MySQL alternative',
		hasUser: true,
		hasDbName: true,
		hasRootPassword: true
	},
	{
		id: 'redis',
		name: 'Redis',
		defaultImage: 'docker.io/library/redis:7-alpine',
		popularTags: ['redis:7-alpine', 'redis:6-alpine', 'redis:7.4'],
		defaultPort: 6379,
		defaultUser: 'default',
		defaultDbName: '0',
		defaultStoragePath: '/data',
		desc: 'In-memory data structure store, cache, and message broker',
		hasUser: false,
		hasDbName: false
	},
	{
		id: 'mongodb',
		name: 'MongoDB',
		defaultImage: 'docker.io/library/mongo:7.0',
		popularTags: ['mongo:7.0', 'mongo:6.0', 'mongo:latest'],
		defaultPort: 27017,
		defaultUser: 'admin',
		defaultDbName: 'gopod',
		defaultStoragePath: '/data/db',
		desc: 'Document-oriented distributed NoSQL database',
		hasUser: true,
		hasDbName: true
	},
	{
		id: 'clickhouse',
		name: 'ClickHouse',
		defaultImage: 'clickhouse/clickhouse-server:latest',
		popularTags: [
			'clickhouse/clickhouse-server:latest',
			'clickhouse/clickhouse-server:24.3',
			'clickhouse/clickhouse-server:23.8'
		],
		defaultPort: 8123,
		defaultUser: 'default',
		defaultDbName: 'default',
		defaultStoragePath: '/var/lib/clickhouse',
		desc: 'Fast open-source column-oriented analytical database',
		hasUser: true,
		hasDbName: true
	},
	{
		id: 'libsql',
		name: 'libSQL (Turso/SQLite)',
		defaultImage: 'ghcr.io/tursodatabase/libsql:latest',
		popularTags: ['ghcr.io/tursodatabase/libsql:latest'],
		defaultPort: 8080,
		defaultUser: 'admin',
		defaultDbName: 'main',
		defaultStoragePath: '/var/lib/sqld',
		desc: 'Open-contribution fork of SQLite with server capabilities',
		hasUser: true,
		hasDbName: true
	},
	{
		id: 'custom',
		name: 'Custom Database',
		defaultImage: '',
		popularTags: [],
		defaultPort: 5432,
		defaultUser: 'admin',
		defaultDbName: 'gopod',
		defaultStoragePath: '/data',
		desc: 'Any custom database image or container from registry',
		hasUser: true,
		hasDbName: true
	}
];

export interface DatabaseConfig {
	engine: DatabaseEngine;
	name: string;
	description: string;
	image: string;
	databaseName: string;
	user: string;
	password: string;
	rootPassword?: string;
	internalPort: number;
	hostPort?: number;
	volumeName: string;
	volumeMountPath: string;
	envVars: EnvVar[];
	command?: string;
}

/**
 * Generates an unguessable, database-safe random password.
 * Avoids quotes ('"), dollar signs ($), backticks (`), and backslashes (\) that break shell execution or URI encoding.
 */
export function generateDatabasePassword(length = 20): string {
	const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#%_-+';
	if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
		const values = new Uint8Array(length);
		crypto.getRandomValues(values);
		let res = '';
		for (let i = 0; i < length; i++) {
			res += chars[values[i] % chars.length];
		}
		return res;
	}
	let res = '';
	for (let i = 0; i < length; i++) {
		res += chars.charAt(Math.floor(Math.random() * chars.length));
	}
	return res;
}

/**
 * Validates database password to ensure compatibility with shell environments, Docker, and connection strings.
 */
export function validateDatabasePassword(password: string): { valid: boolean; error?: string } {
	if (!password || password.trim().length === 0) {
		return { valid: false, error: 'Password is required' };
	}
	if (/\s/.test(password)) {
		return { valid: false, error: 'Password must not contain spaces' };
	}
	if (/['"`\\]/.test(password)) {
		return { valid: false, error: 'Password must not contain quotes (\' ") or backslashes (\\)' };
	}
	if (password.includes('$')) {
		return {
			valid: false,
			error: 'Password must not contain dollar signs ($) to prevent shell interpolation'
		};
	}
	if (password.length < 6) {
		return { valid: false, error: 'Password should be at least 6 characters' };
	}
	return { valid: true };
}

/**
 * Detects database engine from service image name or environment variable keys.
 */
export function detectDatabaseEngine(service: Service): DatabaseEngine {
	const img = (service.image || '').toLowerCase();
	if (img.includes('postgres') || img.includes('pgsql') || img.includes('psql')) return 'postgres';
	if (img.includes('mariadb')) return 'mariadb';
	if (img.includes('mysql')) return 'mysql';
	if (img.includes('redis') || img.includes('valkey')) return 'redis';
	if (img.includes('mongo')) return 'mongodb';
	if (img.includes('clickhouse')) return 'clickhouse';
	if (img.includes('libsql') || img.includes('sqld')) return 'libsql';

	for (const env of service.envVars || []) {
		if (env.key.startsWith('POSTGRES_')) return 'postgres';
		if (env.key.startsWith('MARIADB_')) return 'mariadb';
		if (env.key.startsWith('MYSQL_')) return 'mysql';
		if (env.key.startsWith('REDIS_')) return 'redis';
		if (env.key.startsWith('MONGO_')) return 'mongodb';
		if (env.key.startsWith('CLICKHOUSE_')) return 'clickhouse';
	}

	return 'custom';
}

/**
 * Builds environment variables array tailored to the chosen database engine.
 */
export function buildDatabaseEnvVars(config: DatabaseConfig): EnvVar[] {
	const vars: EnvVar[] = [];

	switch (config.engine) {
		case 'postgres':
			vars.push(
				{ key: 'POSTGRES_DB', value: config.databaseName || 'gopod', secret: false },
				{ key: 'POSTGRES_USER', value: config.user || 'postgres', secret: false },
				{ key: 'POSTGRES_PASSWORD', value: config.password, secret: true },
				{
					key: 'PGDATA',
					value: `${config.volumeMountPath || '/var/lib/postgresql/data'}/pgdata`,
					secret: false
				}
			);
			break;
		case 'mysql':
			vars.push(
				{ key: 'MYSQL_DATABASE', value: config.databaseName || 'gopod', secret: false },
				{ key: 'MYSQL_ROOT_PASSWORD', value: config.rootPassword || config.password, secret: true }
			);
			if (config.user && config.user !== 'root') {
				vars.push(
					{ key: 'MYSQL_USER', value: config.user, secret: false },
					{ key: 'MYSQL_PASSWORD', value: config.password, secret: true }
				);
			}
			break;
		case 'mariadb':
			vars.push(
				{ key: 'MARIADB_DATABASE', value: config.databaseName || 'gopod', secret: false },
				{
					key: 'MARIADB_ROOT_PASSWORD',
					value: config.rootPassword || config.password,
					secret: true
				}
			);
			if (config.user && config.user !== 'root') {
				vars.push(
					{ key: 'MARIADB_USER', value: config.user, secret: false },
					{ key: 'MARIADB_PASSWORD', value: config.password, secret: true }
				);
			}
			break;
		case 'redis':
			vars.push({ key: 'REDIS_PASSWORD', value: config.password, secret: true });
			break;
		case 'mongodb':
			vars.push(
				{ key: 'MONGO_INITDB_ROOT_USERNAME', value: config.user || 'admin', secret: false },
				{ key: 'MONGO_INITDB_ROOT_PASSWORD', value: config.password, secret: true },
				{ key: 'MONGO_INITDB_DATABASE', value: config.databaseName || 'gopod', secret: false }
			);
			break;
		case 'clickhouse':
			vars.push(
				{ key: 'CLICKHOUSE_DB', value: config.databaseName || 'default', secret: false },
				{ key: 'CLICKHOUSE_USER', value: config.user || 'default', secret: false },
				{ key: 'CLICKHOUSE_PASSWORD', value: config.password, secret: true },
				{ key: 'CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT', value: '1', secret: false }
			);
			break;
		case 'libsql':
			vars.push({ key: 'SQLD_NODE', value: 'primary', secret: false });
			if (config.password) {
				vars.push({ key: 'SQLD_AUTH_JWT_KEY', value: config.password, secret: true });
			}
			break;
		case 'custom':
			if (config.databaseName) {
				vars.push({ key: 'DB_NAME', value: config.databaseName, secret: false });
			}
			if (config.user) {
				vars.push({ key: 'DB_USER', value: config.user, secret: false });
			}
			if (config.password) {
				vars.push({ key: 'DB_PASSWORD', value: config.password, secret: true });
			}
			break;
	}

	return vars;
}

/**
 * Extracts database credentials, connection parameters, and volume configuration from an existing Service.
 */
export function extractDatabaseCredentials(service: Service) {
	const engine = detectDatabaseEngine(service);
	const def = DATABASE_ENGINES.find((e) => e.id === engine) ?? DATABASE_ENGINES[0];

	let password = '';
	let rootPassword = '';
	let user = '';
	let databaseName = '';

	const envs = service.envVars || [];

	for (const env of envs) {
		const k = env.key;
		const v = env.value;

		if (
			k === 'POSTGRES_PASSWORD' ||
			k === 'REDIS_PASSWORD' ||
			k === 'MYSQL_PASSWORD' ||
			k === 'MARIADB_PASSWORD' ||
			k === 'MONGO_INITDB_ROOT_PASSWORD' ||
			k === 'CLICKHOUSE_PASSWORD' ||
			k === 'DB_PASSWORD'
		) {
			password = v;
		} else if (k === 'MYSQL_ROOT_PASSWORD' || k === 'MARIADB_ROOT_PASSWORD') {
			rootPassword = v;
		} else if (
			k === 'POSTGRES_USER' ||
			k === 'MYSQL_USER' ||
			k === 'MARIADB_USER' ||
			k === 'MONGO_INITDB_ROOT_USERNAME' ||
			k === 'CLICKHOUSE_USER' ||
			k === 'DB_USER'
		) {
			user = v;
		} else if (
			k === 'POSTGRES_DB' ||
			k === 'MYSQL_DATABASE' ||
			k === 'MARIADB_DATABASE' ||
			k === 'MONGO_INITDB_DATABASE' ||
			k === 'CLICKHOUSE_DB' ||
			k === 'DB_NAME'
		) {
			databaseName = v;
		}
	}

	// Fallback to root password if user password wasn't separately set
	if (!password && rootPassword) {
		password = rootPassword;
	}
	if (!rootPassword && (engine === 'mysql' || engine === 'mariadb')) {
		rootPassword = password;
	}

	// Engine-specific defaults if not in envVars
	if (!user) {
		user = def.defaultUser;
	}
	if (!databaseName) {
		databaseName = def.defaultDbName;
	}

	const port = service.port || def.defaultPort;
	const hostPort = service.hostPort;
	const image = service.image || def.defaultImage;

	const volumeInfo = service.advanced?.storage?.volumes?.[0];
	const volumeName = volumeInfo?.source || `${service.id || service.name}-data`;
	const volumeMountPath = volumeInfo?.target || def.defaultStoragePath;

	return {
		engine,
		user,
		databaseName,
		password,
		rootPassword,
		port,
		hostPort,
		image,
		volumeName,
		volumeMountPath
	};
}

/**
 * Builds standard connection URI string for any database engine.
 */
export function buildConnectionString(params: {
	engine: DatabaseEngine;
	user?: string;
	password?: string;
	host: string;
	port: number;
	databaseName?: string;
}): string {
	const { engine, host, port } = params;
	const user = encodeURIComponent(params.user || '');
	const pass = params.password || '';
	const db = encodeURIComponent(params.databaseName || '');

	switch (engine) {
		case 'postgres':
			return pass
				? `postgresql://${user}:${pass}@${host}:${port}/${db}`
				: `postgresql://${user}@${host}:${port}/${db}`;
		case 'mysql':
		case 'mariadb':
			return pass
				? `mysql://${user}:${pass}@${host}:${port}/${db}`
				: `mysql://${user}@${host}:${port}/${db}`;
		case 'redis':
			return pass ? `redis://:${pass}@${host}:${port}` : `redis://${host}:${port}`;
		case 'mongodb':
			return pass
				? `mongodb://${user}:${pass}@${host}:${port}/${db}?authSource=admin`
				: `mongodb://${user}@${host}:${port}/${db}`;
		case 'clickhouse':
			return pass
				? `clickhouse://${user}:${pass}@${host}:${port}/${db}`
				: `clickhouse://${user}@${host}:${port}/${db}`;
		case 'libsql':
			return `libsql://${host}:${port}`;
		default:
			return pass
				? `db://${user}:${pass}@${host}:${port}/${db}`
				: `db://${user}@${host}:${port}/${db}`;
	}
}

/**
 * Generates ready-to-run Podman CLI command for interactive container shell.
 */
export function buildCliCommand(
	service: Service,
	creds: {
		engine: DatabaseEngine;
		user: string;
		password: string;
		databaseName: string;
	}
): string {
	const cName = service.id || service.name;
	switch (creds.engine) {
		case 'postgres':
			return `podman exec -it ${cName} psql -U ${creds.user} -d ${creds.databaseName}`;
		case 'redis':
			return creds.password
				? `podman exec -it ${cName} redis-cli -a ${creds.password}`
				: `podman exec -it ${cName} redis-cli`;
		case 'mysql':
		case 'mariadb':
			return `podman exec -it ${cName} mysql -u ${creds.user} -p${creds.password} ${creds.databaseName}`;
		case 'mongodb':
			return `podman exec -it ${cName} mongosh -u ${creds.user} -p ${creds.password}`;
		case 'clickhouse':
			return `podman exec -it ${cName} clickhouse-client -u ${creds.user} --password ${creds.password} -d ${creds.databaseName}`;
		default:
			return `podman exec -it ${cName} sh`;
	}
}

/**
 * Pure function that updates a Service with new DatabaseConfig parameters,
 * preserving any custom env vars and accurately mutating database credentials.
 */
export function updateServiceWithDatabaseConfig(
	service: Service,
	newConfig: DatabaseConfig
): Service {
	const newEnvVars = buildDatabaseEnvVars(newConfig);

	// Merge with existing env vars: replace managed keys, preserve custom additions
	const managedKeys = new Set(newEnvVars.map((e) => e.key));
	// Also remove older variant keys for other engines if engine switched
	const allPossibleManagedKeys = new Set([
		'POSTGRES_DB',
		'POSTGRES_USER',
		'POSTGRES_PASSWORD',
		'PGDATA',
		'MYSQL_DATABASE',
		'MYSQL_ROOT_PASSWORD',
		'MYSQL_USER',
		'MYSQL_PASSWORD',
		'MARIADB_DATABASE',
		'MARIADB_ROOT_PASSWORD',
		'MARIADB_USER',
		'MARIADB_PASSWORD',
		'REDIS_PASSWORD',
		'MONGO_INITDB_ROOT_USERNAME',
		'MONGO_INITDB_ROOT_PASSWORD',
		'MONGO_INITDB_DATABASE',
		'CLICKHOUSE_DB',
		'CLICKHOUSE_USER',
		'CLICKHOUSE_PASSWORD',
		'CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT',
		'SQLD_NODE',
		'SQLD_AUTH_JWT_KEY',
		'DB_NAME',
		'DB_USER',
		'DB_PASSWORD'
	]);

	const existingCustom = (service.envVars || []).filter(
		(env) => !allPossibleManagedKeys.has(env.key)
	);

	const mergedEnvVars: EnvVar[] = [...newEnvVars, ...existingCustom];

	// Command update for Redis
	let command = service.command;
	if (newConfig.engine === 'redis') {
		command = `redis-server --requirepass ${newConfig.password}`;
	} else if (newConfig.command !== undefined) {
		command = newConfig.command;
	}

	// Update volume mount
	const safeSlug = (newConfig.name || service.name)
		.toLowerCase()
		.trim()
		.replace(/[^a-z0-9_-]/g, '-')
		.replace(/-+/g, '-');

	const volumeName = newConfig.volumeName || `${safeSlug}-data`;
	const mountPath = newConfig.volumeMountPath;

	const updatedAdvanced = {
		...(service.advanced || {
			runtime: { mode: 'rootless' as const, userNamespace: '', devices: [] },
			lifecycle: {
				quadletEnabled: false,
				systemdUnitName: `${safeSlug}.service`,
				restartPolicy: 'always'
			},
			security: {
				privileged: false,
				selinuxLabel: 'container_file_t',
				capAdd: [],
				capDrop: [],
				noNewPrivileges: true
			},
			storage: { volumes: [] },
			resources: { cpuLimit: '0', memoryLimit: '0', pidsLimit: 2048, swapLimit: '0' }
		}),
		storage: {
			volumes: [{ source: volumeName, target: mountPath, options: 'Z' }]
		}
	};

	return {
		...service,
		name: newConfig.name || service.name,
		description: newConfig.description || service.description,
		image: newConfig.image || service.image,
		port: newConfig.internalPort,
		hostPort: newConfig.hostPort,
		command,
		envVars: mergedEnvVars,
		advanced: updatedAdvanced,
		updatedAt: new Date().toISOString()
	};
}
