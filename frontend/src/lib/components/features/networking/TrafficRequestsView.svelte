<script lang="ts">
	import { accessLogs, domains } from '$lib/data';
	import { Button } from '$lib/components/primitives';
	import type { CaddyAccessLog } from '$lib/types';
	import {
		Pulse,
		ArrowsLeftRight,
		Clock,
		CheckCircle,
		WarningCircle,
		XCircle,
		Funnel,
		MagnifyingGlass,
		Trash,
		SlidersHorizontal
	} from 'phosphor-svelte';

	// Filters
	let searchQuery = $state('');
	let selectedHost = $state('all');
	let selectedStatusGroup = $state('all');
	let selectedMethod = $state('all');
	let timeRange = $state<'1h' | '24h' | '7d'>('24h');
	let isRetentionModalOpen = $state(false);
	let retentionDays = $state(7);

	// Unique hosts for dropdown
	let uniqueHosts = $derived([
		'all',
		...Array.from(new Set(accessLogs.map((l) => l.host)))
	]);

	// Filtered logs
	let filteredLogs = $derived.by(() => {
		return accessLogs.filter((log) => {
			if (selectedHost !== 'all' && log.host !== selectedHost) return false;
			if (selectedMethod !== 'all' && log.method !== selectedMethod) return false;

			if (selectedStatusGroup !== 'all') {
				if (selectedStatusGroup === '2xx' && (log.status < 200 || log.status >= 300)) return false;
				if (selectedStatusGroup === '3xx' && (log.status < 300 || log.status >= 400)) return false;
				if (selectedStatusGroup === '4xx' && (log.status < 400 || log.status >= 500)) return false;
				if (selectedStatusGroup === '5xx' && log.status < 500) return false;
			}

			if (searchQuery.trim()) {
				const q = searchQuery.toLowerCase().trim();
				const matchUri = log.uri.toLowerCase().includes(q);
				const matchIp = log.clientIp.toLowerCase().includes(q);
				const matchHost = log.host.toLowerCase().includes(q);
				if (!matchUri && !matchIp && !matchHost) return false;
			}

			return true;
		});
	});

	// Telemetry stats
	let stats = $derived.by(() => {
		const total = accessLogs.length;
		const s2xx = accessLogs.filter((l) => l.status >= 200 && l.status < 300).length;
		const s3xx = accessLogs.filter((l) => l.status >= 300 && l.status < 400).length;
		const s4xx = accessLogs.filter((l) => l.status >= 400 && l.status < 500).length;
		const s5xx = accessLogs.filter((l) => l.status >= 500).length;
		const successRate = total > 0 ? (((s2xx + s3xx) / total) * 100).toFixed(1) : '100.0';
		const avgDuration =
			total > 0
				? (accessLogs.reduce((acc, cur) => acc + cur.durationMs, 0) / total).toFixed(1)
				: '0.0';

		return {
			total,
			successRate,
			s4xx,
			s5xx,
			avgDuration
		};
	});

	function getStatusColor(status: number): { text: string; bg: string; border: string } {
		if (status >= 200 && status < 300) {
			return {
				text: 'text-[var(--status-green)]',
				bg: 'bg-[var(--status-green-muted)]',
				border: 'border-[var(--status-green)]/30'
			};
		}
		if (status === 101) {
			return {
				text: 'text-[var(--accent)]',
				bg: 'bg-[var(--accent-muted)]',
				border: 'border-[var(--accent)]/30'
			};
		}
		if (status >= 300 && status < 400) {
			return {
				text: 'text-[var(--accent)]',
				bg: 'bg-[var(--bg-panel)]',
				border: 'border-[var(--border)]'
			};
		}
		if (status >= 400 && status < 500) {
			return {
				text: 'text-[var(--status-amber)]',
				bg: 'bg-[var(--status-amber-muted)]',
				border: 'border-[var(--status-amber)]/30'
			};
		}
		return {
			text: 'text-[var(--status-red)]',
			bg: 'bg-[var(--status-red-muted)]',
			border: 'border-[var(--status-red)]/30'
		};
	}

	function resetFilters() {
		searchQuery = '';
		selectedHost = 'all';
		selectedStatusGroup = 'all';
		selectedMethod = 'all';
	}
