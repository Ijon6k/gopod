// ──────────────────────────────────────────────
// GOPOD — Timeseries helpers
// Converts store TimeSeriesPoint rows into the [timestampMs, value] pairs
// the kumo TimeseriesChart component consumes. Drops non-parsable samples
// instead of inventing timestamps — charts only ever draw real samples.
// ──────────────────────────────────────────────
import type { TimeSeriesPoint } from '$lib/types';

export type TimeseriesTuple = [number, number];

export function toTimeseriesTuples(points: TimeSeriesPoint[] | undefined | null): TimeseriesTuple[] {
	if (!points?.length) return [];
	const out: TimeseriesTuple[] = [];
	for (const point of points) {
		const ts = Date.parse(point.timestamp ?? point.time ?? '');
		if (!Number.isNaN(ts)) out.push([ts, point.value]);
	}
	return out;
}

/** Shared axis tick formatter: HH:MM in the viewer's timezone. */
export function timeseriesTimeTick(value: number): string {
	const d = new Date(value);
	return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`;
}
