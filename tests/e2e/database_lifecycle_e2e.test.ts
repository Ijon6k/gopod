import { describe, test, expect, beforeAll } from "bun:test";
import { Database } from "bun:sqlite";
import {
	DATABASE_ENGINES,
	type DatabaseConfig,
	type DatabaseEngine,
	generateDatabasePassword,
	validateDatabasePassword,
	detectDatabaseEngine,
	buildDatabaseEnvVars,
	extractDatabaseCredentials,
	buildConnectionString,
	buildCliCommand,
	updateServiceWithDatabaseConfig
} from "../../frontend/src/lib/components/features/services/database/database-utils";
import type { Service } from "../../frontend/src/lib/api/services/types";

const BASE_URL = process.env.API_URL || "http://127.0.0.1:8000";
const DB_PATH = process.env.GOPOD_DB_PATH || "./data/gopod.db";

let authToken = "";
let apiAvailable = false;

async function api(path: string, options: RequestInit = {}) {
	const headers: Record<string, string> = {};
	if (!(options.body instanceof FormData)) {
		headers["Content-Type"] = "application/json";
	}
	if (options.headers) {
		Object.assign(headers, options.headers);
	}
	if (authToken) {
		headers["Authorization"] = `Bearer ${authToken}`;
	}
	const res = await fetch(`${BASE_URL}${path}`, {
		...options,
		headers,
	});
	const text = await res.text();
	let json: any = null;
	try {
		json = JSON.parse(text);
	} catch {
		json = text;
	}
	return { status: res.status, ok: res.ok, data: json };
}

beforeAll(async () => {
	try {
		const statusRes = await api("/api/auth/status");
		if (statusRes.ok) {
			apiAvailable = true;
			if (!statusRes.data.initialized) {
				const setupRes = await api("/api/auth/setup", {
					method: "POST",
					body: JSON.stringify({
						name: "Administrator",
						email: "admin@gopod.local",
						password: "Administrator123!",
					}),
				});
				if (setupRes.ok && setupRes.data.token) {
					authToken = setupRes.data.token;
				}
			}
		}
	} catch {
		apiAvailable = false;
	}

	if (apiAvailable && !authToken) {
		try {
			const db = new Database(DB_PATH);
			const sessionRow = db
				.query("SELECT token FROM sessions ORDER BY created_at DESC LIMIT 1")
				.get() as { token: string } | null;
			if (sessionRow?.token) {
				authToken = sessionRow.token;
			}
			db.close();
		} catch {
			// Database file not accessible or table absent
		}
	}
});

// ─────────────────────────────────────────────────────────────
// 1. DATABASE UTILITY & ENGINE PRESETS TESTS
// ─────────────────────────────────────────────────────────────

describe("Database Engine Presets & Metadata Audit", () => {
	test("provides all required standard and modern database presets", () => {
		const engineIds = DATABASE_ENGINES.map((e) => e.id);
		expect(engineIds).toContain("postgres");
		expect(engineIds).toContain("mysql");
		expect(engineIds).toContain("mariadb");
		expect(engineIds).toContain("redis");
		expect(engineIds).toContain("mongodb");
		expect(engineIds).toContain("clickhouse");
		expect(engineIds).toContain("libsql");
		expect(engineIds).toContain("custom");
	});

	test("each database preset has valid ports, non-empty default users, and mount targets", () => {
		for (const eng of DATABASE_ENGINES) {
			if (eng.id !== "custom") {
				expect(eng.defaultPort).toBeGreaterThan(0);
				expect(eng.defaultPort).toBeLessThan(65536);
				expect(eng.defaultStoragePath).toStartWith("/");
				expect(eng.popularTags.length).toBeGreaterThan(0);
			}
		}
	});

	test("PostgreSQL uses standard 5432 port, postgres user, and /var/lib/postgresql/data", () => {
		const pg = DATABASE_ENGINES.find((e) => e.id === "postgres")!;
		expect(pg.defaultPort).toBe(5432);
		expect(pg.defaultUser).toBe("postgres");
		expect(pg.defaultStoragePath).toBe("/var/lib/postgresql/data");
		expect(pg.hasUser).toBeTrue();
		expect(pg.hasDbName).toBeTrue();
	});

	test("Redis uses 6379, /data storage path, and does not require username/database name", () => {
		const redis = DATABASE_ENGINES.find((e) => e.id === "redis")!;
		expect(redis.defaultPort).toBe(6379);
		expect(redis.defaultStoragePath).toBe("/data");
		expect(redis.hasUser).toBeFalse();
		expect(redis.hasDbName).toBeFalse();
	});

	test("MySQL & MariaDB declare root password support", () => {
		const mysql = DATABASE_ENGINES.find((e) => e.id === "mysql")!;
		const mariadb = DATABASE_ENGINES.find((e) => e.id === "mariadb")!;
		expect(mysql.hasRootPassword).toBeTrue();
		expect(mariadb.hasRootPassword).toBeTrue();
		expect(mysql.defaultPort).toBe(3306);
		expect(mariadb.defaultPort).toBe(3306);
	});
});

