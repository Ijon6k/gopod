import { describe, it, expect } from 'bun:test';
import { canonicalPodNameFor, podServiceNames, slugLower } from './pod-naming';

describe('topology canonical pod naming (must mirror backend PodNameForService)', () => {
	it('derives unique names per project+service, stable across calls', () => {
		const a = canonicalPodNameFor({ id: 'self-hosted-core-b2fb14', name: 'core' });
		const b = canonicalPodNameFor({ id: 'other-core-9f8e7d', name: 'core' });
		expect(a).toBe('pod_self-hosted-core-b2fb14');
		expect(a).not.toBe(b);
		expect(canonicalPodNameFor({ id: 'self-hosted-core-b2fb14', name: 'core' })).toBe(a);
	});

	it('falls back to name slug without id', () => {
		expect(canonicalPodNameFor({ name: 'My Pod' })).toBe('pod_my-pod');
	});

	it('truncates slugs longer than 48 chars like the backend', () => {
		const long = canonicalPodNameFor({
			id: 'unified-pod-sync-32111984-e2e-sync-32111984-68c8extra-long-suffix-here'
		});
		expect(long.length).toBeLessThanOrEqual(52); // pod_ + 48
		expect(long.startsWith('pod_')).toBeTrue();
	});

	it('lists legacy name forms after the canonical one', () => {
		const names = podServiceNames({
			id: 'self-hosted-core-b2fb14',
			name: 'core',
			podId: 'pod_self-hosted-core-b2fb14'
		});
		expect(names[0]).toBe('pod_self-hosted-core-b2fb14');
		expect(names).toContain('pod_core');
		expect(slugLower('pod_WebLab')).toBe('weblab');
	});
});
