<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, Tabs } from '$lib/components/ui';
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
		Check
	} from 'phosphor-svelte';

	let activeTab = $derived(page.params.tab ?? 'domains');

	// Domain Modal states
	let isDomainModalOpen = $state(false);
	let editingDomain = $state<Domain | null>(null);
	let copiedIp = $state(false);

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
		<div
			class="overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)]"
		>
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<tr>
							<th
								class="px-3.5 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Domain</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Upstream Service</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>DNS Status</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>TLS Certificate</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Features</th
							>
							<th
								class="px-3.5 py-2.5 text-right text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Actions</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#if domains.length === 0}
							<tr>
								<td colspan="6" class="py-12 text-center text-xs text-[var(--text-tertiary)]">
									No domain routes configured yet. Click "Add Domain" to create your first Caddy
									route.
								</td>
							</tr>
						{:else}
							{#each domains as domain (domain.id)}
								<tr class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
									<!-- Domain Hostname -->
									<td class="px-3.5 py-3 whitespace-nowrap">
										<div class="flex items-center gap-2">
											<span class="font-mono text-xs font-medium text-[var(--text-primary)]">
												{domain.hostname}
											</span>
											{#if domain.pathPrefix && domain.pathPrefix !== '/'}
												<span
													class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10.5px] text-[var(--text-tertiary)]"
												>
													{domain.pathPrefix}
												</span>
											{/if}
											<a
												href={`http${domain.tls ? 's' : ''}://${domain.hostname}${domain.pathPrefix || ''}`}
												target="_blank"
												rel="noopener noreferrer"
												class="text-[var(--text-tertiary)] transition-colors hover:text-[var(--accent)]"
												title="Open in new tab"
											>
												<ArrowSquareOut size={13} />
											</a>
										</div>
									</td>

									<!-- Upstream Service & Container Port -->
									<td class="px-3 py-3 whitespace-nowrap">
										<div class="flex flex-col">
											<span class="font-medium text-[var(--text-primary)]">
												{domain.serviceName}
											</span>
											<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
												{domain.projectId} · port {domain.containerPort}
											</span>
										</div>
									</td>

									<!-- DNS Status -->
									<td class="px-3 py-3 whitespace-nowrap">
										{#if domain.dnsStatus === 'pending'}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-amber)]/30 bg-[var(--status-amber-muted)] px-2 py-0.5 text-[11px] font-medium text-[var(--status-amber)]"
											>
												<Warning size={12} /> Pending DNS
											</span>
										{:else}
											<span
												class="inline-flex items-center gap-1 rounded border border-[var(--status-green)]/30 bg-[var(--status-green-muted)] px-2 py-0.5 text-[11px] font-medium text-[var(--status-green)]"
											>
												<CheckCircle size={12} /> Resolves ({server.ip})
											</span>
										{/if}
									</td>

									<!-- TLS Certificate -->
									<td class="px-3 py-3 whitespace-nowrap">
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
									</td>

									<!-- Ingress Features Pills -->
									<td class="px-3 py-3">
										<div class="flex flex-wrap items-center gap-1">
											{#if domain.websocket}
												<span
													class="rounded bg-[var(--accent-muted)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--accent)]"
												>
													WS 101
												</span>
											{/if}
											{#if domain.cors}
												<span
													class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-secondary)]"
												>
													CORS
												</span>
											{/if}
											{#if domain.hsts}
												<span
													class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-secondary)]"
												>
													HSTS
												</span>
											{/if}
											{#if domain.basicAuth}
												<span
													class="rounded bg-[var(--status-amber-muted)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--status-amber)]"
												>
													Basic Auth
												</span>
											{/if}
											{#if !domain.websocket && !domain.cors && !domain.basicAuth && !domain.hsts}
												<span class="text-[11px] text-[var(--text-tertiary)]">Standard</span>
											{/if}
										</div>
									</td>

									<!-- Actions -->
									<td class="px-3.5 py-3 text-right whitespace-nowrap">
										<div class="inline-flex items-center gap-1">
											<button
												type="button"
												onclick={() => handleEditDomain(domain)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
												title="Edit Domain Settings"
											>
												<PencilSimple size={14} />
											</button>
											<button
												type="button"
												onclick={() => handleDeleteDomain(domain.id)}
												class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--status-red-muted)] hover:text-[var(--status-red)]"
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
			<div
				class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5 text-xs text-[var(--text-tertiary)]"
			>
				<span>{domains.length} active domain routing configurations</span>
				<span class="font-mono text-[11px]">Engine: Caddy v{server.caddy} (Rootless Ingress)</span>
			</div>
		</div>

		<!-- TAB 2: TRAFFIC & REQUESTS TELEMETRY (DOKPLOY PARITY) -->
	{:else if activeTab === 'requests'}
		<TrafficRequestsView />

		<!-- TAB 3: HOST PORT BINDINGS -->
	{:else if activeTab === 'ports'}
		<div
			class="overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)]"
		>
			<div class="w-full overflow-x-auto md:overflow-x-visible">
				<table class="w-full border-collapse text-left text-xs">
					<thead
						class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
					>
						<tr>
							<th
								class="px-3.5 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Host Port</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Protocol</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Target Service / Container</th
							>
							<th
								class="px-3 py-2.5 text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Bind Address</th
							>
							<th
								class="px-3.5 py-2.5 text-right text-[11px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
								>Status</th
							>
						</tr>
					</thead>
					<tbody class="divide-y divide-[var(--border-subtle)]">
						{#each ports as p (p.id)}
							<tr class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
								<!-- Host Port -->
								<td class="px-3.5 py-3 whitespace-nowrap">
									<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
										:{p.hostPort}
									</span>
								</td>

								<!-- Protocol -->
								<td class="px-3 py-3 whitespace-nowrap">
									<span
										class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10.5px] font-medium text-[var(--text-secondary)] uppercase"
									>
										{p.protocol}
									</span>
								</td>

								<!-- Target Service -->
								<td class="px-3 py-3 whitespace-nowrap">
									<div class="flex flex-col">
										<span class="font-medium text-[var(--text-primary)]">{p.serviceName}</span>
										<span class="font-mono text-[11px] text-[var(--text-tertiary)]"
											>{p.containerName} ➔ :{p.containerPort}</span
										>
									</div>
								</td>

								<!-- Binding Scope -->
								<td class="px-3 py-3 whitespace-nowrap">
									{#if p.isPublic}
										<span
											class="inline-flex items-center gap-1 font-mono text-[11px] font-medium text-[var(--accent)]"
										>
											{p.bindAddress} (Public Ingress)
										</span>
									{:else}
										<span
											class="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--text-tertiary)]"
										>
											{p.bindAddress} (Local Only)
										</span>
									{/if}
								</td>

								<!-- Status -->
								<td class="px-3.5 py-3 text-right whitespace-nowrap">
									<span
										class="inline-flex items-center gap-1.5 text-xs font-medium text-[var(--status-green)]"
									>
										<span class="h-1.5 w-1.5 rounded-full bg-[var(--status-green)]"></span>
										Listening
									</span>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>

			<!-- Footer -->
			<div
				class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5 text-xs text-[var(--text-tertiary)]"
			>
				<span>{ports.length} ports mapped on host network</span>
				<span class="font-mono text-[11px]">Kernel Netfilter / pasta rootless bridge</span>
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
