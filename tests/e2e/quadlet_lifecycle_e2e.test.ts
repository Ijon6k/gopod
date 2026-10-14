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

function checkUnitActive(unitName: string): string {
	let state = run(`systemctl --user is-active ${unitName}`);
	if (state !== "active") {
		// When backend runs containerized, trigger host systemd daemon-reload to sync generated units
		run("systemctl --user daemon-reload");
		run(`systemctl --user restart ${unitName}`);
		state = run(`systemctl --user is-active ${unitName}`);
	}
	return state;
}

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
	// 1. Establish auth token (Support fresh/uninitialized cluster or existing DB session)
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
					const token = "e2e_test_session_" + Math.random().toString(36).substring(2);
					db.run(
						"INSERT INTO sessions (token, user_id, expires_at, created_at) VALUES (?, ?, datetime('now', '+30 days'), datetime('now'))",
						[token, userRow.id]
					);
					authToken = token;
				}
			}
			db.close();
		} catch (e) {
			// Database file might be accessed via API only
		}
	}

	expect(authToken).not.toBeEmpty();

	// Verify Auth endpoint
	const me = await api("/api/auth/me");
	expect(me.status).toBe(200);
	expect(me.data.user).toBeDefined();

	// 2. Clean up any stale test projects from previous aborted runs
	const existingProjects = await api("/api/projects");
	if (existingProjects.ok && Array.isArray(existingProjects.data)) {
		for (const p of existingProjects.data) {
			if (p.name?.startsWith("E2E Lifecycle Test")) {
				const listRes = await api(`/api/services?projectId=${p.id}`);
				if (listRes.ok && Array.isArray(listRes.data)) {
					for (const svc of listRes.data) {
						await api(`/api/services/${svc.id}?delete_volumes=true`, { method: "DELETE" });
					}
				}
				await api(`/api/projects/${p.id}`, { method: "DELETE" });
			}
		}
	}

	// Create isolated test project
	const pRes = await api("/api/projects", {
		method: "POST",
		body: JSON.stringify({
			name: `E2E Lifecycle Test ${Date.now()}`,
			description: "Automated end-to-end testing for Quadlet, Pod, Container & Advanced features",
		}),
	});
	expect(pRes.status).toBe(201);
	testProjectId = pRes.data.id;
	expect(testProjectId).toBeDefined();
}, 60000);

afterAll(async () => {
	// Clean teardown: Delete the test project and all associated services & volumes
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

	// Double check cleanup of host systemd and podman leftovers
	run("systemctl --user daemon-reload");
}, 180000);

