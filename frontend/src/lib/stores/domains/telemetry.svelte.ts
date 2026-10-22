import type { Server, CaddyAccessLog, TimeSeriesPoint } from '$lib/types';
import { api } from '$lib/api';
import { computeNetRate, type NetSample } from '$lib/utils/net-rate';

export class TelemetryDomainStore {
	server = $state<Server>({
		hostname: 'localhost',
		ip: '127.0.0.1',
		os: 'Linux',
		kernel: '—',
		vcpu: 4,
		memory: 16,
		storage: 100,
		storageUsed: 20,
		memoryUsed: 2,
		cpuUsage: 5,
		status: 'online',
		podmanVersion: '',
		engine: '',
		engineVersion: '',
		rootless: true,
		systemd: true,
		uptime: ''
	});

	accessLogs = $state<CaddyAccessLog[]>([]);
	isStreaming = $state<boolean>(false);
	private stopStream: (() => void) | null = null;

	// Live reactive streaming telemetry buffers, session-scoped and empty until
	// real samples arrive
	cpuHistory = $state<TimeSeriesPoint[]>([]);
	memoryHistory = $state<TimeSeriesPoint[]>([]);
	storageHistory = $state<TimeSeriesPoint[]>([]);
	networkHistory = $state<TimeSeriesPoint[]>([]);

	// Most recent measured aggregate throughput across all containers, bytes per second
	lastNetworkRateBps = $state(0);
	private prevNetSample: NetSample | null = null;
	// Pending SSE restart scheduled by the backoff handler; cancelled by stop
	private sseRestartTimer: ReturnType<typeof setTimeout> | null = null;

	// Session-scoped rolling buffers. 240 samples at the ~2s stream cadence
	// span roughly the last 8 minutes — long enough to read a trend, short
	// enough that every pixel is recent data.
	// Wall time of the last accepted system sample. The watchdog compares it
	// against the clock to detect a wedged or silently-dead SSE stream and
	// falls back to REST polling so charts keep breathing.
	private lastSystemAtMs = 0;
	private watchdog: ReturnType<typeof setInterval> | null = null;
	private watchdogBusy = false;

	private recordPoint(series: TimeSeriesPoint[], val: number, timeStr: string, labelStr: string) {
		series.push({ time: timeStr, label: labelStr, value: Math.max(0, val) });
		if (series.length > 240) {
			series.shift();
		}
	}

	startStreamingStats(onStats?: (stats: any[]) => void) {
		if (typeof window === 'undefined') return;
		if (this.stopStream) return;

		// EventSource fires onerror for every network hiccup while it keeps
		// auto-reconnecting; re-fetching inside that handler without closing
		// caused overlapping refetch + reconnect storms. Close on error and
		// restart with backoff instead, each restart fully re-baselines.
		let restartAttempt = 0;
		const open = () => {
			this.isStreaming = true;
			this.stopStream = api.system.streamStats(
				(data) => {
					restartAttempt = 0;
					if (data.system) {
						this.applySystemUpdate(data.system);
					}
					if (data.stats && Array.isArray(data.stats)) {
						// Podman network counters accumulate over the container lifetime;
						// ingest time derives a per-second throughput so charts plot
						// activity instead of an ever-climbing uptime total.
						let netTotalBytes = 0;
						for (const s of data.stats) {
							netTotalBytes += (s.netRx || 0) + (s.netTx || 0);
						}
						const now = new Date();
						const nowMs = now.getTime();
						const rate = computeNetRate(this.prevNetSample, {
							totalBytes: netTotalBytes,
							atMs: nowMs
						});
						this.prevNetSample = { totalBytes: netTotalBytes, atMs: nowMs };
						if (rate !== null) {
							this.lastNetworkRateBps = rate;
							this.recordPoint(
								this.networkHistory,
								rate,
								now.toISOString(),
								`${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`
							);
						}

						if (onStats) {
							onStats(data.stats);
						}
					}
				},
				() => {
					// The transport dropped or errored. Poll once as fallback so the
					// UI keeps a fresh sample, then reconnect with capped backoff.
					restartAttempt = Math.min(restartAttempt + 1, 5);
					this.stopStreamingStats();
					this.fetchLiveStats();
					const delay = Math.min(1000 * 2 ** restartAttempt, 15_000);
					this.sseRestartTimer = setTimeout(() => {
						this.sseRestartTimer = null;
						if (!this.isStreaming) open();
					}, delay);
				}
			);
		};

		this.startWatchdog();

		try {
			open();
		} catch {
			this.fetchLiveStats();
		}
	}

