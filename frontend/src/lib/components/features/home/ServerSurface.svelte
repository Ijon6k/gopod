<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { StatusBadge, MetricCard } from '$lib/components/ui';
	import type { Server } from '$lib/types';
	import { formatCpu, formatStorage } from '$lib/utils/format';

	interface Props {
		server: Server;
		class?: string;
	}

	let { server, class: className = '' }: Props = $props();
</script>

<section
	class={cn(
		'bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-card)] px-[34px] pt-[30px] pb-[34px]',
		className
	)}
>
	<div class="flex items-start justify-between">
		<div>
			<div class="flex items-center gap-[13px]">
				<h2
					class="m-0 text-xl font-semibold tracking-[-0.02em] text-[var(--text-primary)]"
				>
					{server.hostname}
				</h2>
				<StatusBadge status="online" size="sm" />
			</div>
			<p class="mt-2 mb-0 text-[var(--text-tertiary)] text-base">
				{server.os} · Podman {server.podmanVersion} · Rootless{server.systemd ? ' · systemd' : ''}
			</p>
		</div>
	</div>
	<div
		class="grid grid-cols-4 gap-6 mt-[30px] pt-[26px] border-t border-[var(--border-subtle)] max-sm:grid-cols-2 max-sm:gap-y-[22px] max-sm:gap-x-4"
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
</section>
