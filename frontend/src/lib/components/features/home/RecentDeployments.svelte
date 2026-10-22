<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { DataTable, StatusBadge } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { goto } from '$app/navigation';
	import { formatTimeAgo } from '$lib/utils/format';
	import type { Deployment } from '$lib/types';

	interface Props {
		deployments: Deployment[];
		class?: string;
	}

	let { deployments, class: className = '' }: Props = $props();

	// Latest rollout per service. A service with ten green builds contributes
	// one row, so this board ranks services that changed rather than repeating
	// builds of the same service — the full history lives behind "View all".
	let latest = $derived.by(() => {
		const seen = new Set<string>();
		const out: Deployment[] = [];
		for (const d of deployments) {
			const key = d.serviceId || `${d.projectId}-${d.serviceName}`;
			if (seen.has(key)) continue;
			seen.add(key);
			out.push(d);
			if (out.length >= 6) break;
		}
		return out;
	});

	// Always computed fresh from the timestamp; the precomputed timeAgo string
	// freezes between watcher pushes and reads as a lie under a live page.
	function whenFor(d: Deployment): string {
		const ts = Date.parse(d.startedAt ?? '');
		return Number.isNaN(ts) ? d.timeAgo || '—' : formatTimeAgo(ts);
	}

	function whenTitle(d: Deployment): string | undefined {
		const ts = Date.parse(d.startedAt ?? '');
		return Number.isNaN(ts) ? undefined : new Date(ts).toLocaleString();
	}
</script>

<LayerCard class={cn('overflow-hidden p-0', className)}>
	<div
		class="flex items-center justify-between border-b border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5"
	>
		<h2 class="m-0 text-[13px] font-semibold text-[var(--text-secondary)]">Recent deployments</h2>
		<button
			onclick={() => goto('/deployments')}
			class="inline-flex cursor-pointer items-center border-0 bg-transparent p-0 text-[13px] font-[var(--font-sans)] whitespace-nowrap text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
		>
			View all
		</button>
	</div>
	<DataTable
		columns={[
			{ key: 'name', label: 'Service', render: serviceCell },
			{ key: 'status', label: 'Status', render: statusCell },
			{ key: 'when', label: 'When', render: whenCell }
		]}
		rows={latest}
		keyExtractor={(d) => d.id}
		onRowClick={(d) => d.projectId && goto(`/projects/${d.projectId}`)}
		emptyMessage="No deployments recorded yet"
		itemName="deployments"
		showRowsPerPage={false}
	/>
</LayerCard>

{#snippet serviceCell(row: Deployment)}
	<span class="flex min-w-0 flex-col">
		<span class="truncate text-[13px] font-medium text-[var(--text-primary)]">
			{row.serviceName || row.projectName || row.serviceId}
		</span>
		{#if row.serviceName && row.projectName && row.serviceName !== row.projectName}
			<span class="truncate text-xs text-[var(--text-tertiary)]">{row.projectName}</span>
		{/if}
	</span>
{/snippet}

{#snippet statusCell(row: Deployment)}
	<StatusBadge status={row.status} size="sm" />
{/snippet}

{#snippet whenCell(row: Deployment)}
	<span
		class="text-[13px] whitespace-nowrap text-[var(--text-secondary)] tabular-nums"
		title={whenTitle(row)}
	>
		{whenFor(row)}
	</span>
{/snippet}
