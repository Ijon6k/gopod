<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		PageHeader,
		Tabs,
		DataTable,
		StatusBadge,
		Modal,
		CodeEditor,
		ConfirmDialog,
		RowActionsMenu,
		Select
	} from '$lib/components/ui';
	import type { RowAction } from '$lib/components/ui/RowActionsMenu.svelte';
	import { Button, Badge, useKumoToastManager } from 'kumo-svelte';
	import { Play, Stop, ArrowClockwise, FileCode, Trash } from 'phosphor-svelte';
	import { dataStore } from '$lib/stores/data.svelte';
	import { runtimeApi } from '$lib/api/runtime/api';
	import type { QuadletUnit, Container } from '$lib/types';

	const toast = useKumoToastManager();

	let activeTab = $derived(page.params.tab ?? 'containers');

	let containers = $derived(dataStore.containers);
	let pods = $derived(dataStore.pods);
	let images = $derived(dataStore.images);
	let volumes = $derived(dataStore.volumes);
	let networks = $derived(dataStore.networks);
	let quadlets = $derived(dataStore.quadlets);

	let viewingQuadlet = $state<QuadletUnit | null>(null);
	let isReloading = $state(false);

	// One keyed entry per row (`<tab>:<row key>`): rows act independently, a
	// single row cannot run two overlapping actions.
	let busyRows = $state<Record<string, boolean>>({});

	type RequestedConfirm = {
		title: string;
		description?: string;
		matchValue?: string;
		matchLabel?: string;
		run: () => Promise<void>;
	};
	let requestedConfirm = $state<RequestedConfirm | null>(null);
	let isConfirmBusy = $state(false);

	// Container logs modal. The runtime logs endpoint is a plain tail fetch,
	// not a stream; selecting a tail or reopening the container refetches.
	let logsContainer = $state<Container | null>(null);
	let logsTail = $state(100);
	let logsText = $state('');
	let logsError = $state('');
	let isLogsLoading = $state(false);
	let logsRequestSeq = $state(0);

	const tailOptions = [
		{ label: 'Last 50', value: '50' },
		{ label: 'Last 100', value: '100' },
		{ label: 'Last 500', value: '500' },
		{ label: 'Last 1000', value: '1000' }
	];

	onMount(() => {
		dataStore.fetchRuntimeData();
		dataStore.fetchLiveStats();
	});

	$effect(() => {
		if (!logsContainer) return;
		// The sequence counter is bumped untracked: reading it back would turn
		// this effect into its own trigger and reload the log forever.
		const seq = untrack(() => ++logsRequestSeq);
		const target = logsContainer;
		const tail = logsTail;
		logsError = '';
		isLogsLoading = true;
		runtimeApi.containers
			.logs(target.id, tail)
			.then((res) => {
				if (seq === logsRequestSeq) logsText = res.logs;
			})
			.catch((err: any) => {
				if (seq === logsRequestSeq) {
					logsText = '';
					logsError =
						err?.message || 'Failed to load container logs (the container may have been removed)';
				}
			})
			.finally(() => {
				if (seq === logsRequestSeq) isLogsLoading = false;
			});
	});

	// Rows carry gopod ownership labels or compose labels; a row with neither
	// is foreign to this deployment and is flagged wherever a delete is offered.
	function isGopodManaged(labels?: Record<string, string> | null): boolean {
		if (!labels) return false;
		return Boolean(
			labels['io.gopod.name'] ||
			labels['io.gopod.service'] ||
			labels['io.gopod.project'] ||
			labels['com.docker.compose.project'] ||
			labels['io.podman.compose.project']
		);
	}

	// Wraps every row action: per-row busy state plus a success / failure toast.
	// A thrown error keeps the server's reason (engine refusals included), and
	// optimistic store changes revert on their own.
	async function runRowAction(
		tab: string,
		rowKey: string,
		successTitle: string,
		description: string,
		op: () => Promise<void>
	) {
		const busyKey = `${tab}:${rowKey}`;
		busyRows = { ...busyRows, [busyKey]: true };
		try {
			await op();
			toast.add({ title: successTitle, description });
		} catch (err: any) {
			toast.add({
				title: `${successTitle} failed`,
				description: err?.message || 'The request did not succeed.',
				variant: 'error'
			});
		} finally {
			busyRows = { ...busyRows, [busyKey]: false };
		}
	}

	// Every destructive row action requires typing the exact name; foreign
	// (unlabeled) rows carry an ownership warning and running workloads are
	// called out as force-removed.
	function requestDeleteConfirm(args: {
		kind: string;
		name: string;
		managed: boolean;
		tab: string;
		rowKey: string;
		op: () => Promise<void>;
		project?: string;
		service?: string;
		running?: boolean;
	}) {
		const notes = [
			args.managed
				? undefined
				: `${args.name} is not managed by gopod. Double-check that nothing outside this deployment depends on it.`,
			args.managed && args.project
				? `Owner: ${args.project}${args.service ? ` · ${args.service}` : ''}.`
				: undefined,
			args.running
				? 'It is running right now; deletion will force-remove its live workload.'
				: undefined
		]
			.filter(Boolean)
			.join(' ');
		const kindLabel = `${args.kind.charAt(0).toUpperCase()}${args.kind.slice(1)}`;
		requestedConfirm = {
			title: `Delete ${args.kind} ${args.name}?`,
			description: notes,
			matchValue: args.name,
			matchLabel: 'To confirm, type the exact name:',
			run: () =>
				runRowAction(
					args.tab,
					args.rowKey,
					`${kindLabel} deleted`,
					`${args.name} was removed.`,
					args.op
				)
		};
	}

	function parsePorts(ports: string | undefined | null): string[] {
		if (!ports || ports === '—') return [];
		return ports
			.split(/[\n,]+/)
			.map((p) => p.trim().replaceAll('→', ':').replaceAll('->', ':'))
			.filter(Boolean);
	}

	function isBusy(tab: string, rowKey: string | null | undefined): boolean {
		return Boolean(rowKey && busyRows[`${tab}:${rowKey}`]);
	}

	const tabs = [
		{ id: 'containers', label: 'Containers' },
		{ id: 'quadlets', label: 'Quadlets' },
		{ id: 'pods', label: 'Pods' },
		{ id: 'images', label: 'Images' },
		{ id: 'volumes', label: 'Volumes' },
		{ id: 'networks', label: 'Networks' }
	];

	function onTabChange(id: string) {
		goto(`/runtime/${id}`, { replaceState: true });
	}
