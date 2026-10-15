import { describe, it, expect } from 'bun:test';
import {
	parseDetailedPorts,
	parseK8sPodPorts,
	serializePodComposeYaml,
	serializePodK8sYaml,
	normalizePortKey,
	portBindingId,
	type PortBindingItem
} from './pod-ports';

describe('pod-ports parser & serializer', () => {
	it('parseDetailedPorts returns empty array on empty yaml', () => {
		expect(parseDetailedPorts('')).toEqual([]);
		expect(parseDetailedPorts('   ')).toEqual([]);
	});

	it('parseDetailedPorts extracts single and multiple ports with protocols', () => {
		const yaml = `version: "3.8"
services:
  cluster-demo:
    image: nginx:alpine
    ports:
      - "8090:80"
      - "6379:6379/tcp"
      - "9000:9000/udp"
`;
		const result = parseDetailedPorts(yaml);
		expect(result).toHaveLength(3);
		expect(result[0]).toEqual({
			id: 'port-0.0.0.0-8090-80-tcp',
			hostPort: 8090,
			podPort: 80,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
		expect(result[1]).toEqual({
			id: 'port-0.0.0.0-6379-6379-tcp',
			hostPort: 6379,
			podPort: 6379,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
		expect(result[2]).toEqual({
			id: 'port-0.0.0.0-9000-9000-udp',
			hostPort: 9000,
			podPort: 9000,
			protocol: 'udp',
			hostIp: '0.0.0.0'
		});
	});

	it('serializePodComposeYaml creates clean compose yaml from scratch', () => {
		const ports: PortBindingItem[] = [
			{ id: 'p1', hostPort: 8090, podPort: 80, protocol: 'tcp', hostIp: '0.0.0.0' },
			{ id: 'p2', hostPort: 6379, podPort: 6379, protocol: 'tcp', hostIp: '0.0.0.0' }
		];
		const yaml = serializePodComposeYaml('cluster-demo', ports);
		expect(yaml).toContain('version: "3.8"');
		expect(yaml).toContain('cluster-demo:');
		expect(yaml).toContain('ports:');
		expect(yaml).toContain('- "8090:80"');
		expect(yaml).toContain('- "6379:6379"');
	});

	it('serializePodComposeYaml updates ports in existing multi-service compose yaml', () => {
		const existing = `version: "3.8"
services:
  cluster-demo:
    image: nginx:alpine
    ports:
      - "8080:80"
    restart: always
  redis:
    image: redis:alpine
`;
		const updatedPorts: PortBindingItem[] = [
			{ id: 'p1', hostPort: 8095, podPort: 80, protocol: 'tcp', hostIp: '0.0.0.0' },
			{ id: 'p2', hostPort: 6379, podPort: 6379, protocol: 'tcp', hostIp: '0.0.0.0' }
		];

		const result = serializePodComposeYaml('cluster-demo', updatedPorts, existing);
		expect(result).toContain('- "8095:80"');
		expect(result).toContain('- "6379:6379"');
		expect(result).toContain('redis:');
		expect(result).toContain('image: redis:alpine');
	});

	it('serializePodK8sYaml outputs valid Kubernetes Pod manifest', () => {
		const ports: PortBindingItem[] = [
			{ id: 'p1', hostPort: 8090, podPort: 80, protocol: 'tcp', hostIp: '0.0.0.0' },
			{ id: 'p2', hostPort: 5353, podPort: 53, protocol: 'udp', hostIp: '127.0.0.1' }
		];
		const k8s = serializePodK8sYaml('cluster-demo', ports);
		expect(k8s).toContain('apiVersion: v1');
		expect(k8s).toContain('kind: Pod');
		expect(k8s).toContain('name: cluster-demo');
		expect(k8s).toContain('containerPort: 80');
		expect(k8s).toContain('hostPort: 8090');
		expect(k8s).toContain('protocol: TCP');
		expect(k8s).toContain('containerPort: 53');
		expect(k8s).toContain('hostPort: 5353');
		expect(k8s).toContain('protocol: UDP');
	});

	it('parseK8sPodPorts correctly extracts ports and protocols from Kubernetes Pod YAML', () => {
		const k8sYaml = `apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
spec:
  containers:
    - name: infra
      ports:
        - containerPort: 80
          hostPort: 8091
          protocol: TCP
        - containerPort: 6379
          hostPort: 6380
          protocol: TCP
        - containerPort: 53
          hostPort: 5353
          protocol: UDP
`;
		const parsed = parseK8sPodPorts(k8sYaml);
		expect(parsed).toHaveLength(3);
		expect(parsed[0]).toEqual({
			id: 'port-0.0.0.0-8091-80-tcp',
			hostPort: 8091,
			podPort: 80,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
		expect(parsed[1]).toEqual({
			id: 'port-0.0.0.0-6380-6379-tcp',
			hostPort: 6380,
			podPort: 6379,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
		expect(parsed[2]).toEqual({
			id: 'port-0.0.0.0-5353-53-udp',
			hostPort: 5353,
			podPort: 53,
			protocol: 'udp',
			hostIp: '0.0.0.0'
		});
	});

	it('serializePodK8sYaml correctly serializes multiple member containers with specific ports', () => {
		const ports: PortBindingItem[] = [
			{ id: 'p1', hostPort: 8091, podPort: 80, protocol: 'tcp', hostIp: '0.0.0.0' },
			{ id: 'p2', hostPort: 6380, podPort: 6379, protocol: 'tcp', hostIp: '0.0.0.0' }
		];
		const members = [
			{ name: 'web', image: 'docker.io/library/nginx:alpine', port: 80 },
			{ name: 'cache', image: 'docker.io/library/redis:alpine', port: 6379 }
		];

		const k8s = serializePodK8sYaml('demo-pod', ports, members);
		expect(k8s).toContain('apiVersion: v1');
		expect(k8s).toContain('kind: Pod');
		expect(k8s).toContain('name: demo-pod');
		expect(k8s).toContain('- name: web');
		expect(k8s).toContain('image: docker.io/library/nginx:alpine');
		expect(k8s).toContain('containerPort: 80');
		expect(k8s).toContain('hostPort: 8091');
		expect(k8s).toContain('- name: cache');
		expect(k8s).toContain('image: docker.io/library/redis:alpine');
		expect(k8s).toContain('containerPort: 6379');
		expect(k8s).toContain('hostPort: 6380');
	});

	it('parseDetailedPorts handles host IP binding prefix and plain port numbers', () => {
		const yaml = `version: "3.8"
services:
  app:
    ports:
      - "0.0.0.0:8080:80"
      - "127.0.0.1:3000:3000"
      - 5000
`;
		const result = parseDetailedPorts(yaml);
		expect(result).toHaveLength(3);
		expect(result[0]).toEqual({
			id: 'port-0.0.0.0-8080-80-tcp',
			hostPort: 8080,
			podPort: 80,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
		expect(result[1]).toEqual({
			id: 'port-127.0.0.1-3000-3000-tcp',
			hostPort: 3000,
			podPort: 3000,
			protocol: 'tcp',
			hostIp: '127.0.0.1'
		});
		expect(result[2]).toEqual({
			id: 'port-0.0.0.0-5000-5000-tcp',
			hostPort: 5000,
			podPort: 5000,
			protocol: 'tcp',
			hostIp: '0.0.0.0'
		});
	});

	it('parseK8sPodPorts handles invalid yaml and empty specs gracefully', () => {
		expect(parseK8sPodPorts('')).toEqual([]);
		expect(parseK8sPodPorts('invalid: yaml: [')).toEqual([]);
		expect(parseK8sPodPorts('apiVersion: v1\nkind: Pod\nspec:\n  containers: []')).toEqual([]);
		expect(parseK8sPodPorts('apiVersion: v1\nkind: Pod')).toEqual([]);
	});

	it('parseK8sPodPorts handles containers without hostPort defaulting to containerPort', () => {
		const k8sYaml = `apiVersion: v1
kind: Pod
metadata:
  name: demo-pod
spec:
  containers:
    - name: app
      ports:
        - containerPort: 8080
`;
		const parsed = parseK8sPodPorts(k8sYaml);
		expect(parsed).toHaveLength(1);
		expect(parsed[0].podPort).toBe(8080);
		expect(parsed[0].hostPort).toBe(8080);
		expect(parsed[0].protocol).toBe('tcp');
		expect(parsed[0].hostIp).toBe('0.0.0.0');
	});

	it('serializePodK8sYaml cleans service name slug for Kubernetes metadata.name', () => {
		const ports: PortBindingItem[] = [
			{ id: 'p1', hostPort: 80, podPort: 80, protocol: 'tcp', hostIp: '0.0.0.0' }
		];
		const yaml1 = serializePodK8sYaml('pod_my-app_test', ports);
		expect(yaml1).toContain('name: my-app-test');

		const yaml2 = serializePodK8sYaml('My Special Service! #1', ports);
		expect(yaml2).toContain('name: my-special-service-1');
	});

	it('parseDetailedPorts ignores nested member ports blocks and keeps stable ids', () => {
		const yaml = `version: "3.8"
services:
  podsvc:
    ports:
      - "8090:80"
  member:
    build:
      ports:
        - "9999:9999"
`;
		const first = parseDetailedPorts(yaml);
		const second = parseDetailedPorts(yaml);
		expect(first.map((p) => p.id)).toEqual(second.map((p) => p.id));
		expect(first.map((p) => p.hostPort).sort()).toEqual([8090, 9999]);
	});

	it('parseDetailedPorts/serializePodComposeYaml round-trip per-binding host IPs', () => {
		const yaml = `version: "3.8"
services:
  demo:
    ports:
      - "18401:80"
      - "127.0.0.1:18402:5230"
      - "18401:81/udp"
      - "127.0.0.1:18402:5231/udp"
`;
		const parsed = parseDetailedPorts(yaml);
		expect(parsed).toHaveLength(4);
		expect(parsed.map((p) => p.id)).toEqual([
			'port-0.0.0.0-18401-80-tcp',
			'port-127.0.0.1-18402-5230-tcp',
			'port-0.0.0.0-18401-81-udp',
			'port-127.0.0.1-18402-5231-udp'
		]);

		const reserialized = serializePodComposeYaml('demo', parsed, yaml);
		expect(reserialized).toContain('- "18401:80"');
		expect(reserialized).toContain('- "127.0.0.1:18402:5230"');
		expect(reserialized).toContain('- "18401:81/udp"');
		expect(reserialized).toContain('- "127.0.0.1:18402:5231/udp"');

		// Round-trip is stable: re-parsing yields identical ids
		expect(parseDetailedPorts(reserialized).map((p) => p.id)).toEqual(
			parsed.map((p) => p.id)
		);
	});

	it('same host/pod ports on different bind addresses are distinct bindings', () => {
		const yaml = `version: "3.8"
services:
  demo:
    ports:
      - "0.0.0.0:8080:80"
      - "127.0.0.1:8080:80"
`;
		const parsed = parseDetailedPorts(yaml);
		expect(parsed).toHaveLength(2);
		expect(parsed[0].id).toBe('port-0.0.0.0-8080-80-tcp');
		expect(parsed[1].id).toBe('port-127.0.0.1-8080-80-tcp');
		expect(parsed[0].id).not.toBe(parsed[1].id);
		expect(normalizePortKey(parsed[0])).toBe('0.0.0.0:8080:80');
		expect(normalizePortKey(parsed[1])).toBe('127.0.0.1:8080:80');
	});

	it('portBindingId and normalizePortKey include the bind address', () => {
		expect(portBindingId(18401, 80, 'tcp')).toBe('port-0.0.0.0-18401-80-tcp');
		expect(portBindingId(18402, 5230, 'tcp', '127.0.0.1')).toBe(
			'port-127.0.0.1-18402-5230-tcp'
		);
		expect(portBindingId(18402, 5230, 'udp', '127.0.0.1')).toBe(
			'port-127.0.0.1-18402-5230-udp'
		);
		const udpLoopback: PortBindingItem = {
			id: 'x',
			hostPort: 18402,
			podPort: 5230,
			protocol: 'udp',
			hostIp: '127.0.0.1'
		};
		expect(normalizePortKey(udpLoopback)).toBe('127.0.0.1:18402:5230/udp');
	});
});