// ─────────────────────────────────────────────────────────────
// 2. PASSWORD GENERATION & SECURITY VALIDATION
// ─────────────────────────────────────────────────────────────

describe("Database Password Generator & Security Validation", () => {
	test("generates high-entropy passwords with required length", () => {
		for (let i = 0; i < 50; i++) {
			const pwd = generateDatabasePassword(24);
			expect(pwd.length).toBe(24);
			// Ensures password does NOT contain characters that break CLI escaping or URLs
			expect(pwd).not.toMatch(/[\s'"`\\$]/);
		}
	});

	test("rejects invalid passwords that contain spaces, quotes, or dangerous shell characters", () => {
		expect(validateDatabasePassword("").valid).toBeFalse();
		expect(validateDatabasePassword("   ").valid).toBeFalse();
		expect(validateDatabasePassword("my password with spaces").valid).toBeFalse();
		expect(validateDatabasePassword("pass'quote").valid).toBeFalse();
		expect(validateDatabasePassword('pass"doublequote').valid).toBeFalse();
		expect(validateDatabasePassword("pass`backtick`").valid).toBeFalse();
		expect(validateDatabasePassword("pass\\backslash").valid).toBeFalse();
		expect(validateDatabasePassword("pass$shellvar").valid).toBeFalse();
		expect(validateDatabasePassword("12345").valid).toBeFalse(); // too short
	});

	test("accepts valid secure passwords with safe special characters", () => {
		const validResult = validateDatabasePassword("SuperSecure_Pass-2026!#%+");
		expect(validResult.valid).toBeTrue();
		expect(validResult.error).toBeUndefined();
	});
});

// ─────────────────────────────────────────────────────────────
// 3. ENGINE DETECTION TESTS
// ─────────────────────────────────────────────────────────────

describe("Database Engine Auto-Detection", () => {
	test("accurately detects engine from Docker image name", () => {
		const mockSvc = (image: string): Service => ({
			id: "test-svc",
			projectId: "test-proj",
			name: "db",
			type: "database",
			status: "stopped",
			image,
			port: 5432,
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: [],
			createdAt: new Date().toISOString()
		});

		expect(detectDatabaseEngine(mockSvc("postgres:17-alpine"))).toBe("postgres");
		expect(detectDatabaseEngine(mockSvc("docker.io/library/postgres:16"))).toBe("postgres");
		expect(detectDatabaseEngine(mockSvc("mysql:8.4"))).toBe("mysql");
		expect(detectDatabaseEngine(mockSvc("mariadb:11.4"))).toBe("mariadb");
		expect(detectDatabaseEngine(mockSvc("redis:7-alpine"))).toBe("redis");
		expect(detectDatabaseEngine(mockSvc("mongo:7.0"))).toBe("mongodb");
		expect(detectDatabaseEngine(mockSvc("clickhouse/clickhouse-server:latest"))).toBe("clickhouse");
		expect(detectDatabaseEngine(mockSvc("ghcr.io/tursodatabase/libsql:latest"))).toBe("libsql");
		expect(detectDatabaseEngine(mockSvc("custom-registry.com/my-image:1.0"))).toBe("custom");
	});

	test("accurately falls back to environment variable keys when image is non-standard", () => {
		const mockSvcWithEnv = (envKey: string): Service => ({
			id: "test-svc",
			projectId: "test-proj",
			name: "db",
			type: "database",
			status: "stopped",
			image: "my-custom-container:latest",
			port: 5432,
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: [{ key: envKey, value: "someval", secret: false }],
			createdAt: new Date().toISOString()
		});

		expect(detectDatabaseEngine(mockSvcWithEnv("POSTGRES_DB"))).toBe("postgres");
		expect(detectDatabaseEngine(mockSvcWithEnv("MYSQL_DATABASE"))).toBe("mysql");
		expect(detectDatabaseEngine(mockSvcWithEnv("MARIADB_DATABASE"))).toBe("mariadb");
		expect(detectDatabaseEngine(mockSvcWithEnv("REDIS_PASSWORD"))).toBe("redis");
		expect(detectDatabaseEngine(mockSvcWithEnv("MONGO_INITDB_DATABASE"))).toBe("mongodb");
		expect(detectDatabaseEngine(mockSvcWithEnv("CLICKHOUSE_DB"))).toBe("clickhouse");
	});
});

// ─────────────────────────────────────────────────────────────
// 4. ENVIRONMENT VARIABLE BUILDER & ROUND-TRIP CREDENTIALS
// ─────────────────────────────────────────────────────────────

describe("Database EnvVars Generation & Credentials Extraction", () => {
	test("PostgreSQL generates POSTGRES_DB, USER, PASSWORD, and PGDATA", () => {
		const config: DatabaseConfig = {
			engine: "postgres",
			name: "pg-svc",
			description: "postgres test",
			image: "postgres:17-alpine",
			databaseName: "customer_db",
			user: "app_user",
			password: "SecretPostgresPass1!",
			internalPort: 5432,
			volumeName: "pg-svc-data",
			volumeMountPath: "/var/lib/postgresql/data",
			envVars: []
		};

		const envVars = buildDatabaseEnvVars(config);
		const envMap = Object.fromEntries(envVars.map((e) => [e.key, e.value]));

		expect(envMap["POSTGRES_DB"]).toBe("customer_db");
		expect(envMap["POSTGRES_USER"]).toBe("app_user");
		expect(envMap["POSTGRES_PASSWORD"]).toBe("SecretPostgresPass1!");
		expect(envMap["PGDATA"]).toBe("/var/lib/postgresql/data/pgdata");

		// Secret flag check
		const passEnv = envVars.find((e) => e.key === "POSTGRES_PASSWORD");
		expect(passEnv?.secret).toBeTrue();
	});

	test("MySQL generates MYSQL_DATABASE, ROOT_PASSWORD, USER, and PASSWORD", () => {
		const config: DatabaseConfig = {
			engine: "mysql",
			name: "mysql-svc",
			description: "mysql test",
			image: "mysql:8.4",
			databaseName: "store_db",
			user: "store_admin",
			password: "StoreUserPass123!",
			rootPassword: "RootAdminPass456!",
			internalPort: 3306,
			volumeName: "mysql-data",
			volumeMountPath: "/var/lib/mysql",
			envVars: []
		};

		const envVars = buildDatabaseEnvVars(config);
		const envMap = Object.fromEntries(envVars.map((e) => [e.key, e.value]));

		expect(envMap["MYSQL_DATABASE"]).toBe("store_db");
		expect(envMap["MYSQL_ROOT_PASSWORD"]).toBe("RootAdminPass456!");
		expect(envMap["MYSQL_USER"]).toBe("store_admin");
		expect(envMap["MYSQL_PASSWORD"]).toBe("StoreUserPass123!");
	});

	test("Redis generates REDIS_PASSWORD without creating unused DB name/user vars", () => {
		const config: DatabaseConfig = {
			engine: "redis",
			name: "redis-cache",
			description: "redis test",
			image: "redis:7-alpine",
			databaseName: "",
			user: "",
			password: "RedisSecretAuth!",
			internalPort: 6379,
			volumeName: "redis-data",
			volumeMountPath: "/data",
			envVars: []
		};

		const envVars = buildDatabaseEnvVars(config);
		expect(envVars.length).toBe(1);
		expect(envVars[0].key).toBe("REDIS_PASSWORD");
		expect(envVars[0].value).toBe("RedisSecretAuth!");
		expect(envVars[0].secret).toBeTrue();
	});

	test("extractDatabaseCredentials performs flawless round-trip extraction", () => {
		const initialConfig: DatabaseConfig = {
			engine: "postgres",
			name: "orders-db",
			description: "Orders Database",
			image: "postgres:17-alpine",
			databaseName: "orders",
			user: "order_svc",
			password: "OrderSecret123!",
			internalPort: 5432,
			hostPort: 15432,
			volumeName: "orders-data",
			volumeMountPath: "/var/lib/postgresql/data",
			envVars: []
		};

		const svc: Service = {
			id: "orders-db-1",
			projectId: "shop",
			name: "orders-db",
			type: "database",
			status: "stopped",
			image: initialConfig.image,
			port: initialConfig.internalPort,
			hostPort: initialConfig.hostPort,
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: buildDatabaseEnvVars(initialConfig),
			advanced: {
				runtime: { mode: "rootless", userNamespace: "", devices: [] },
				lifecycle: { quadletEnabled: false, systemdUnitName: "orders-db.service", restartPolicy: "always" },
				security: { privileged: false, selinuxLabel: "container_file_t", capAdd: [], capDrop: [], noNewPrivileges: true },
				storage: { volumes: [{ source: "orders-data", target: "/var/lib/postgresql/data", options: "Z" }] },
				resources: { cpuLimit: "0", memoryLimit: "0", pidsLimit: 2048, swapLimit: "0" }
			},
			createdAt: new Date().toISOString()
		};

		const extracted = extractDatabaseCredentials(svc);
		expect(extracted.engine).toBe("postgres");
		expect(extracted.databaseName).toBe("orders");
		expect(extracted.user).toBe("order_svc");
		expect(extracted.password).toBe("OrderSecret123!");
		expect(extracted.port).toBe(5432);
		expect(extracted.hostPort).toBe(15432);
		expect(extracted.volumeName).toBe("orders-data");
		expect(extracted.volumeMountPath).toBe("/var/lib/postgresql/data");
	});
});

// ─────────────────────────────────────────────────────────────
// 5. SERVICE MUTATION & CREDENTIALS EDITING
// ─────────────────────────────────────────────────────────────

describe("Database Service Credentials Mutation (User Freedom & No Hardcoding)", () => {
	test("updates database user, password, db name, image version, and custom port", () => {
		const baseService: Service = {
			id: "main-db-abc",
			projectId: "finance",
			name: "main-db",
			description: "Initial database",
			type: "database",
			status: "running",
			image: "docker.io/library/postgres:15-alpine",
			port: 5432,
			hostPort: undefined,
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: [
				{ key: "POSTGRES_DB", value: "old_db", secret: false },
				{ key: "POSTGRES_USER", value: "old_user", secret: false },
				{ key: "POSTGRES_PASSWORD", value: "OldPass!", secret: true },
				{ key: "CUSTOM_UNTOUCHED_VAR", value: "important_config", secret: false }
			],
			advanced: {
				runtime: { mode: "rootless", userNamespace: "", devices: [] },
				lifecycle: { quadletEnabled: false, systemdUnitName: "main-db.service", restartPolicy: "always" },
				security: { privileged: false, selinuxLabel: "container_file_t", capAdd: [], capDrop: [], noNewPrivileges: true },
				storage: { volumes: [{ source: "main-db-data", target: "/var/lib/postgresql/data", options: "Z" }] },
				resources: { cpuLimit: "0", memoryLimit: "0", pidsLimit: 2048, swapLimit: "0" }
			},
			createdAt: new Date().toISOString()
		};

		const updatedConfig: DatabaseConfig = {
			engine: "postgres",
			name: "main-db-renamed",
			description: "Upgraded PostgreSQL 17",
			image: "docker.io/library/postgres:17-alpine",
			databaseName: "new_enterprise_db",
			user: "new_admin_user",
			password: "BrandNewSecurePassword123!",
			internalPort: 5433,
			hostPort: 25433,
			volumeName: "main-db-custom-vol",
			volumeMountPath: "/var/lib/postgresql/data",
			envVars: []
		};

		const updatedService = updateServiceWithDatabaseConfig(baseService, updatedConfig);

		// Verify fields updated
		expect(updatedService.name).toBe("main-db-renamed");
		expect(updatedService.description).toBe("Upgraded PostgreSQL 17");
		expect(updatedService.image).toBe("docker.io/library/postgres:17-alpine");
		expect(updatedService.port).toBe(5433);
		expect(updatedService.hostPort).toBe(25433);

		// Verify volume updated
		expect(updatedService.advanced?.storage.volumes[0].source).toBe("main-db-custom-vol");

		// Verify environment variables updated
		const envMap = Object.fromEntries((updatedService.envVars || []).map((e) => [e.key, e.value]));
		expect(envMap["POSTGRES_DB"]).toBe("new_enterprise_db");
		expect(envMap["POSTGRES_USER"]).toBe("new_admin_user");
		expect(envMap["POSTGRES_PASSWORD"]).toBe("BrandNewSecurePassword123!");

		// Verify custom env vars are preserved!
		expect(envMap["CUSTOM_UNTOUCHED_VAR"]).toBe("important_config");
	});

	test("switching engine to Redis removes obsolete PostgreSQL env vars and adds redis command", () => {
		const existingService: Service = {
			id: "cache-svc-1",
			projectId: "app",
			name: "cache",
			type: "database",
			status: "stopped",
			image: "postgres:17-alpine",
			port: 5432,
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: [
				{ key: "POSTGRES_DB", value: "test", secret: false },
				{ key: "POSTGRES_USER", value: "test", secret: false },
				{ key: "POSTGRES_PASSWORD", value: "pass", secret: true }
			],
			createdAt: new Date().toISOString()
		};

		const redisConfig: DatabaseConfig = {
			engine: "redis",
			name: "redis-cache",
			description: "Redis in-memory store",
			image: "docker.io/library/redis:7-alpine",
			databaseName: "",
			user: "",
			password: "RedisNewPass123!",
			internalPort: 6379,
			volumeName: "redis-data",
			volumeMountPath: "/data",
			envVars: []
		};

		const mutated = updateServiceWithDatabaseConfig(existingService, redisConfig);

		expect(mutated.port).toBe(6379);
		expect(mutated.image).toBe("docker.io/library/redis:7-alpine");
		expect(mutated.command).toBe("redis-server --requirepass RedisNewPass123!");

		const envMap = Object.fromEntries((mutated.envVars || []).map((e) => [e.key, e.value]));
		expect(envMap["POSTGRES_DB"]).toBeUndefined();
		expect(envMap["POSTGRES_USER"]).toBeUndefined();
		expect(envMap["REDIS_PASSWORD"]).toBe("RedisNewPass123!");
	});
});

// ─────────────────────────────────────────────────────────────
// 6. CONNECTION URI & CLI ACCESS STRING TESTS
// ─────────────────────────────────────────────────────────────

describe("Connection URI & CLI Terminal Command Formatting", () => {
	test("formats PostgreSQL connection string with URI encoding for special characters", () => {
		const uri = buildConnectionString({
			engine: "postgres",
			user: "app@admin",
			password: "mypassword",
			host: "pg-cluster",
			port: 5432,
			databaseName: "my db"
		});
		expect(uri).toBe("postgresql://app%40admin:mypassword@pg-cluster:5432/my%20db");
	});

	test("formats MySQL / MariaDB connection string", () => {
		const uri = buildConnectionString({
			engine: "mysql",
			user: "root",
			password: "secret",
			host: "mysql-db",
			port: 3306,
			databaseName: "gopod"
		});
		expect(uri).toBe("mysql://root:secret@mysql-db:3306/gopod");
	});

	test("formats Redis connection string", () => {
		const uri = buildConnectionString({
			engine: "redis",
			password: "redispassword",
			host: "cache",
			port: 6379
		});
		expect(uri).toBe("redis://:redispassword@cache:6379");
	});

	test("formats MongoDB connection string with authSource=admin", () => {
		const uri = buildConnectionString({
			engine: "mongodb",
			user: "admin",
			password: "mongopassword",
			host: "mongo-instance",
			port: 27017,
			databaseName: "products"
		});
		expect(uri).toBe("mongodb://admin:mongopassword@mongo-instance:27017/products?authSource=admin");
	});

	test("builds correct Podman CLI commands for container interaction", () => {
		const mockSvc: Service = {
			id: "postgres-svc-789",
			projectId: "test",
			name: "postgres-svc",
			type: "database",
			status: "running",
			cpu: 0,
			memory: 0,
			replicas: 1,
			envVars: [],
			createdAt: new Date().toISOString()
		};

		const pgCmd = buildCliCommand(mockSvc, {
			engine: "postgres",
			user: "postgres",
			password: "pass",
			databaseName: "gopod"
		});
		expect(pgCmd).toBe("podman exec -it postgres-svc-789 psql -U postgres -d gopod");

		const redisCmd = buildCliCommand(mockSvc, {
			engine: "redis",
			user: "default",
			password: "secretpass",
			databaseName: "0"
		});
		expect(redisCmd).toBe("podman exec -it postgres-svc-789 redis-cli -a secretpass");
	});
});

// ─────────────────────────────────────────────────────────────
// 7. END-TO-END API WORKFLOW (IF BACKEND IS RUNNING)
// ─────────────────────────────────────────────────────────────

describe("End-to-End API Database Service Lifecycle", () => {
	test("API Workflow: create project -> create database -> update credentials -> delete", async () => {
		if (!apiAvailable || !authToken) {
			console.log("ℹ️ Skipping live HTTP API workflow test (Go backend dev server not currently listening on port 8000)");
			return;
		}

		// 1. Create temporary test project
		const projId = `e2e-db-proj-${Date.now()}`;
		const projRes = await api("/api/projects", {
			method: "POST",
			body: JSON.stringify({
				id: projId,
				name: "E2E Database Project",
				description: "Testing database service life cycle"
			})
		});
		expect(projRes.ok).toBeTrue();

		// 2. Create custom database service via API
		const svcId = `${projId}-pg-test`;
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				id: svcId,
				projectId: projId,
				name: "Custom Postgres Test",
				type: "database",
				image: "docker.io/library/postgres:16-alpine",
				port: 5432,
				hostPort: 18432,
				envVars: [
					{ key: "POSTGRES_DB", value: "initial_db", secret: false },
					{ key: "POSTGRES_USER", value: "initial_user", secret: false },
					{ key: "POSTGRES_PASSWORD", value: "InitialPass123!", secret: true }
				],
				description: "E2E database test service"
			})
		});
		expect(createRes.ok).toBeTrue();

		// 3. Fetch created database service
		const getRes = await api(`/api/services/${svcId}`);
		expect(getRes.ok).toBeTrue();
		expect(getRes.data.name).toBe("Custom Postgres Test");
		expect(getRes.data.port).toBe(5432);
		expect(getRes.data.hostPort).toBe(18432);

		// 4. Update credentials via API (Dokploy edit workflow)
		const updateRes = await api(`/api/services/${svcId}`, {
			method: "PUT",
			body: JSON.stringify({
				...getRes.data,
				name: "Updated Postgres Test",
				image: "docker.io/library/postgres:17-alpine",
				port: 5433,
				hostPort: 28432,
				envVars: [
					{ key: "POSTGRES_DB", value: "updated_db", secret: false },
					{ key: "POSTGRES_USER", value: "updated_user", secret: false },
					{ key: "POSTGRES_PASSWORD", value: "UpdatedPass456!", secret: true }
				]
			})
		});
		expect(updateRes.ok).toBeTrue();

		// 5. Verify updated values persisted
		const getUpdatedRes = await api(`/api/services/${svcId}`);
		expect(getUpdatedRes.ok).toBeTrue();
		expect(getUpdatedRes.data.name).toBe("Updated Postgres Test");
		expect(getUpdatedRes.data.image).toBe("docker.io/library/postgres:17-alpine");
		expect(getUpdatedRes.data.port).toBe(5433);
		expect(getUpdatedRes.data.hostPort).toBe(28432);

		// 6. Clean up service and project
		const delSvcRes = await api(`/api/services/${svcId}`, { method: "DELETE" });
		expect(delSvcRes.ok).toBeTrue();

		const delProjRes = await api(`/api/projects/${projId}`, { method: "DELETE" });
		expect(delProjRes.ok).toBeTrue();
	});

	test("Deferred Engine Selection: create blank database service -> configure PostgreSQL inside -> switch to Redis via API", async () => {
		if (!apiAvailable || !authToken) {
			console.log("ℹ️ Skipping live HTTP API workflow test (Go backend dev server not currently listening on port 8000)");
			return;
		}

		const projId = `e2e-deferred-db-${Date.now()}`;
		const projRes = await api("/api/projects", {
			method: "POST",
			body: JSON.stringify({
				id: projId,
				name: "Deferred DB Project",
				description: "Testing deferred database selection inside service"
			})
		});
		expect(projRes.ok).toBeTrue();

		// Step 1: User creates service with just type 'database' and empty image
		const svcId = `${projId}-blank-db`;
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				id: svcId,
				projectId: projId,
				name: "Production DB",
				type: "database",
				image: "", // Unconfigured
				port: 0,
				envVars: [],
				description: "Dedicated database instance"
			})
		});
		expect(createRes.ok).toBeTrue();

		// Step 2: Fetch service inside service detail view — verify it is blank/unconfigured
		const blankRes = await api(`/api/services/${svcId}`);
		expect(blankRes.ok).toBeTrue();
		expect(blankRes.data.image).toBe("");
		expect(blankRes.data.port).toBe(0);

		// Step 3: User chooses PostgreSQL from catalog inside service detail view
		const pgConfig: DatabaseConfig = {
			engine: "postgres",
			name: "Production DB",
			description: "PostgreSQL Database",
			image: "docker.io/library/postgres:17-alpine",
			databaseName: "prod_db",
			user: "app_user",
			password: "UltraSecretPassword99!",
			internalPort: 5432,
			hostPort: 15432,
			volumeName: `${svcId}-data`,
			volumeMountPath: "/var/lib/postgresql/data",
			envVars: []
		};
		const updatedPg = updateServiceWithDatabaseConfig(blankRes.data, pgConfig);
		const savePgRes = await api(`/api/services/${svcId}`, {
			method: "PUT",
			body: JSON.stringify(updatedPg)
		});
		expect(savePgRes.ok).toBeTrue();

		const getPgRes = await api(`/api/services/${svcId}`);
		expect(getPgRes.ok).toBeTrue();
		expect(getPgRes.data.image).toBe("docker.io/library/postgres:17-alpine");
		expect(getPgRes.data.port).toBe(5432);
		expect(getPgRes.data.hostPort).toBe(15432);

		const extractedPg = extractDatabaseCredentials(getPgRes.data);
		expect(extractedPg.engine).toBe("postgres");
		expect(extractedPg.user).toBe("app_user");
		expect(extractedPg.databaseName).toBe("prod_db");
		expect(extractedPg.password).toBe("UltraSecretPassword99!");

		// Step 4: User switches engine inside service from PostgreSQL to Redis
		const redisConfig: DatabaseConfig = {
			engine: "redis",
			name: "Production DB",
			description: "Redis In-Memory Cache",
			image: "docker.io/library/redis:7-alpine",
			databaseName: "0",
			user: "default",
			password: "RedisSecretPassword88!",
			internalPort: 6379,
			hostPort: 16379,
			volumeName: `${svcId}-redis-data`,
			volumeMountPath: "/data",
			envVars: []
		};
		const updatedRedis = updateServiceWithDatabaseConfig(getPgRes.data, redisConfig);
		const saveRedisRes = await api(`/api/services/${svcId}`, {
			method: "PUT",
			body: JSON.stringify(updatedRedis)
		});
		expect(saveRedisRes.ok).toBeTrue();

		const getRedisRes = await api(`/api/services/${svcId}`);
		expect(getRedisRes.ok).toBeTrue();
		expect(getRedisRes.data.image).toBe("docker.io/library/redis:7-alpine");
		expect(getRedisRes.data.port).toBe(6379);
		expect(getRedisRes.data.hostPort).toBe(16379);

		const extractedRedis = extractDatabaseCredentials(getRedisRes.data);
		expect(extractedRedis.engine).toBe("redis");
		expect(extractedRedis.password).toBe("RedisSecretPassword88!");
		// Verify PostgreSQL vars were wiped out
		const pgVar = getRedisRes.data.envVars.find((e: any) => e.key === "POSTGRES_PASSWORD");
		expect(pgVar).toBeUndefined();

		// Cleanup
		await api(`/api/services/${svcId}`, { method: "DELETE" });
		await api(`/api/projects/${projId}`, { method: "DELETE" });
	});
});
