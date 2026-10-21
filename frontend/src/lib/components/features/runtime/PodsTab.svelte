<script lang="ts">
	import { Badge, StatusBadge, DataTable, RowActionsMenu } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import { isGopodManaged } from './runtime-helpers';
	import type { Pod } from '$lib/types';

	interface Props {
		rows: Pod[];
		loading?: boolean;
		actions: RuntimeRowActions;
	}

	let { rows, loading = false, actions }: Props = $props();
</script>

<DataTable
	columns={[
		{ key: 'name', label: 'Name', render: podName },
		{ key: 'projectName', label: 'Project' },
		{ key: 'status', label: 'Status', render: podStatus },
		{ key: 'containers', label: 'Containers', render: podContainers },
		{ key: 'network', label: 'Network' }
	]}
	{rows}
	keyExtractor={(p) => p.id}
	pageSize={10}
	itemName="pods"
	{loading}
	stickyActions={true}
	actions={podActions}
/>

{#snippet podName(row: Pod)}
	<div class="flex items-center gap-1.5">
		<span>{row.name}</span>
		{#if isGopodManaged(row.labels)}
			<Badge variant="secondary" class="text-[10px] text-[var(--text-tertiary)]">gopod</Badge>
		{/if}
	</div>
{/snippet}

{#snippet podStatus(row: Pod)}
	<StatusBadge status={row.status} size="md" />
{/snippet}

{#snippet podContainers(row: Pod)}
	<span>{row.containers.length}</span>
{/snippet}

{#snippet podActions(row: Pod)}
	<RowActionsMenu
		itemName={row.name}
		busy={actions.isBusy('pods', row.id)}
		actions={[
			{
				label: 'Start',
				disabled: row.status === 'running',
				onselect: () =>
					actions.run('pods', row.id, 'Pod started', `${row.name} is now running.`, () =>
						dataStore.startPod(row.id)
					)
			},
			{
				label: 'Stop',
				disabled: row.status === 'stopped',
				onselect: () =>
					actions.run('pods', row.id, 'Pod stopped', `${row.name} is now stopped.`, () =>
						dataStore.stopPod(row.id)
					)
			},
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'pod',
						name: row.name,
						managed: isGopodManaged(row.labels),
						project: row.projectName,
						running: row.status === 'running',
						tab: 'pods',
						rowKey: row.id,
						op: () => dataStore.deletePod(row.id, row.status === 'running')
					})
			}
		]}
	/>
{/snippet}
