<script lang="ts">
	import * as echarts from 'echarts';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { Switch, Chart } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { Pulse, ArrowsClockwise } from 'phosphor-svelte';

	export interface BucketInfo {
		index: number;
		label: string;
		total: number;
		s2xx: number;
		s3xx: number;
		s4xx: number;
		s5xx: number;
		avgLatency: number;
	}

	interface Props {
		bucketItems: BucketInfo[];
		peakRequests: number;
		timeRange: '1h' | '24h' | '7d';
		excludeDashboard: boolean;
		isLivePolling: boolean;
		isFetching: boolean;
		stats: {
			total: number;
			avgDuration: string;
			successRate: string;
		};
	}

	let {
		bucketItems,
		peakRequests,
		timeRange = $bindable(),
		excludeDashboard = $bindable(),
		isLivePolling = $bindable(),
		isFetching,
		stats
	}: Props = $props();

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

					const latencyRow =
						latencyVal > 0
							? `<div style="margin-top:6px;padding-top:4px;border-top:1px solid ${isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)'};display:flex;justify-content:space-between;gap:16px;color:#8b5cf6;">
						<span>Avg Latency</span>
						<span style="font-weight:600;font-family:monospace;">${latencyVal} ms</span>
					</div>`
							: '';

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
</script>

<LayerCard class="flex min-h-[460px] flex-col gap-4 p-5">
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
			<div
				class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] px-2.5 py-1 text-xs"
			>
				<Switch bind:checked={excludeDashboard} size="sm" label="Exclude dashboard traffic" />
			</div>

			<!-- Live Polling Toggle -->
			<button
				type="button"
				onclick={() => (isLivePolling = !isLivePolling)}
				class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
				aria-label="Toggle live polling"
			>
				<ArrowsClockwise
					size={13}
					class={isLivePolling && isFetching ? 'animate-spin text-[var(--accent)]' : ''}
				/>
				<span>{isLivePolling ? 'Live stream' : 'Paused'}</span>
			</button>

			<!-- Time Range Selector -->
			<div
				class="flex items-center gap-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
			>
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
	<div class="relative h-[460px] min-h-[460px] w-full">
		<Chart
			{echarts}
			options={echartOptions}
			isDarkMode={isDark}
			height={460}
			optionUpdateBehavior={{ notMerge: true }}
			class="h-full w-full"
		/>
	</div>

	<!-- Telemetry Summary Footer -->
	<div
		class="flex flex-col justify-between gap-2 border-t border-[var(--border-subtle)] pt-3 text-xs text-[var(--text-tertiary)] sm:flex-row sm:items-center"
	>
		<div class="flex items-center gap-4 font-mono text-[11px]">
			<span
				>Window: <strong
					>{timeRange === '1h'
						? 'Last 1 hour'
						: timeRange === '24h'
							? 'Last 24 hours'
							: 'Last 7 days'}</strong
				></span
			>
			<span>Peak: <strong>{peakRequests} req/bucket</strong></span>
			<span>Total in window: <strong>{stats.total} reqs</strong></span>
		</div>

		<div class="flex items-center gap-4 font-mono text-[11px]">
			<span>Avg latency: <strong class="text-[var(--accent)]">{stats.avgDuration} ms</strong></span>
			<span
				>Success rate: <strong class="text-[var(--status-green)]">{stats.successRate}%</strong
				></span
			>
		</div>
	</div>
</LayerCard>
