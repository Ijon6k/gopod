import type { Server, CaddyAccessLog, TimeSeriesPoint } from '$lib/types';
import { api } from '$lib/api';

function initSeries(baseVal: number, count = 20): TimeSeriesPoint[] {
	const now = Date.now();
	return Array.from({ length: count }, (_, i) => {
		const t = new Date(now - (count - 1 - i) * 2000);
		return {
			time: t.toISOString(),
			label: `${t.getHours().toString().padStart(2, '0')}:${t.getMinutes().toString().padStart(2, '0')}:${t.getSeconds().toString().padStart(2, '0')}`,
			value: baseVal
		};
	});
}

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
	private sseSource: EventSource | null = null;

	// Live reactive streaming telemetry buffers
	cpuHistory = $state<TimeSeriesPoint[]>(initSeries(5));
	memoryHistory = $state<TimeSeriesPoint[]>(initSeries(2));
	storageHistory = $state<TimeSeriesPoint[]>(initSeries(20));
	networkHistory = $state<TimeSeriesPoint[]>(initSeries(10));

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
		if (this.sseSource) return;

		try {
			this.sseSource = new EventSource('/api/stats/stream');
			this.isStreaming = true;

			this.sseSource.onmessage = (event) => {
				try {
					const data = JSON.parse(event.data);
					if (data.system) {
						this.applySystemUpdate(data.system);
					}
					if (data.stats && Array.isArray(data.stats)) {
						let netTotal = 0;
						for (const s of data.stats) {
							if (s.Network) {
								for (const iface in s.Network) {
									netTotal += (s.Network[iface].RxBytes || 0) + (s.Network[iface].TxBytes || 0);
								}
							}
						}
						const netMB = parseFloat((netTotal / (1024 * 1024)).toFixed(2));
						const now = new Date();
						const timeStr = now.toISOString();
						const labelStr = `${now.getHours().toString().padStart(2, '0')}:${now.getMinutes().toString().padStart(2, '0')}:${now.getSeconds().toString().padStart(2, '0')}`;
						this.recordPoint(this.networkHistory, netMB, timeStr, labelStr);

						if (onStats) {
							onStats(data.stats);
						}
					}
				} catch {
					// Ignore parse error
				}
			};

			this.sseSource.onerror = () => {
				// Reconnects automatically
			};
		} catch {
			this.fetchLiveStats();
		}
	}

	stopStreamingStats() {
		if (this.sseSource) {
			this.sseSource.close();
			this.sseSource = null;
			this.isStreaming = false;
		}
	}

	applySystemUpdate(sys: any) {
		this.server = {
			...this.server,
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
