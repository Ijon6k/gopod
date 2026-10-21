<script lang="ts">
	import { DataTable, RowActionsMenu } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import type { Network } from '$lib/types';

	interface Props {
		rows: Network[];
		loading?: boolean;
		actions: RuntimeRowActions;
	}

	let { rows, loading = false, actions }: Props = $props();
</script>

<DataTable
	columns={[
		{ key: 'name', label: 'Name' },
		{ key: 'driver', label: 'Driver' },
		{ key: 'containers', label: 'Containers' },
		{ key: 'subnet', label: 'Subnet', mono: true },
		{ key: 'gateway', label: 'Gateway', mono: true }
	]}
	{rows}
	keyExtractor={(n) => n.id}
	pageSize={10}
	itemName="networks"
	{loading}
	stickyActions={true}
	actions={networkActions}
/>

{#snippet networkActions(row: Network)}
	<RowActionsMenu
		itemName={row.name}
		busy={actions.isBusy('networks', row.name)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'network',
						name: row.name,
						managed: false,
						tab: 'networks',
						rowKey: row.name,
						op: () => dataStore.removeNetwork(row.name)
					})
			}
		]}
	/>
{/snippet}
