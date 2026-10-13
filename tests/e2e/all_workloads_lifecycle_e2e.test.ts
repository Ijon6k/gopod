import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { Database } from "bun:sqlite";
import { execSync } from "node:child_process";
import path from "node:path";
import fs from "node:fs";

declare const Bun: any;

function findDbPath(): string {
	if (process.env.GOPOD_DB_PATH && fs.existsSync(process.env.GOPOD_DB_PATH)) {
		return process.env.GOPOD_DB_PATH;
	}
	const candidates = [
		path.resolve(process.cwd(), "data/gopod.db"),
		path.resolve(__dirname, "../../data/gopod.db"),
		path.resolve(__dirname, "../../../data/gopod.db"),
		"./data/gopod.db"
	];
	for (const c of candidates) {
		if (fs.existsSync(c)) return c;
	}
	return path.resolve(__dirname, "../../data/gopod.db");
}

const BASE_URL = process.env.API_URL || "http://127.0.0.1:8000";
const DB_PATH = findDbPath();

let authToken = "";
let testProjectId = "";
let apiAvailable = false;

function run(cmd: string): string {
	try {
		return execSync(cmd, {
			encoding: "utf-8",
			timeout: 25000,
			env: process.env,
			shell: "/bin/bash",
		}).trim();
	} catch (e: any) {
		return e.stdout?.toString() || e.stderr?.toString() || e.message || "";
	}
}

async function api(apiPath: string, options: RequestInit = {}) {
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
	const res = await fetch(`${BASE_URL}${apiPath}`, {
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

async function pollDeployment(depId: string, maxAttempts = 60) {
	for (let i = 0; i < maxAttempts; i++) {
		const res = await api(`/api/deployments/${depId}`);
		if (res.ok && res.data) {
			const status = res.data.status;
			if (status === "success" || status === "failed" || status === "cancelled") {
				return res.data;
			}
		}
		await new Promise((r) => setTimeout(r, 1000));
	}
	throw new Error(`Deployment ${depId} timed out waiting for terminal state`);
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

	if (!authToken) {
		try {
			const db = new Database(DB_PATH);
			const sessionRow = db
				.query("SELECT token FROM sessions ORDER BY created_at DESC LIMIT 1")
				.get() as { token: string } | null;

			if (sessionRow?.token) {
				authToken = sessionRow.token;
			} else {
				const userRow = db
					.query("SELECT id FROM users LIMIT 1")
					.get() as { id: string } | null;
				if (userRow?.id) {
					const token = "e2e_workloads_test_" + Math.random().toString(36).substring(2);
					db.run(
						"INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, datetime('now', '+30 days'), datetime('now'))",
						[token, userRow.id]
					);
					authToken = token;
				}
			}
			db.close();
		} catch {
			// Database file not accessible or table absent
		}
	}

	if (apiAvailable && authToken) {
		const pRes = await api("/api/projects", {
			method: "POST",
			body: JSON.stringify({
				name: `E2E All Workloads ${Date.now()}`,
				description: "Automated test for Kubernetes Play, Pod, Application, Compose and Ingress lifecycles",
			}),
		});
		if (pRes.ok) {
			testProjectId = pRes.data.id;
		}
	}
}, 60000);

afterAll(async () => {
	if (testProjectId) {
		const listRes = await api(`/api/services?projectId=${testProjectId}`);
		if (listRes.ok && Array.isArray(listRes.data)) {
			await Promise.all(
				listRes.data.map((svc) =>
					api(`/api/services/${svc.id}?delete_volumes=true`, { method: "DELETE" })
				)
			);
		}
		await api(`/api/projects/${testProjectId}`, { method: "DELETE" });
	}
}, 180000);

// ─────────────────────────────────────────────────────────────
// 1. KUBERNETES PLAY (podman play kube) LIFECYCLE
// ─────────────────────────────────────────────────────────────

describe("Kubernetes Manifest Workload Lifecycle (podman play kube)", () => {
	let kubeSvcId = "";

	test("Scenario K1: Create and deploy Kubernetes YAML workload via podman play kube", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const k8sManifest = `apiVersion: v1
kind: Pod
metadata:
  name: e2e-kube-pod
spec:
  containers:
    - name: web
      image: docker.io/library/nginx:alpine
      ports:
        - containerPort: 80
`;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "kube-nginx-app",
				type: "kubernetes",
				k8sYaml: k8sManifest,
				port: 80,
				hostPort: 8190,
				description: "Kubernetes Play native manifest service",
			}),
		});
		expect(createRes.status).toBe(201);
		kubeSvcId = createRes.data.id;
		expect(kubeSvcId).toBeDefined();

		// Trigger deployment
		const depRes = await api(`/api/services/${kubeSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);

		const completed = await pollDeployment(depRes.data.id, 90);
		expect(completed.status).toBe("success");

		// Verify Podman created the pod and container
		const podList = run("podman pod ps --format '{{.Name}}'");
		expect(podList).toContain("e2e-kube-pod");
	}, 120000);

	test("Scenario K2: Lifecycle operations (Stop, Start, Restart) on Kubernetes workload", async () => {
		if (!apiAvailable || !authToken || !kubeSvcId) return;

		// 1. Stop service
		const stopRes = await api(`/api/services/${kubeSvcId}/stop`, { method: "POST" });
		expect(stopRes.status).toBe(200);

		// 2. Start service
		const startRes = await api(`/api/services/${kubeSvcId}/start`, { method: "POST" });
		expect(startRes.status).toBe(200);

		// 3. Restart service
		const restartRes = await api(`/api/services/${kubeSvcId}/restart`, { method: "POST" });
		expect(restartRes.status).toBe(200);
	}, 60000);

	test("Scenario K3: Update Kubernetes YAML manifest and verify redeploy", async () => {
		if (!apiAvailable || !authToken || !kubeSvcId) return;

		const updatedManifest = `apiVersion: v1
kind: Pod
metadata:
  name: e2e-kube-pod
spec:
  containers:
    - name: web
      image: docker.io/library/alpine:latest
      command: ["sleep", "infinity"]
`;

		const updateRes = await api(`/api/services/${kubeSvcId}`, {
			method: "PUT",
			body: JSON.stringify({
				id: kubeSvcId,
				projectId: testProjectId,
				name: "kube-nginx-app",
				type: "kubernetes",
				k8sYaml: updatedManifest,
				port: 80,
			}),
		});
		expect(updateRes.ok).toBeTrue();

		// Redeploy with updated manifest
		const redeployRes = await api(`/api/services/${kubeSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(redeployRes.status).toBe(202);
		const completed = await pollDeployment(redeployRes.data.id, 90);
		expect(completed.status).toBe("success");
	}, 120000);

	test("Scenario K4: Clean teardown of Kubernetes service", async () => {
		if (!apiAvailable || !authToken || !kubeSvcId) return;

		const delRes = await api(`/api/services/${kubeSvcId}?delete_volumes=true`, { method: "DELETE" });
		expect(delRes.ok).toBeTrue();
		kubeSvcId = "";
	}, 60000);
});

