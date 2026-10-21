<script lang="ts">
	import { Badge, StatusBadge, DataTable, MountPath, RowActionsMenu } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import { isGopodManaged } from './runtime-helpers';
	import type { Volume } from '$lib/types';

	interface Props {
		rows: Volume[];
		loading?: boolean;
		actions: RuntimeRowActions;
	}

	let { rows, loading = false, actions }: Props = $props();
</script>

<DataTable
	columns={[
		{ key: 'name', label: 'Name', render: volumeName },
		{ key: 'projectName', label: 'Project' },
		{ key: 'serviceName', label: 'Service' },
		{ key: 'mount', label: 'Mount', render: volumeMount },
		{ key: 'size', label: 'Size' },
		{ key: 'status', label: 'Status', render: volumeStatus }
	]}
	{rows}
	keyExtractor={(v) => v.id}
	pageSize={10}
	itemName="volumes"
	{loading}
	stickyActions={true}
	actions={volumeActions}
/>

{#snippet volumeName(row: Volume)}
	<div class="flex items-center gap-1.5">
		<span>{row.name}</span>
		{#if isGopodManaged(row.labels)}
			<Badge variant="secondary" class="text-[10px] text-[var(--text-tertiary)]">gopod</Badge>
		{/if}
	</div>
{/snippet}

{#snippet volumeMount(row: Volume)}
	<MountPath value={row.mount} />
{/snippet}

{#snippet volumeStatus(row: Volume)}
	<StatusBadge status={row.status} size="md" />
{/snippet}

{#snippet volumeActions(row: Volume)}
	<RowActionsMenu
		itemName={row.name}
		busy={actions.isBusy('volumes', row.name)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'volume',
						name: row.name,
						managed: isGopodManaged(row.labels),
						tab: 'volumes',
						rowKey: row.name,
						op: () => dataStore.removeVolume(row.name)
					})
			}
		]}
	/>
{/snippet}
