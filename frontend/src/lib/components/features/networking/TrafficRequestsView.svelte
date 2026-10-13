<script lang="ts">
	import { untrack } from 'svelte';
	import * as echarts from 'echarts';
	import { dataStore } from '$lib/data';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { api } from '$lib/api';
	import { Button, Select, SearchInput, Switch, Chart, TablePagination } from '$lib/components/ui';
	import {
		LayerCard,
		Badge,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from 'kumo-svelte';
	import type { CaddyAccessLog } from '$lib/types';
	import {
		Pulse,
		ArrowsClockwise,
		SlidersHorizontal,
		Copy,
		Check,
		Globe,
		CaretLeft,
		CaretRight
	} from 'phosphor-svelte';

	// Filter and view state
	let searchQuery = $state('');
	let selectedHost = $state('all');
	let selectedProjectId = $state('all');
	let selectedServiceId = $state('all');
	let selectedStatusGroup = $state('all');
	let selectedMethod = $state('all');
	let timeRange = $state<'1h' | '24h' | '7d'>('24h');
	let excludeDashboard = $state(true);
	let isRetentionModalOpen = $state(false);
	let retentionDays = $state(7);
	let isLivePolling = $state(true);
	let isFetching = $state(false);
	let copiedId = $state<string | null>(null);

	// Pagination state
	let currentPage = $state(1);
	let pageSize = $state(25);

	// Real access logs fetched from Caddy reverse proxy API
	let liveLogs = $state<CaddyAccessLog[]>([]);

	// Initial fetch & live polling
	$effect(() => {
		let active = true;

		async function fetchLogs() {
			try {
				isFetching = true;
				const fetched = await api.traffic.getRequests({
					limit: 500,
					projectId: selectedProjectId !== 'all' ? selectedProjectId : undefined,
					serviceId: selectedServiceId !== 'all' ? selectedServiceId : undefined,
					status: selectedStatusGroup !== 'all' ? selectedStatusGroup : undefined,
					timeRange: timeRange !== '24h' ? timeRange : undefined,
					search: searchQuery.trim() || undefined,
					excludeDashboard
				});
				if (active && Array.isArray(fetched)) {
					liveLogs = fetched;
				}
			} catch (err) {
				console.warn('[TrafficRequestsView] Access log fetch error:', err);
			} finally {
				if (active) isFetching = false;
			}
		}

		untrack(() => {
			fetchLogs();
		});

		const interval = setInterval(() => {
			if (isLivePolling) {
				fetchLogs();
			}
		}, 3000);

		return () => {
			active = false;
			clearInterval(interval);
		};
	});

	// Reset page when filters change
	$effect(() => {
		if (searchQuery || selectedHost || selectedStatusGroup || selectedMethod || selectedProjectId || selectedServiceId || excludeDashboard) {
			currentPage = 1;
		}
	});

	// Unique hosts for dropdown
	let uniqueHosts = $derived(['all', ...Array.from(new Set(liveLogs.map((l) => l.host).filter(Boolean)))]);

	// Filtered logs
	let filteredLogs = $derived.by(() => {
		return liveLogs.filter((log) => {
			if (selectedHost !== 'all' && log.host !== selectedHost) return false;
			if (selectedMethod !== 'all' && log.method !== selectedMethod) return false;
			if (selectedProjectId !== 'all' && (log as any).projectId && (log as any).projectId !== selectedProjectId) return false;
			if (selectedServiceId !== 'all' && log.serviceId && log.serviceId !== selectedServiceId) return false;

			if (selectedStatusGroup !== 'all') {
				if (selectedStatusGroup === '2xx' && (log.status < 200 || log.status >= 300)) return false;
				if (selectedStatusGroup === '3xx' && (log.status < 300 || log.status >= 400)) return false;
				if (selectedStatusGroup === '4xx' && (log.status < 400 || log.status >= 500)) return false;
				if (selectedStatusGroup === '5xx' && log.status < 500) return false;
			}

			if (searchQuery.trim()) {
				const q = searchQuery.toLowerCase().trim();
				const matchUri = log.uri?.toLowerCase().includes(q);
				const matchIp = log.clientIp?.toLowerCase().includes(q);
				const matchHost = log.host?.toLowerCase().includes(q);
				if (!matchUri && !matchIp && !matchHost) return false;
			}

			return true;
		});
	});

	// Pagination calculations
	let totalPages = $derived(Math.max(1, Math.ceil(filteredLogs.length / pageSize)));
	let paginatedLogs = $derived.by(() => {
		const start = (currentPage - 1) * pageSize;
		return filteredLogs.slice(start, start + pageSize);
	});

	// Real-time telemetry metrics derived from active logs
	let stats = $derived.by(() => {
		const total = filteredLogs.length;
		const s2xx = filteredLogs.filter((l) => l.status >= 200 && l.status < 300).length;
		const s3xx = filteredLogs.filter((l) => l.status >= 300 && l.status < 400).length;
		const s4xx = filteredLogs.filter((l) => l.status >= 400 && l.status < 500).length;
		const s5xx = filteredLogs.filter((l) => l.status >= 500).length;
		const successRate = total > 0 ? (((s2xx + s3xx) / total) * 100).toFixed(1) : '100.0';
		const avgDuration =
			total > 0
				? (filteredLogs.reduce((acc, cur) => acc + (cur.durationMs || 0), 0) / total).toFixed(1)
				: '0.0';

		return {
			total,
			successRate,
			s2xx,
			s3xx,
			s4xx,
			s5xx,
			avgDuration
		};
	});

	// Generous 1/2 Page Height Timeseries Histogram & Trend Data via ECharts
	interface BucketInfo {
		index: number;
		label: string;
		total: number;
		s2xx: number;
		s3xx: number;
		s4xx: number;
		s5xx: number;
		avgLatency: number;
	}

	let bucketItems = $derived.by(() => {
		const numBuckets = 24;
		const now = Date.now();
		const windowDuration =
			timeRange === '1h'
				? 3600 * 1000
				: timeRange === '24h'
					? 24 * 3600 * 1000
					: 7 * 24 * 3600 * 1000;
		const startTime = now - windowDuration;
		const bucketDuration = windowDuration / numBuckets;

		const buckets: {
			total: number;
			s2xx: number;
			s3xx: number;
			s4xx: number;
			s5xx: number;
			totalLatency: number;
			timestamp: number;
		}[] = Array.from({ length: numBuckets }, (_, i) => ({
			total: 0,
			s2xx: 0,
			s3xx: 0,
			s4xx: 0,
			s5xx: 0,
			totalLatency: 0,
			timestamp: startTime + i * bucketDuration
		}));

		for (const log of filteredLogs) {
			const t = log.timestamp ? new Date(log.timestamp).getTime() : now;
			const bucketIdx = Math.floor((t - startTime) / bucketDuration);
			const clampedIdx = Math.min(Math.max(0, bucketIdx), numBuckets - 1);
			if (t >= startTime && t <= now + bucketDuration) {
				const b = buckets[clampedIdx];
				b.total++;
				if ((log.status >= 200 && log.status < 300) || log.status === 101) b.s2xx++;
				else if (log.status >= 300 && log.status < 400) b.s3xx++;
				else if (log.status >= 400 && log.status < 500) b.s4xx++;
				else if (log.status >= 500) b.s5xx++;
				b.totalLatency += log.durationMs || 0;
			}
		}

		return buckets.map((b, i) => {
			const date = new Date(b.timestamp);
			let label = '';
			if (timeRange === '1h') {
				label = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
			} else if (timeRange === '24h') {
				label = date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
			} else {
				label = `${date.getMonth() + 1}/${date.getDate()}`;
			}

			return {
				index: i,
				label,
				total: b.total,
				s2xx: b.s2xx,
				s3xx: b.s3xx,
				s4xx: b.s4xx,
				s5xx: b.s5xx,
				avgLatency: b.total > 0 ? parseFloat((b.totalLatency / b.total).toFixed(1)) : 0
			};
		});
	});

	let peakRequests = $derived(
		Math.max(...bucketItems.map((b) => b.total), 0)
	);

	let isDark = $derived(uiStore.theme === 'dark');
	let chartTheme = $derived({
		textColor: isDark ? '#a1a1aa' : '#52525b',
		titleColor: isDark ? '#f4f4f5' : '#18181b',
		borderColor: isDark ? '#3f3f46' : '#e4e4e7',
		splitLineColor: isDark ? '#27272a' : '#f0f1f4',
		tooltipBg: isDark ? '#18181b' : '#ffffff',
		tooltipBorder: isDark ? '#27272a' : '#e4e4e7',
		tooltipText: isDark ? '#f4f4f5' : '#18181b'
	});

	let echartOptions = $derived.by(() => {
		const labels = bucketItems.map((b) => b.label);
		const data2xx = bucketItems.map((b) => b.s2xx);
		const data3xx = bucketItems.map((b) => b.s3xx);
		const data4xx = bucketItems.map((b) => b.s4xx);
		const data5xx = bucketItems.map((b) => b.s5xx);
		const dataLatency = bucketItems.map((b) => b.avgLatency);

		return {
			animation: true,
			animationDuration: 300,
			animationDurationUpdate: 300,
			tooltip: {
				trigger: 'axis' as const,
				backgroundColor: chartTheme.tooltipBg,
				borderColor: chartTheme.tooltipBorder,
				borderWidth: 1,
				textStyle: {
					color: chartTheme.tooltipText,
					fontSize: 12
				},
				axisPointer: {
					type: 'cross' as const,
					crossStyle: {
						color: '#888'
					}
				},
				dangerousHtmlFormatter: (params: any) => {
					if (!Array.isArray(params) || params.length === 0) return '';
					const titleColor = isDark ? '#ffffff' : '#18181b';
					const subColor = isDark ? '#a1a1aa' : '#71717a';
					const title = `<div style="font-weight:600;margin-bottom:6px;color:${titleColor};">${params[0].axisValue}</div>`;
					let totalReqs = 0;
					let rows = '';
					let latencyVal = 0;

					for (const p of params) {
						if (p.seriesName === 'Latency') {
							latencyVal = p.value || 0;
						} else {
							const val = p.value || 0;
							totalReqs += val;
							if (val > 0) {
								rows += `<div style="display:flex;align-items:center;justify-content:space-between;gap:16px;margin-top:3px;">
									<span style="display:inline-flex;align-items:center;gap:6px;">
										<span style="display:inline-block;width:8px;height:8px;border-radius:2px;background:${p.color};"></span>
										<span style="color:${subColor};">${p.seriesName}</span>
									</span>
									<span style="font-weight:600;font-family:monospace;color:${titleColor};">${val}</span>
								</div>`;
							}
						}
					}

					const summary = `<div style="display:flex;justify-content:space-between;gap:16px;border-bottom:1px solid ${isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'};padding-bottom:4px;margin-bottom:4px;">
						<span style="color:${subColor};">Total Requests</span>
						<span style="font-weight:700;font-family:monospace;color:${titleColor};">${totalReqs}</span>
					</div>`;

					const latencyRow = latencyVal > 0 ? `<div style="margin-top:6px;padding-top:4px;border-top:1px solid ${isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'};display:flex;justify-content:space-between;gap:16px;color:#8b5cf6;">
						<span>Avg Latency</span>
						<span style="font-weight:600;font-family:monospace;">${latencyVal} ms</span>
					</div>` : '';

					return `<div style="min-width:160px;padding:2px 0;">${title}${summary}${rows || `<div style="color:${subColor};font-style:italic;">No traffic in bucket</div>`}${latencyRow}</div>`;
				}
			},
			legend: {
				data: ['2xx Success', '3xx Redirect', '4xx Client Err', '5xx Server Err', 'Latency'],
				textStyle: {
					color: chartTheme.textColor,
					fontSize: 11
				},
				icon: 'roundRect',
				itemWidth: 10,
				itemHeight: 10,
				top: 4,
				left: 'center'
			},
			grid: {
				top: 48,
				left: 20,
				right: 28,
				bottom: 16,
				containLabel: true
			},
			xAxis: [
				{
					type: 'category' as const,
					data: labels,
					axisPointer: {
						type: 'shadow' as const
					},
					axisLine: {
						lineStyle: {
							color: chartTheme.borderColor
						}
					},
					axisLabel: {
						color: chartTheme.textColor,
						fontSize: 11
					}
				}
			],
			yAxis: [
				{
					type: 'value' as const,
					name: 'Requests',
					minInterval: 1,
					splitLine: {
						lineStyle: {
							color: chartTheme.splitLineColor,
							type: 'dashed' as const
						}
					},
					axisLabel: {
						color: chartTheme.textColor,
						fontSize: 11
					},
					nameTextStyle: {
						color: chartTheme.titleColor,
						fontSize: 11,
						align: 'left' as const,
						padding: [0, 0, 8, -10]
					}
				},
				{
					type: 'value' as const,
					name: 'Latency (ms)',
					splitLine: {
						show: false
					},
					axisLabel: {
						color: chartTheme.textColor,
						fontSize: 11,
						formatter: '{value} ms'
					},
					nameTextStyle: {
						color: chartTheme.titleColor,
						fontSize: 11,
						align: 'right' as const,
						padding: [0, 0, 8, 0]
					}
				}
			],
			series: [
				{
					name: '2xx Success',
					type: 'bar' as const,
					stack: 'status',
					emphasis: { focus: 'series' as const },
					itemStyle: {
						color: '#10b981'
					},
					barMaxWidth: 30,
					data: data2xx
				},
				{
					name: '3xx Redirect',
					type: 'bar' as const,
					stack: 'status',
					emphasis: { focus: 'series' as const },
					itemStyle: {
						color: '#3b82f6'
					},
					barMaxWidth: 30,
					data: data3xx
				},
				{
					name: '4xx Client Err',
					type: 'bar' as const,
					stack: 'status',
					emphasis: { focus: 'series' as const },
					itemStyle: {
						color: '#f59e0b'
					},
					barMaxWidth: 30,
					data: data4xx
				},
				{
					name: '5xx Server Err',
					type: 'bar' as const,
					stack: 'status',
					emphasis: { focus: 'series' as const },
					itemStyle: {
						color: '#ef4444'
					},
					barMaxWidth: 30,
					data: data5xx
				},
				{
					name: 'Latency',
					type: 'line' as const,
					yAxisIndex: 1,
					smooth: true,
					symbol: 'circle',
					symbolSize: 5,
					showSymbol: false,
					emphasis: {
						focus: 'series' as const
					},
					lineStyle: {
						color: '#8b5cf6',
						width: 2.2
					},
					itemStyle: {
						color: '#8b5cf6'
					},
					areaStyle: {
						color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
							{ offset: 0, color: 'rgba(139, 92, 246, 0.22)' },
							{ offset: 1, color: 'rgba(139, 92, 246, 0.01)' }
						])
					},
					data: dataLatency
				}
			]
		};
	});

	function getStatusVariant(status: number): 'success' | 'neutral' | 'warning' | 'error' {
		if (status >= 200 && status < 300) return 'success';
		if (status === 101 || (status >= 300 && status < 400)) return 'neutral';
		if (status >= 400 && status < 500) return 'warning';
		return 'error';
	}

	function resetFilters() {
		searchQuery = '';
		selectedHost = 'all';
		selectedProjectId = 'all';
		selectedServiceId = 'all';
		selectedStatusGroup = 'all';
		selectedMethod = 'all';
		excludeDashboard = true;
		currentPage = 1;
	}

	function handleCopyUri(uri: string, id: string) {
		navigator.clipboard.writeText(uri);
		copiedId = id;
		setTimeout(() => {
			if (copiedId === id) copiedId = null;
		}, 1500);
	}
