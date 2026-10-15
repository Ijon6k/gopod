import { describe, it, expect } from 'bun:test';
import { isPodMember, findPodMembers } from './pod-membership';

const freshPod = { id: 'p1-kanban-0ea220', name: 'kanban', podId: '', type: 'pod', projectId: 'p1' };
const deployedPod = {
	id: 'p1-core-b2fb14',
	name: 'core',
	podId: 'pod_p1-core-b2fb14',
	type: 'pod',
	projectId: 'p1'
};
const standalone = (id: string, name: string, extra = {}) => ({
	id,
	name,
	podId: '',
	type: 'image',
	projectId: 'p1',
	...extra
});

describe('pod membership matching', () => {
	it('fresh pod (empty podId) claims NOTHING, even standalone services with empty podId', () => {
		const services = [
			standalone('p1-worker-1', 'worker'),
			standalone('p1-qweb-2', 'qweb'),
			standalone('p1-landing-3', 'landing page')
		];
		expect(findPodMembers(freshPod, services)).toEqual([]);
		for (const s of services) {
			expect(isPodMember(freshPod, s)).toBeFalse();
		}
	});

	it('matches canonical, service-id, legacy name and slug podId forms', () => {
		const mk = (podId: string) => standalone('m1', 'web', { podId });
		expect(isPodMember(deployedPod, mk('pod_p1-core-b2fb14'))).toBeTrue();
		expect(isPodMember(deployedPod, mk('p1-core-b2fb14'))).toBeTrue();
		expect(isPodMember(deployedPod, mk('core'))).toBeTrue();
		expect(isPodMember(deployedPod, mk('pod_core'))).toBeTrue();
	});

	it('excludes other-pod members, self, pod/compose types, other projects', () => {
		expect(
			isPodMember(deployedPod, standalone('m2', 'x', { podId: 'pod_p1-other-999' }))
		).toBeFalse();
		expect(isPodMember(deployedPod, { ...deployedPod })).toBeFalse();
		expect(
			isPodMember(deployedPod, standalone('m3', 'y', { podId: 'pod_p1-core-b2fb14', type: 'pod' }))
		).toBeFalse();
		expect(
			isPodMember(deployedPod, standalone('m4', 'z', { podId: 'pod_p1-core-b2fb14', type: 'compose' }))
		).toBeFalse();
		expect(
			isPodMember(deployedPod, standalone('m5', 'w', { podId: 'pod_p1-core-b2fb14', projectId: 'p2' }))
		).toBeFalse();
		expect(isPodMember(deployedPod, standalone('m6', 'v', { podId: '   ' }))).toBeFalse();
	});
});
