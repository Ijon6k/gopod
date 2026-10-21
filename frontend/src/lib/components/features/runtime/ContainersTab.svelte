<script lang="ts">
	import { Badge, StatusBadge, DataTable, MountPath, RowActionsMenu } from '$lib/components/ui';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import { isGopodManaged, parsePorts } from './runtime-helpers';
	import type { Container } from '$lib/types';

	interface Props {
		rows: Container[];
		loading?: boolean;
		actions: RuntimeRowActions;
		onViewLogs: (container: Container) => void;
	}

	let { rows, loading = false, actions, onViewLogs }: Props = $props();
</script>

{#snippet managedName(name: string, labels?: Record<string, string> | null)}
	<div class="flex items-center gap-1.5">
		<span>{name}</span>
		{#if isGopodManaged(labels)}
			<Badge variant="secondary" class="text-[10px] text-[var(--text-tertiary)]">gopod</Badge>
		{/if}
	</div>
{/snippet}

<DataTable
	columns={[
		{ key: 'name', label: 'Name', render: containerName },
		{ key: 'projectName', label: 'Project' },
		{ key: 'serviceName', label: 'Service' },
		{ key: 'image', label: 'Image', mono: true },
		{ key: 'status', label: 'Status', render: containerStatus },
		{ key: 'ports', label: 'Ports', render: containerPorts },
		{ key: 'startedAt', label: 'Started' }
	]}
	{rows}
	keyExtractor={(c) => c.id}
	pageSize={10}
	itemName="containers"
	{loading}
	stickyActions={true}
	actions={containerActions}
/>

{#snippet containerName(row: Container)}
	{@render managedName(row.name, row.labels)}
{/snippet}

{#snippet containerStatus(row: Container)}
	<StatusBadge status={row.status} size="md" />
{/snippet}

{#snippet containerPorts(row: Container)}
	{@const portList = parsePorts(row.ports)}
	{#if portList.length > 0}
		<div class="flex flex-col gap-1 py-0.5">
			{#each portList as port (port)}
				<span
					class="text-base leading-tight font-[var(--font-mono)] whitespace-nowrap text-[var(--text-primary)]"
				>
					{port}
				</span>
			{/each}
		</div>
	{:else}
		<span class="text-base text-[var(--text-tertiary)]">—</span>
	{/if}
{/snippet}

{#snippet containerActions(row: Container)}
	<RowActionsMenu
		itemName={row.name}
		busy={actions.isBusy('containers', row.id)}
		actions={[
			{
				label: 'Logs',
				onselect: () => onViewLogs(row)
			},
			{
				label: 'Start',
				disabled: row.status === 'running',
				onselect: () =>
					actions.run(
						'containers',
						row.id,
						'Container started',
						`${row.name} is now running.`,
						() => dataStore.startContainer(row.id)
					)
			},
			{
				label: 'Stop',
				disabled: row.status === 'stopped',
				onselect: () =>
					actions.run(
						'containers',
						row.id,
						'Container stopped',
						`${row.name} is now stopped.`,
						() => dataStore.stopContainer(row.id)
					)
			},
			{
				label: 'Restart',
				onselect: () =>
					actions.run('containers', row.id, 'Container restarted', `${row.name} restarted.`, () =>
						dataStore.restartContainer(row.id)
					)
			},
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'container',
						name: row.name,
						managed: isGopodManaged(row.labels),
						project: row.projectName,
						service: row.serviceName,
						running: row.status === 'running',
						tab: 'containers',
						rowKey: row.id,
						op: () => dataStore.deleteContainer(row.id, row.status === 'running')
					})
			}
		]}
	/>
{/snippet}