</script>

<svelte:head>
	<title>Runtime — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader title="Runtime" subtitle="Containers, pods, images, volumes, and networks." />
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	{#if activeTab === 'containers'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name', render: containerNameSnippet },
				{ key: 'projectName', label: 'Project' },
				{ key: 'serviceName', label: 'Service' },
				{ key: 'image', label: 'Image', mono: true },
				{ key: 'status', label: 'Status', render: containerStatusSnippet },
				{ key: 'ports', label: 'Ports', render: containerPortsSnippet },
				{ key: 'startedAt', label: 'Started' }
			]}
			rows={containers}
			keyExtractor={(c) => c.id}
			pageSize={10}
			itemName="containers"
			actions={containerActionsSnippet}
		/>
	{:else if activeTab === 'quadlets'}
		<div class="flex flex-col gap-4">
			<div class="flex items-center justify-between">
				<p class="m-0 text-xs text-[var(--text-secondary)]">
					Systemd Quadlet units installed in declarative search paths (<code class="font-mono"
						>~/.config/containers/systemd/</code
					>).
				</p>
				<Button
					variant="secondary"
					size="sm"
					disabled={isReloading}
					onclick={async () => {
						isReloading = true;
						try {
							await dataStore.reloadQuadlets();
						} finally {
							isReloading = false;
						}
					}}
				>
					<ArrowClockwise size={14} class={isReloading ? 'animate-spin' : ''} />
					<span>Daemon Reload</span>
				</Button>
			</div>

			<DataTable
				columns={[
					{ key: 'name', label: 'Unit File', mono: true },
					{ key: 'type', label: 'Type', render: quadletTypeSnippet },
					{ key: 'unitName', label: 'Service Unit', mono: true },
					{ key: 'status', label: 'Status', render: quadletStatusSnippet },
					{ key: 'path', label: 'Path', mono: true }
				]}
				rows={quadlets}
				keyExtractor={(q) => q.name}
				actions={quadletActionsSnippet}
				pageSize={10}
				itemName="quadlets"
			/>
		</div>
	{:else if activeTab === 'pods'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name', render: podNameSnippet },
				{ key: 'projectName', label: 'Project' },
				{ key: 'status', label: 'Status', render: podStatusSnippet },
				{ key: 'containers', label: 'Containers', render: podContainersSnippet },
				{ key: 'network', label: 'Network' }
			]}
			rows={pods}
			keyExtractor={(p) => p.id}
			pageSize={10}
			itemName="pods"
			actions={podActionsSnippet}
		/>
	{:else if activeTab === 'images'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Repository', mono: true },
				{ key: 'tag', label: 'Tag', mono: true },
				{ key: 'size', label: 'Size' },
				{ key: 'usedBy', label: 'Used by' },
				{ key: 'createdAt', label: 'Created' }
			]}
			rows={images}
			keyExtractor={(img) => img.id}
			pageSize={10}
			itemName="images"
			actions={imageActionsSnippet}
		/>
	{:else if activeTab === 'volumes'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name', render: volumeNameSnippet },
				{ key: 'projectName', label: 'Project' },
				{ key: 'serviceName', label: 'Service' },
				{ key: 'mount', label: 'Mount', mono: true },
				{ key: 'size', label: 'Size' },
				{ key: 'status', label: 'Status', render: volumeStatusSnippet }
			]}
			rows={volumes}
			keyExtractor={(v) => v.id}
			pageSize={10}
			itemName="volumes"
			actions={volumeActionsSnippet}
		/>
	{:else if activeTab === 'networks'}
		<DataTable
			columns={[
				{ key: 'name', label: 'Name' },
				{ key: 'driver', label: 'Driver' },
				{ key: 'containers', label: 'Containers' },
				{ key: 'subnet', label: 'Subnet', mono: true },
				{ key: 'gateway', label: 'Gateway', mono: true }
			]}
			rows={networks}
			keyExtractor={(n) => n.id}
			pageSize={10}
			itemName="networks"
			actions={networkActionsSnippet}
		/>
	{/if}