// ─────────────────────────────────────────────────────────────
// 2. PODMAN POD MULTI-CONTAINER SHARED NETWORK LIFECYCLE
// ─────────────────────────────────────────────────────────────

describe("First-Class Podman Pod Multi-Container Lifecycle", () => {
	let podSvcId = "";

	test("Scenario P1: Create dedicated Podman Pod service and deploy infra container", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-cluster-pod",
				type: "pod",
				port: 8092,
				hostPort: 8092,
				description: "Shared localhost multi-container pod",
			}),
		});
		expect(createRes.status).toBe(201);
		podSvcId = createRes.data.id;
		expect(podSvcId).toBeDefined();

		// Deploy pod
		const depRes = await api(`/api/services/${podSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const completed = await pollDeployment(depRes.data.id, 60);
		expect(completed.status).toBe("success");

		// Verify pod exists in Podman (named pod_e2e-cluster-pod)
		const pods = run("podman pod ps --format '{{.Name}}'");
		expect(pods).toContain("pod_e2e-cluster-pod");
	}, 60000);

	test("Scenario P2: Group standalone container into the active Podman Pod", async () => {
		if (!apiAvailable || !authToken || !podSvcId) return;

		// 1. Create a container service to join the pod
		const sidecarRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "sidecar-worker",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/alpine:latest",
				command: "sleep infinity",
				restartPolicy: "always",
			}),
		});
		expect(sidecarRes.status).toBe(201);
		const sidecarId = sidecarRes.data.id;

		// 2. Join the service into the pod via group-pod API
		const groupRes = await api(`/api/services/${sidecarId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({ podName: "pod_e2e-cluster-pod", createPod: false }),
		});
		expect(groupRes.status).toBe(200);

		// Verify sidecar state in DB
		const sidecarGet = await api(`/api/services/${sidecarId}`);
		expect(sidecarGet.data.inPod).toBeTrue();
		expect(sidecarGet.data.podId).toBe("pod_e2e-cluster-pod");

		// Clean up sidecar
		await api(`/api/services/${sidecarId}?delete_volumes=true`, { method: "DELETE" });
	}, 120000);

	test("Scenario P3: Teardown Podman Pod and verify clean state", async () => {
		if (!apiAvailable || !authToken || !podSvcId) return;

		const delRes = await api(`/api/services/${podSvcId}?delete_volumes=true`, { method: "DELETE" });
		expect(delRes.ok).toBeTrue();
		podSvcId = "";
	}, 60000);
});

