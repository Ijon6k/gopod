<script lang="ts">
	import { PageHeader, Tabs, DataTable, StatusBadge } from '$lib/components/ui';
	import { auditLogs } from '$lib/data';

	let activeCategory = $state('all');

	const tabs = [
		{ id: 'all', label: 'All Events' },
		{ id: 'deployment', label: 'Deployments' },
		{ id: 'runtime', label: 'Runtime' },
		{ id: 'settings', label: 'Settings' },
		{ id: 'security', label: 'Security' }
	];

	let filteredLogs = $derived(
		activeCategory === 'all'
			? auditLogs
			: auditLogs.filter((log) => log.category === activeCategory)
	);
</script>

<svelte:head>
	<title>Audit Log — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-6">
	<PageHeader
		title="Audit Log"
		subtitle="Security events, infrastructure operations, and operator activity history."
	/>

	<Tabs {tabs} bind:active={activeCategory} />

	<DataTable
		columns={[
			{ key: 'action', label: 'Action', render: actionSnippet },
			{ key: 'actor', label: 'Operator' },
			{ key: 'target', label: 'Target' },
			{ key: 'ip', label: 'Source IP', mono: true },
			{ key: 'status', label: 'Status', render: statusSnippet },
			{ key: 'timeAgo', label: 'When' }
		]}
		rows={filteredLogs}
		keyExtractor={(d) => d.id}
	/>
</div>

{#snippet actionSnippet(row: typeof auditLogs[0])}
	<span class="font-mono text-base text-[var(--text-primary)] font-medium">
		{row.action}
	</span>
{/snippet}

{#snippet statusSnippet(row: typeof auditLogs[0])}
	<StatusBadge status={row.status} size="sm" />
{/snippet}
