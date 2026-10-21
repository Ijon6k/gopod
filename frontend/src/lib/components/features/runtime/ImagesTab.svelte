<script lang="ts">
	import { DataTable, RowActionsMenu } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import type { Image } from '$lib/types';

	interface Props {
		rows: Image[];
		loading?: boolean;
		actions: RuntimeRowActions;
	}

	let { rows, loading = false, actions }: Props = $props();
</script>

<DataTable
	columns={[
		{ key: 'name', label: 'Repository', mono: true },
		{ key: 'tag', label: 'Tag', mono: true },
		{ key: 'size', label: 'Size' },
		{ key: 'usedBy', label: 'Used by' },
		{ key: 'createdAt', label: 'Created' }
	]}
	{rows}
	keyExtractor={(img) => img.id}
	pageSize={10}
	itemName="images"
	{loading}
	stickyActions={true}
	actions={imageActions}
/>

{#snippet imageActions(row: Image)}
	<RowActionsMenu
		itemName={row.name ?? row.id}
		busy={actions.isBusy('images', row.id)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'image',
						name: row.id,
						managed: false,
						tab: 'images',
						rowKey: row.id,
						op: () => dataStore.removeImage(row.id)
					})
			}
		]}
	/>
{/snippet}