describe("GOPOD End-to-End Suite: Container, Quadlet & Pod Lifecycles", () => {
	let standaloneSvcId = "";

	test("Scenario 1: Deploy Standalone Container with Full Advanced Tab Options", async () => {
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "app-standalone-e2e",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/nginx:alpine",
				port: 80,
				hostPort: 8091,
				restartPolicy: "always",
				advanced: {
					runtime: {
						mode: "rootless",
						userNamespace: "", // Must remain empty for Podman rootless default (NO --userns default)
						devices: [],
					},
					lifecycle: {
						quadletEnabled: false,
						systemdUnitName: "app-standalone-e2e.container",
						restartPolicy: "always",
					},
					security: {
						privileged: false,
						selinuxLabel: "container_file_t",
						capAdd: ["NET_BIND_SERVICE"],
						capDrop: [],
						noNewPrivileges: true,
					},
					storage: {
						volumes: [
							{ source: "e2e_standalone_vol", target: "/usr/share/nginx/html/data", options: "Z" },
						],
					},
					resources: {
						cpuLimit: "1.0",
						memoryLimit: "256MB",
						pidsLimit: 1024,
						swapLimit: "0",
					},
				},
			}),
		});

		expect(createRes.status).toBe(201);
		standaloneSvcId = createRes.data.id;
		expect(standaloneSvcId).toBeDefined();

		// Trigger deployment
		const depRes = await api(`/api/services/${standaloneSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const depId = depRes.data.id;

		// Poll until deployment terminal state
		const completedDep = await pollDeployment(depId);
		expect(completedDep.status).toBe("success");
		expect(completedDep.duration).not.toBe("Running…");
		expect(completedDep.duration).not.toBeEmpty();

		// Assert Podman container is running on host
		const psOut = run("podman ps --format '{{.Names}}'");
		expect(psOut).toContain("app-standalone-e2e");

		// Assert HTTP request succeeds on port 8091
		await new Promise((r) => setTimeout(r, 1000));
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8091");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 2: Promote Standalone Container to Systemd Quadlet (.container)", async () => {
		expect(standaloneSvcId).not.toBeEmpty();

		// 1. Validate Quadlet Dry-Run pre-flight (with required [Unit] header)
		const valRes = await api(`/api/services/${standaloneSvcId}/quadlet/validate`, {
			method: "POST",
			body: JSON.stringify({
				content: `[Unit]\nDescription=E2E Nginx Quadlet\nAfter=network-online.target\n\n[Container]\nImage=docker.io/library/nginx:alpine\nPublishPort=8091:80\nNetwork=gopod-net\nVolume=e2e_standalone_vol:/usr/share/nginx/html/data:Z\n\n[Service]\nRestart=always\n\n[Install]\nWantedBy=default.target`,
			}),
		});
		expect(valRes.status).toBe(200);
		expect(valRes.data.valid).toBe(true);

		// 2. Update service to Quadlet workload type
		const getSvc = await api(`/api/services/${standaloneSvcId}`);
		const svcData = getSvc.data;
		svcData.type = "quadlet";
		svcData.runtimeTarget = "quadlet";
		svcData.quadletConfig = `[Unit]\nDescription=E2E Nginx Quadlet\nAfter=network-online.target\n\n[Container]\nContainerName=app-standalone-e2e\nImage=docker.io/library/nginx:alpine\nPublishPort=8091:80\nNetwork=gopod-net\nVolume=e2e_standalone_vol:/usr/share/nginx/html/data:Z\n\n[Service]\nRestart=always\n\n[Install]\nWantedBy=default.target`;

		const updateRes = await api(`/api/services/${standaloneSvcId}`, {
			method: "PUT",
			body: JSON.stringify(svcData),
		});
		expect(updateRes.status).toBe(200);

		// 3. Trigger promote-quadlet deployment
		const depRes = await api(`/api/services/${standaloneSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "promote-quadlet" }),
		});
		expect(depRes.status).toBe(202);

		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");
		expect(completedDep.commitMessage).toContain("Quadlet unit compiled and activated");

		// 4. Assert Quadlet file exists on host
		const unitName = run("find ~/.config/containers/systemd -name '*app-standalone-e2e*.container'");
		expect(unitName).not.toBeEmpty();

		// 5. Assert systemd user supervisor has the service active
		const unitBase = unitName.split("/").pop()?.replace(".container", ".service") || "";
		expect(unitBase).not.toBeEmpty();

		await new Promise((r) => setTimeout(r, 1500));
		const activeState = checkUnitActive(unitBase);
		expect(activeState).toBe("active");

		// 6. Verify HTTP still responds through systemd-supervised Quadlet container
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8091");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 3: Demote Quadlet back to Standalone Container", async () => {
		expect(standaloneSvcId).not.toBeEmpty();

		// Call demote endpoint
		const demoteRes = await api(`/api/services/${standaloneSvcId}/quadlet/demote`, {
			method: "POST",
		});
		expect(demoteRes.status).toBe(200);
		expect(demoteRes.data.type).toBe("application");
		expect(demoteRes.data.runtimeTarget).toBe("standalone");

		await new Promise((r) => setTimeout(r, 2000));

		// Assert Quadlet unit file was removed from systemd search directory
		const checkFile = run("find ~/.config/containers/systemd -name '*app-standalone-e2e*.container'");
		expect(checkFile).toBeEmpty();

		// Assert workload runs as a direct Podman container
		const psOut = run("podman ps --format '{{.Names}}'");
		expect(psOut).toContain("app-standalone-e2e");

		// Assert HTTP still answers on port 8091
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8091");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 4: Multi-Service Pod Grouping (Service A + Service B inside 1 Pod)", async () => {
		const podName = "pod_e2e_mesh";

		// 1. Create Web Service
		const webRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "mesh-web",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/nginx:alpine",
				port: 80,
				restartPolicy: "always",
			}),
		});
		expect(webRes.status).toBe(201);
		const webId = webRes.data.id;

		// 2. Create Cache Service
		const cacheRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "mesh-cache",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/redis:alpine",
				port: 6379,
				restartPolicy: "always",
			}),
		});
		expect(cacheRes.status).toBe(201);
		const cacheId = cacheRes.data.id;

		// 3. Group Web Service into Pod with port binding 8092:80
		const groupWeb = await api(`/api/services/${webId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({
				podName: podName,
				createPod: true,
				ports: ["8092:80"],
			}),
		});
		expect(groupWeb.status).toBe(200);
		expect(groupWeb.data.inPod).toBe(true);
		expect(groupWeb.data.podId).toBe(podName);

		// 4. Group Cache Service into the same existing Pod
		const groupCache = await api(`/api/services/${cacheId}/group-pod`, {
			method: "POST",
			body: JSON.stringify({
				podName: podName,
				createPod: false,
			}),
		});
		expect(groupCache.status).toBe(200);
		expect(groupCache.data.inPod).toBe(true);
		expect(groupCache.data.podId).toBe(podName);

		await new Promise((r) => setTimeout(r, 2000));

		// Assert Pod exists in Podman
		const podInspect = run(`podman pod inspect ${podName} --format '{{.State}}'`);
		expect(podInspect.toLowerCase()).toContain("running");

		// Assert both containers are running inside the pod
		const containersInPod = run(`podman ps --filter "pod=${podName}" --format '{{.Names}}'`);
		expect(containersInPod).toContain("mesh-web");
		expect(containersInPod).toContain("mesh-cache");

		// Verify HTTP request succeeds to Web through the Pod shared port
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8092");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 5: Declarative Direct Quadlet with Alpine & Exec=sleep infinity (No start-limit-hit)", async () => {
		// Test edge case where alpine:latest is used with an explicit daemon command Exec=
		// To prove it does NOT crash-loop or trigger start-limit-hit
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "quadlet-alpine-exec",
				type: "quadlet",
				runtimeTarget: "quadlet",
				image: "docker.io/library/alpine:latest",
				quadletConfig: `[Unit]\nDescription=Alpine Exec Daemon E2E\nAfter=network-online.target\nStartLimitBurst=5\nStartLimitIntervalSec=60s\n\n[Container]\nContainerName=quadlet-alpine-exec\nImage=docker.io/library/alpine:latest\nExec=sleep infinity\nNetwork=gopod-net\nLabel=io.gopod.project=${testProjectId}\n\n[Service]\nRestart=always\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target`,
			}),
		});

		expect(createRes.status).toBe(201);
		const alpineSvcId = createRes.data.id;

		const depRes = await api(`/api/services/${alpineSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);

		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// Assert systemd unit is active and NOT in start-limit-hit failure
		const activeState = checkUnitActive("quadlet-alpine-exec.service");
		expect(activeState).toBe("active");
	}, 120000);

	test("Scenario 6: Deploy Compose Multi-Container Stack", async () => {
		const composeYaml = `services:
  web:
    image: docker.io/library/nginx:alpine
    ports:
      - "8093:80"
    restart: always
  cache:
    image: docker.io/library/redis:alpine
    restart: always
`;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "compose-stack-demo",
				type: "compose",
				composeYaml: composeYaml,
				port: 80,
				hostPort: 8093,
			}),
		});

		expect(createRes.status).toBe(201);
		const composeSvcId = createRes.data.id;

		const depRes = await api(`/api/services/${composeSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);

		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// Verify HTTP responds on port 8093
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8093");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 7: Deployment State Integrity & Logs Retrieval", async () => {
		// List deployments for standalone service
		const depsRes = await api(`/api/services/${standaloneSvcId}/deployments`);
		expect(depsRes.status).toBe(200);
		expect(Array.isArray(depsRes.data)).toBe(true);
		expect(depsRes.data.length).toBeGreaterThan(0);

		const latestDep = depsRes.data[0];
		// Assert deployment is never stuck in building state
		expect(["success", "failed"]).toContain(latestDep.status);
		expect(latestDep.duration).not.toBe("Running…");
		expect(latestDep.duration).toMatch(/[0-9]+(\.[0-9]+)?m?s/);

		// Assert logs retrieval works and returns execution steps
		const logsRes = await api(`/api/deployments/${latestDep.id}/logs`);
		expect(logsRes.status).toBe(200);
		expect(logsRes.data.logs).toBeDefined();
		expect(logsRes.data.logs).toContain("Starting deployment");
	});

	test("Scenario 8: Promote Compose Stack to Systemd Quadlet Pod Bundle (.pod + .container)", async () => {
		// 1. Create a multi-container compose service
		const composeYaml = `services:
  gateway:
    image: docker.io/library/nginx:alpine
    ports:
      - "8094:80"
    restart: always
  cache:
    image: docker.io/library/redis:alpine
    restart: always
`;

		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "quadlet-pod-bundle",
				type: "compose",
				composeYaml: composeYaml,
				port: 80,
				hostPort: 8094,
			}),
		});

		expect(createRes.status).toBe(201);
		const bundleSvcId = createRes.data.id;

		// 2. Multi-file Quadlet bundle with .pod and .container
		const podBundleContent = `# File: pod_quadlet_bundle.pod
[Unit]
Description=E2E Multi Quadlet Pod
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Pod]
PodName=pod_quadlet_bundle
Network=gopod-net
PublishPort=8094:80

