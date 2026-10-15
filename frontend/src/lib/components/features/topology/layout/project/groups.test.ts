import { describe, it, expect } from 'bun:test';
import {
	buildPodGroups,
	buildServiceGroups,
	buildUnmanagedGroup,
	type GroupBuilderContext
} from './groups';

function makeCtx(): GroupBuilderContext {
	return {
		projectId: 'p1',
		nodes: [],
		containerIdMap: new Map(),
		serviceToFirstContainerMap: new Map(),
		serviceGroupMap: new Map(),
		serviceYMap: new Map(),
		currentWorkloadY: 60,
		groupWidth: 360,
		childWidth: 328,
		childHeight: 62,
		childGap: 10,
		groupPaddingTop: 50,
		groupPaddingBottom: 16,
		colWorkloadsX: 620
	};
}

const podSvc: any = {
	id: 'p1-core-abc123',
	name: 'core',
	type: 'pod',
	status: 'running',
	projectId: 'p1',
	podId: 'pod_p1-core-abc123',
	port: 80
};
const memberSvc: any = {
	id: 'p1-api-def456',
	name: 'api',
	type: 'image',
	status: 'running',
	projectId: 'p1',
	podId: 'pod_p1-core-abc123',
	inPod: true,
	port: 80
};
const standaloneSvc: any = {
	id: 'p1-worker-789000',
	name: 'worker',
	type: 'image',
	status: 'running',
	projectId: 'p1',
	image: 'docker.io/library/nginx:alpine',
	port: 8080
};

function container(id: string, name: string, labels: any = {}) {
	return {
		id,
		name,
		projectId: 'p1',
		serviceId: '',
		image: 'img:latest',
		status: 'running' as const,
		cpu: 0,
		memory: 0,
		ports: '',
		labels
	};
}

describe('topology grouping (no phantom members, no swallowed services)', () => {
	it('renders one pod group; member services get no own group; later services still render', () => {
		const ctx = makeCtx();
		const services = [podSvc, memberSvc, standaloneSvc];
		const ids = new Set(services.map((s) => s.id));
		const infra = container('c-infra', 'xxx-infra', { 'io.gopod.service': podSvc.id });
		const api = container('c-api', 'p1-api-def456', { 'io.gopod.service': memberSvc.id });
		const work = container('c-work', 'p1-worker-789000', { 'io.gopod.service': standaloneSvc.id });
		const projContainers = [infra, api, work];
		const assigned = new Set<string>();
		const assignment = new Map<string, any>();
		// infra + api live in the runtime pod; worker standalone
		assignment.set('c-infra', { kind: 'pod', podName: 'pod_p1-core-abc123', podServiceId: podSvc.id });
		assignment.set('c-api', { kind: 'pod', podName: 'pod_p1-core-abc123', podServiceId: podSvc.id });

		const rtPods = [{ name: 'pod_p1-core-abc123', status: 'Running' }];
		buildPodGroups(ctx, {
			podServices: [podSvc],
			allPods: rtPods,
			projContainers,
			projServices: services,
			projServiceIds: ids,
			containerAssignment: assignment,
			assignedContainerIds: assigned
		});
		buildServiceGroups(ctx, {
			projServices: services,
			projContainers,
			projServiceIds: ids,
			assignedContainerIds: assigned
		});
		buildUnmanagedGroup(ctx, projContainers, assigned);

		const groups = ctx.nodes.filter((n) => n.isGroup);
		const titles = groups.map((g) => g.title);
		// exactly one pod group containing infra + member, plus standalone worker
		expect(titles.filter((t) => t === 'pod_p1-core-abc123')).toHaveLength(1);
		const podGroup = groups.find((g) => g.title === 'pod_p1-core-abc123')!;
		const kids = ctx.nodes.filter((n) => n.parentId === podGroup.id).map((n) => n.title);
		expect(kids).toContain('xxx-infra');
		expect(kids).toContain('p1-api-def456');
		expect(kids).not.toContain('p1-worker-789000');
		// member service renders NO own group (regression: return-in-loop swallowed the rest)
		expect(titles).not.toContain('api');
		// standalone service AFTER the pod still renders (the actual bug)
		expect(titles).toContain('worker');
		// nothing unmanaged
		expect(titles).not.toContain('Unmanaged');
	});
});
