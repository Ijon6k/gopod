<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { LayerCard } from 'kumo-svelte';
	import { engineLabel, engineVersionLabel } from '$lib/utils/engine';
	import type { Server } from '$lib/types';

	interface Props {
		server: Server;
		class?: string;
	}

	let { server, class: className = '' }: Props = $props();

	// Slim full-width fact strip: one row, no panel of its own. Facts the
	// operator compares against their mental model of the machine — nothing
	// decorative, nothing duplicated from the status strip above.
	let facts = $derived([
		{
			label: 'Engine',
			value: [engineLabel(server), engineVersionLabel(server)].filter(Boolean).join(' ')
		},
		{
			label: 'Mode',
			value: [server.rootless ? 'Rootless' : 'Rootful', server.systemd ? 'systemd' : '']
				.filter(Boolean)
				.join(' · ')
		},
		{ label: 'Kernel', value: server.kernel || '—' },
		{ label: 'Capacity', value: `${server.vcpu} vCPUs` },
		{
			label: 'Swap',
			value: server.swapTotal ? `${(server.swapUsed ?? 0).toFixed(1)} / ${server.swapTotal} GB` : 'None'
		}
	]);
</script>

<LayerCard class={cn('px-5 py-3', className)}>
	<dl class="m-0 flex flex-wrap items-center gap-x-8 gap-y-1.5">
		{#each facts as fact (fact.label)}
			<div class="flex min-w-0 items-baseline gap-1.5">
				<dt class="m-0 shrink-0 text-xs text-[var(--text-tertiary)]">{fact.label}</dt>
				<dd
					class="m-0 truncate text-xs font-medium text-[var(--text-secondary)]"
					title={fact.value || undefined}
				>
					{fact.value || '—'}
				</dd>
			</div>
		{/each}
	</dl>
</LayerCard>
