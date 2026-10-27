<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, Button } from '$lib/components/ui';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { server, dataStore } from '$lib/data';
	import type { Domain } from '$lib/types';
	import { api } from '$lib/api';
	import { DomainSettingsModal, TrafficRequestsView } from '$lib/components/features/networking';
	import DomainsTabContent from '$lib/components/features/networking/tabs/DomainsTabContent.svelte';
	import PortBindingsTabContent from '$lib/components/features/networking/tabs/PortBindingsTabContent.svelte';
	import CaddyIngressTabContent from '$lib/components/features/networking/tabs/CaddyIngressTabContent.svelte';
	import { Plus, ArrowsClockwise } from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let activeTab = $derived(page.params.tab ?? 'domains');

	// Domain Modal states
	let isDomainModalOpen = $state(false);
	let editingDomain = $state<Domain | null>(null);

	// Caddy Ingress states
	let caddyfileContent = $state('');
	let caddyStatus = $state('active');
	let caddyAdminUrl = $state('http://localhost:2019');
	let caddyHttpPort = $state(80);
	let caddyHttpsPort = $state(443);
	let isLoadingCaddyfile = $state(false);
	let isReloadingCaddy = $state(false);

	const tabs = [
		{ id: 'domains', label: 'Domains & Ingress' },
		{ id: 'requests', label: 'Traffic & Requests' },
		{ id: 'ports', label: 'Host Port Bindings' },
		{ id: 'caddy', label: 'Caddy Ingress' }
	];

	function onTabChange(id: string) {
		goto(`/networking/${id}`, { replaceState: true });
	}

	function handleAddDomain() {
		editingDomain = null;
		isDomainModalOpen = true;
	}

	function handleEditDomain(d: Domain) {
		editingDomain = d;
		isDomainModalOpen = true;
	}

	function handleSaveDomain(savedDomain: Domain) {
		let cleanHost = savedDomain.hostname.trim().toLowerCase();
		cleanHost = cleanHost.replace(/^https?:\/\//, '');
		cleanHost = cleanHost.replace(/:\d+$/, '');
		savedDomain.hostname = cleanHost;

		if (editingDomain) {
			dataStore.updateDomain(savedDomain);
		} else {
			dataStore.addDomain(savedDomain);
		}
		isDomainModalOpen = false;
	}

	function handleDeleteDomain(id: string) {
		if (confirm('Are you sure you want to remove this domain routing rule?')) {
			dataStore.deleteDomain(id);
		}
	}

	async function loadCaddyfile() {
		isLoadingCaddyfile = true;
		try {
			const res = await api.domains.getCaddyfile();
			caddyfileContent = res.caddyfile || '';
			caddyStatus = res.status || 'active';
			caddyAdminUrl = res.adminUrl || 'http://localhost:2019';
			if (res.httpPort) caddyHttpPort = res.httpPort;
			if (res.httpsPort) caddyHttpsPort = res.httpsPort;
		} catch (err: any) {
			console.error('Failed to load Caddyfile:', err);
			caddyStatus = 'error';
		} finally {
			isLoadingCaddyfile = false;
		}
	}

	async function handleReloadCaddy() {
		isReloadingCaddy = true;
		try {
			const res = await api.domains.reload();
			toast.add({
				title: 'Ingress Reloaded',
				description: res.message || 'Caddy configuration dynamically reloaded via Admin API.',
				variant: 'success'
			});
			await loadCaddyfile();
		} catch (err: any) {
			toast.add(
				errorToastOptions('Reload Failed', err?.message || 'Failed to reload Caddy ingress daemon.')
			);
		} finally {
			isReloadingCaddy = false;
		}
	}

	$effect(() => {
		// Initial fetch of ingress ports & metadata
		if (!caddyfileContent && !isLoadingCaddyfile) {
			loadCaddyfile();
		}
		if (activeTab === 'ports' && dataStore.ports.length === 0) {
			dataStore.fetchLiveStats();
		}
	});
</script>

<svelte:head>
	<title>Networking — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<!-- Page Header -->
	<PageHeader
		title="Networking & Ingress"
		subtitle="Caddy reverse proxy, Let's Encrypt certificates, DNS validation, and port bindings."
	>
		{#snippet actions()}
			{#if activeTab === 'domains'}
				<Button variant="primary" size="sm" onclick={handleAddDomain}>
					<Plus size={13} /> Add Domain
				</Button>
			{:else if activeTab === 'caddy'}
				<Button
					variant="secondary"
					size="sm"
					onclick={handleReloadCaddy}
					disabled={isReloadingCaddy}
					class="gap-1.5"
				>
					<ArrowsClockwise size={13} class={isReloadingCaddy ? 'animate-spin' : ''} />
					{isReloadingCaddy ? 'Reloading...' : 'Reload Ingress'}
				</Button>
			{/if}
		{/snippet}
	</PageHeader>

	<!-- Tab Navigation -->
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	<!-- TAB 1: DOMAINS & INGRESS -->
	{#if activeTab === 'domains'}
		<DomainsTabContent
			{server}
			{caddyHttpPort}
			{caddyHttpsPort}
			onEditDomain={handleEditDomain}
			onDeleteDomain={handleDeleteDomain}
		/>

		<!-- TAB 2: TRAFFIC & REQUESTS TELEMETRY -->
	{:else if activeTab === 'requests'}
		<TrafficRequestsView />

		<!-- TAB 3: HOST PORT BINDINGS -->
	{:else if activeTab === 'ports'}
		<PortBindingsTabContent />

		<!-- TAB 4: CADDY INGRESS ENGINE & CADDYFILE -->
	{:else if activeTab === 'caddy'}
		<CaddyIngressTabContent
			{server}
			{caddyStatus}
			{caddyAdminUrl}
			{caddyHttpPort}
			{caddyHttpsPort}
			{caddyfileContent}
			{isLoadingCaddyfile}
			{isReloadingCaddy}
			onReloadCaddy={handleReloadCaddy}
			onRetryLoadCaddyfile={loadCaddyfile}
		/>
	{/if}
</div>

<!-- Domain Modal -->
<DomainSettingsModal
	bind:open={isDomainModalOpen}
	domain={editingDomain}
	onclose={() => (isDomainModalOpen = false)}
	onsave={handleSaveDomain}
/>
