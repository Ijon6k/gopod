<script lang="ts">
	import { onMount } from 'svelte';
	import { PageHeader, Tabs, DataTable, StatusBadge } from '$lib/components/ui';
	import { api } from '$lib/api';
	import type { AuditLog } from '$lib/types';
	import { ArrowClockwise } from 'phosphor-svelte';

	let activeCategory = $state('all');
	let logs = $state<AuditLog[]>([]);
	let loading = $state(true);
	let isRefreshing = $state(false);

	const tabs = [
		{ id: 'all', label: 'All Events' },
		{ id: 'deployment', label: 'Deployments' },
		{ id: 'runtime', label: 'Runtime' },
		{ id: 'settings', label: 'Settings' },
		{ id: 'security', label: 'Security' }
	];

	async function fetchAuditLogs() {
		try {
			const res = await api.audit.list();
			if (Array.isArray(res)) {
				logs = res;
			}
		} catch (err) {
			console.error('Failed to load audit logs:', err);
		} finally {
			loading = false;
			isRefreshing = false;
		}
	}

	function handleRefresh() {
		isRefreshing = true;
		fetchAuditLogs();
	}

	onMount(() => {
		fetchAuditLogs();
	});

	let filteredLogs = $derived(
		activeCategory === 'all' ? logs : logs.filter((log) => log.category === activeCategory)
	);
</script>

<svelte:head>
	<title>Audit Log — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<div class="flex items-center justify-between">
		<PageHeader
			title="Audit Log"
			subtitle="Security events, infrastructure operations, and operator activity history."
		/>
		<button
			type="button"
			onclick={handleRefresh}
			disabled={isRefreshing}
			class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 text-xs text-[var(--text-secondary)] shadow-xs transition-colors hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
		>
			<ArrowClockwise size={14} class={isRefreshing ? 'animate-spin' : ''} />
			<span>Refresh</span>
		</button>
	</div>

	<Tabs {tabs} bind:active={activeCategory} />

	{#if loading && logs.length === 0}
		<div
			class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-8 text-center text-sm text-[var(--text-tertiary)]"
		>
			Loading audit events from host daemon...
		</div>
	{:else}
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
	{/if}
</div>

{#snippet actionSnippet(row: AuditLog)}
	<span class="font-mono text-base font-medium text-[var(--text-primary)]">
		{row.action}
	</span>
{/snippet}

{#snippet statusSnippet(row: AuditLog)}
	<StatusBadge
		status={row.status === 'success' ? 'healthy' : row.status}
		label={row.status}
		size="sm"
	/>
{/snippet}
