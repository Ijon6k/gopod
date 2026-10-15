import { describe, test, expect, beforeAll, afterAll } from "bun:test";
import { Database } from "bun:sqlite";
import { execSync } from "node:child_process";
import path from "node:path";
import fs from "node:fs";
import os from "node:os";

// ── Harness: mirrors UI flow Visual -> Manifest -> Apply banner  ─────────────
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
const CADDY_HTTP = process.env.CADDY_HTTP_PORT || "8080";
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

/** Strict variant: throws on non-zero exit so podman assertions cannot false-pass. */
function runStrict(cmd: string): string {
	return execSync(cmd, {
		encoding: "utf-8",
		timeout: 25000,
		env: process.env,
		shell: "/bin/bash",
	}).trim();
}

function pickFreePort(preferred: number): number {
	const used = run("ss -ltn 2>/dev/null || netstat -ltn 2>/dev/null || echo ''");
	for (let p = preferred; p < preferred + 200; p++) {
		if (!used.includes(`:${p} `) && !used.includes(`:${p}\n`)) return p;
	}
	return preferred;
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

async function pollDeployment(depId: string, maxAttempts = 90) {
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

const uid = `${Date.now().toString().slice(-6)}${Math.floor(Math.random() * 90 + 10)}`;

describe("GOPOD Unified Pod: Visual <-> Manifest sync, recreate, Quadlet, ingress", () => {
	let podSvcId = "";
	let webSvcId = "";
	let cacheSvcId = "";
	let domainId = "";
	let webName = "";
	let cacheName = "";
	const podShort = `e2e-sync-${uid}`.toLowerCase();
	let podName = "";
	const HOST_A = pickFreePort(18191);
	const HOST_B = pickFreePort(18291);
	const HOST_C = pickFreePort(18391);

	beforeAll(async () => {
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
			} catch {
				// Fallback
			}
		}

		expect(authToken).not.toBeEmpty();

		const projRes = await api("/api/projects", {
			method: "POST",
			body: JSON.stringify({
				name: `Unified Pod Sync ${uid}`,
				description: "UI-parity verification: Visual ports, Manifest sync, recreate, Quadlet, ingress",
			}),
		});
		expect(projRes.status).toBe(201);
		testProjectId = projRes.data.id;
		expect(testProjectId).toBeDefined();
	});

	afterAll(async () => {
		if (domainId) await api(`/api/domains/${domainId}`, { method: "DELETE" });
		if (webSvcId) await api(`/api/services/${webSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (cacheSvcId) await api(`/api/services/${cacheSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (podSvcId) await api(`/api/services/${podSvcId}?delete_volumes=true`, { method: "DELETE" });
		if (testProjectId) await api(`/api/projects/${testProjectId}`, { method: "DELETE" });
		run(`podman pod rm -f ${podName} 2>/dev/null`);
	});

	test("Test 1: Visual create - pod service with single host port, deploy infra", async () => {
		// Simulates PodPortForwardCard.persistPorts: writes composeYaml + k8sYaml in one PUT.
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: podShort,
				type: "pod",
				port: 80,
				hostPort: HOST_A,
				composeYaml: `version: "3.8"\nservices:\n  ${podShort}:\n    ports:\n      - "${HOST_A}:80"\n`,
				k8sYaml: `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${podShort}\nspec:\n  containers:\n    - name: infra\n      image: registry.k8s.io/pause:3.9\n      ports:\n        - containerPort: 80\n          hostPort: ${HOST_A}\n          protocol: TCP\n`,
			}),
		});
		expect(createRes.status).toBe(201);
		podSvcId = createRes.data.id;
		expect(podSvcId).toBeDefined();

		const depRes = await api(`/api/services/${podSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const depRecord = await pollDeployment(depRes.data.id, 90);
		expect(depRecord.status).toBe("success");

		// Canonical unique runtime name comes from the server (pod_<service-id>).
		const created = await api(`/api/services/${podSvcId}`);
		expect(created.ok).toBeTrue();
		podName = created.data.podId;
		expect(podName).toContain("pod_");

		const podList = runStrict("podman pod ps --format '{{.Name}}'");
		expect(podList).toContain(podName);

		const portInspect = runStrict(`podman pod inspect ${podName} --format '{{json .InfraConfig.PortBindings}}'`);
		expect(portInspect).toContain(String(HOST_A));

		// Quadlet artifact written on every pod deploy (.pod with PublishPort/InfraImage/Share)
		const quadData = path.resolve(process.cwd(), "backend", "data", "quadlets", `${podName}.pod`);
		const quadHome = path.join(os.homedir(), ".config", "containers", "systemd", `${podName}.pod`);
		const quadCandidates = [quadData, quadHome, `./data/quadlets/${podName}.pod`];
		let quadContent = "";
		for (const c of quadCandidates) {
			if (fs.existsSync(c)) {
				quadContent = fs.readFileSync(c, "utf-8");
				break;
			}
		}
		if (!quadContent) {
			const ls = run(`ls ~/.config/containers/systemd/${podName}.pod 2>/dev/null; ls backend/data/quadlets/${podName}.pod 2>/dev/null; ls data/quadlets/${podName}.pod 2>/dev/null`);
			expect(ls).toContain(`${podName}.pod`);
		} else {
			expect(quadContent).toContain("[Pod]");
			expect(quadContent).toContain(`PodName=${podName}`);
			expect(quadContent).toContain(`PublishPort=${HOST_A}:80`);
			expect(quadContent).toContain("InfraImage=");
			expect(quadContent).toContain("Share=");
		}
	}, 180000);

	test("Test 2: Attach members - web + cache join localhost namespace (modal flow)", async () => {
		webName = `web-${uid}`;
		const webRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: webName,
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/traefik/whoami:v1.10.1",
				port: 80,
				restartPolicy: "always",
			}),
		});
		expect(webRes.status).toBe(201);
		webSvcId = webRes.data.id;

		// Simulates PodAttachMemberModal.handleAttachExisting -> groupIntoPod
		const groupWeb = await api(`/api/services/${webSvcId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({ podName, createPod: false }),
		});
		expect(groupWeb.status).toBe(200);
		expect(groupWeb.data.inPod).toBeTrue();

		cacheName = `cache-${uid}`;
		const cacheRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: cacheName,
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/redis:alpine",
				port: 6379,
				restartPolicy: "always",
			}),
		});
		expect(cacheRes.status).toBe(201);
		cacheSvcId = cacheRes.data.id;

		const groupCache = await api(`/api/services/${cacheSvcId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({ podName, createPod: false }),
		});
		expect(groupCache.status).toBe(200);
		expect(groupCache.data.inPod).toBeTrue();

		await new Promise((r) => setTimeout(r, 2000));

		const psInPod = runStrict(`podman ps --filter 'pod=${podName}' --format '{{.Names}}'`);
		expect(psInPod).toContain(webName);
		expect(psInPod).toContain(cacheName);

		let curlOut = "";
		for (let i = 0; i < 20; i++) {
			curlOut = run(`curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:${HOST_A}/`);
			if (curlOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlOut).toBe("200");
	}, 180000);

	test("Test 3: Manifest edit + Apply banner - multi-port recreate with member recovery", async () => {
		// Simulates PodKubernetesEditorCard.handleSaveManifest: k8sYaml edit syncs composeYaml in same PUT.
		const podDetail = await api(`/api/services/${podSvcId}`);
		expect(podDetail.ok).toBeTrue();
		const podData = podDetail.data;

		podData.composeYaml = `version: "3.8"\nservices:\n  ${podShort}:\n    ports:\n      - "${HOST_A}:80"\n      - "${HOST_B}:6379"\n      - "127.0.0.1:${HOST_C}:80"\n`;
		podData.k8sYaml = `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${podShort}\nspec:\n  containers:\n    - name: web\n      image: docker.io/traefik/whoami:v1.10.1\n      ports:\n        - containerPort: 80\n          hostPort: ${HOST_A}\n          protocol: TCP\n    - name: cache\n      image: docker.io/library/redis:alpine\n      ports:\n        - containerPort: 6379\n          hostPort: ${HOST_B}\n          protocol: TCP\n`;

		const updateRes = await api(`/api/services/${podSvcId}`, {
			method: "PUT",
			body: JSON.stringify(podData),
		});
		expect(updateRes.status).toBe(200);

		// Visual <-> Manifest parity: GET must return both manifests in sync (what Visual tab renders)
		const afterSave = await api(`/api/services/${podSvcId}`);
		expect(afterSave.ok).toBeTrue();
		expect(afterSave.data.composeYaml).toContain(`"${HOST_A}:80"`);
		expect(afterSave.data.composeYaml).toContain(`"${HOST_B}:6379"`);
		expect(afterSave.data.composeYaml).toContain(`"127.0.0.1:${HOST_C}:80"`);
		expect(afterSave.data.k8sYaml).toContain(`hostPort: ${HOST_A}`);
		expect(afterSave.data.k8sYaml).toContain(`hostPort: ${HOST_B}`);

		// Banner semantics: runtime still has old bindings until Apply is pressed
		const inspectBefore = runStrict(`podman pod inspect ${podName} --format '{{json .InfraConfig.PortBindings}}'`);
		expect(inspectBefore).not.toContain(String(HOST_B));

		// Simulates Banner [Apply & Recreate Pod]
		const applyRes = await api(`/api/services/${podSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(applyRes.status).toBe(202);
		const depRec = await pollDeployment(applyRes.data.id, 90);
		expect(depRec.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		const inspectAfter = runStrict(`podman pod inspect ${podName} --format '{{json .InfraConfig.PortBindings}}'`);
		expect(inspectAfter).toContain(String(HOST_A));
		expect(inspectAfter).toContain(String(HOST_B));
		expect(inspectAfter).toContain(String(HOST_C));
		expect(inspectAfter).toContain("127.0.0.1");

		// Quadlet artifact carries the loopback publish form.
		const quadPaths = [
			path.resolve(process.cwd(), "backend", "data", "quadlets", `${podName}.pod`),
			path.join(os.homedir(), ".config", "containers", "systemd", `${podName}.pod`),
		];
		const quadFound = quadPaths.find((c) => fs.existsSync(c));
		if (quadFound) {
			const quadContent = fs.readFileSync(quadFound, "utf-8");
			expect(quadContent).toContain(`PublishPort=127.0.0.1:${HOST_C}:80`);
		}

		const psInPod = runStrict(`podman ps --filter 'pod=${podName}' --format '{{.Names}}'`);
		expect(psInPod).toContain(webName);
		expect(psInPod).toContain(cacheName);

		let curlOut = "";
		for (let i = 0; i < 20; i++) {
			curlOut = run(`curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:${HOST_A}/`);
			if (curlOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlOut).toBe("200");

		let redisPing = "";
		for (let i = 0; i < 20; i++) {
			redisPing = run(`echo PING | nc -w 1 127.0.0.1 ${HOST_B}`);
			if (redisPing.includes("PONG")) break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(redisPing).toContain("PONG");

		// Loopback-only binding: bound to 127.0.0.1 (secure local), reachable locally.
		const ssOut = run("ss -ltn");
		expect(ssOut).toContain(`127.0.0.1:${HOST_C}`);
		let loopCurl = "";
		for (let i = 0; i < 20; i++) {
			loopCurl = run(`curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:${HOST_C}/`);
			if (loopCurl === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(loopCurl).toBe("200");
	}, 180000);

	test("Test 4: Ingress routes to Netavark pod DNS name, not member name", async () => {
		const webData = (await api(`/api/services/${webSvcId}`)).data;
		const host = `unified-pod-${uid}.localhost`;

		const domRes = await api("/api/domains", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				serviceId: podSvcId,
				targetContainer: webData.name,
				hostname: host,
				containerPort: 80,
				tls: false,
			}),
		});
		expect(domRes.status).toBe(201);
		domainId = domRes.data.id;

		const caddyText = await pollCaddyfile((txt) => txt.includes(host));
		expect(caddyText).toContain(`reverse_proxy ${podName}:80`);
		expect(caddyText).not.toContain(`reverse_proxy ${webData.name}:80`);

		await new Promise((r) => setTimeout(r, 1000));
		let curlOut = "";
		for (let i = 0; i < 20; i++) {
			curlOut = run(`curl -s -o /dev/null -w '%{http_code}' -H 'Host: ${host}' http://127.0.0.1:${CADDY_HTTP}/`);
			if (curlOut === "200") break;
			await new Promise((r) => setTimeout(r, 500));
		}
		expect(curlOut).toBe("200");
	}, 90000);
});
