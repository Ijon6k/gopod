<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { LayerCard } from 'kumo-svelte';
	import { StatusBadge } from '$lib/components/ui';
	import { engineLabel, engineVersionLabel } from '$lib/utils/engine';
	import { pluralize } from '$lib/utils/format';
	import type { Server } from '$lib/types';

	interface Props {
		server: Server;
		class?: string;
	}

	let { server, class: className = '' }: Props = $props();

	// One derived row list: every value real, straight from /api/system/info,
	// rows that the backend knows nothing about simply don't exist.
	type FactRow = { label: string; value: string; mono?: boolean };

	let facts = $derived.by<FactRow[]>(() => {
		const rows: FactRow[] = [
			{ label: 'Address', value: server.ip || '—', mono: true },
			{ label: 'Operating system', value: server.os || '—' },
			{ label: 'Kernel', value: server.kernel || '—', mono: true },
			{
				label: 'Engine',
				value: `${engineLabel(server)} ${engineVersionLabel(server)}`.trim(),
				mono: true
			},
			{
				label: 'Mode',
				value: `${server.rootless ? 'Rootless' : 'Root'}${server.systemd ? ' · systemd' : ''}`
			},
			{ label: 'Capacity', value: `${server.vcpu} ${pluralize(server.vcpu, 'vCPU')}` }
		];
		if ((server.swapTotal ?? 0) > 0) {
			rows.push({
				label: 'Swap',
				value: `${server.swapUsed?.toFixed(1) ?? '0.0'} / ${(server.swapTotal ?? 0).toFixed(1)} GB`
			});
		}
		if (typeof server.containersRunning === 'number' && typeof server.containersTotal === 'number') {
			rows.push({
				label: 'Containers',
				value: `${server.containersRunning} running of ${server.containersTotal}`
			});
		}
		return rows;
	});
</script>

<LayerCard class={cn('flex flex-col p-5', className)}>
	<div class="mb-1 flex items-center gap-2">
		<h2 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Host facts</h2>
		<StatusBadge status={server.status} size="sm" />
	</div>
	<dl class="m-0 flex flex-col divide-y divide-[var(--border-subtle)]">
		{#each facts as fact (fact.label)}
			<div class="flex items-baseline justify-between gap-4 py-1.5">
				<dt class="shrink-0 text-[13px] text-[var(--text-tertiary)]">{fact.label}</dt>
				<dd
					class="m-0 truncate text-[13px] font-medium text-[var(--text-primary)] {fact.mono
						? 'font-[var(--font-mono)]'
						: ''}"
				>
					{fact.value}
				</dd>
			</div>
		{/each}
	</dl>
</LayerCard>
