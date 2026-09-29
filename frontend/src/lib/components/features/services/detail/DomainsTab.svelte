<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { Globe, Plus, Trash, ShieldCheck, ArrowRight } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	// Domains for this service
	let serviceDomains = $derived(dataStore.domains.filter((d) => d.serviceId === service.id));

	let isAdding = $state(false);
	let newHostname = $state('');
	let customPort = $state<string | null>(null);
	let newPort = $derived(customPort ?? (service.port ? String(service.port) : '3000'));
	let autoHttps = $state(true);

	function handleAddDomain() {
		if (!newHostname.trim()) return;
		const domain: Domain = {
			id: `d-${Date.now()}`,
			hostname: newHostname.trim().toLowerCase(),
			projectId: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			tls: autoHttps,
			status: 'pending', // Honest pending state as requested
			proxyPort: 8000 + Math.floor(Math.random() * 900),
			containerPort: parseInt(newPort, 10) || service.port || 3000,
			publishedPort: parseInt(newPort, 10) || service.port || 3000
		};
		dataStore.addDomain(domain);
		isAdding = false;
		newHostname = '';
	}

	function handleDeleteDomain(id: string) {
		dataStore.deleteDomain(id);
	}
</script>

<div class="w-full flex flex-col gap-6">
	<!-- Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
		<div class="flex flex-col gap-0.5">
			<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">Public Domains</h2>
			<span class="text-xs text-[var(--text-tertiary)]">
				HTTP/HTTPS reverse-proxy routes directing external domains to your service.
			</span>
		</div>

		{#if !isAdding}
			<Button variant="primary" size="sm" onclick={() => (isAdding = true)}>
				<Plus size={13} /> Add Domain
			</Button>
		{/if}
	</div>

	<!-- Compact Add Domain Form -->
	{#if isAdding}
		<div class="p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3">
			<span class="text-xs font-medium text-[var(--text-primary)]">Add Domain Route</span>

			<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
				<div class="sm:col-span-2 flex flex-col gap-1.5">
					<label for="new-domain-host" class="text-xs text-[var(--text-secondary)] font-medium">Domain Name</label>
					<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
						<Input
							id="new-domain-host"
							bind:value={newHostname}
							placeholder="e.g. app.example.com"
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="new-domain-port" class="text-xs text-[var(--text-secondary)] font-medium">Target Port</label>
					<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
						<input
							id="new-domain-port"
							value={newPort}
							oninput={(e) => (customPort = (e.target as HTMLInputElement).value)}
							placeholder="3000"
							class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-mono)]"
						/>
					</div>
				</div>
			</div>

			<div class="flex items-center justify-between pt-2 border-t border-[var(--border-subtle)]">
				<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
					<input type="checkbox" bind:checked={autoHttps} class="accent-[var(--accent)] cursor-pointer" />
					<span>Automatic HTTPS (TLS via Let's Encrypt / ZeroSSL)</span>
				</label>

				<div class="flex items-center gap-2">
					<Button variant="ghost" size="sm" onclick={() => (isAdding = false)}>Cancel</Button>
					<Button variant="primary" size="sm" disabled={!newHostname.trim()} onclick={handleAddDomain}>
						Add Route
					</Button>
				</div>
			</div>
		</div>
	{/if}

	<!-- Domain Routes List -->
	<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-card)] bg-[var(--bg-panel)] overflow-hidden">
		{#each serviceDomains as d}
			<div class="flex items-center justify-between p-4 gap-4">
				<div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-6 min-w-0 flex-1">
					<!-- Domain Hostname -->
					<div class="flex items-center gap-2.5 min-w-[200px]">
						<Globe size={15} class="text-[var(--accent)] flex-shrink-0" />
						<span class="text-sm font-medium text-[var(--text-primary)] font-[var(--font-mono)] truncate">
							{d.hostname}
						</span>
					</div>

					<!-- Route Target -->
					<div class="flex items-center gap-2 text-xs text-[var(--text-secondary)] font-[var(--font-mono)] min-w-[160px]">
						<ArrowRight size={12} class="text-[var(--text-tertiary)]" />
						<span>{service.name} :{d.containerPort}</span>
					</div>

					<!-- TLS state -->
					<div class="flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
						{#if d.tls}
							<ShieldCheck size={14} class="text-[var(--status-green)]" />
							<span class="text-[var(--text-secondary)]">HTTPS</span>
						{:else}
							<span>HTTP</span>
						{/if}
					</div>

					<!-- Real / Honest Status -->
					<div>
						<StatusBadge status={d.status} size="sm" />
					</div>
				</div>

				<button
					type="button"
					onclick={() => handleDeleteDomain(d.id)}
					class="text-[var(--text-tertiary)] hover:text-[var(--status-red)] p-1.5 bg-transparent border-0 cursor-pointer"
					title="Remove domain route"
				>
					<Trash size={14} />
				</button>
			</div>
		{/each}

		{#if serviceDomains.length === 0}
			<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
				No public domains routed to this service. Add a domain above to expose your HTTP/HTTPS service.
			</div>
		{/if}
	</div>

	<!-- Concise technical footer note -->
	<div class="flex items-center justify-between text-[11px] text-[var(--text-tertiary)] pt-1">
		<span>Reverse-proxy routing managed internally by Caddy. WebSockets and HTTP/2 are upgraded automatically.</span>
		<span>Domains route external traffic without requiring direct host port binding.</span>
	</div>
</div>
