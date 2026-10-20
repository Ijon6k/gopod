import { describe, expect, test } from 'bun:test';
import { computeNetRate, type NetSample } from './net-rate';

const sample = (totalBytes: number, atMs: number): NetSample => ({ totalBytes, atMs });

describe('computeNetRate', () => {
	test('returns null for the first sample (no predecessor to differ against)', () => {
		expect(computeNetRate(null, sample(1024, 1_000))).toBeNull();
	});

	test('derives bytes per second from a monotonic counter increase', () => {
		// 1500 bytes in 0.5 s → 3000 B/s
		expect(computeNetRate(sample(1_500, 1_000), sample(3_000, 1_500))).toBe(3_000);
	});

	test('honors long gaps between samples', () => {
		// 6000 extra bytes over 3 s → 2000 B/s
		expect(computeNetRate(sample(1_000, 0), sample(7_000, 3_000))).toBe(2_000);
	});

	test('returns zero when the cumulative counter resets', () => {
		expect(computeNetRate(sample(5_000, 2_000), sample(3_000, 4_000))).toBe(0);
	});

	test('returns zero when no bytes moved', () => {
		expect(computeNetRate(sample(5_000, 2_000), sample(5_000, 4_000))).toBe(0);
	});

	test('returns null when the sample time does not move forward', () => {
		expect(computeNetRate(sample(1_000, 1_000), sample(1_000, 1_000))).toBeNull();
		expect(computeNetRate(sample(1_000, 1_000), sample(1_000, 999))).toBeNull();
	});

	test('returns null for non-finite counter values', () => {
		expect(computeNetRate(sample(Number.NaN, 1_000), sample(2_000, 2_000))).toBeNull();
		expect(
			computeNetRate(sample(1_000, 1_000), sample(Number.POSITIVE_INFINITY, 2_000))
		).toBeNull();
	});
});
