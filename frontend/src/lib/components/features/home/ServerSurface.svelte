<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { StatusBadge, MetricCard } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import type { Server } from '$lib/types';
	import { engineLabel, engineVersionLabel } from '$lib/utils/engine';
	import { formatCpu, formatStorage } from '$lib/utils/format';

	interface Props {
		server: Server;
		class?: string;
	}

	let { server, class: className = '' }: Props = $props();
</script>

<LayerCard class={cn('p-5 md:p-6', className)}>
	<div class="flex items-start justify-between">
		<div>
			<div class="flex items-center gap-3">
				<h2 class="m-0 text-lg font-semibold text-[var(--text-primary)] md:text-xl">
					{server.hostname}
				</h2>
				<StatusBadge status="online" size="sm" />
			</div>
			<p class="mt-1.5 mb-0 text-xs text-[var(--text-tertiary)] md:text-sm">
				{server.os} · {engineLabel(server)} {engineVersionLabel(server)} · {server.rootless
					? 'Rootless'
					: 'Root'}{server.systemd
					? ' · systemd'
					: ''}
			</p>
		</div>
	</div>
	<div
		class="mt-5 grid grid-cols-4 gap-4 border-t border-[var(--border-subtle)] pt-4 max-sm:grid-cols-2 max-sm:gap-3"
	>
		<MetricCard label="CPU" value={formatCpu(server.cpuUsage)} detail="{server.vcpu} vCPU" />
		<MetricCard
			label="Memory"
			value="{server.memoryUsed.toFixed(1)} GB"
			detail="of {server.memory} GB"
		/>
		<MetricCard
			label="Storage"
			value="{server.storageUsed.toFixed(1)} GB"
			detail="of {server.storage} GB"
		/>
		<MetricCard label="Uptime" value={server.uptime} detail="since last boot" />
	</div>
</LayerCard>
