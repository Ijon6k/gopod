/**
 * Derive aggregate network throughput from consecutive cumulative container counters.
 *
 * Podman network byte counters accumulate over the whole container lifetime, so a
 * plotted quantity must be the difference between two samples, not the counter
 * itself. A sample without a predecessor yields no rate. A counter reset or a
 * shrinking sample set (a container left the aggregate) cannot represent traffic
 * that arrived during the interval, so the rate is zero. An interval that does not
 * move forward in time cannot anchor a rate.
 */

export interface NetSample {
	/** Sum of all Rx + Tx byte counters in the sampled set. */
	totalBytes: number;
	/** Wall-clock sample time in milliseconds. */
	atMs: number;
}

const MS_PER_SECOND = 1000;

/**
 * Compute the throughput in bytes per second between `prev` and `cur`.
 * Returns null when no honest rate exists (first sample, invalid values, or a
 * non-positive interval), zero on counter resets and shrinking aggregates, and
 * otherwise the measured delta of bytes per elapsed second.
 */
export function computeNetRate(prev: NetSample | null, cur: NetSample): number | null {
	if (prev === null) return null;
	if (
		!Number.isFinite(prev.totalBytes) ||
		!Number.isFinite(cur.totalBytes) ||
		!Number.isFinite(prev.atMs) ||
		!Number.isFinite(cur.atMs)
	) {
		return null;
	}

	const dtSeconds = (cur.atMs - prev.atMs) / MS_PER_SECOND;
	if (dtSeconds <= 0) return null;

	const delta = cur.totalBytes - prev.totalBytes;
	if (delta <= 0) return 0;
	return delta / dtSeconds;
}