// ─────────────────────────────────────────────────────────────
// 3. APPLICATION & WEBHOOK CI/CD LIFECYCLE
// ─────────────────────────────────────────────────────────────

describe("Application Workload & CI/CD Webhook Trigger Lifecycle", () => {
	let appSvcId = "";
	let webhookToken = "";
	const appHostPort = 18100 + Math.floor(Math.random() * 500);

	test("Scenario A1: Create application service with secrets and retrieve webhook token", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "api-backend-app",
				type: "application",
				image: "docker.io/library/nginx:alpine",
				port: 80,
				hostPort: appHostPort,
				envVars: [
					{ key: "APP_ENV", value: "production", secret: false },
					{ key: "API_SECRET", value: "SecretKey987!", secret: true },
				],
				description: "Production web backend",
			}),
		});
		expect(createRes.status).toBe(201);
		appSvcId = createRes.data.id;
		webhookToken = createRes.data.webhookToken;
		expect(appSvcId).toBeDefined();
		expect(webhookToken).toBeDefined();
		expect(webhookToken.startsWith("wh_")).toBeTrue();
	});

	test("Scenario A2: Manual deployment and logs retrieval", async () => {
		if (!apiAvailable || !authToken || !appSvcId) return;

		const depRes = await api(`/api/services/${appSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const dep = await pollDeployment(depRes.data.id, 90);
		expect(dep.status).toBe("success");

		const logsRes = await api(`/api/deployments/${dep.id}/logs`);
		expect(logsRes.ok).toBeTrue();
		expect(logsRes.data.logs).toBeDefined();
	}, 120000);

	test("Scenario A3: Trigger redeployment via public CI/CD Webhook token", async () => {
		if (!apiAvailable || !webhookToken || !appSvcId) return;

		// Public webhook endpoint accepts payload and queues deployment
		const hookRes = await fetch(`${BASE_URL}/api/deploy/webhook/${webhookToken}`, {
			method: "POST",
		});
		expect(hookRes.status).toBe(202);
		const hookData = await hookRes.json();
		expect(hookData.message).toContain("Deployment triggered successfully");

		// Give it a brief moment and verify a new deployment record was appended
		await new Promise((r) => setTimeout(r, 2000));
		const list = await api(`/api/services/${appSvcId}/deployments`);
		expect(list.ok).toBeTrue();
		expect(list.data.length).toBeGreaterThanOrEqual(2);
	}, 60000);

	test("Scenario A4: Tampered webhook token is rejected with 404/401", async () => {
		if (!apiAvailable) return;

		const invalidHookRes = await fetch(`${BASE_URL}/api/deploy/webhook/invalid_tampered_token_999`, {
			method: "POST",
		});
		expect(invalidHookRes.status).toBeGreaterThanOrEqual(400);
	});

	test("Scenario A5: Create and bind Ingress Domain rule to application", async () => {
		if (!apiAvailable || !authToken || !appSvcId) return;

		const domainName = `test-app-${Date.now()}.local`;
		const domRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: appSvcId,
				hostname: domainName,
				containerPort: 80,
				tls: false,
			}),
		});
		expect(domRes.status).toBe(201);
		const domainId = domRes.data.id;
		expect(domainId).toBeDefined();

		// Verify list domains includes new domain
		const listDom = await api(`/api/domains?projectId=${testProjectId}`);
		expect(listDom.ok).toBeTrue();
		const match = listDom.data.find((d: any) => d.id === domainId);
		expect(match).toBeDefined();

		// Cleanup domain
		const delDom = await api(`/api/domains/${domainId}`, { method: "DELETE" });
		expect(delDom.ok).toBeTrue();
	});

	test("Scenario A6: Clean teardown of Application workload", async () => {
		if (!apiAvailable || !authToken || !appSvcId) return;

		const delRes = await api(`/api/services/${appSvcId}?delete_volumes=true`, { method: "DELETE" });
		expect(delRes.ok).toBeTrue();
		appSvcId = "";
	}, 60000);
});

// ─────────────────────────────────────────────────────────────
// 4. MULTI-CONTAINER COMPOSE WORKLOAD LIFECYCLE
// ─────────────────────────────────────────────────────────────

describe("Multi-Container Compose Stack Lifecycle", () => {
	let compSvcId = "";

	test("Scenario C1: Create and deploy Compose multi-service stack", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const composeYaml = `version: "3.8"
services:
  frontend:
    image: docker.io/library/alpine:latest
    command: sleep infinity
    restart: always
  backend:
    image: docker.io/library/alpine:latest
    command: sleep infinity
    restart: always
`;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "microservices-compose",
				type: "compose",
				composeYaml,
				description: "Multi-container compose microservices",
			}),
		});
		expect(createRes.status).toBe(201);
		compSvcId = createRes.data.id;
		expect(compSvcId).toBeDefined();

		// Deploy compose stack
		const depRes = await api(`/api/services/${compSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);

		const completed = await pollDeployment(depRes.data.id, 90);
		expect(completed.status).toBe("success");
	}, 120000);

	test("Scenario C2: Compose stack Stop and Start operations", async () => {
		if (!apiAvailable || !authToken || !compSvcId) return;

		const stopRes = await api(`/api/services/${compSvcId}/stop`, { method: "POST" });
		expect(stopRes.status).toBe(200);

		const startRes = await api(`/api/services/${compSvcId}/start`, { method: "POST" });
		expect(startRes.status).toBe(200);
	}, 60000);

	test("Scenario C3: Teardown Compose stack", async () => {
		if (!apiAvailable || !authToken || !compSvcId) return;

		const delRes = await api(`/api/services/${compSvcId}?delete_volumes=true`, { method: "DELETE" });
		expect(delRes.ok).toBeTrue();
		compSvcId = "";
	}, 60000);
});

