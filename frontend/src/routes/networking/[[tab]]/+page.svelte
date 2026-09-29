<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, DataTable, StatusBadge } from '$lib/components/ui';
	import { domains } from '$lib/data';
	import { Lock, LockOpen } from 'phosphor-svelte';

	let tabParam = $derived(page.params.tab ?? 'domains');
	let activeTab = $state('domains');

	$effect(() => {
		activeTab = tabParam;
	});

	const tabs = [
		{ id: 'domains', label: 'Domains' },
		{ id: 'ports', label: 'Ports' }
	];

	function onTabChange(id: string) {
		goto(`/networking/${id}`, { replaceState: true });
	}
</script>

<svelte:head>
	<title>Networking — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-6">
	<PageHeader title="Networking" subtitle="Domains, TLS certificates, and port mappings." />
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	{#if activeTab === 'domains'}
		<DataTable
			columns={[
				{ key: 'hostname', label: 'Domain' },
				{ key: 'serviceName', label: 'Service' },
				{ key: 'tls', label: 'TLS', render: tlsSnippet },
				{ key: 'status', label: 'Status', render: domainStatusSnippet },
				{ key: 'proxyPort', label: 'Proxy port', mono: true },
				{ key: 'containerPort', label: 'Container port', mono: true }
			]}
			rows={domains}
			keyExtractor={(d) => d.id}
		/>
	{:else}
		<div class="text-[var(--text-tertiary)] py-12 text-center text-xs">
			Port management coming soon.
		</div>
	{/if}
</div>

{#snippet tlsSnippet(row: typeof domains[0])}
	<span class="flex items-center gap-1 text-{row.tls ? '[var(--status-green)]' : '[var(--text-tertiary)]'}">
		{#if row.tls}
			<Lock size={13} /> Active
		{:else}
			<LockOpen size={13} /> None
		{/if}
	</span>
{/snippet}
{#snippet domainStatusSnippet(row: typeof domains[0])}
	<StatusBadge status={row.status} size="sm" />
{/snippet}