</script>

<div class="flex flex-col gap-5 w-full">
	<!-- 1. Top Summary Metric Tiles (Dokploy Request Dashboard Standard) -->
	<div class="grid grid-cols-2 md:grid-cols-4 gap-3">
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col gap-1">
			<span class="text-[11px] font-medium text-[var(--text-tertiary)] uppercase tracking-wider">
				Total Requests
			</span>
			<div class="flex items-baseline gap-2">
				<span class="text-2xl font-bold font-mono text-[var(--text-primary)]">
					{stats.total * 1420}
				</span>
				<span class="text-[11px] text-[var(--status-green)] font-medium">
					+12.4%
				</span>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				In the last {timeRange}
			</span>
		</div>

		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col gap-1">
			<span class="text-[11px] font-medium text-[var(--text-tertiary)] uppercase tracking-wider">
				Success Rate
			</span>
			<div class="flex items-baseline gap-2">
				<span class="text-2xl font-bold font-mono text-[var(--status-green)]">
					{stats.successRate}%
				</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">2xx & 3xx</span>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Caddy reverse proxy
			</span>
		</div>

		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col gap-1">
			<span class="text-[11px] font-medium text-[var(--text-tertiary)] uppercase tracking-wider">
				Client / Server Errors
			</span>
			<div class="flex items-baseline gap-2">
				<span class="text-2xl font-bold font-mono text-[var(--text-primary)]">
					<span class="{stats.s4xx > 0 ? 'text-[var(--status-amber)]' : 'text-[var(--text-primary)]'}">{stats.s4xx}</span>
					<span class="text-[var(--text-tertiary)] text-lg font-normal">/</span>
					<span class="{stats.s5xx > 0 ? 'text-[var(--status-red)]' : 'text-[var(--text-primary)]'}">{stats.s5xx}</span>
				</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">4xx / 5xx</span>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				{stats.s5xx === 0 ? '0 server failures' : 'Requires inspection'}
			</span>
		</div>

		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col gap-1">
			<span class="text-[11px] font-medium text-[var(--text-tertiary)] uppercase tracking-wider">
				Avg Upstream Latency
			</span>
			<div class="flex items-baseline gap-2">
				<span class="text-2xl font-bold font-mono text-[var(--text-primary)]">
					{stats.avgDuration}
				</span>
				<span class="text-xs font-mono text-[var(--text-tertiary)]">ms</span>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				p95: 48.2ms · 128 MB/h
			</span>
		</div>
	</div>

	<!-- 2. Request Volume Timeline (Spark Graph) -->
	<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col gap-3">
		<div class="flex items-center justify-between">
			<div class="flex items-center gap-2">
				<Pulse size={15} class="text-[var(--accent)]" />
				<h4 class="text-xs font-semibold text-[var(--text-primary)] m-0">
					Request Volume & Ingress Rate
				</h4>
			</div>

			<div class="flex items-center gap-1 bg-[var(--bg-panel)] p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)]">
				<button
					type="button"
					onclick={() => (timeRange = '1h')}
					class="px-2 py-0.5 rounded text-[11px] cursor-pointer border-0 transition-colors {timeRange === '1h' ? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium' : 'bg-transparent text-[var(--text-tertiary)]'}"
				>
					1h
				</button>
				<button
					type="button"
					onclick={() => (timeRange = '24h')}
					class="px-2 py-0.5 rounded text-[11px] cursor-pointer border-0 transition-colors {timeRange === '24h' ? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium' : 'bg-transparent text-[var(--text-tertiary)]'}"
				>
					24h
				</button>
				<button
					type="button"
					onclick={() => (timeRange = '7d')}
					class="px-2 py-0.5 rounded text-[11px] cursor-pointer border-0 transition-colors {timeRange === '7d' ? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium' : 'bg-transparent text-[var(--text-tertiary)]'}"
				>
					7d
				</button>
			</div>
		</div>

		<!-- SVG Timeline Chart -->
		<div class="h-20 w-full pt-1">
			<svg class="w-full h-full overflow-visible" preserveAspectRatio="none" viewBox="0 0 400 60">
				<defs>
					<linearGradient id="reqGrad" x1="0%" y1="0%" x2="0%" y2="100%">
						<stop offset="0%" stop-color="var(--accent)" stop-opacity="0.32" />
						<stop offset="100%" stop-color="var(--accent)" stop-opacity="0.0" />
					</linearGradient>
				</defs>
				<!-- Area -->
				<polygon
					points="0,55 20,48 40,50 60,35 80,42 100,28 120,30 140,22 160,26 180,18 200,24 220,15 240,20 260,12 280,18 300,10 320,15 340,8 360,14 380,6 400,10 400,60 0,60"
					fill="url(#reqGrad)"
				/>
				<!-- Line -->
				<polyline
					points="0,55 20,48 40,50 60,35 80,42 100,28 120,30 140,22 160,26 180,18 200,24 220,15 240,20 260,12 280,18 300,10 320,15 340,8 360,14 380,6 400,10"
					fill="none"
					stroke="var(--accent)"
					stroke-width="2"
					stroke-linecap="round"
					stroke-linejoin="round"
				/>
			</svg>
		</div>

		<div class="flex items-center justify-between text-[10.5px] text-[var(--text-tertiary)] font-mono border-t border-[var(--border-subtle)] pt-1.5">
			<span>24 hours ago</span>
			<span>Peak: 148 req/s</span>
			<span>Now (Live Caddy Ingress)</span>
		</div>
	</div>

	<!-- 3. Filter Bar & Search -->
	<div class="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-2.5">
		<div class="flex flex-wrap items-center gap-2 flex-1">
			<!-- Search query -->
			<div class="relative min-w-[200px] flex-1 max-w-xs">
				<MagnifyingGlass size={13} class="absolute left-2.5 top-1/2 -translate-y-1/2 text-[var(--text-tertiary)]" />
				<input
					type="text"
					bind:value={searchQuery}
					placeholder="Filter path, IP, hostname..."
					class="w-full pl-7 pr-3 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] text-[var(--text-primary)] text-xs outline-none focus:border-[var(--accent)]"
				/>
			</div>

			<!-- Host select -->
			<select
				bind:value={selectedHost}
				class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] text-[var(--text-secondary)] text-xs outline-none cursor-pointer"
			>
				<option value="all">All Hostnames</option>
				{#each uniqueHosts.filter((h) => h !== 'all') as h}
					<option value={h}>{h}</option>
				{/each}
			</select>

			<!-- Status category -->
			<select
				bind:value={selectedStatusGroup}
				class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] text-[var(--text-secondary)] text-xs outline-none cursor-pointer"
			>
				<option value="all">All Statuses</option>
				<option value="2xx">2xx Success</option>
				<option value="3xx">3xx Redirect</option>
				<option value="4xx">4xx Client Error</option>
				<option value="5xx">5xx Server Error</option>
			</select>

			<!-- Method select -->
			<select
				bind:value={selectedMethod}
				class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] text-[var(--text-secondary)] text-xs outline-none cursor-pointer"
			>
				<option value="all">All Methods</option>
				<option value="GET">GET</option>
				<option value="POST">POST</option>
				<option value="PUT">PUT</option>
				<option value="DELETE">DELETE</option>
				<option value="OPTIONS">OPTIONS</option>
			</select>

			{#if searchQuery || selectedHost !== 'all' || selectedStatusGroup !== 'all' || selectedMethod !== 'all'}
				<button
					type="button"
					onclick={resetFilters}
					class="text-[11px] text-[var(--accent)] hover:underline cursor-pointer bg-transparent border-0 p-0"
				>
					Reset
				</button>
			{/if}
		</div>

		<!-- Log Retention & Purge Settings (Dokploy Parity) -->
		<div class="flex items-center gap-2 shrink-0">
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Retention: <strong class="text-[var(--text-secondary)]">{retentionDays} days</strong>
			</span>
			<button
				type="button"
				onclick={() => (isRetentionModalOpen = !isRetentionModalOpen)}
				class="px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] text-xs flex items-center gap-1.5 cursor-pointer"
				title="Configure access log retention"
			>
				<SlidersHorizontal size={12} />
				<span>Policy</span>
			</button>
		</div>
	</div>

	<!-- Retention Setting Dropdown / Inline Drawer -->
	{#if isRetentionModalOpen}
		<div class="p-3.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
			<div class="flex flex-col gap-0.5">
				<span class="font-medium text-[var(--text-primary)]">Caddy Access Log Rotation & Retention</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Access logs are stored in <code class="font-mono text-[var(--text-primary)]">/var/log/caddy/access.log</code>. Automatically purged via logrotate cron (<code class="font-mono text-[var(--text-secondary)]">0 0 * * *</code>).
				</span>
			</div>
			<div class="flex items-center gap-2">
				<select
					bind:value={retentionDays}
					class="px-2 py-1 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-primary)]"
				>
					<option value={3}>3 Days</option>
					<option value={7}>7 Days (Recommended)</option>
					<option value={14}>14 Days</option>
					<option value={30}>30 Days</option>
				</select>
				<Button variant="secondary" size="xs" onclick={() => (isRetentionModalOpen = false)}>
					Apply
				</Button>
			</div>
		</div>
	{/if}

	<!-- 4. Real Structured Access Logs Table (Clean Table Standards) -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
		<!-- Table Container with desktop sticky preservation -->
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<table class="w-full border-collapse text-left text-xs">
				<thead class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)]">
					<tr>
						<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Status</th>
						<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Method</th>
						<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Request URI & Host</th>
						<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Upstream Latency</th>
						<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Client IP</th>
						<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)] text-right">Time</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-[var(--border-subtle)]">
					{#if filteredLogs.length === 0}
						<tr>
							<td colspan="6" class="py-12 text-center text-[var(--text-tertiary)] text-xs">
								No access logs matching your filter criteria.
							</td>
						</tr>
					{:else}
						{#each filteredLogs as log (log.id)}
							{@const color = getStatusColor(log.status)}
							<tr class="hover:bg-[var(--bg-table-row-alt)] transition-colors">
								<!-- Status -->
								<td class="py-2.5 px-3.5 whitespace-nowrap">
									<span class="inline-flex items-center gap-1 font-mono font-medium text-[11px] px-1.5 py-0.5 rounded border {color.bg} {color.text} {color.border}">
										{log.status}
									</span>
								</td>

								<!-- Method -->
								<td class="py-2.5 px-3 whitespace-nowrap">
									<span class="font-mono font-semibold text-[10.5px] px-1.5 py-0.5 rounded bg-[var(--bg-panel)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">
										{log.method}
									</span>
								</td>

								<!-- URI & Host -->
								<td class="py-2.5 px-3 min-w-[240px]">
									<div class="flex flex-col">
										<span class="font-mono text-xs text-[var(--text-primary)] font-medium truncate max-w-md">
											{log.uri}
										</span>
										<span class="text-[11px] text-[var(--text-tertiary)] truncate">
											{log.host} ➔ {log.upstream}
										</span>
									</div>
								</td>

								<!-- Latency -->
								<td class="py-2.5 px-3 whitespace-nowrap font-mono text-[11.5px] text-[var(--text-secondary)]">
									<span class="{log.durationMs > 100 ? 'text-[var(--status-amber)]' : 'text-[var(--text-primary)]'}">
										{log.durationMs}ms
									</span>
									<span class="text-[10px] text-[var(--text-tertiary)] block">
										{log.bytesSent}
									</span>
								</td>

								<!-- Client IP -->
								<td class="py-2.5 px-3 whitespace-nowrap font-mono text-[11.5px] text-[var(--text-secondary)]">
									{log.clientIp}
								</td>

								<!-- Time -->
								<td class="py-2.5 px-3.5 whitespace-nowrap text-right text-[11px] text-[var(--text-tertiary)]">
									{log.timeAgo}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Table Footer -->
		<div class="px-4 py-2.5 border-t border-[var(--border)] bg-[var(--bg-table-header)] flex items-center justify-between text-xs text-[var(--text-tertiary)]">
			<span>Showing {filteredLogs.length} of {accessLogs.length} recent requests</span>
			<span class="text-[11px] font-mono">Stream: Caddy /var/log/caddy/access.log</span>
		</div>
	</div>
</div>