// ─────────────────────────────────────────────────────────────
// 5. SYSTEM RESILIENCE & ERROR HANDLING SAFEGUARDS
// ─────────────────────────────────────────────────────────────

describe("System Resilience, State Reconcile & Error Safeguards", () => {
	test("Non-existent service operations return 404/error without crashing", async () => {
		if (!apiAvailable || !authToken) return;

		const nonExistent = await api("/api/services/non_existent_svc_12345");
		expect(nonExistent.status).toBeGreaterThanOrEqual(400);

		const stopNonExistent = await api("/api/services/non_existent_svc_12345/stop", { method: "POST" });
		expect(stopNonExistent.status).toBeGreaterThanOrEqual(400);
	});

	test("Validation rejects invalid service payloads (empty name)", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const badRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "", // Invalid empty name
				type: "application",
			}),
		});
		expect(badRes.status).toBe(400);
	});
});

// ─────────────────────────────────────────────────────────────
// 6. CENTRALIZED CREDENTIALS LIFECYCLE (SSH, REGISTRY, SECRETS)
// ─────────────────────────────────────────────────────────────

describe("Centralized Credentials Management Lifecycle", () => {
	let createdKeyId = "";
	let createdRegistryId = "";
	let createdSecretId = "";

	test("Scenario CR1: Generate ED25519 SSH Keypair and verify fingerprint", async () => {
		if (!apiAvailable || !authToken) return;

		const genRes = await api("/api/credentials/ssh-keys/generate", {
			method: "POST",
			body: JSON.stringify({ name: "e2e-ci-deploy-key" }),
		});
		expect(genRes.status).toBe(200);
		expect(genRes.data.id).toBeDefined();
		expect(genRes.data.publicKey).toContain("ssh-ed25519");
		expect(genRes.data.privateKey).toContain("BEGIN OPENSSH PRIVATE KEY");
		expect(genRes.data.fingerprint).toContain("SHA256:");
		expect(genRes.data.type).toBe("ed25519");

		// Persist generated keypair to credential repository
		const saveRes = await api("/api/credentials/ssh-keys", {
			method: "POST",
			body: JSON.stringify({
				name: genRes.data.name,
				publicKey: genRes.data.publicKey,
				privateKey: genRes.data.privateKey,
			}),
		});
		expect(saveRes.status).toBe(201);
		expect(saveRes.data.id).toBeDefined();
		createdKeyId = saveRes.data.id;

		// List SSH keys and verify presence
		const listRes = await api("/api/credentials/ssh-keys");
		expect(listRes.ok).toBeTrue();
		const found = listRes.data.find((k: any) => k.id === createdKeyId);
		expect(found).toBeDefined();
	});

	test("Scenario CR2: Create container registry credential and list registries", async () => {
		if (!apiAvailable || !authToken) return;

		const regRes = await api("/api/credentials/registries", {
			method: "POST",
			body: JSON.stringify({
				name: "E2E GitHub Packages",
				url: "ghcr.io",
				username: "gopod-bot",
				password: "ghp_MockAccessTokenForE2E1234567890",
			}),
		});
		expect(regRes.status).toBe(201);
		expect(regRes.data.id).toBeDefined();
		createdRegistryId = regRes.data.id;
		expect(regRes.data.name).toBe("E2E GitHub Packages");
		expect(regRes.data.url).toBe("ghcr.io");

		// List registries
		const listRes = await api("/api/credentials/registries");
		expect(listRes.ok).toBeTrue();
		const found = listRes.data.find((r: any) => r.id === createdRegistryId);
		expect(found).toBeDefined();
	});

	test("Scenario CR3: Create centralized secret and list secrets", async () => {
		if (!apiAvailable || !authToken) return;

		const secRes = await api("/api/credentials/secrets", {
			method: "POST",
			body: JSON.stringify({
				name: "PAYMENT_GATEWAY_KEY",
				value: "sk_live_secret_token_e2e_verified_9988",
			}),
		});
		expect(secRes.status).toBe(201);
		expect(secRes.data.id).toBeDefined();
		createdSecretId = secRes.data.id;
		expect(secRes.data.name).toBe("PAYMENT_GATEWAY_KEY");

		// List secrets
		const listRes = await api("/api/credentials/secrets");
		expect(listRes.ok).toBeTrue();
		const found = listRes.data.find((s: any) => s.id === createdSecretId);
		expect(found).toBeDefined();
	});

	test("Scenario CR4: Clean teardown of credentials", async () => {
		if (!apiAvailable || !authToken) return;

		if (createdKeyId) {
			const delKey = await api(`/api/credentials/ssh-keys/${createdKeyId}`, { method: "DELETE" });
			expect(delKey.ok).toBeTrue();
		}
		if (createdRegistryId) {
			const delReg = await api(`/api/credentials/registries/${createdRegistryId}`, { method: "DELETE" });
			expect(delReg.ok).toBeTrue();
		}
		if (createdSecretId) {
			const delSec = await api(`/api/credentials/secrets/${createdSecretId}`, { method: "DELETE" });
			expect(delSec.ok).toBeTrue();
		}
	});
});

