import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { Database } from "bun:sqlite";
import { execSync } from "node:child_process";
import path from "node:path";
import fs from "node:fs";

function findDbPath(): string {
	if (process.env.GOPOD_DB_PATH && fs.existsSync(process.env.GOPOD_DB_PATH)) {
		return process.env.GOPOD_DB_PATH;
	}
	const candidates = [
		path.resolve(process.cwd(), "data/gopod.db"),
		path.resolve(__dirname, "../../data/gopod.db"),
		path.resolve(__dirname, "../../../data/gopod.db"),
		"./data/gopod.db",
	];
	for (const c of candidates) {
		if (fs.existsSync(c)) return c;
	}
	return path.resolve(__dirname, "../../data/gopod.db");
}

const BASE_URL = process.env.API_URL || "http://127.0.0.1:8085";
const DB_PATH = findDbPath();

let authToken = "";
let testProjectId = "";

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

async function pollCaddyfile(predicate: (caddyText: string) => boolean, maxAttempts = 30): Promise<string> {
	for (let i = 0; i < maxAttempts; i++) {
		const res = await api("/api/ingress/caddyfile");
		if (res.ok && res.data?.caddyfile && predicate(res.data.caddyfile)) {
			return res.data.caddyfile;
		}
		await new Promise((r) => setTimeout(r, 400));
	}
	const fallback = await api("/api/ingress/caddyfile");
	return fallback.data?.caddyfile || "";
}