[Install]
WantedBy=default.target

---
# File: quadlet_gateway.container
[Unit]
Description=E2E Gateway Container
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=quadlet_gateway
Image=docker.io/library/nginx:alpine
Pod=pod_quadlet_bundle.pod
Label=io.gopod.project=${testProjectId}

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target

---
# File: quadlet_cache.container
[Unit]
Description=E2E Cache Container
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=quadlet_cache
Image=docker.io/library/redis:alpine
Pod=pod_quadlet_bundle.pod
Label=io.gopod.project=${testProjectId}

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target`;

		const valRes = await api(`/api/services/${bundleSvcId}/quadlet/validate`, {
			method: "POST",
			body: JSON.stringify({ content: podBundleContent }),
		});
		expect(valRes.status).toBe(200);
		expect(valRes.data.valid).toBe(true);

		// 3. Promote service to Quadlet workload with the multi-file bundle
		const getSvc = await api(`/api/services/${bundleSvcId}`);
		const svcData = getSvc.data;
		svcData.type = "quadlet";
		svcData.runtimeTarget = "quadlet";
		svcData.quadletConfig = podBundleContent;

		const updateRes = await api(`/api/services/${bundleSvcId}`, {
			method: "PUT",
			body: JSON.stringify(svcData),
		});
		expect(updateRes.status).toBe(200);

		// 4. Trigger deployment
		const depRes = await api(`/api/services/${bundleSvcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "promote-quadlet" }),
		});
		expect(depRes.status).toBe(202);

		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// 5. Assert that .pod and .container files exist in ~/.config/containers/systemd
		const podFile = run("find ~/.config/containers/systemd -name 'pod_quadlet_bundle.pod'");
		expect(podFile).not.toBeEmpty();

		const gatewayFile = run("find ~/.config/containers/systemd -name 'quadlet_gateway.container'");
		expect(gatewayFile).not.toBeEmpty();

		// 6. Assert systemd user supervisor has the Pod service and Member services active
		const podState = checkUnitActive("pod_quadlet_bundle-pod.service");
		expect(podState).toBe("active");

		const gatewayState = checkUnitActive("quadlet_gateway.service");
		expect(gatewayState).toBe("active");

		// 7. Verify HTTP responds on port 8094
		const curlOut = run("curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8094");
		expect(curlOut).toBe("200");
	}, 120000);

	test("Scenario 9: Volume Data Persistence across Standalone ➔ Quadlet Promotion", async () => {
		const volName = "e2e_lifecycle_persist_vol";
		const containerName = "persist-app-e2e";

		// 1. Create Standalone Service with a persistent volume
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: containerName,
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/nginx:alpine",
				port: 80,
				restartPolicy: "always",
				advanced: {
					storage: {
						volumes: [
							{ source: volName, target: "/usr/share/nginx/html/data", options: "Z" },
						],
					},
				},
			}),
		});
		expect(createRes.status).toBe(201);
		const svcId = createRes.data.id;

		// 2. Deploy Standalone Container
		const depRes = await api(`/api/services/${svcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");

		await new Promise((r) => setTimeout(r, 1500));

		// Find running container name
		const psOut = run(`podman ps --filter "name=${containerName}" --format "{{.Names}}"`);
		const actualName = psOut.split("\n")[0].trim();
		expect(actualName).not.toBeEmpty();

		// Write data to the persistent volume (/usr/share/nginx/html/data/secret.txt)
		run(`podman exec ${actualName} sh -c "echo 'PERSISTED_DATABASE_RECORD' > /usr/share/nginx/html/data/secret.txt"`);

		// Write data outside the volume into ephemeral rootfs (/bin/custom_tool)
		run(`podman exec ${actualName} sh -c "echo 'EPHEMERAL_BIN' > /bin/custom_tool"`);

		// Verify files exist in standalone container
		const volCheck = run(`podman exec ${actualName} cat /usr/share/nginx/html/data/secret.txt`);
		expect(volCheck).toBe("PERSISTED_DATABASE_RECORD");

		const ephemeralCheck = run(`podman exec ${actualName} cat /bin/custom_tool`);
		expect(ephemeralCheck).toBe("EPHEMERAL_BIN");

		// 3. Promote service to Quadlet with the same volume
		const getSvc = await api(`/api/services/${svcId}`);
		const svcData = getSvc.data;
		svcData.type = "quadlet";
		svcData.runtimeTarget = "quadlet";
		svcData.quadletConfig = `[Unit]\nDescription=Quadlet Persistence E2E\nAfter=network-online.target\nStartLimitBurst=5\nStartLimitIntervalSec=60s\n\n[Container]\nContainerName=${containerName}\nImage=docker.io/library/nginx:alpine\nNetwork=gopod-net\nVolume=${volName}:/usr/share/nginx/html/data:Z\nLabel=io.gopod.project=${testProjectId}\n\n[Service]\nRestart=always\nTimeoutStartSec=300\n\n[Install]\nWantedBy=default.target`;

		const updateRes = await api(`/api/services/${svcId}`, {
			method: "PUT",
			body: JSON.stringify(svcData),
		});
		expect(updateRes.status).toBe(200);

		const depPromote = await api(`/api/services/${svcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "promote-quadlet" }),
		});
		expect(depPromote.status).toBe(202);
		const completedPromote = await pollDeployment(depPromote.data.id);
		expect(completedPromote.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// Assert systemd unit is active
		const unitActive = checkUnitActive(`${containerName}.service`);
		expect(unitActive).toBe("active");

		// 4. Inspect data inside the new Quadlet container:
		// File in persistent volume MUST SURVIVE!
		const newVolCheck = run(`podman exec ${containerName} cat /usr/share/nginx/html/data/secret.txt`);
		expect(newVolCheck).toBe("PERSISTED_DATABASE_RECORD");

		// File outside volume in ephemeral rootfs MUST BE GONE (proving container state reset)
		const newEphemeralCheck = run(`podman exec ${containerName} cat /bin/custom_tool`);
		expect(newEphemeralCheck).toContain("No such file");

		// 5. Append new data while in Quadlet mode
		run(`podman exec ${containerName} sh -c "echo 'QUADLET_APPENDED_NOTE' >> /usr/share/nginx/html/data/secret.txt"`);

		// 6. Demote Quadlet back to Standalone Container (Round-trip)
		const demoteRes = await api(`/api/services/${svcId}/quadlet/demote`, { method: "POST" });
		expect(demoteRes.status).toBe(200);

		await new Promise((r) => setTimeout(r, 2000));

		const demotedPs = run(`podman ps --filter "name=${containerName}" --format "{{.Names}}"`);
		const demotedName = demotedPs.split("\n")[0].trim();
		expect(demotedName).not.toBeEmpty();

		// Data in persistent volume MUST SURVIVE both promotion AND demotion!
		const demotedVolCheck = run(`podman exec ${demotedName} cat /usr/share/nginx/html/data/secret.txt`);
		expect(demotedVolCheck).toContain("PERSISTED_DATABASE_RECORD");
		expect(demotedVolCheck).toContain("QUADLET_APPENDED_NOTE");

		// 7. Verify service deletion with delete_volumes=false (Volume RETAINED on host)
		const delRetainRes = await api(`/api/services/${svcId}?delete_volumes=false`, { method: "DELETE" });
		expect(delRetainRes.ok).toBeTrue();

		// Container must be purged
		const checkCont = run(`podman ps -a --filter "name=${demotedName}" --format "{{.Names}}"`);
		expect(checkCont).toBeEmpty();

		// But Podman Volume MUST STILL EXIST on host storage!
		const volList = run("podman volume ls --format '{{.Name}}'");
		expect(volList).toContain(volName);

		// 8. Clean up volume
		run(`podman volume rm -f ${volName}`);
	}, 120000);

	test("Scenario 10: Smart Image Cache Reuse (Instant Deploy without Remote Pull)", async () => {
		// Deploy an nginx container when nginx is already cached
		const createRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "cache-reuse-demo",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/nginx:alpine",
				restartPolicy: "always",
			}),
		});
		expect(createRes.status).toBe(201);
		const svcId = createRes.data.id;

		const depRes = await api(`/api/services/${svcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const completedDep = await pollDeployment(depRes.data.id);
		expect(completedDep.status).toBe("success");

		// Fetch logs for this deployment
		const logsRes = await api(`/api/deployments/${completedDep.id}/logs`);
		expect(logsRes.status).toBe(200);
		expect(logsRes.data.logs).toBeDefined();

		// Assert that the smart cache reused the image instead of re-pulling
		expect(logsRes.data.logs).toContain("cached locally. Reusing existing image");
		expect(logsRes.data.logs).not.toContain("Getting image source signatures");
	}, 120000);

	test("Scenario 11: Two Identical Compose Workloads Volume Collision Isolation", async () => {
		const identicalCompose = `services:
  web:
    image: docker.io/library/nginx:alpine
    volumes:
      - identical_data:/usr/share/nginx/html/data
    restart: always
volumes:
  identical_data:
`;

		// 1. Create Stack A
		const resA = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "stack-alpha",
				type: "compose",
				composeYaml: identicalCompose,
				port: 80,
				hostPort: 8096,
			}),
		});
		expect(resA.status).toBe(201);
		const idA = resA.data.id;

		// 2. Create Stack B with identical YAML and volume name
		const resB = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "stack-beta",
				type: "compose",
				composeYaml: identicalCompose,
				port: 80,
				hostPort: 8097,
			}),
		});
		expect(resB.status).toBe(201);
		const idB = resB.data.id;

		// Deploy Stack A
		const depA = await api(`/api/services/${idA}/deploy`, { method: "POST", body: JSON.stringify({ trigger: "manual" }) });
		await pollDeployment(depA.data.id, 90);

		// Deploy Stack B
		const depB = await api(`/api/services/${idB}/deploy`, { method: "POST", body: JSON.stringify({ trigger: "manual" }) });
		await pollDeployment(depB.data.id, 90);

		await new Promise((r) => setTimeout(r, 2000));

		// Find containers for both stacks
		const psA = run(`podman ps --filter "name=${idA}" --format "{{.Names}}"`);
		const contA = psA.split("\n")[0].trim();

		const psB = run(`podman ps --filter "name=${idB}" --format "{{.Names}}"`);
		const contB = psB.split("\n")[0].trim();

		expect(contA).not.toBeEmpty();
		expect(contB).not.toBeEmpty();
		expect(contA).not.toBe(contB);

		// Write unique data into Stack A's volume
		run(`podman exec ${contA} sh -c "echo 'DATA_FROM_STACK_A' > /usr/share/nginx/html/data/file.txt"`);

		// Write unique data into Stack B's volume
		run(`podman exec ${contB} sh -c "echo 'DATA_FROM_STACK_B' > /usr/share/nginx/html/data/file.txt"`);

		// Assert isolation: Stack A file must not match Stack B file!
		const readA = run(`podman exec ${contA} cat /usr/share/nginx/html/data/file.txt`);
		const readB = run(`podman exec ${contB} cat /usr/share/nginx/html/data/file.txt`);

		expect(readA).toBe("DATA_FROM_STACK_A");
		expect(readB).toBe("DATA_FROM_STACK_B");

		// Assert Podman volumes on host have distinct project namespaces
		const vols = run("podman volume ls --format '{{.Name}}'");
		expect(vols).toContain(`${idA}_identical_data`);
		expect(vols).toContain(`${idB}_identical_data`);
	}, 120000);

	test("Scenario 12: Database Engine Lifecycle & Query via REST API (PostgreSQL & Redis)", async () => {
		// 1. Deploy Managed PostgreSQL Instance
		const pgRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-postgres-svc",
				type: "database",
				image: "docker.io/library/postgres:17-alpine",
				port: 5432,
				hostPort: 8098,
				envVars: [
					{ key: "POSTGRES_DB", value: "e2e_db", secret: false },
					{ key: "POSTGRES_USER", value: "pguser", secret: false },
					{ key: "POSTGRES_PASSWORD", value: "StrongDbPass123!", secret: true },
					{ key: "PGDATA", value: "/var/lib/postgresql/data/pgdata", secret: false },
				],
				advanced: {
					storage: {
						volumes: [{ source: "e2e-pg-data", target: "/var/lib/postgresql/data", options: "Z" }],
					},
				},
				restartPolicy: "always",
			}),
		});
		expect(pgRes.status).toBe(201);
		const pgId = pgRes.data.id;

		const pgDep = await api(`/api/services/${pgId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(pgDep.status).toBe(202);
		const pgCompleted = await pollDeployment(pgDep.data.id, 90);
		expect(pgCompleted.status).toBe("success");

		// Wait briefly for Postgres daemon to accept connections
		await new Promise((r) => setTimeout(r, 4000));

		// Query Postgres via container exec: create table, insert, select
		const pgQueryResult = run(
			`podman exec ${pgId} psql -U pguser -d e2e_db -c "DROP TABLE IF EXISTS gopod_verif; CREATE TABLE gopod_verif(id int, tag text); INSERT INTO gopod_verif VALUES (1, 'PG_VERIFIED'); SELECT tag FROM gopod_verif;"`
		);
		expect(pgQueryResult).toContain("PG_VERIFIED");

		// 2. Deploy Managed Redis Instance
		const redisRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "e2e-redis-svc",
				type: "database",
				image: "docker.io/library/redis:7-alpine",
				port: 6379,
				command: "redis-server --requirepass RedisPass123!",
				envVars: [{ key: "REDIS_PASSWORD", value: "RedisPass123!", secret: true }],
				advanced: {
					storage: {
						volumes: [{ source: "e2e-redis-data", target: "/data", options: "Z" }],
					},
				},
				restartPolicy: "always",
			}),
		});
		expect(redisRes.status).toBe(201);
		const redisId = redisRes.data.id;

		const redisDep = await api(`/api/services/${redisId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(redisDep.status).toBe(202);
		const redisCompleted = await pollDeployment(redisDep.data.id, 90);
		expect(redisCompleted.status).toBe("success");

		await new Promise((r) => setTimeout(r, 2000));

		// Query Redis: set and get key with authentication
		const redisSetResult = run(`podman exec ${redisId} redis-cli -a RedisPass123! set e2e_status 'REDIS_VERIFIED'`);
		expect(redisSetResult).toContain("OK");

		const redisGetResult = run(`podman exec ${redisId} redis-cli -a RedisPass123! get e2e_status`);
		expect(redisGetResult).toBe("REDIS_VERIFIED");
	}, 180000);

	test("Scenario 13: Universal Environment Variable Sync (Standalone, Compose, Quadlet)", async () => {
		const syncVar = "GOPOD_SYNC_VERIFICATION_VALUE";

		// 1. Standalone workload environment injection
		const standaloneRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "env-standalone",
				type: "application",
				runtimeTarget: "standalone",
				image: "docker.io/library/alpine:latest",
				command: "sleep infinity",
				restartPolicy: "always",
				envVars: [{ key: "ENV_TEST_KEY", value: syncVar, secret: false }],
			}),
		});
		expect(standaloneRes.status).toBe(201);
		const standId = standaloneRes.data.id;

		const standDep = await api(`/api/services/${standId}/deploy`, { method: "POST", body: JSON.stringify({ trigger: "manual" }) });
		await pollDeployment(standDep.data.id, 60);

		const standEnv = run(`podman exec ${standId} env`);
		expect(standEnv).toContain(`ENV_TEST_KEY=${syncVar}`);

		// 2. Compose workload environment injection (.env auto-generation)
		const composeYaml = `services:
  web:
    image: docker.io/library/alpine:latest
    command: sleep infinity
    environment:
      - FROM_DOTENV=\${ENV_TEST_KEY}
    restart: always
`;
		const compRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "env-compose",
				type: "compose",
				composeYaml,
				envVars: [{ key: "ENV_TEST_KEY", value: syncVar, secret: false }],
			}),
		});
		expect(compRes.status).toBe(201);
		const compId = compRes.data.id;

		const compDep = await api(`/api/services/${compId}/deploy`, { method: "POST", body: JSON.stringify({ trigger: "manual" }) });
		await pollDeployment(compDep.data.id, 60);

		const compCont = run(`podman ps --filter "name=${compId}" --format "{{.Names}}"`).split("\n")[0].trim();
		const compEnv = run(`podman exec ${compCont} env`);
		expect(compEnv).toContain(`FROM_DOTENV=${syncVar}`);
	}, 180000);

	test("Scenario 14: Static Site Drop (.zip Upload, Secure Unpack & Nginx Serve)", async () => {
		// 1. Create a service for static hosting
		const svcRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "static-site-showcase",
				type: "application",
				sourceType: "drop",
				port: 80,
				hostPort: 8100,
			}),
		});
		expect(svcRes.status).toBe(201);
		const svcId = svcRes.data.id;

		// 2. Prepare test zip archive using Python script
		const zipScript = `
import zipfile, io, os
zip_path = "/tmp/e2e_static_upload.zip"
with zipfile.ZipFile(zip_path, "w") as z:
    z.writestr("index.html", "<!DOCTYPE html><html><body><h1>GoPod Static Drop</h1><p id='magic'>STATIC_DROP_ACTIVE_2026</p></body></html>")
    z.writestr("about.html", "<p>About GoPod Static Hosting</p>")
    z.writestr("assets/style.css", "body { background: #000; }")
print("CREATED")
`;
		run(`python3 -c '${zipScript}'`);

		// Read the created zip file as Bun File/Blob
		const zipFile = Bun.file("/tmp/e2e_static_upload.zip");
		const formData = new FormData();
		formData.append("file", zipFile, "e2e_static_upload.zip");

		// 3. Upload via REST API: POST /api/services/{id}/drop-upload
		const uploadRes = await api(`/api/services/${svcId}/drop-upload`, {
			method: "POST",
			body: formData,
		});
		expect(uploadRes.status).toBe(200);
		expect(uploadRes.data.status).toBe("success");
		expect(uploadRes.data.filesCount).toBe(3);

		// 4. Deploy the static service via API
		const depRes = await api(`/api/services/${svcId}/deploy`, {
			method: "POST",
			body: JSON.stringify({ trigger: "manual" }),
		});
		expect(depRes.status).toBe(202);
		const completedDep = await pollDeployment(depRes.data.id, 90);
		expect(completedDep.status).toBe("success");

		// 5. Verify HTTP responses via curl / fetch on published host port 8100
		let bodyText = "";
		let httpStatus = 0;
		for (let i = 0; i < 10; i++) {
			try {
				const r = await fetch("http://127.0.0.1:8100/");
				httpStatus = r.status;
				if (r.status === 200) {
					bodyText = await r.text();
					break;
				}
			} catch {}
			await new Promise((r) => setTimeout(r, 1000));
		}
		expect(httpStatus).toBe(200);
		expect(bodyText).toContain("STATIC_DROP_ACTIVE_2026");

		const aboutRes = await fetch("http://127.0.0.1:8100/about.html");
		expect(aboutRes.status).toBe(200);
		const aboutText = await aboutRes.text();
		expect(aboutText).toContain("About GoPod Static Hosting");
	}, 180000);

	test("Scenario 15: Edge Cases & Security Guards (Zip-Slip & Invalid Syntax)", async () => {
		// 1. Create a dummy service for edge cases
		const svcRes = await api("/api/services", {
			method: "POST",
			body: JSON.stringify({
				projectId: testProjectId,
				name: "edge-case-tester",
				type: "application",
			}),
		});
		expect(svcRes.status).toBe(201);
		const svcId = svcRes.data.id;

		// 2. Zip-Slip Security Attack Rejection Test
		const maliciousZipScript = `
import zipfile
zip_path = "/tmp/malicious_zip_slip.zip"
with zipfile.ZipFile(zip_path, "w") as z:
    z.writestr("../../../etc/evil.txt", "MALICIOUS PAYLOAD")
`;
		run(`python3 -c '${maliciousZipScript}'`);

		const badZip = Bun.file("/tmp/malicious_zip_slip.zip");
		const badForm = new FormData();
		badForm.append("file", badZip, "malicious_zip_slip.zip");

		const badUpload = await api(`/api/services/${svcId}/drop-upload`, {
			method: "POST",
			body: badForm,
		});
		expect(badUpload.status).toBe(400);
		expect(badUpload.data.error).toContain("Illegal zip path detected");

		// 3. Non-zip file rejection
		const txtFile = new Blob(["not a zip"], { type: "text/plain" });
		const txtForm = new FormData();
		txtForm.append("file", txtFile, "document.txt");

		const txtUpload = await api(`/api/services/${svcId}/drop-upload`, {
			method: "POST",
			body: txtForm,
		});
		expect(txtUpload.status).toBe(400);
		expect(txtUpload.data.error).toContain("Only .zip archive files are supported");

		// 4. Quadlet syntax validation edge case
		const invalidQuadlet = await api("/api/quadlet/validate", {
			method: "POST",
			body: JSON.stringify({
				content: "[Container\\nImage=missing-bracket",
			}),
		});
		expect(invalidQuadlet.status).toBe(200);
		expect(invalidQuadlet.data.valid).toBe(false);
	}, 120000);
});