</div>

{#snippet managedName(name: string, labels?: Record<string, string> | null)}
	<div class="flex items-center gap-1.5">
		<span>{name}</span>
		{#if isGopodManaged(labels)}
			<Badge variant="secondary" class="text-[10px] text-[var(--text-tertiary)]">gopod</Badge>
		{/if}
	</div>
{/snippet}

{#snippet containerNameSnippet(row: (typeof containers)[0])}
	{@render managedName(row.name, row.labels)}
{/snippet}

{#snippet containerStatusSnippet(row: (typeof containers)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet containerPortsSnippet(row: (typeof containers)[0])}
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
{#snippet podStatusSnippet(row: (typeof pods)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}
{#snippet podContainersSnippet(row: (typeof pods)[0])}
	<span>{row.containers.length}</span>
{/snippet}
{#snippet volumeStatusSnippet(row: (typeof volumes)[0])}
	<StatusBadge status={row.status} size="md" />
{/snippet}

{#snippet quadletTypeSnippet(row: (typeof quadlets)[0])}
	<Badge variant="secondary" class="font-mono text-[11px] uppercase">{row.type}</Badge>
{/snippet}

{#snippet quadletStatusSnippet(row: (typeof quadlets)[0])}
	<StatusBadge
		status={row.status === 'active' ? 'running' : row.status === 'failed' ? 'failed' : 'stopped'}
		size="md"
	/>
{/snippet}

{#snippet quadletActionsSnippet(row: (typeof quadlets)[0])}
	<div class="flex items-center justify-end gap-1">
		{#if row.status === 'active'}
			<button
				type="button"
				title="Stop Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--status-red)]"
				onclick={() => dataStore.stopQuadlet(row.name)}
			>
				<Stop size={14} />
			</button>
			<button
				type="button"
				title="Restart Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
				onclick={() => dataStore.restartQuadlet(row.name)}
			>
				<ArrowClockwise size={14} />
			</button>
		{:else}
			<button
				type="button"
				title="Start Quadlet"
				class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--status-green)]"
				onclick={() => dataStore.startQuadlet(row.name)}
			>
				<Play size={14} />
			</button>
		{/if}
		<button
			type="button"
			title="View Unit Source"
			class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
			onclick={() => (viewingQuadlet = row)}
		>
			<FileCode size={14} />
		</button>
		<button
			type="button"
			title="Delete Quadlet Unit"
			class="cursor-pointer rounded p-1.5 text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--status-red)]"
			onclick={async () => {
				if (
					confirm(
						`Are you sure you want to delete Quadlet unit '${row.name}'? This will stop the systemd service, purge the unit file, and remove the container.`
					)
				) {
					await dataStore.deleteQuadlet(row.name);
				}
			}}
		>
			<Trash size={14} />
		</button>
	</div>
{/snippet}

{#snippet containerActionsSnippet(row: (typeof containers)[0])}
	<RowActionsMenu
		itemName={row.name}
		busy={isBusy('containers', row.id)}
		actions={[
			{
				label: 'Logs',
				onselect: () => {
					logsTail = 100;
					logsContainer = row;
				}
			},
			{
				label: 'Start',
				disabled: row.status === 'running',
				onselect: () =>
					runRowAction(
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
					runRowAction(
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
					runRowAction('containers', row.id, 'Container restarted', `${row.name} restarted.`, () =>
						dataStore.restartContainer(row.id)
					)
			},
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					requestDeleteConfirm({
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

{#snippet podNameSnippet(row: (typeof pods)[0])}
	{@render managedName(row.name, row.labels)}
{/snippet}

{#snippet podActionsSnippet(row: (typeof pods)[0])}
	<RowActionsMenu
		itemName={row.name}
		busy={isBusy('pods', row.id)}
		actions={[
			{
				label: 'Start',
				disabled: row.status === 'running',
				onselect: () =>
					runRowAction('pods', row.id, 'Pod started', `${row.name} is now running.`, () =>
						dataStore.startPod(row.id)
					)
			},
			{
				label: 'Stop',
				disabled: row.status === 'stopped',
				onselect: () =>
					runRowAction('pods', row.id, 'Pod stopped', `${row.name} is now stopped.`, () =>
						dataStore.stopPod(row.id)
					)
			},
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					requestDeleteConfirm({
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

{#snippet imageActionsSnippet(row: (typeof images)[0])}
	<RowActionsMenu
		itemName={row.name ?? row.id}
		busy={isBusy('images', row.id)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					requestDeleteConfirm({
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

{#snippet volumeNameSnippet(row: (typeof volumes)[0])}
	{@render managedName(row.name, row.labels)}
{/snippet}

{#snippet volumeActionsSnippet(row: (typeof volumes)[0])}
	<RowActionsMenu
		itemName={row.name}
		busy={isBusy('volumes', row.name)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					requestDeleteConfirm({
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

{#snippet networkActionsSnippet(row: (typeof networks)[0])}
	<RowActionsMenu
		itemName={row.name}
		busy={isBusy('networks', row.name)}
		actions={[
			{
				label: 'Delete',
				danger: true,
				onselect: () =>
					requestDeleteConfirm({
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

<!-- Tabs array drives the page tabs; the list itself is defined once for Tabs (runtime tabs cannot change) -->

<Modal
	open={!!viewingQuadlet}
	onclose={() => (viewingQuadlet = null)}
	title={viewingQuadlet?.name ?? 'Quadlet Unit'}
	subtitle={`Systemd unit source at ${viewingQuadlet?.path ?? ''}`}
	size="lg"
>
	{#if viewingQuadlet}
		<div class="flex flex-col gap-3">
			<CodeEditor
				value={viewingQuadlet.content}
				language="quadlet"
				readOnly={true}
				height="260px"
			/>
			<div class="flex justify-end">
				<Button variant="secondary" onclick={() => (viewingQuadlet = null)}>Close</Button>
			</div>
		</div>
	{/if}
</Modal>

<Modal
	open={!!logsContainer}
	onclose={() => (logsContainer = null)}
	title={logsContainer?.name ? `Container logs — ${logsContainer.name}` : 'Container logs'}
	subtitle={`Last ${logsTail} lines from ${logsContainer?.image ?? ''}`}
	size="lg"
>
	<div class="flex flex-col gap-3">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="w-[140px]">
				<Select
					value={String(logsTail)}
					onValueChange={(val: any) => {
						if (val) logsTail = Number(val);
					}}
					options={tailOptions}
					size="xs"
				/>
			</div>
			<span class="text-[11px] text-[var(--text-tertiary)]"
				>Fetched on open; select a tail to reload</span
			>
		</div>
		{#if isLogsLoading}
			<div
				class="rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3 text-xs text-[var(--text-tertiary)]"
			>
				Loading logs…
			</div>
		{:else if logsError}
			<div
				class="rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3 text-xs text-[var(--status-red)]"
			>
				{logsError}
			</div>
		{:else}
			<pre
				class="m-0 max-h-[420px] overflow-auto rounded border border-[var(--border-subtle)] bg-[var(--bg-terminal)] p-3 font-mono text-xs leading-relaxed whitespace-pre-wrap text-[var(--text-primary)]">{logsText ||
					'No log output for this container and tail setting.'}</pre>
		{/if}
		<div class="flex justify-end">
			<Button variant="secondary" onclick={() => (logsContainer = null)}>Close</Button>
		</div>
	</div>
</Modal>

<ConfirmDialog
	open={!!requestedConfirm}
	title={requestedConfirm?.title ?? ''}
	description={requestedConfirm?.description}
	variant="danger"
	confirmText="Delete"
	isConfirming={isConfirmBusy}
	matchValue={requestedConfirm?.matchValue}
	matchLabel={requestedConfirm?.matchLabel}
	oncancel={() => (requestedConfirm = null)}
	onconfirm={async () => {
		if (!requestedConfirm) return;
		isConfirmBusy = true;
		try {
			await requestedConfirm.run();
			requestedConfirm = null;
		} finally {
			isConfirmBusy = false;
		}
	}}
/>
