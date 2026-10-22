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

	let latest = $derived(deployments.slice(0, 8));
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
		{ key: 'version', label: 'Version', mono: true, render: versionCell },
		{ key: 'trigger', label: 'Trigger', render: triggerCell },
		{ key: 'when', label: 'When', mono: true, render: whenCell }
	]}
		rows={latest}
		keyExtractor={(d) => d.id}
		onRowClick={(d) => d.projectId && goto(`/projects/${d.projectId}`)}
		emptyMessage="No deployments recorded yet"
		itemName="deployments"
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

{#snippet versionCell(row: Deployment)}
	<span class="font-[var(--font-mono)]">{row.version || row.commitHash?.slice(0, 7) || '—'}</span>
{/snippet}

{#snippet triggerCell(row: Deployment)}
	{row.trigger
		? row.trigger
				.split('-')
				.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
				.join(' ')
		: 'Manual'}
{/snippet}

{#snippet whenCell(row: Deployment)}
	<span class="font-[var(--font-mono)]" title={row.startedAt}>
		{row.timeAgo || formatTimeAgo(row.startedAt)}
	</span>
{/snippet}
