import { describe, it, expect } from 'bun:test';
import {
	parseKubernetesPodManifest,
	parseComposeManifest,
	checkReservedHostPort
} from './manifest-engine';

describe('manifest-engine', () => {
	it('parses user multi-container pod with bash heredoc without error', () => {
		const raw = `
apiVersion: v1
kind: Pod
metadata:
  name: demo-gui-pod
  labels:
    app: demo-gui
spec:
  volumes:
    - name: shared-html
      emptyDir: {}
  containers:
    - name: web-gui
      image: docker.io/library/nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 8088
      volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
      resources:
        limits:
          memory: 64Mi
    - name: worker-sidecar
      image: docker.io/library/alpine:latest
      command: ["/bin/sh", "-c"]
      args:
        - |
          COUNT=1
          while true; do
            cat <<EOF > /usr/share/nginx/html/index.html
            <html><body>Test \$COUNT</body></html>
            EOF
            COUNT=\$((COUNT + 1))
            sleep 2
          done
      volumeMounts:
        - name: shared-html
          mountPath: /usr/share/nginx/html
`;
		const res = parseKubernetesPodManifest(raw);
		expect(res.valid).toBeTrue();
		expect(res.errors).toEqual([]);
		expect(res.podName).toBe('demo-gui-pod');
		expect(res.volumes).toEqual(['shared-html']);
		expect(res.workloads).toHaveLength(2);

		const [web, worker] = res.workloads;
		expect(web.name).toBe('web-gui');
		expect(web.role).toBe('main');
		expect(web.ports).toHaveLength(1);
		expect(web.ports[0].hostPort).toBe(8088);
		expect(web.ports[0].containerPort).toBe(80);

		expect(worker.name).toBe('worker-sidecar');
		expect(worker.role).toBe('sidecar');
		expect(worker.volumeMounts).toEqual(['shared-html -> /usr/share/nginx/html']);
	});

	it('detects syntax errors with line number', () => {
		const broken = `
apiVersion: v1
kind: Pod
metadata:
  name: [broken-syntax
`;
		const res = parseKubernetesPodManifest(broken);
		expect(res.valid).toBeFalse();
		expect(res.errors.length).toBeGreaterThan(0);
		expect(res.errors[0]).toContain('YAML syntax error');
	});

	it('flags port 8080 collision with Caddy ingress warning', () => {
		const raw = `
apiVersion: v1
kind: Pod
metadata:
  name: collision-demo
spec:
  containers:
    - name: web
      image: nginx:alpine
      ports:
        - containerPort: 80
          hostPort: 8080
`;
		const res = parseKubernetesPodManifest(raw);
		expect(res.valid).toBeTrue();
		expect(res.warnings.some((w) => w.includes('reserved by GOPOD Caddy Ingress'))).toBeTrue();
	});

	it('parses compose manifests with ports and services', () => {
		const compose = `
version: "3.8"
services:
  web:
    image: nginx:alpine
    ports:
      - "8081:80"
  db:
    image: postgres:15
    ports:
      - "127.0.0.1:5432:5432"
volumes:
  dbdata:
`;
		const res = parseComposeManifest(compose);
		expect(res.valid).toBeTrue();
		expect(res.workloads).toHaveLength(2);
		expect(res.volumes).toEqual(['dbdata']);
		expect(res.workloads[0].name).toBe('web');
		expect(res.workloads[1].name).toBe('db');
		expect(res.workloads[1].ports[0].hostIp).toBe('127.0.0.1');
	});

	it('checkReservedHostPort matches known ports', () => {
		expect(checkReservedHostPort(8080)).not.toBeNull();
		expect(checkReservedHostPort(8443)).not.toBeNull();
		expect(checkReservedHostPort(8085)).not.toBeNull();
		expect(checkReservedHostPort(9999)).toBeNull();
	});
});
