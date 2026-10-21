/**
 * Interval poller with two guards every background fetch loop in the app
 * needs: an in-flight latch (a slow response never overlaps the next tick)
 * and an optional pause while the browser tab is hidden (data watchers keep
 * running otherwise and pile up work for the user's return).
 *
 * The returned handle is a plain object; hold it in a component-scoped
 * variable and call `stop()` in the destroy path (onMount cleanup or $effect
 * teardown).
 */
export interface PollerOptions {
	/** Skip ticks while `document.hidden`; existing interval resumes on show. */
	pauseWhenHidden?: boolean;
	/** Run one tick immediately on start (default false). */
	leading?: boolean;
}

export interface Poller {
	start(): void;
	stop(): void;
	/** Kick a single guarded tick without waiting for the next interval. */
	refresh(): void;
	readonly running: boolean;
}

export function createPoller(
	intervalMs: number,
	fn: () => void | Promise<void>,
	opts: PollerOptions = {}
): Poller {
	let timer: ReturnType<typeof setInterval> | null = null;
	let inFlight = false;
	let started = false;
	const tick = async () => {
		if (opts.pauseWhenHidden && typeof document !== 'undefined' && document.hidden) return;
		if (inFlight) return;
		inFlight = true;
		try {
			await fn();
		} finally {
			inFlight = false;
		}
	};
	return {
		start() {
			if (started) return;
			started = true;
			if (opts.leading) void tick();
			timer = setInterval(() => void tick(), intervalMs);
		},
		stop() {
			if (timer) clearInterval(timer);
			timer = null;
			started = false;
		},
		refresh() {
			void tick();
		},
		get running() {
			return started;
		}
	};
}
