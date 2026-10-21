<script lang="ts">
	import { Badge, Button, StatusBadge, DataTable, RowActionsMenu } from '$lib/components/ui';
	import { ArrowClockwise, FileCode, Play, Stop } from 'phosphor-svelte';
	import { dataStore } from '$lib/stores/data.svelte';
	import type { RuntimeRowActions } from './runtime-actions.svelte';
	import type { QuadletUnit } from '$lib/types';

	interface Props {
		rows: QuadletUnit[];
		loading?: boolean;
		actions: RuntimeRowActions;
		onViewSource: (unit: QuadletUnit) => void;
	}

	let { rows, loading = false, actions, onViewSource }: Props = $props();

	let isReloading = $state(false);

	async function daemonReload() {
		isReloading = true;
		try {
			await dataStore.reloadQuadlets();
		} finally {
			isReloading = false;
		}
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex items-center justify-between">
		<p class="m-0 text-xs text-[var(--text-secondary)]">
			Systemd Quadlet units installed in declarative search paths (<code class="font-mono">~/.config/containers/systemd/</code
			>).
		</p>
		<Button variant="secondary" size="sm" disabled={isReloading} onclick={daemonReload}>
			<ArrowClockwise size={14} class={isReloading ? 'animate-spin' : ''} />
			<span>Daemon Reload</span>
		</Button>
	</div>

	<DataTable
		columns={[
			{ key: 'name', label: 'Unit File', mono: true },
			{ key: 'type', label: 'Type', render: quadletType },
			{ key: 'unitName', label: 'Service Unit', mono: true },
			{ key: 'status', label: 'Status', render: quadletStatus },
			{ key: 'path', label: 'Path', mono: true }
		]}
		{rows}
		keyExtractor={(q) => q.name}
		pageSize={10}
		itemName="quadlets"
		{loading}
		stickyActions={true}
		actions={quadletActions}
	/>
</div>

{#snippet quadletType(row: QuadletUnit)}
	<Badge variant="secondary" class="font-mono text-[11px] uppercase">{row.type}</Badge>
{/snippet}

{#snippet quadletStatus(row: QuadletUnit)}
	<StatusBadge
		status={row.status === 'active' ? 'running' : row.status === 'failed' ? 'failed' : 'stopped'}
		size="md"
	/>
{/snippet}

{#snippet quadletActions(row: QuadletUnit)}
	<RowActionsMenu
		itemName={row.name}
		busy={actions.isBusy('quadlets', row.name)}
		actions={[
			{
				label: 'View Unit Source',
				onselect: () => onViewSource(row)
			},
			{
				label: 'Start',
				disabled: row.status === 'active',
				onselect: () =>
					actions.run('quadlets', row.name, 'Quadlet started', `${row.name} is now active.`, () =>
						dataStore.startQuadlet(row.name)
					)
			},
			{
				label: 'Stop',
				disabled: row.status !== 'active',
				onselect: () =>
					actions.run('quadlets', row.name, 'Quadlet stopped', `${row.name} is now inactive.`, () =>
						dataStore.stopQuadlet(row.name)
					)
			},
			{
				label: 'Restart',
				onselect: () =>
					actions.run('quadlets', row.name, 'Quadlet restarted', `${row.name} restarted.`, () =>
						dataStore.restartQuadlet(row.name)
					)
			},
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					actions.requestDeleteConfirm({
						kind: 'quadlet unit',
						name: row.name,
						managed: true,
						tab: 'quadlets',
						rowKey: row.name,
						op: () => dataStore.deleteQuadlet(row.name)
					})
			}
		]}
	/>
{/snippet}
