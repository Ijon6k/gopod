<script lang="ts">
	import { LayerCard, Badge } from 'kumo-svelte';

	interface Props {
		total: number;
		successRate: string;
		s2xx: number;
		s3xx: number;
		s4xx: number;
		s5xx: number;
		avgDuration: string;
		timeRange: '1h' | '24h' | '7d';
	}

	let { total, successRate, s2xx, s3xx, s4xx, s5xx, avgDuration, timeRange }: Props = $props();
</script>

<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
	<!-- Total Requests -->
	<LayerCard class="flex flex-col gap-1.5 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-tertiary)]">Total requests</span>
			<Badge variant="neutral" class="font-mono text-[11px]">
				{timeRange}
			</Badge>
		</div>
		<div class="flex items-baseline gap-2">
			<span class="font-mono text-2xl font-semibold text-[var(--text-primary)]">
				{total}
			</span>
		</div>
		<span class="text-xs text-[var(--text-tertiary)]">Recent access log window</span>
	</LayerCard>

	<!-- Success Rate -->
	<LayerCard class="flex flex-col gap-1.5 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-tertiary)]">Success rate</span>
			<Badge variant="success" class="text-[11px]">2xx / 3xx</Badge>
		</div>
		<div class="flex items-baseline gap-2">
			<span class="font-mono text-2xl font-semibold text-[var(--status-green)]">
				{successRate}%
			</span>
		</div>
		<span class="text-xs text-[var(--text-tertiary)]">Caddy reverse proxy</span>
	</LayerCard>

	<!-- Client & Server Errors -->
	<LayerCard class="flex flex-col gap-1.5 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-tertiary)]">Client and server errors</span>
			<div class="flex items-center gap-1">
				<Badge variant="warning" class="font-mono text-[10px]">
					{s4xx} (4xx)
				</Badge>
				<Badge variant="error" class="font-mono text-[10px]">
					{s5xx} (5xx)
				</Badge>
			</div>
		</div>
		<div class="flex items-baseline gap-2">
			<span
				class="font-mono text-2xl font-semibold {s4xx + s5xx > 0
					? 'text-[var(--status-amber)]'
					: 'text-[var(--text-primary)]'}"
			>
				{s4xx + s5xx}
			</span>
		</div>
		<span class="text-xs text-[var(--text-tertiary)]">Responses with status ≥ 400</span>
	</LayerCard>

	<!-- Average Latency -->
	<LayerCard class="flex flex-col gap-1.5 p-4">
		<div class="flex items-center justify-between">
			<span class="text-xs font-medium text-[var(--text-tertiary)]">Average response latency</span>
			<Badge variant="neutral" class="font-mono text-[11px]">rtt</Badge>
		</div>
		<div class="flex items-baseline gap-1.5">
			<span class="font-mono text-2xl font-semibold text-[var(--text-primary)]">
				{avgDuration}
			</span>
			<span class="font-mono text-xs text-[var(--text-tertiary)]">ms</span>
		</div>
		<span class="text-xs text-[var(--text-tertiary)]">Container dial and response time</span>
	</LayerCard>
</div>
