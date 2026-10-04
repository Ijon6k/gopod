<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs, StatusBadge } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { domains, server, ports, dataStore } from '$lib/data';
	import type { Domain } from '$lib/types';
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
		WifiHigh,
		ArrowsLeftRight,
		ShieldCheck,
		ClockCounterClockwise
	} from 'phosphor-svelte';

	let tabParam = $derived(page.params.tab ?? 'domains');
	let activeTab = $state('domains');

	// Domain Modal states
	let isDomainModalOpen = $state(false);
	let editingDomain = $state<Domain | null>(null);
	let copiedIp = $state(false);

	$effect(() => {
		activeTab = tabParam;
	});

	const tabs = [
		{ id: 'domains', label: 'Domains & Ingress' },
		{ id: 'requests', label: 'Traffic & Requests' },
		{ id: 'ports', label: 'Host Port Bindings' }
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
</script>

<svelte:head>
	<title>Networking — GOPOD</title>
</svelte:head>

<div class="w-full flex flex-col gap-6">
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
			{/if}
		{/snippet}
	</PageHeader>

	<!-- Tab Navigation -->
	<Tabs {tabs} bind:active={activeTab} onchange={onTabChange} />

	<!-- TAB 1: DOMAINS & INGRESS -->
	{#if activeTab === 'domains'}
		<!-- DNS Configuration Banner -->
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] flex flex-col sm:flex-row sm:items-center justify-between gap-3">
			<div class="flex items-start gap-3">
				<div class="w-8 h-8 rounded-lg bg-[var(--accent-muted)] text-[var(--accent)] flex items-center justify-center shrink-0 mt-0.5">
					<Globe size={18} />
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						Server Public IP for DNS Records
					</span>
					<span class="text-xs text-[var(--text-tertiary)] leading-relaxed">
						Create an <strong class="text-[var(--text-secondary)]">A Record</strong> in your DNS registrar pointing to your host IP. Caddy automatically requests and renews Let's Encrypt TLS certificates once DNS propagates.
					</span>
				</div>
			</div>

			<div class="flex items-center gap-2 self-start sm:self-auto shrink-0 bg-[var(--bg-panel)] px-3 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)]">
				<span class="font-mono text-xs text-[var(--text-primary)] font-medium">{server.ip}</span>
				<button
					type="button"
					onclick={copyIp}
					class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer p-0.5 rounded transition-colors"
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
		<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)]">
						<tr>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Domain</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Upstream Service</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">DNS Status</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">TLS Certificate</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Features</th>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)] text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if domains.length === 0}
							<tr>
								<td colspan="6" class="py-12 text-center text-[var(--text-tertiary)] text-xs">
									No domain routes configured yet. Click "Add Domain" to create your first Caddy route.
								</td>
							</tr>
						{:else}
							{#each domains as domain (domain.id)}
								<tr class="hover:bg-[var(--bg-table-row-alt)] transition-colors">
									<!-- Domain Hostname -->
									<td class="py-3 px-3.5 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{domain.hostname}
											</span>
											{#if domain.pathPrefix && domain.pathPrefix !== '/'}
												<span class="font-mono text-[10.5px] text-[var(--text-tertiary)] bg-[var(--bg-panel)] px-1.5 py-0.5 rounded border border-[var(--border-subtle)]">
													{domain.pathPrefix}
												</span>
											{/if}
											<a
												href={`http${domain.tls ? 's' : ''}://${domain.hostname}${domain.pathPrefix || ''}`}
												target="_blank"
												rel="noopener noreferrer"
												class="text-[var(--text-tertiary)] hover:text-[var(--accent)] transition-colors"
												title="Open in new tab"
											>
												<ArrowSquareOut size={13} />
											</a>
										</div>
									</td>

									<!-- Upstream Service & Container Port -->
									<td class="py-3 px-3 whitespace-nowrap">
										<div class="flex flex-col">
											<span class="text-[var(--text-primary)] font-medium">
												{domain.serviceName}
											</span>
											<span class="text-[11px] font-mono text-[var(--text-tertiary)]">
												{domain.projectId} · port {domain.containerPort}
											</span>
										</div>
									</td>

									<!-- DNS Status -->
									<td class="py-3 px-3 whitespace-nowrap">
										{#if domain.dnsStatus === 'pending'}
											<span class="inline-flex items-center gap-1 text-[11px] font-medium text-[var(--status-amber)] bg-[var(--status-amber-muted)] px-2 py-0.5 rounded border border-[var(--status-amber)]/30">
												<Warning size={12} /> Pending DNS
											</span>
										{:else}
											<span class="inline-flex items-center gap-1 text-[11px] font-medium text-[var(--status-green)] bg-[var(--status-green-muted)] px-2 py-0.5 rounded border border-[var(--status-green)]/30">
												<CheckCircle size={12} /> Resolves ({server.ip})
											</span>
										{/if}
									</td>

									<!-- TLS Certificate -->
									<td class="py-3 px-3 whitespace-nowrap">
										{#if domain.tls}
											<span class="inline-flex items-center gap-1.5 text-xs text-[var(--status-green)]">
												<Lock size={13} /> Let's Encrypt ACME
											</span>
										{:else}
											<span class="inline-flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
												<LockOpen size={13} /> Plain HTTP
											</span>
										{/if}
									</td>

									<!-- Ingress Features Pills -->
									<td class="py-3 px-3">
										<div class="flex flex-wrap items-center gap-1">
											{#if domain.websocket}
												<span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--accent-muted)] text-[var(--accent)] font-medium">
													WS 101
												</span>
											{/if}
											{#if domain.cors}
												<span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--bg-panel)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">
													CORS
												</span>
											{/if}
											{#if domain.hsts}
												<span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--bg-panel)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">
													HSTS
												</span>
											{/if}
											{#if domain.basicAuth}
												<span class="px-1.5 py-0.5 rounded text-[10px] font-mono bg-[var(--status-amber-muted)] text-[var(--status-amber)] font-medium">
													Basic Auth
												</span>
											{/if}
											{#if !domain.websocket && !domain.cors && !domain.basicAuth && !domain.hsts}
												<span class="text-[11px] text-[var(--text-tertiary)]">Standard</span>
											{/if}
										</div>
									</td>

									<!-- Actions -->
									<td class="py-3 px-3.5 whitespace-nowrap text-right">
										<div class="inline-flex items-center gap-1">
											<button
												type="button"
												onclick={() => handleEditDomain(domain)}
												class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
												title="Edit Domain Settings"
											>
												<PencilSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteDomain(domain.id)}
												class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--status-red)] hover:bg-[var(--status-red-muted)] border-0 bg-transparent cursor-pointer transition-colors"
												title="Remove Domain"
											>
												<Trash size={14} />
											</button>
										</div>
									</td>
								</tr>
							{/each}
						{/if}
					</tbody>
				</table>
			</div>

			<!-- Footer -->
			<div class="px-4 py-2.5 border-t border-[var(--border)] bg-[var(--bg-table-header)] flex items-center justify-between text-xs text-[var(--text-tertiary)]">
				<span>{domains.length} active domain routing configurations</span>
				<span class="text-[11px] font-mono">Engine: Caddy v{server.caddy} (Rootless Ingress)</span>
			</div>
		</div>

	<!-- TAB 2: TRAFFIC & REQUESTS TELEMETRY (DOKPLOY PARITY) -->
	{:else if activeTab === 'requests'}
		<TrafficRequestsView />

	<!-- TAB 3: HOST PORT BINDINGS -->
	{:else if activeTab === 'ports'}
		<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)]">
						<tr>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Host Port</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Protocol</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Target Service / Container</th>
							<th class="py-2.5 px-3 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)]">Bind Address</th>
							<th class="py-2.5 px-3.5 font-medium text-[11px] uppercase tracking-wider text-[var(--text-tertiary)] text-right">Status</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#each ports as p (p.id)}
							<tr class="hover:bg-[var(--bg-table-row-alt)] transition-colors">
								<!-- Host Port -->
								<td class="py-3 px-3.5 whitespace-nowrap">
									<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
										:{p.hostPort}
									</span>
								</td>

								<!-- Protocol -->
								<td class="py-3 px-3 whitespace-nowrap">
									<span class="font-mono text-[10.5px] uppercase font-medium px-1.5 py-0.5 rounded bg-[var(--bg-panel)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">
										{p.protocol}
									</span>
								</td>

								<!-- Target Service -->
								<td class="py-3 px-3 whitespace-nowrap">
									<div class="flex flex-col">
										<span class="font-medium text-[var(--text-primary)]">{p.serviceName}</span>
										<span class="text-[11px] font-mono text-[var(--text-tertiary)]">{p.containerName} ➔ :{p.containerPort}</span>
									</div>
								</td>

								<!-- Binding Scope -->
								<td class="py-3 px-3 whitespace-nowrap">
									{#if p.isPublic}
										<span class="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--accent)] font-medium">
											{p.bindAddress} (Public Ingress)
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--text-tertiary)]">
											{p.bindAddress} (Local Only)
										</span>
									{/if}
								</td>

								<!-- Status -->
								<td class="py-3 px-3.5 whitespace-nowrap text-right">
									<span class="inline-flex items-center gap-1.5 text-xs text-[var(--status-green)] font-medium">
										<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-green)]"></span>
										Listening
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Footer -->
			<div class="px-4 py-2.5 border-t border-[var(--border)] bg-[var(--bg-table-header)] flex items-center justify-between text-xs text-[var(--text-tertiary)]">
				<span>{ports.length} ports mapped on host network</span>
				<span class="text-[11px] font-mono">Kernel Netfilter / pasta rootless bridge</span>
			</div>
		</div>
	{/if}
</div>

<!-- Domain Modal -->
<DomainSettingsModal
	bind:open={isDomainModalOpen}
	domain={editingDomain}
	onclose={() => (isDomainModalOpen = false)}
	onsave={handleSaveDomain}
/>
