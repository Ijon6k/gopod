<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, Button, CodeEditor } from '$lib/components/ui';
	import {
		LayerCard,
		Badge,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		useKumoToastManager
	} from 'kumo-svelte';
	import { domains, server, dataStore } from '$lib/data';
	import type { Domain } from '$lib/types';
	import { api } from '$lib/api';
	import { DomainSettingsModal, TrafficRequestsView } from '$lib/components/features/networking';
	import {
		Globe,
		Lock,
		LockOpen,
		Plus,
		CheckCircle,
		Warning,
		ArrowSquareOut,
		PencilSimple,
		Trash,
		Copy,
		Check,
		ArrowsClockwise
	} from 'phosphor-svelte';

	const toast = useKumoToastManager();

	let activeTab = $derived(page.params.tab ?? 'domains');

	// Domain Modal states
	let isDomainModalOpen = $state(false);
	let editingDomain = $state<Domain | null>(null);
	let copiedIp = $state(false);

	// Caddy Ingress states
	let caddyfileContent = $state('');
	let caddyStatus = $state('active');
	let caddyAdminUrl = $state('http://localhost:2019');
	let caddyHttpPort = $state(80);
	let caddyHttpsPort = $state(443);
	let isLoadingCaddyfile = $state(false);
	let isReloadingCaddy = $state(false);
	let copiedCaddyfile = $state(false);
	let revalidatingIds = $state<Set<string>>(new Set());

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

	function getDomainUrl(d: Domain): string {
		const host = d.hostname.trim().replace(/^https?:\/\//, '').replace(/:\d+$/, '');
		const path = d.pathPrefix && d.pathPrefix !== '/' ? d.pathPrefix : '';
		const isLocal = host.endsWith('.localhost') || host.endsWith('.local') || host.includes('127.0.0.1') || host === 'localhost';
		if (isLocal) {
			const port = d.tls ? (caddyHttpsPort !== 443 ? `:${caddyHttpsPort}` : '') : (caddyHttpPort !== 80 ? `:${caddyHttpPort}` : '');
			const proto = d.tls ? 'https' : 'http';
			return `${proto}://${host}${port}${path}`;
		}
		return `${d.tls ? 'https' : 'http'}://${host}${path}`;
	}

	async function handleRevalidate(domainId: string) {
		const next = new Set(revalidatingIds);
		next.add(domainId);
		revalidatingIds = next;
		try {
			const updated = await dataStore.revalidateDomain(domainId);
			const isLoopback = updated.hostname.endsWith('.localhost') || updated.hostname.endsWith('.local') || updated.hostname.includes('127.0.0.1') || updated.hostname === 'localhost';
			toast.add({
				title: 'DNS Verified',
				description: isLoopback
					? `${updated.hostname} verified as Local Loopback.`
					: updated.dnsStatus === 'valid'
						? `${updated.hostname} DNS record verified (${server.ip}).`
						: `${updated.hostname} DNS record pending propagation.`,
				variant: updated.dnsStatus === 'valid' || isLoopback ? 'success' : 'warning'
			});
		} catch (err: any) {
			toast.add({
				title: 'Verification Failed',
				description: err?.message || 'Failed to revalidate DNS record.',
				variant: 'error'
			});
		} finally {
			const done = new Set(revalidatingIds);
			done.delete(domainId);
			revalidatingIds = done;
		}
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

	async function copyIp() {
		try {
			await navigator.clipboard.writeText(server.ip);
			copiedIp = true;
			setTimeout(() => (copiedIp = false), 2000);
		} catch {
			// Clipboard API fallback
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
			toast.add({
				title: 'Reload Failed',
				description: err?.message || 'Failed to reload Caddy ingress daemon.',
				variant: 'error'
			});
		} finally {
			isReloadingCaddy = false;
		}
	}

	async function handleCopyCaddyfile() {
		try {
			await navigator.clipboard.writeText(caddyfileContent);
			copiedCaddyfile = true;
			setTimeout(() => (copiedCaddyfile = false), 2000);
		} catch {
			// fallback
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
		<!-- DNS Configuration Banner -->
		<div
			class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-4 sm:flex-row sm:items-center"
		>
			<div class="flex items-start gap-3">
				<div
					class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]"
				>
					<Globe size={18} />
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						Server Public IP for DNS Records
					</span>
					<span class="text-xs leading-relaxed text-[var(--text-tertiary)]">
						Create an <strong class="text-[var(--text-secondary)]">A Record</strong> in your DNS registrar
						pointing to your host IP. Caddy automatically requests and renews Let's Encrypt TLS certificates
						once DNS propagates.
					</span>
				</div>
			</div>

			<div
				class="flex shrink-0 items-center gap-2 self-start rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-1.5 sm:self-auto"
			>
				<span class="font-mono text-xs font-medium text-[var(--text-primary)]">{server.ip}</span>
				<button
					type="button"
					onclick={copyIp}
					class="cursor-pointer rounded border-0 bg-transparent p-0.5 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]"
					title="Copy IP Address"
				>
					{#if copiedIp}
						<Check size={14} class="text-[var(--status-green)]" />
					{:else}
						<Copy size={14} />
					{/if}
				</button>
			</div>
		</div>

		<!-- Domains Table -->
		<LayerCard class="overflow-hidden p-0">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full text-left text-xs">
					<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
						<TableRow>
							<TableHead class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Domain
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Upstream service
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								DNS status
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								TLS certificate
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Features
							</TableHead>
							<TableHead class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Actions
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#if domains.length === 0}
							<TableRow>
								<TableCell colspan={6} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
									No domain routes configured yet. Click "Add Domain" to create your first Caddy
									route.
								</TableCell>
							</TableRow>
						{:else}
							{#each domains as domain, idx (domain.id)}
								<TableRow class={idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]' : 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}>
									<!-- Domain Hostname -->
									<TableCell class="px-3.5 py-3 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{domain.hostname}
											</span>
											{#if domain.pathPrefix && domain.pathPrefix !== '/'}
												<span
													class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--text-tertiary)]"
												>
													{domain.pathPrefix}
												</span>
											{/if}
											<a
												href={getDomainUrl(domain)}
												target="_blank"
												rel="noopener noreferrer"
												class="text-[var(--text-tertiary)] hover:text-[var(--accent)]"
												title="Open in new tab"
											>
												<ArrowSquareOut size={13} />
											</a>
										</div>
									</TableCell>

									<!-- Upstream Service & Container Port -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										<div class="flex flex-col">
											<span class="font-medium text-[var(--text-primary)]">
												{domain.serviceName}
											</span>
											<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
												{domain.projectId} · port {domain.containerPort}
											</span>
										</div>
									</TableCell>

									<!-- DNS Status -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										{#if domain.hostname.endsWith('.localhost') || domain.hostname.endsWith('.local') || domain.hostname.includes('127.0.0.1') || domain.hostname === 'localhost'}
											<Badge variant="success" class="gap-1 text-xs">
												<CheckCircle size={12} /> Local Loopback
											</Badge>
										{:else if domain.dnsStatus === 'valid'}
											<Badge variant="success" class="gap-1 text-xs">
												<CheckCircle size={12} /> Resolves ({server.ip})
											</Badge>
										{:else}
											<Badge variant="warning" class="gap-1 text-xs">
												<Warning size={12} /> Pending DNS
											</Badge>
										{/if}
									</TableCell>

									<!-- TLS Certificate -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										{#if domain.tls}
											<span
												class="inline-flex items-center gap-1.5 text-xs text-[var(--status-green)]"
											>
												<Lock size={13} /> Let's Encrypt ACME
											</span>
										{:else}
											<span
												class="inline-flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]"
											>
												<LockOpen size={13} /> Plain HTTP
											</span>
										{/if}
									</TableCell>

									<!-- Ingress Features Pills -->
									<TableCell class="px-3 py-3">
										<div class="flex flex-wrap items-center gap-1">
											{#if domain.websocket}
												<Badge variant="neutral" class="font-mono text-[10.5px]">
													WS 101
												</Badge>
											{/if}
											{#if domain.cors}
												<Badge variant="neutral" class="font-mono text-[10.5px]">
													CORS
												</Badge>
											{/if}
											{#if domain.hsts}
												<Badge variant="neutral" class="font-mono text-[10.5px]">
													HSTS
												</Badge>
											{/if}
											{#if domain.basicAuth}
												<Badge variant="warning" class="font-mono text-[10.5px]">
													Basic auth
												</Badge>
											{/if}
											{#if !domain.websocket && !domain.cors && !domain.basicAuth && !domain.hsts}
												<span class="text-[11px] text-[var(--text-tertiary)]">Standard</span>
											{/if}
										</div>
									</TableCell>

									<!-- Actions -->
									<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
										<div class="inline-flex items-center gap-1">
											<button
												type="button"
												onclick={() => handleRevalidate(domain.id)}
												disabled={revalidatingIds.has(domain.id)}
												class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:opacity-50"
												title="Revalidate DNS Record"
												aria-label="Revalidate DNS Record"
											>
												<ArrowsClockwise size={14} class={revalidatingIds.has(domain.id) ? 'animate-spin' : ''} />
											</button>
											<button
												type="button"
												onclick={() => handleEditDomain(domain)}
												class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
												title="Edit Domain Settings"
												aria-label="Edit Domain Settings"
											>
												<PencilSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteDomain(domain.id)}
												class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
												title="Remove Domain"
												aria-label="Remove Domain"
											>
												<Trash size={14} />
											</button>
										</div>
									</TableCell>
								</TableRow>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>

			<!-- Footer -->
			<div
				class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5 text-xs text-[var(--text-tertiary)]"
			>
				<span>{domains.length} active domain routing configurations</span>
				<span class="font-mono text-[11px]">Engine: Caddy v{server.caddy || '2.8'} (Rootless Ingress)</span>
			</div>
		</LayerCard>

		<!-- TAB 2: TRAFFIC & REQUESTS TELEMETRY -->
	{:else if activeTab === 'requests'}
		<TrafficRequestsView />

		<!-- TAB 3: HOST PORT BINDINGS -->
	{:else if activeTab === 'ports'}
		<LayerCard class="overflow-hidden p-0">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<Table class="w-full text-left text-xs">
					<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
						<TableRow>
							<TableHead class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Host port
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Protocol
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Target service / container
							</TableHead>
							<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Bind address
							</TableHead>
							<TableHead class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
								Status
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody class="divide-y divide-[var(--border-subtle)]">
						{#if dataStore.ports.length === 0}
							<TableRow>
								<TableCell colspan={5} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
									No host port bindings currently active. Host port bindings appear when services or Quadlet containers specify published ports (e.g., -p 8080:80 or PublishPort=...).
								</TableCell>
							</TableRow>
						{:else}
							{#each dataStore.ports as p, idx (p.id)}
								<TableRow class={idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]' : 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}>
									<!-- Host Port -->
									<TableCell class="px-3.5 py-3 whitespace-nowrap">
										<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
											{p.bindAddress === 'Internal network' ? `Internal (${p.containerPort})` : `:${p.hostPort}`}
										</span>
									</TableCell>

									<!-- Protocol -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										<Badge variant="neutral" class="font-mono text-[10.5px] uppercase">
											{p.protocol}
										</Badge>
									</TableCell>

									<!-- Target Service -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										<div class="flex flex-col">
											<span class="font-medium text-[var(--text-primary)]">{p.serviceName}</span>
											<span class="font-mono text-[11px] text-[var(--text-tertiary)]"
												>{p.containerName} ➔ :{p.containerPort}</span
											>
										</div>
									</TableCell>

									<!-- Binding Scope -->
									<TableCell class="px-3 py-3 whitespace-nowrap">
										{#if p.isPublic}
											<span
												class="inline-flex items-center gap-1 font-mono text-[11px] font-medium text-[var(--accent)]"
											>
												{p.bindAddress} (Public ingress)
											</span>
										{:else}
											<span
												class="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--text-tertiary)]"
											>
												{p.bindAddress} (Local / Bridge)
											</span>
										{/if}
									</TableCell>

									<!-- Status -->
									<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
										<Badge variant={p.status === 'listening' ? 'success' : 'neutral'} class="gap-1.5 text-xs capitalize">
											{p.status || 'Listening'}
										</Badge>
									</TableCell>
								</TableRow>
							{/each}
						{/if}
					</TableBody>
				</Table>
			</div>

			<!-- Footer -->
			<div
				class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5 text-xs text-[var(--text-tertiary)]"
			>
				<span>{dataStore.ports.length} ports mapped on host network</span>
				<span class="font-mono text-[11px]">Kernel Netfilter / pasta rootless bridge</span>
			</div>
		</LayerCard>

		<!-- TAB 4: CADDY INGRESS ENGINE & CADDYFILE -->
	{:else if activeTab === 'caddy'}
		<!-- Caddy Ingress Architecture & Control -->
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<!-- Ingress Status -->
			<LayerCard class="flex flex-col gap-2 p-4">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Ingress Daemon</span>
					<Badge variant={caddyStatus === 'error' ? 'error' : 'success'} class="gap-1.5 text-xs">
						<CheckCircle size={12} />
						{caddyStatus === 'error' ? 'Degraded' : 'Active (Rootless)'}
					</Badge>
				</div>
				<div class="mt-1 flex items-baseline gap-2">
					<span class="text-lg font-semibold text-[var(--text-primary)]">Caddy v{server.caddy || '2.8'}</span>
					<span class="font-mono text-xs text-[var(--text-tertiary)]">rootless socket</span>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">
					Bound to host ports <span class="font-mono text-[11px] text-[var(--text-secondary)]">:{caddyHttpPort}</span> (HTTP) & <span class="font-mono text-[11px] text-[var(--text-secondary)]">:{caddyHttpsPort}</span> (HTTPS)
				</span>
			</LayerCard>

			<!-- Admin API -->
			<LayerCard class="flex flex-col gap-2 p-4">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Admin API Engine</span>
					<Badge variant="neutral" class="font-mono text-[10.5px]">
						Internal Bridge
					</Badge>
				</div>
				<div class="mt-1 font-mono text-xs font-medium text-[var(--text-primary)]">
					{caddyAdminUrl || 'http://localhost:2019'}
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">
					Isolated inside private bridge network <span class="font-mono text-[11px] text-[var(--text-secondary)]">gopod-net</span>
				</span>
			</LayerCard>

			<!-- TLS & Security Policy -->
			<LayerCard class="flex flex-col gap-2 p-4">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">TLS Automation</span>
					<Badge variant="success" class="gap-1 text-xs">
						<Lock size={12} /> ACME v2
					</Badge>
				</div>
				<div class="mt-1 flex items-baseline gap-2">
					<span class="text-sm font-semibold text-[var(--text-primary)]">Let's Encrypt + Internal</span>
				</div>
				<span class="text-xs text-[var(--text-tertiary)]">
					Dual HTTP/HTTPS local development + automatic ACME on public FQDNs
				</span>
			</LayerCard>
		</div>

		<!-- Active Caddyfile Viewer Card -->
		<LayerCard class="overflow-hidden p-0 shadow-xs">
			<div class="flex flex-col justify-between gap-3 border-b border-[var(--border)] bg-[var(--bg-surface)] px-4 py-3 sm:flex-row sm:items-center">
				<div class="flex flex-col gap-0.5">
					<div class="flex items-center gap-2">
						<span class="text-sm font-semibold text-[var(--text-primary)]">Active Caddyfile</span>
						<Badge variant="neutral" class="font-mono text-[11px]">Auto-reconciled</Badge>
					</div>
					<span class="text-xs text-[var(--text-tertiary)]">
						Declaratively generated by GOPOD Ingress Reconciler and synchronized via Caddy Admin API.
					</span>
				</div>

				<div class="flex items-center gap-2 self-start sm:self-auto">
					<Button
						variant="secondary"
						size="sm"
						onclick={handleCopyCaddyfile}
						disabled={!caddyfileContent}
						class="gap-1.5"
					>
						{#if copiedCaddyfile}
							<Check size={13} class="text-[var(--status-green)]" /> Copied
						{:else}
							<Copy size={13} /> Copy Caddyfile
						{/if}
					</Button>

					<Button
						variant="primary"
						size="sm"
						onclick={handleReloadCaddy}
						disabled={isReloadingCaddy}
						class="gap-1.5"
					>
						<ArrowsClockwise size={13} class={isReloadingCaddy ? 'animate-spin' : ''} />
						{isReloadingCaddy ? 'Reloading...' : 'Reload Ingress'}
					</Button>
				</div>
			</div>

			<!-- Code Block / Editor -->
			<div class="w-full bg-[var(--bg-panel)] p-2">
				{#if isLoadingCaddyfile}
					<div class="flex h-80 items-center justify-center text-xs text-[var(--text-tertiary)]">
						<ArrowsClockwise size={18} class="mr-2 animate-spin text-[var(--accent)]" /> Loading current Caddyfile...
					</div>
				{:else if !caddyfileContent}
					<div class="flex h-80 flex-col items-center justify-center gap-2 text-center text-xs text-[var(--text-tertiary)]">
						<span>No active Caddyfile loaded or Caddy ingress container offline.</span>
						<Button variant="secondary" size="xs" onclick={loadCaddyfile}>Retry</Button>
					</div>
				{:else}
					<CodeEditor
						value={caddyfileContent}
						language="caddyfile"
						title="Caddyfile"
						readOnly={true}
						height="auto"
						lineNumbers={true}
					/>
				{/if}
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2 text-xs text-[var(--text-tertiary)]">
				<span class="text-[11px]">Caddy reverse proxy and automatic HTTPS configuration</span>
				<span class="font-mono text-[11px]">/etc/caddy/Caddyfile</span>
			</div>
		</LayerCard>
	{/if}
</div>

<!-- Domain Modal -->
<DomainSettingsModal
	bind:open={isDomainModalOpen}
	domain={editingDomain}
	onclose={() => (isDomainModalOpen = false)}
	onsave={handleSaveDomain}
/>
