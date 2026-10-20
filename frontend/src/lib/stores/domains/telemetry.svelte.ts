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
		podmanVersion: '5.x',
		rootless: true,
		systemd: true,
		quadlet: true,
		caddy: '2.x',
		podmanHealth: 'healthy',
		networkHealth: 'healthy',
		storageHealth: 'healthy',
		uptime: 'Active'
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

	get monitoringData(): Record<string, TimeSeriesPoint[]> {
		return {
			cpu: this.cpuHistory,
			memory: this.memoryHistory,
			storage: this.storageHistory,
			network: this.networkHistory
		};
	}

	private recordPoint(series: TimeSeriesPoint[], val: number, timeStr: string, labelStr: string) {
		series.push({ time: timeStr, label: labelStr, value: Math.max(0, val) });
		if (series.length > 30) {
			series.shift();
		}
	}

	startStreamingStats(onStats?: (stats: any[]) => void) {
		if (typeof window === 'undefined') return;
		if (this.stopStream) return;

		try {
			this.isStreaming = true;
			this.stopStream = api.system.streamStats(
				(data) => {
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
					// Fallback to polled stats if stream drops
					this.fetchLiveStats();
				}
			);
		} catch {
			this.fetchLiveStats();
		}
	}

	stopStreamingStats() {
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
		this.server = {
			...this.server,
			ip: sys.ip || this.server.ip,
			hostname: sys.hostname || this.server.hostname,
			os: sys.os || this.server.os,
			kernel: sys.kernel || this.server.kernel,
			vcpu: sys.vcpu || this.server.vcpu,
			memory: parseFloat(sys.memoryTotalGB?.toFixed(1)) || this.server.memory,
			memoryUsed: parseFloat(sys.memoryUsedGB?.toFixed(1)) || this.server.memoryUsed,
			memoryAvailable:
				sys.memoryAvailableGB != null
					? parseFloat(sys.memoryAvailableGB.toFixed(1))
					: this.server.memory - this.server.memoryUsed,
			swapUsed: sys.swapUsedMB != null ? sys.swapUsedMB : this.server.swapUsed,
			swapTotal: sys.swapTotalGB != null ? sys.swapTotalGB : this.server.swapTotal,
			cpuUsage: parseFloat(sys.cpuUsage?.toFixed(1)) || this.server.cpuUsage,
			podmanVersion: sys.podmanVersion || this.server.podmanVersion,
			rootless: sys.rootless ?? this.server.rootless,
			uptime: sys.uptime || this.server.uptime,
			status: 'online'
		};

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