</script>

<div class="flex w-full flex-col gap-6">
	<!-- 1. Top Summary Metric Cards (Cloudflare Kumo UI Standard) -->
	<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
		<!-- Total Requests -->
		<LayerCard class="flex flex-col gap-1.5 p-4">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-tertiary)]">
					Total requests
				</span>
				<Badge variant="neutral" class="font-mono text-[11px]">
					{timeRange}
				</Badge>
			</div>
			<div class="flex items-baseline gap-2">
				<span class="font-mono text-2xl font-semibold text-[var(--text-primary)]">
					{stats.total}
				</span>
			</div>
			<span class="text-xs text-[var(--text-tertiary)]">
				Recent access log window
			</span>
		</LayerCard>

		<!-- Success Rate -->
		<LayerCard class="flex flex-col gap-1.5 p-4">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-tertiary)]">
					Success rate
				</span>
				<Badge variant="success" class="text-[11px]">
					2xx / 3xx
				</Badge>
			</div>
			<div class="flex items-baseline gap-2">
				<span class="font-mono text-2xl font-semibold text-[var(--status-green)]">
					{stats.successRate}%
				</span>
			</div>
			<span class="text-xs text-[var(--text-tertiary)]">
				Caddy reverse proxy
			</span>
		</LayerCard>

		<!-- Client & Server Errors -->
		<LayerCard class="flex flex-col gap-1.5 p-4">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-tertiary)]">
					Client and server errors
				</span>
				<div class="flex items-center gap-1">
					<Badge variant="warning" class="font-mono text-[10px]">
						{stats.s4xx} (4xx)
					</Badge>
					<Badge variant="error" class="font-mono text-[10px]">
						{stats.s5xx} (5xx)
					</Badge>
				</div>
			</div>
			<div class="flex items-baseline gap-2">
				<span
					class="font-mono text-2xl font-semibold {stats.s4xx + stats.s5xx > 0
						? 'text-[var(--status-amber)]'
						: 'text-[var(--text-primary)]'}"
				>
					{stats.s4xx + stats.s5xx}
				</span>
			</div>
			<span class="text-xs text-[var(--text-tertiary)]">
				Responses with status ≥ 400
			</span>
		</LayerCard>

		<!-- Average Latency -->
		<LayerCard class="flex flex-col gap-1.5 p-4">
			<div class="flex items-center justify-between">
				<span class="text-xs font-medium text-[var(--text-tertiary)]">
					Average response latency
				</span>
				<Badge variant="neutral" class="font-mono text-[11px]">
					rtt
				</Badge>
			</div>
			<div class="flex items-baseline gap-1.5">
				<span class="font-mono text-2xl font-semibold text-[var(--text-primary)]">
					{stats.avgDuration}
				</span>
				<span class="font-mono text-xs text-[var(--text-tertiary)]">ms</span>
			</div>
			<span class="text-xs text-[var(--text-tertiary)]">
				Container dial and response time
			</span>
		</LayerCard>
	</div>

	<!-- 2. Request Volume and Distribution Chart (Generous 460px Height View) -->
	<LayerCard class="flex flex-col gap-4 p-5 min-h-[460px]">
		<!-- Chart Header -->
		<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex flex-col gap-0.5">
				<div class="flex items-center gap-2">
					<Pulse size={16} class="text-[var(--accent)]" />
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						Request volume and distribution
					</h3>
				</div>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Time-bucketed ingress throughput, status code composition, and response latency.
				</p>
			</div>

			<!-- Header Controls -->
			<div class="flex flex-wrap items-center gap-3">
				<!-- Exclude Dashboard Toggle -->
				<div class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] px-2.5 py-1 text-xs">
					<Switch
						bind:checked={excludeDashboard}
						size="sm"
						label="Exclude dashboard traffic"
					/>
				</div>

				<!-- Live Polling Toggle -->
				<button
					type="button"
					onclick={() => (isLivePolling = !isLivePolling)}
					class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
					aria-label="Toggle live polling"
				>
					<ArrowsClockwise size={13} class={isLivePolling && isFetching ? 'animate-spin text-[var(--accent)]' : ''} />
					<span>{isLivePolling ? 'Live stream' : 'Paused'}</span>
				</button>

				<!-- Time Range Selector -->
				<div class="flex items-center gap-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5">
					<button
						type="button"
						onclick={() => (timeRange = '1h')}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs {timeRange === '1h'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'}"
					>
						1h
					</button>
					<button
						type="button"
						onclick={() => (timeRange = '24h')}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs {timeRange === '24h'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'}"
					>
						24h
					</button>
					<button
						type="button"
						onclick={() => (timeRange = '7d')}
						class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs {timeRange === '7d'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'}"
					>
						7d
					</button>
				</div>
			</div>
		</div>

		<!-- Native ECharts Container (Generous 460px Height) -->
		<div class="relative w-full h-[460px] min-h-[460px]">
			<Chart
				{echarts}
				options={echartOptions}
				isDarkMode={isDark}
				height={460}
				optionUpdateBehavior={{ notMerge: true }}
				class="w-full h-full"
			/>
		</div>

		<!-- Telemetry Summary Footer -->
		<div class="flex flex-col justify-between gap-2 border-t border-[var(--border-subtle)] pt-3 text-xs text-[var(--text-tertiary)] sm:flex-row sm:items-center">
			<div class="flex items-center gap-4 font-mono text-[11px]">
				<span>Window: <strong>{timeRange === '1h' ? 'Last 1 hour' : timeRange === '24h' ? 'Last 24 hours' : 'Last 7 days'}</strong></span>
				<span>Peak: <strong>{peakRequests} req/bucket</strong></span>
				<span>Total in window: <strong>{stats.total} reqs</strong></span>
			</div>

			<div class="flex items-center gap-4 font-mono text-[11px]">
				<span>Avg latency: <strong class="text-[var(--accent)]">{stats.avgDuration} ms</strong></span>
				<span>Success rate: <strong class="text-[var(--status-green)]">{stats.successRate}%</strong></span>
			</div>
		</div>
	</LayerCard>

	<!-- 3. Filter Bar & Search -->
	<div class="flex flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center">
		<div class="flex flex-1 flex-wrap items-center gap-2.5">
			<!-- Search query -->
			<div class="w-full sm:w-56">
				<SearchInput
					bind:value={searchQuery}
					placeholder="Filter path, IP, host…"
				/>
			</div>

			<!-- Project Filter -->
			<div class="w-36">
				<Select
					value={selectedProjectId}
					onValueChange={(val) => {
						if (typeof val === 'string') selectedProjectId = val;
					}}
					options={[
						{ label: 'All projects', value: 'all' },
						...dataStore.projects.map((p) => ({ label: p.name, value: p.id }))
					]}
					size="sm"
				/>
			</div>

			<!-- Service Filter -->
			<div class="w-36">
				<Select
					value={selectedServiceId}
					onValueChange={(val) => {
						if (typeof val === 'string') selectedServiceId = val;
					}}
					options={[
						{ label: 'All services', value: 'all' },
						...dataStore.services
							.filter((s) => selectedProjectId === 'all' || s.projectId === selectedProjectId)
							.map((s) => ({ label: s.name, value: s.id }))
					]}
					size="sm"
				/>
			</div>

			<!-- Host select -->
			<div class="w-36">
				<Select
					value={selectedHost}
					onValueChange={(val) => {
						if (typeof val === 'string') selectedHost = val;
					}}
					options={[
						{ label: 'All hosts', value: 'all' },
						...uniqueHosts.filter((h) => h !== 'all').map((h) => ({ label: h, value: h }))
					]}
					size="sm"
				/>
			</div>

			<!-- Status category -->
			<div class="w-32">
				<Select
					value={selectedStatusGroup}
					onValueChange={(val) => {
						if (typeof val === 'string') selectedStatusGroup = val as any;
					}}
					options={[
						{ label: 'All statuses', value: 'all' },
						{ label: '2xx Success', value: '2xx' },
						{ label: '3xx Redirect', value: '3xx' },
						{ label: '4xx Client error', value: '4xx' },
						{ label: '5xx Server error', value: '5xx' }
					]}
					size="sm"
				/>
			</div>

			<!-- Method select -->
			<div class="w-28">
				<Select
					value={selectedMethod}
					onValueChange={(val) => {
						if (typeof val === 'string') selectedMethod = val;
					}}
					options={[
						{ label: 'All methods', value: 'all' },
						{ label: 'GET', value: 'GET' },
						{ label: 'POST', value: 'POST' },
						{ label: 'PUT', value: 'PUT' },
						{ label: 'DELETE', value: 'DELETE' },
						{ label: 'OPTIONS', value: 'OPTIONS' }
					]}
					size="sm"
				/>
			</div>

			{#if searchQuery || selectedHost !== 'all' || selectedProjectId !== 'all' || selectedServiceId !== 'all' || selectedStatusGroup !== 'all' || selectedMethod !== 'all'}
				<button
					type="button"
					onclick={resetFilters}
					class="cursor-pointer border-0 bg-transparent p-0 text-xs text-[var(--accent)] hover:underline"
				>
					Reset filters
				</button>
			{/if}
		</div>

		<!-- Retention Setting Drawer -->
		<div class="flex shrink-0 items-center gap-2">
			<span class="text-xs text-[var(--text-tertiary)]">
				Retention: <strong class="font-medium text-[var(--text-secondary)]">{retentionDays} days</strong>
			</span>
			<button
				type="button"
				onclick={() => (isRetentionModalOpen = !isRetentionModalOpen)}
				class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
				aria-label="Configure access log retention"
			>
				<SlidersHorizontal size={13} />
				<span>Log policy</span>
			</button>
		</div>
	</div>

	<!-- Retention Setting Dropdown / Inline Drawer -->
	{#if isRetentionModalOpen}
		<div
			class="flex flex-col items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-4 text-xs sm:flex-row"
		>
			<div class="flex flex-col gap-1">
				<span class="font-medium text-[var(--text-primary)]">Caddy access log rotation & retention</span>
				<span class="text-xs text-[var(--text-tertiary)]">
					Caddy writes JSON telemetry into <code class="rounded bg-[var(--bg-surface)] px-1 py-0.5 font-mono text-[var(--text-primary)]">/var/log/caddy/access.log</code>. Automatically rotated via roll_size 20MiB.
				</span>
			</div>
			<div class="flex items-center gap-2">
				<div class="w-48">
					<Select
						value={retentionDays}
						onValueChange={(val) => {
							if (typeof val === 'number') retentionDays = val;
						}}
						options={[
							{ label: '3 days', value: 3 },
							{ label: '7 days (recommended)', value: 7 },
							{ label: '14 days', value: 14 },
							{ label: '30 days', value: 30 }
						]}
						size="sm"
					/>
				</div>
				<Button variant="secondary" size="sm" onclick={() => (isRetentionModalOpen = false)}>
					Done
				</Button>
			</div>
		</div>
	{/if}

	<!-- 4. Real Structured Access Logs Table (Clean Table Standards + Pagination) -->
	<LayerCard class="overflow-hidden p-0">
		<!-- Table Container with desktop sticky preservation -->
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-xs">
				<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<TableRow>
						<TableHead class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Status
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Method
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Request URI and host
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Upstream service
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Latency
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Client IP
						</TableHead>
						<TableHead class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Time
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if filteredLogs.length === 0}
						<TableRow>
							<TableCell colspan={7} class="py-14 text-center">
								<div class="mx-auto flex max-w-md flex-col items-center justify-center gap-2.5 text-center">
									<div class="flex h-10 w-10 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]">
										<Globe size={22} />
									</div>
									<h4 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
										{#if searchQuery || selectedHost !== 'all' || selectedProjectId !== 'all' || selectedServiceId !== 'all' || selectedStatusGroup !== 'all' || selectedMethod !== 'all'}
											No matching requests found
										{:else}
											No Caddy reverse proxy requests recorded yet
										{/if}
									</h4>
									<p class="m-0 text-xs text-[var(--text-tertiary)] leading-relaxed">
										{#if searchQuery || selectedHost !== 'all' || selectedProjectId !== 'all' || selectedServiceId !== 'all' || selectedStatusGroup !== 'all' || selectedMethod !== 'all'}
											Clear active search filters to view recorded reverse proxy requests.
										{:else}
											Caddy is listening on port 80 and active domain routes. Incoming HTTP requests proxied through Caddy will appear here in real time.
										{/if}
									</p>
									{#if searchQuery || selectedHost !== 'all' || selectedProjectId !== 'all' || selectedServiceId !== 'all' || selectedStatusGroup !== 'all' || selectedMethod !== 'all'}
										<Button variant="secondary" size="sm" onclick={resetFilters} class="mt-2">
											Clear filters
										</Button>
									{/if}
								</div>
							</TableCell>
						</TableRow>
					{:else}
						{#each paginatedLogs as log, idx (log.id)}
							{@const variant = getStatusVariant(log.status)}
							<TableRow class={idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]' : 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}>
								<!-- Status -->
								<TableCell class="px-3.5 py-3 whitespace-nowrap">
									<Badge {variant} class="font-mono text-xs">
										{log.status}
									</Badge>
								</TableCell>

								<!-- Method -->
								<TableCell class="px-3 py-3 whitespace-nowrap">
									<Badge variant="neutral" class="font-mono text-xs">
										{log.method}
									</Badge>
								</TableCell>

								<!-- URI & Host -->
								<TableCell class="min-w-[240px] px-3 py-3">
									<div class="flex flex-col">
										<div class="flex items-center gap-1.5">
											<span
												class="max-w-md truncate font-mono text-[13px] font-medium text-[var(--text-primary)]"
											>
												{log.uri}
											</span>
											<button
												type="button"
												onclick={() => handleCopyUri(log.uri, log.id)}
												class="flex h-7 w-7 items-center justify-center rounded cursor-pointer border-0 bg-transparent text-[var(--text-tertiary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
												aria-label="Copy request URI"
											>
												{#if copiedId === log.id}
													<Check size={14} class="text-[var(--status-green)]" />
												{:else}
													<Copy size={14} />
												{/if}
											</button>
										</div>
										<span class="truncate text-xs text-[var(--text-tertiary)]">
											{log.host}
										</span>
									</div>
								</TableCell>

								<!-- Upstream Service -->
								<TableCell class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]">
									{log.upstream || log.serviceName || '—'}
								</TableCell>

								<!-- Latency -->
								<TableCell class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]">
									<span
										class={(log.durationMs || 0) > 100
											? 'text-[var(--status-amber)]'
											: 'text-[var(--text-primary)]'}
									>
										{(log.durationMs || 0).toFixed(1)}ms
									</span>
									<span class="block text-[11px] text-[var(--text-tertiary)]">
										{log.bytesSent || '—'}
									</span>
								</TableCell>

								<!-- Client IP -->
								<TableCell class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]">
									{log.clientIp}
								</TableCell>

								<!-- Time -->
								<TableCell class="px-3.5 py-3 text-right text-xs whitespace-nowrap text-[var(--text-tertiary)]">
									{log.timeAgo}
								</TableCell>
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>

		<!-- Table Footer with Full Pagination (Kumo Parity) -->
		<TablePagination
			bind:page={currentPage}
			bind:pageSize
			totalCount={filteredLogs.length}
			pageSizeOptions={[15, 25, 50, 100]}
			itemName="requests"
		/>
	</LayerCard>
</div>