// ─────────────────────────────────────────────────────────────
// 7. VOLUME STORAGE SNAPSHOTS & BACKUP SCHEDULES LIFECYCLE
// ─────────────────────────────────────────────────────────────

describe("Volume Storage Snapshots & Automated Backup Schedules Lifecycle", () => {
	const testVolName = `e2e_snap_vol_${Date.now()}`;
	let createdSnapId = "";
	let createdSchedId = "";

	beforeAll(() => {
		// Create host podman volume for snapshot testing
		run(`podman volume create ${testVolName}`);
	});

	afterAll(() => {
		// Clean up host volume
		run(`podman volume rm -f ${testVolName}`);
	});

	test("Scenario ST1: Create on-demand volume snapshot and list snapshots", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const snapRes = await api("/api/storage/snapshots", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				volumeName: testVolName,
			}),
		});
		expect(snapRes.status).toBe(201);
		expect(snapRes.data.id).toBeDefined();
		createdSnapId = snapRes.data.id;
		expect(snapRes.data.volumeName).toBe(testVolName);
		expect(snapRes.data.compression).toBe("gzip");

		// List snapshots
		const listRes = await api(`/api/storage/snapshots?projectId=${testProjectId}`);
		expect(listRes.ok).toBeTrue();
		const found = listRes.data.find((s: any) => s.id === createdSnapId);
		expect(found).toBeDefined();
	}, 60000);

	test("Scenario ST2: Restore volume snapshot", async () => {
		if (!apiAvailable || !authToken || !createdSnapId) return;

		const restoreRes = await api(`/api/storage/snapshots/${createdSnapId}/restore`, {
			method: "POST",
		});
		expect(restoreRes.ok).toBeTrue();
		expect(restoreRes.data.message).toContain("restored successfully");
	}, 60000);

	test("Scenario ST3: Create and toggle automated backup schedule", async () => {
		if (!apiAvailable || !authToken || !testProjectId) return;

		const schedRes = await api("/api/storage/schedules", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				volumeName: testVolName,
				cron: "0 2 * * *",
				label: "Daily 2 AM Backup",
				retentionCount: 7,
				enabled: true,
			}),
		});
		expect(schedRes.status).toBe(201);
		expect(schedRes.data.id).toBeDefined();
		createdSchedId = schedRes.data.id;
		expect(schedRes.data.cron).toBe("0 2 * * *");
		expect(schedRes.data.enabled).toBeTrue();

		// Toggle schedule enabled state
		const toggleRes = await api(`/api/storage/schedules/${createdSchedId}/toggle`, {
			method: "POST",
		});
		expect(toggleRes.ok).toBeTrue();
		expect(toggleRes.data.enabled).toBeFalse();
	});

	test("Scenario ST4: Teardown snapshot and schedule", async () => {
		if (!apiAvailable || !authToken) return;

		if (createdSnapId) {
			const delSnap = await api(`/api/storage/snapshots/${createdSnapId}`, { method: "DELETE" });
			expect(delSnap.ok).toBeTrue();
		}
		if (createdSchedId) {
			const delSched = await api(`/api/storage/schedules/${createdSchedId}`, { method: "DELETE" });
			expect(delSched.ok).toBeTrue();
		}
	});
});