describe("GOPOD Podman Pods, Quadlet Lifecycle & Caddy Ingress E2E Suite", () => {
	let quadletSvcId = "";
	let podSvcId = "";
	let webSvcId = "";
	let cacheSvcId = "";
	let podDomainId = "";
	let memberDomainId = "";
	let secureDomId = "";
	let standaloneDomId = "";
	let clusterPodName = "";

	beforeAll(async () => {
		// 1. Establish auth token
		const statusRes = await api("/api/auth/status");
		if (statusRes.ok && !statusRes.data.initialized) {
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
						const token = "e2e_session_" + Math.random().toString(36).substring(2);
						db.run(
							"INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, datetime('now', '+30 days'), datetime('now'))",
							[token, userRow.id]
						);
						authToken = token;
					}
				}
				db.close();
			} catch (e) {
				// Fallback
			}
		}

		expect(authToken).not.toBeEmpty();

		// 2. Create isolated test project
		const projRes = await api("/api/projects", {
			method: "POST",
			body: JSON.stringify({
				name: `E2E Pod Ingress ${Date.now()}`,
				description: "Verification of Podman Pods, Quadlets, and Caddy Netavark Ingress",
			}),
		});
		expect(projRes.status).toBe(201);
		testProjectId = projRes.data.id;
		expect(testProjectId).toBeDefined();
	});

	afterAll(async () => {
		// Teardown domains
		if (podDomainId) await api(`/api/domains/${podDomainId}`, { method: "DELETE" });
		if (memberDomainId) await api(`/api/domains/${memberDomainId}`, { method: "DELETE" });
		if (secureDomId) await api(`/api/domains/${secureDomId}`, { method: "DELETE" });
		if (standaloneDomId) await api(`/api/domains/${standaloneDomId}`, { method: "DELETE" });

		// Teardown services
		if (webSvcId) await api(`/api/services/${webSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (cacheSvcId) await api(`/api/services/${cacheSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (podSvcId) await api(`/api/services/${podSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (quadletSvcId) await api(`/api/services/${quadletSvcId}?delete_volumes=true`, { method: "DELETE" });

		// Teardown test project
		if (testProjectId) {
			await api(`/api/projects/${testProjectId}`, { method: "DELETE" });
		}

		// Ensure Podman pod is stopped
		if (clusterPodName) run(`podman pod rm -f ${clusterPodName}`);
	});

	// ─────────────────────────────────────────────────────────────
	// PHASE 1: QUADLET STATUS SYNCHRONIZATION (BUG 1 FIX)
	// ─────────────────────────────────────────────────────────────
	test("Phase 1: Quadlet deployment transitions to ready/running and never locks in 'deploying'", async () => {
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-quadlet-sync",
				type: "quadlet",
				runtimeTarget: "quadlet",
				image: "docker.io/traefik/whoami:v1.10.1",
				port: 80,
				quadletConfig: `[Unit]\nDescription=E2E Quadlet Sync Test\nAfter=network-online.target\nStartLimitBurst=5\nStartLimitIntervalSec=60s\n\n[Container]\nContainerName=e2e-quadlet-sync\nImage=docker.io/traefik/whoami:v1.10.1\nNetwork=gopod-net\nLabel=io.gopod.project=${testProjectId}\n\n[Service]\nRestart=always\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target`,
			}),
		});
		expect(createRes.status).toBe(201);
		quadletSvcId = createRes.data.id;
		expect(quadletSvcId).toBeDefined();

		// Trigger Quadlet deployment
		const depRes = await api(`/api/services/${quadletSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const depRecord = await pollDeployment(depRes.data.id, 60);
		expect(depRecord.status).toBe("success");

		// Crucial Assertion: Service status must NOT be stuck in "deploying"
		const svcDetail = await api(`/api/services/${quadletSvcId}`);
		expect(svcDetail.ok).toBeTrue();
		expect(svcDetail.data.status).not.toBe("deploying");
		expect(["ready", "running", "stopped"]).toContain(svcDetail.data.status);

		// Batch status check: GET /api/services
		const batchRes = await api(`/api/services?projectId=${testProjectId}`);
		expect(batchRes.ok).toBeTrue();
		const foundSvc = batchRes.data.find((s: any) => s.id === quadletSvcId);
		expect(foundSvc).toBeDefined();
		expect(foundSvc.status).not.toBe("deploying");

		// Trigger redeployment: must succeed and not be blocked by stale status
		const redeployRes = await api(`/api/services/${quadletSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(redeployRes.status).toBe(202);
		const redepRecord = await pollDeployment(redeployRes.data.id, 60);
		expect(redepRecord.status).toBe("success");

		const afterRedeploy = await api(`/api/services/${quadletSvcId}`);
		expect(afterRedeploy.data.status).not.toBe("deploying");
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 2: PODMAN POD CREATION & MULTI-CONTAINER MEMBERSHIP
	// ─────────────────────────────────────────────────────────────
	test("Phase 2: Create Podman Pod service and group member containers into shared localhost", async () => {
		// 1. Create Pod Service
		const podRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-cluster-demo",
				type: "pod",
				port: 8096,
				hostPort: 8096,
				description: "E2E Podman Multi-Container Pod",
			}),
		});
		expect(podRes.status).toBe(201);
		podSvcId = podRes.data.id;
		expect(podSvcId).toBeDefined();

		// Deploy Pod
		const podDepRes = await api(`/api/services/${podSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(podDepRes.status).toBe(202);
		const podDepRecord = await pollDeployment(podDepRes.data.id, 60);
		expect(podDepRecord.status).toBe("success");

		// Verify Pod is running in Podman under its canonical unique name (pod_<service-id>)
		const podDetail = await api(`/api/services/${podSvcId}`);
		expect(podDetail.ok).toBeTrue();
		clusterPodName = podDetail.data.podId;
		expect(clusterPodName).toContain("pod_");
		const podInspect = run("podman pod ps --format '{{.Name}}'");
		expect(podInspect).toContain(clusterPodName);

		// 2. Create Web Member Service
		const webRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-web-app",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/traefik/whoami:v1.10.1",
				port: 80,
				restartPolicy: "always",
			}),
		});
		expect(webRes.status).toBe(201);
		webSvcId = webRes.data.id;

		// Group Web Member into Pod
		const groupWeb = await api(`/api/services/${webSvcId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({
				podName: clusterPodName,
				createPod: false,
			}),
		});
		expect(groupWeb.status).toBe(200);
		expect(groupWeb.data.inPod).toBeTrue();
		expect(groupWeb.data.podId).toBe(clusterPodName);

		// 3. Create Cache Member Service
		const cacheRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-cache-db",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/redis:alpine",
				port: 6379,
				restartPolicy: "always",
			}),
		});
		expect(cacheRes.status).toBe(201);
		cacheSvcId = cacheRes.data.id;

		// Group Cache Member into Pod
		const groupCache = await api(`/api/services/${cacheSvcId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({
				podName: clusterPodName,
				createPod: false,
			}),
		});
		expect(groupCache.status).toBe(200);
		expect(groupCache.data.inPod).toBeTrue();
		expect(groupCache.data.podId).toBe(clusterPodName);

		await new Promise((r) => setTimeout(r, 2000));

		// Verify both member containers are running inside pod_e2e-cluster-demo
		const psInPod = run(`podman ps --filter 'pod=${clusterPodName}' --format '{{.Names}}'`);
		expect(psInPod).toContain("e2e-web-app");
		expect(psInPod).toContain("e2e-cache-db");
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 3: CADDY INGRESS NETAVARK DNS RESOLUTION (BUG 3 & 4 FIX)
	// ─────────────────────────────────────────────────────────────
	test("Phase 3: Caddyfile routes domain traffic to Pod Netavark DNS name avoiding 502", async () => {
		// 1. Create Domain Route targeting the Pod service with targetContainer: e2e-web-app
		const domainRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: podSvcId,
				targetContainer: "e2e-web-app",
				hostname: "e2e-podmesh.localhost",
				containerPort: 80,
				tls: false,
			}),
		});
		expect(domainRes.status).toBe(201);
		podDomainId = domainRes.data.id;

		// 2. Fetch active Caddyfile configuration (poll until async reconcile completes)
		const caddyText = await pollCaddyfile((txt) => txt.includes("e2e-podmesh.localhost"));

		// Crucial Assertion: Caddy MUST dial pod_e2e-cluster-demo:80 (NOT e2e-web-app:80)
		expect(caddyText).toContain(`reverse_proxy ${clusterPodName}:80`);
		expect(caddyText).not.toContain("reverse_proxy e2e-web-app:80");

		// 3. Test HTTP request through Caddy reverse proxy on port 8080
		await new Promise((r) => setTimeout(r, 1000));
		let curlOut = "";
		for (let i = 0; i < 15; i++) {
			curlOut = run("curl -s -o /dev/null -w '%{http_code}' -H 'Host: e2e-podmesh.localhost' http://127.0.0.1:8080/");
			if (curlOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlOut).toBe("200");

		// 4. Create Domain Route directly on member service e2e-web-app
		const memberDomainRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: webSvcId,
				hostname: "e2e-member-direct.localhost",
				containerPort: 80,
				tls: false,
			}),
		});
		expect(memberDomainRes.status).toBe(201);
		memberDomainId = memberDomainRes.data.id;

		const caddyText2 = await pollCaddyfile((txt) => txt.includes("e2e-member-direct.localhost"));
		expect(caddyText2).toContain("e2e-member-direct.localhost");
		expect(caddyText2).toContain(`reverse_proxy ${clusterPodName}:80`);

		// 5. Test HTTP request to member-direct route through Caddy
		await new Promise((r) => setTimeout(r, 1000));
		let curlMemberOut = "";
		for (let i = 0; i < 15; i++) {
			curlMemberOut = run("curl -s -o /dev/null -w '%{http_code}' -H 'Host: e2e-member-direct.localhost' http://127.0.0.1:8080/");
			if (curlMemberOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlMemberOut).toBe("200");
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 4: TERMINAL EXEC TICKET & POD CONTAINER RESOLUTION (BUG 6 FIX)
	// ─────────────────────────────────────────────────────────────
	test("Phase 4: Terminal ticket issuance resolves Pod member container and pod fallback", async () => {
		// 1. Issue ticket for member container directly
		const webTicketRes = await api("/api/containers/e2e-web-app/terminal-ticket", {
			method: "POST",
		});
		expect(webTicketRes.status).toBe(200);
		expect(webTicketRes.data.ticket).toBeDefined();
		expect(webTicketRes.data.ticket.length).toBeGreaterThan(10);

		// 2. Issue ticket for second member container
		const cacheTicketRes = await api("/api/containers/e2e-cache-db/terminal-ticket", {
			method: "POST",
		});
		expect(cacheTicketRes.status).toBe(200);
		expect(cacheTicketRes.data.ticket).toBeDefined();

		// 3. Issue ticket using Pod name (fallback resolution inside Pod)
		const podTicketRes = await api(`/api/containers/${clusterPodName}/terminal-ticket`, {
			method: "POST",
		});
		expect(podTicketRes.status).toBe(200);
		expect(podTicketRes.data.ticket).toBeDefined();
	}, 60000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 5: UNGROUPING MEMBER SERVICE FROM POD & STANDALONE ROUTING
	// ─────────────────────────────────────────────────────────────
	test("Phase 5: Ungroup member service restores standalone networking and updates ingress", async () => {
		// 1. Fetch current cache service
		const getCache = await api(`/api/services/${cacheSvcId}`);
		expect(getCache.ok).toBeTrue();
		const cacheData = getCache.data;
		expect(cacheData.inPod).toBeTrue();

		// 2. Ungroup: set inPod = false, podId = "", runtimeTarget = "standalone"
		cacheData.inPod = false;
		cacheData.podId = "";
		cacheData.runtimeTarget = "standalone";
		const updateRes = await api(`/api/services/${cacheSvcId}`, {
			method: "PUT",
			body: JSON.stringify(cacheData),
		});
		expect(updateRes.status).toBe(200);

		// 3. Deploy standalone cache service
		const deployRes = await api(`/api/services/${cacheSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "ungroup-pod" }),
		});
		expect(deployRes.status).toBe(202);
		const depRec = await pollDeployment(deployRes.data.id, 60);
		expect(depRec.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// 4. Assert: e2e-cache-db is not a member of pod_e2e-cluster-demo
		const psInPod = run(`podman ps --filter 'pod=${clusterPodName}' --format '{{.Names}}'`);
		expect(psInPod).not.toContain("e2e-cache-db");
		expect(psInPod).toContain("e2e-web-app");

		// 5. Create Domain route targeting standalone cache service
		const standaloneDomRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: cacheSvcId,
				hostname: "e2e-cache-standalone.localhost",
				containerPort: 6379,
				tls: false,
			}),
		});
		expect(standaloneDomRes.status).toBe(201);
		standaloneDomId = standaloneDomRes.data.id;

		const caddyText = await pollCaddyfile((txt) => txt.includes("e2e-cache-standalone.localhost"));
		expect(caddyText).toContain("e2e-cache-standalone.localhost");
		// Must dial the standalone service ID or name, NOT the pod!
		expect(caddyText).not.toContain(`reverse_proxy ${clusterPodName}:6379`);
		expect(caddyText).toContain(`reverse_proxy ${cacheSvcId}:6379`);
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 6: CADDY INGRESS CORS & BASIC AUTH REAL-TRAFFIC ENFORCEMENT
	// ─────────────────────────────────────────────────────────────
	test("Phase 6: Ingress enforces CORS headers and BasicAuth credentials over HTTP", async () => {
		// 1. Create secure domain with CORS and BasicAuth enabled targeting the Pod web app
		const secureDomRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: podSvcId,
				targetContainer: "e2e-web-app",
				hostname: "e2e-secure.localhost",
				containerPort: 80,
				tls: false,
				cors: true,
				basicAuth: true,
				basicAuthUser: "secadmin",
				basicAuthPass: "Pa$$w0rd123!",
			}),
		});
		expect(secureDomRes.status).toBe(201);
		secureDomId = secureDomRes.data.id;

		// 2. Poll Caddyfile until configuration reflects CORS & basicauth
		const caddyText = await pollCaddyfile((txt) => txt.includes("e2e-secure.localhost") && txt.includes("basicauth"));
		expect(caddyText).toContain("header Access-Control-Allow-Origin *");
		expect(caddyText).toContain("basicauth {");
		expect(caddyText).toContain("secadmin $2");

		await new Promise((r) => setTimeout(r, 1000));

		// 3. Test HTTP Unauthenticated: Caddy must return 401 Unauthorized
		let unauthCode = "";
		for (let i = 0; i < 15; i++) {
			unauthCode = run("curl -s -o /dev/null -w '%{http_code}' -H 'Host: e2e-secure.localhost' http://127.0.0.1:8080/");
			if (unauthCode === "401") break;
			await new Promise((r) => setTimeout(r, 400));
		}
		expect(unauthCode).toBe("401");

		// 4. Test HTTP Authenticated: Caddy must return 200 OK
		let authCode = "";
		for (let i = 0; i < 15; i++) {
			authCode = run("curl -s -o /dev/null -w '%{http_code}' -u 'secadmin:Pa$$w0rd123!' -H 'Host: e2e-secure.localhost' http://127.0.0.1:8080/");
			if (authCode === "200") break;
			await new Promise((r) => setTimeout(r, 400));
		}
		expect(authCode).toBe("200");

		// 5. Test CORS Preflight Options request
		const corsHeaders = run("curl -s -I -X OPTIONS -H 'Host: e2e-secure.localhost' -H 'Origin: http://example.com' http://127.0.0.1:8080/");
		expect(corsHeaders.toLowerCase()).toContain("access-control-allow-origin");
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 7: QUADLET SYSTEMD HOST UNIT FILE ARTIFACTS (BUG 2 FIX)
	// ─────────────────────────────────────────────────────────────
	test("Phase 7: Quadlet generates valid .container unit file on host filesystem", async () => {
		// Read Quadlet service details
		const quadDetail = await api(`/api/services/${quadletSvcId}`);
		expect(quadDetail.ok).toBeTrue();

		// The Quadlet unit files reside in ~/.config/containers/systemd/
		const homeDir = process.env.HOME || "/root";
		const quadletDir = path.resolve(homeDir, ".config/containers/systemd");

		// List files in quadlet directory
		const unitFiles = run(`ls -1 "${quadletDir}"`);
		const hasContainer = unitFiles.includes("e2e-quadlet-sync.container") || unitFiles.includes(`${quadletSvcId}.container`);
		expect(hasContainer).toBeTrue();

		const matchingFile = unitFiles.includes("e2e-quadlet-sync.container")
			? "e2e-quadlet-sync.container"
			: `${quadletSvcId}.container`;

		// Read unit content
		const unitContent = run(`cat "${quadletDir}/${matchingFile}"`);
		expect(unitContent).toContain("[Container]");
		expect(unitContent).toContain("Image=docker.io/traefik/whoami:v1.10.1");
		expect(unitContent).toContain("Network=gopod-net");
		expect(unitContent).toContain(`Label=io.gopod.project=${testProjectId}`);
	}, 60000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 8: POD STOP & RESTART LIFECYCLE CONTROLS
	// ─────────────────────────────────────────────────────────────
	test("Phase 8: Stop and restart Pod service safely via lifecycle APIs", async () => {
		// 1. Stop Pod service
		const stopRes = await api(`/api/services/${podSvcId}/stop`, {
			method: "POST",
		});
		expect(stopRes.status).toBe(200);

		await new Promise((r) => setTimeout(r, 2000));

		// Check pod status via podman
		const podStatus = run(`podman pod ps --filter 'name=${clusterPodName}' --format '{{.Status}}'`);
		expect(["Degraded", "Exited", "Stopped"]).toContain(podStatus);

		// 2. Restart Pod service
		const restartRes = await api(`/api/services/${podSvcId}/restart`, {
			method: "POST",
		});
		expect(restartRes.status).toBe(200);

		await new Promise((r) => setTimeout(r, 3000));

		// Verify traffic routes again
		let curlOut = "";
		for (let i = 0; i < 15; i++) {
			curlOut = run("curl -s -o /dev/null -w '%{http_code}' -H 'Host: e2e-podmesh.localhost' http://127.0.0.1:8080/");
			if (curlOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlOut).toBe("200");
	}, 120000);

	// ─────────────────────────────────────────────────────────────
	// PHASE 9: RUNTIME CONTAINER INSPECTION & INFRAPAUSE ISOLATION
	// ─────────────────────────────────────────────────────────────
	test("Phase 9: Runtime container inspection lists member containers and isolates infra container", async () => {
		// 1. Fetch all containers from runtime API
		const containersRes = await api("/api/containers");
		expect(containersRes.ok).toBeTrue();
		expect(Array.isArray(containersRes.data)).toBeTrue();

		// 2. Find containers associated with the canonical test pod
		const podContainers = containersRes.data.filter((c: any) => {
			const podSuffix = clusterPodName.replace(/^pod_/, "");
			const inPod = c.pod === clusterPodName || (c.pod && c.pod.includes(podSuffix));
			const hasName = c.names && c.names.some((n: string) => n.includes("e2e-web-app"));
			return inPod || hasName;
		});
		expect(podContainers.length).toBeGreaterThan(0);

		// 3. Verify member container has accurate image and status
		const webMember = podContainers.find((c: any) => c.names?.some((n: string) => n.includes("e2e-web-app")));
		expect(webMember).toBeDefined();
		expect(webMember.image).toContain("whoami");

		// 4. Verify logs API can retrieve logs for member container
		const logsRes = await api(`/api/containers/${webMember.id}/logs?tail=20`);
		expect(logsRes.ok).toBeTrue();
		expect(logsRes.data).toBeDefined();
	}, 60000);
});
