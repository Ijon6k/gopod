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

	// Health-tone the bar by load (green → amber → red) instead of kumo's
	// brand gradient. `bg-none` cancels the gradient stops so the solid
	// semantic token paints through tailwind-merge.
	function loadTone(pct: number): string {
		if (pct > 85) return 'bg-none bg-[var(--status-red)]';
		if (pct > 60) return 'bg-none bg-[var(--status-amber)]';
		return 'bg-none bg-[var(--status-green)]';
	}
</script>

<LayerCard class={cn('px-5 py-4', className)}>
	<div class="flex flex-wrap items-center gap-x-8 gap-y-4">
		<!-- Identity cluster: what machine, what engine, how it runs -->
		<div class="min-w-[230px] flex-1">
			<div class="flex items-center gap-2.5">
				<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">{server.hostname}</h2>
				<StatusBadge status={server.status} size="sm" />
				<span class="flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
					<span
						class="size-1.5 rounded-full bg-[var(--status-green)] animate-pulse"
						aria-hidden="true"
					></span>
					Live
				</span>
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

		<!-- Vitals: instrument rows — hero value, thin load-toned track. Every
		     number here flows from the 2s live stream, nothing sampled once. -->
		<div class="grid min-w-[360px] flex-[2] grid-cols-3 gap-x-6 gap-y-3 max-sm:grid-cols-1">
			<Meter
				label="CPU"
				value={server.cpuUsage}
				max={100}
				customValue="{server.cpuUsage.toFixed(1)}%"
				trackClassName="h-1"
				indicatorClassName={loadTone(server.cpuUsage)}
			/>
			<Meter
				label="Memory"
				value={memoryPct}
				max={100}
				customValue="{server.memoryUsed.toFixed(1)} / {server.memory.toFixed(0)} GB"
				trackClassName="h-1"
				indicatorClassName={loadTone(memoryPct)}
			/>
			<Meter
				label="Storage"
				value={storagePct}
				max={100}
				customValue="{server.storageUsed.toFixed(1)} / {server.storage.toFixed(0)} GB"
				trackClassName="h-1"
				indicatorClassName={loadTone(storagePct)}
			/>
		</div>
	</div>
</LayerCard>