// ─────────────────────────────────────────────────────────────
// 8. HOST TELEMETRY & RUNTIME INVENTORY INTROSPECTION
// ─────────────────────────────────────────────────────────────

describe("Host Telemetry, cgroups v2 & Container Runtime Introspection", () => {
	test("Scenario RT1: Verify host telemetry and cgroups v2 detection", async () => {
		if (!apiAvailable || !authToken) return;

		const sysRes = await api("/api/system");
		expect(sysRes.ok).toBeTrue();
		const data = sysRes.data;

		expect(data.vcpu).toBeGreaterThan(0);
		expect(data.memoryTotal).toBeGreaterThan(0);
		expect(data.podmanVersion).toBeDefined();
		expect(data.rootless).toBeTrue();
		expect(data.cgroupVersion).toBe("v2");
	});

	test("Scenario RT2: Query live container inventories across domains", async () => {
		if (!apiAvailable || !authToken) return;

		const containersRes = await api("/api/containers");
		expect(containersRes.ok).toBeTrue();
		expect(Array.isArray(containersRes.data)).toBeTrue();

		const podsRes = await api("/api/pods");
		expect(podsRes.ok).toBeTrue();
		expect(Array.isArray(podsRes.data)).toBeTrue();

		const imagesRes = await api("/api/images");
		expect(imagesRes.ok).toBeTrue();
		expect(Array.isArray(imagesRes.data)).toBeTrue();

		const volumesRes = await api("/api/volumes");
		expect(volumesRes.ok).toBeTrue();
		expect(Array.isArray(volumesRes.data)).toBeTrue();

		const networksRes = await api("/api/networks");
		expect(networksRes.ok).toBeTrue();
		expect(Array.isArray(networksRes.data)).toBeTrue();
	});
});

// ─────────────────────────────────────────────────────────────
// 9. AUDIT TRAIL & SYSTEM COMPLIANCE VERIFICATION
// ─────────────────────────────────────────────────────────────

describe("Operator Audit Trail & Security Compliance Verification", () => {
	test("Scenario AU1: Verify audit log captures service and system events", async () => {
		if (!apiAvailable || !authToken) return;

		const auditRes = await api("/api/audit-log?limit=20");
		expect(auditRes.ok).toBeTrue();
		expect(Array.isArray(auditRes.data)).toBeTrue();
		expect(auditRes.data.length).toBeGreaterThan(0);

		const firstEntry = auditRes.data[0];
		expect(firstEntry.id).toBeDefined();
		expect(firstEntry.action).toBeDefined();
		expect(firstEntry.actor).toBeDefined();
		expect(firstEntry.category).toBeDefined();
		expect(firstEntry.status).toBeDefined();
	});
});