	// Every 3s: if no system sample arrived through the stream for 8s
	// (wedged SSE, dropped connection without error), poll the REST endpoint
	// so the rolling buffers never go blind.
	private startWatchdog() {
		this.stopWatchdog();
		this.watchdog = setInterval(() => {
			if (Date.now() - this.lastSystemAtMs > 8000 && !this.watchdogBusy) {
				this.watchdogBusy = true;
				this.fetchLiveStats().finally(() => {
					this.watchdogBusy = false;
				});
			}
		}, 3000);
	}

	private stopWatchdog() {
		if (this.watchdog) {
			clearInterval(this.watchdog);
			this.watchdog = null;
		}
	}

	stopStreamingStats() {
		this.stopWatchdog();
		if (this.sseRestartTimer) {
			clearTimeout(this.sseRestartTimer);
			this.sseRestartTimer = null;
		}
		if (this.stopStream) {
			this.stopStream();
			this.stopStream = null;
			this.isStreaming = false;
			// Re-baseline the differ so a later restart derives rates from the next
			// fresh sample pair instead of a long stale interval
			this.prevNetSample = null;
		}
	}

	applySystemUpdate(sys: any) {
		// Field-level writes instead of object replacement: every 2s SSE tick
		// must not swap the shared `server` object identity, otherwise every
		// consumer of any `server.*` field re-derives on every tick.
		const s = this.server;
		this.lastSystemAtMs = Date.now();
		s.ip = sys.ip || s.ip;
		s.hostname = sys.hostname || s.hostname;
		s.os = sys.os || s.os;
		s.kernel = sys.kernel || s.kernel;
		s.vcpu = sys.vcpu || s.vcpu;
		s.memory = parseFloat(sys.memoryTotalGB?.toFixed(1)) || s.memory;
		s.memoryUsed =
			sys.memoryUsedGB != null ? parseFloat(sys.memoryUsedGB.toFixed(1)) : s.memoryUsed;
		s.memoryAvailable =
			sys.memoryAvailableGB != null
				? parseFloat(sys.memoryAvailableGB.toFixed(1))
				: s.memory - s.memoryUsed;
		s.swapUsed = sys.swapUsedMB != null ? sys.swapUsedMB : s.swapUsed;
		s.swapTotal = sys.swapTotalGB != null ? sys.swapTotalGB : s.swapTotal;
		// A real 0.0% must overwrite the previous sample — the former `||`
		// treated zero as falsy and froze CPU at its last non-zero reading.
		s.cpuUsage = sys.cpuUsage != null ? parseFloat(Number(sys.cpuUsage).toFixed(1)) : s.cpuUsage;
		s.podmanVersion = sys.podmanVersion || s.podmanVersion;
		// The engine identity only changes on real host changes; assign through
		// so a Docker-only host reports "Docker" even when podmanVersion missed.
		if (sys.engine !== undefined) s.engine = sys.engine || s.engine;
		if (sys.engineVersion !== undefined) s.engineVersion = sys.engineVersion || s.engineVersion;
		s.rootless = sys.rootless ?? s.rootless;
		s.uptime = sys.uptime || s.uptime;
		s.status = 'online';

		const now = new Date();
		const timeStr = now.toISOString();
		const labelStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`;
		this.recordPoint(this.cpuHistory, this.server.cpuUsage, timeStr, labelStr);
		this.recordPoint(this.memoryHistory, this.server.memoryUsed, timeStr, labelStr);
		this.recordPoint(this.storageHistory, this.server.storageUsed, timeStr, labelStr);
	}

	async fetchLiveStats() {
		try {
			const [sys, stats] = await Promise.all([
				api.system.info().catch(() => null),
				api.system.stats().catch(() => null)
			]);
			if (sys) {
				this.applySystemUpdate(sys);
			}
			return stats;
		} catch {
			return null;
		}
	}

	async fetchTrafficLogs(limit = 50) {
		try {
			const logs = await api.traffic.getRequests(limit).catch(() => []);
			if (Array.isArray(logs) && logs.length > 0) {
				this.accessLogs = logs;
			}
		} catch (err) {
			console.warn('[TelemetryDomainStore] Failed to fetch traffic logs:', err);
		}
	}
}
