<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { Meter, LayerCard } from 'kumo-svelte';
	import { StatusBadge } from '$lib/components/ui';
	import { engineLabel, engineVersionLabel } from '$lib/utils/engine';
	import type { Server } from '$lib/types';

	interface Props {
		server: Server;
		class?: string;
	}

	let { server, class: className = '' }: Props = $props();

	let memoryPct = $derived(server.memory > 0 ? (server.memoryUsed / server.memory) * 100 : 0);
	let storagePct = $derived(server.storage > 0 ? (server.storageUsed / server.storage) * 100 : 0);
</script>

<LayerCard class={cn('px-5 py-4', className)}>
	<div class="flex flex-wrap items-center gap-x-8 gap-y-4">
		<!-- Identity cluster: what machine, what engine, how it runs -->
		<div class="min-w-[230px] flex-1">
			<div class="flex items-center gap-2.5">
				<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">{server.hostname}</h2>
				<StatusBadge status={server.status} size="sm" />
			</div>
			<p class="mt-1 mb-0 text-[13px] text-[var(--text-tertiary)]">
				{engineLabel(server)}
				{engineVersionLabel(server)} · {server.rootless
					? 'Rootless'
					: 'Root'}{server.systemd
					? ' · systemd'
					: ''}{server.uptime
					? ` · up ${server.uptime}`
					: ''}
			</p>
		</div>

		<!-- Vitals: every number here comes straight from /api/system/info -->
		<div class="grid min-w-[360px] flex-[2] grid-cols-3 gap-6 max-sm:grid-cols-1 max-sm:gap-3">
			<Meter label="CPU" value={server.cpuUsage} max={100} customValue="{server.cpuUsage.toFixed(1)}%" />
			<Meter
				label="Memory"
				value={memoryPct}
				max={100}
				customValue="{server.memoryUsed.toFixed(1)} / {server.memory.toFixed(0)} GB"
			/>
			<Meter
				label="Storage"
				value={storagePct}
				max={100}
				customValue="{server.storageUsed.toFixed(1)} / {server.storage.toFixed(0)} GB"
			/>
		</div>
	</div>
</LayerCard>
