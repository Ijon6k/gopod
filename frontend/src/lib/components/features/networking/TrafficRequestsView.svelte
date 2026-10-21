<script lang="ts">
	import { onDestroy, untrack } from 'svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { createPoller } from '$lib/utils/poller';
	import type { CaddyAccessLog } from '$lib/types';
	import {
		TrafficMetricsDeck,
		TrafficChartGrid,
		TrafficFilterBar,
		TrafficRequestsTable,
		type BucketInfo
	} from './traffic';

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

	// Pagination state
	let currentPage = $state(1);
	let pageSize = $state(25);

	// Real access logs fetched from Caddy reverse proxy API
	let liveLogs = $state<CaddyAccessLog[]>([]);

	let active = true;

	async function fetchLogs() {
		if (!active) return;
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

	// In-flight guard + hidden-tab pause: a slow 500-row fetch never overlaps
	// the next tick, and the tab going background stops the 3s hammering.
	const poller = createPoller(3000, fetchLogs, { pauseWhenHidden: true });

	// Initial fetch & live polling; the live toggle owns the poller lifecycle.
	$effect(() => {
		untrack(() => {
			fetchLogs();
		});
		if (isLivePolling) {
			poller.start();
		} else {
			poller.stop();
		}
	});

	onDestroy(() => {
		active = false;
		poller.stop();
	});

	// Reset page when filters change
	$effect(() => {
		if (
			searchQuery ||
			selectedHost ||
			selectedStatusGroup ||
			selectedMethod ||
			selectedProjectId ||
			selectedServiceId ||
			excludeDashboard
		) {
			currentPage = 1;
		}
	});

	// Unique hosts for dropdown
	let uniqueHosts = $derived([
		'all',
		...Array.from(new Set(liveLogs.map((l) => l.host).filter(Boolean)))
	]);

	// Filtered logs
	let filteredLogs = $derived.by(() => {
		return liveLogs.filter((log) => {
			if (selectedHost !== 'all' && log.host !== selectedHost) return false;
			if (selectedMethod !== 'all' && log.method !== selectedMethod) return false;
			if (
				selectedProjectId !== 'all' &&
				(log as any).projectId &&
				(log as any).projectId !== selectedProjectId
			)
				return false;
			if (selectedServiceId !== 'all' && log.serviceId && log.serviceId !== selectedServiceId)
				return false;

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

	// Timeseries Histogram & Trend Data
	let bucketItems = $derived.by<BucketInfo[]>(() => {
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
			let label: string;
			if (timeRange === '1h' || timeRange === '24h') {
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

	let peakRequests = $derived(Math.max(...bucketItems.map((b) => b.total), 0));

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
</script>

<div class="flex w-full flex-col gap-6">
	<!-- 1. Top Summary Metric Cards -->
	<TrafficMetricsDeck
		total={stats.total}
		successRate={stats.successRate}
		s2xx={stats.s2xx}
		s3xx={stats.s3xx}
		s4xx={stats.s4xx}
		s5xx={stats.s5xx}
		avgDuration={stats.avgDuration}
		{timeRange}
	/>

	<!-- 2. Request Volume and Distribution Chart -->
	<TrafficChartGrid
		{bucketItems}
		{peakRequests}
		bind:timeRange
		bind:excludeDashboard
		bind:isLivePolling
		{isFetching}
		{stats}
	/>

	<!-- 3. Filter Bar & Search -->
	<TrafficFilterBar
		bind:searchQuery
		bind:selectedHost
		bind:selectedProjectId
		bind:selectedServiceId
		bind:selectedStatusGroup
		bind:selectedMethod
		bind:retentionDays
		bind:isRetentionModalOpen
		{uniqueHosts}
		projects={dataStore.projects}
		services={dataStore.services}
		onResetFilters={resetFilters}
	/>

	<!-- 4. Real Structured Access Logs Table & Pagination -->
	<TrafficRequestsTable
		{filteredLogs}
		{paginatedLogs}
		bind:currentPage
		bind:pageSize
		hasActiveFilters={Boolean(
			searchQuery ||
			selectedHost !== 'all' ||
			selectedProjectId !== 'all' ||
			selectedServiceId !== 'all' ||
			selectedStatusGroup !== 'all' ||
			selectedMethod !== 'all'
		)}
		onResetFilters={resetFilters}
	/>
</div>
