<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import { Button, Input, CopyButton, FormField } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
	import { dataStore, server } from '$lib/data';
	import {
		Globe,
		Plus,
		Trash,
		ShieldCheck,
		ArrowRight,
		ArrowSquareOut,
		CaretDown,
		CaretUp,
		PencilSimple
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let serviceDomains = $derived(dataStore.domains.filter((d) => d.serviceId === service.id));

	let isAdding = $state(false);
	let editingDomainId = $state<string | null>(null);

	let domainName = $state('');
	let targetPort = $state('3000');
	let autoHttps = $state(true);

	$effect(() => {
		if (service.port) {
			targetPort = String(service.port);
		}
	});

	// Advanced Caddy settings
	let showAdvanced = $state(false);
	let maxBodySize = $state('');
	let wwwRedirect = $state<'none' | 'to-www' | 'to-non-www'>('none');
	let basicAuthEnabled = $state(false);
	let basicAuthUser = $state('');
	let basicAuthPass = $state('');

	let serverIp = $derived(server.ip || '49.12.84.112');

	function startEdit(d: Domain) {
		editingDomainId = d.id;
		domainName = d.hostname;
		targetPort = String(d.containerPort);
		autoHttps = d.tls;
		isAdding = true;
	}

	function handleSaveDomain() {
		if (!domainName.trim()) return;

		const portNum = parseInt(targetPort, 10) || service.port || 3000;
		const cleanHost = domainName.trim().toLowerCase();

		if (editingDomainId) {
			const existing = dataStore.domains.find((d) => d.id === editingDomainId);
			if (existing) {
				existing.hostname = cleanHost;
				existing.containerPort = portNum;
				existing.publishedPort = portNum;
				existing.tls = autoHttps;
			}
		} else {
			const domain: Domain = {
				id: `d-${Date.now()}`,
				hostname: cleanHost,
				projectId: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				tls: autoHttps,
				status: 'active',
				proxyPort: 8000 + Math.floor(Math.random() * 900),
				containerPort: portNum,
				publishedPort: portNum
			};
			dataStore.addDomain(domain);
		}

		resetForm();
	}

	function resetForm() {
		isAdding = false;
		editingDomainId = null;
		domainName = '';
		targetPort = String(service.port || 3000);
		autoHttps = true;
		showAdvanced = false;
		maxBodySize = '';
		basicAuthEnabled = false;
		basicAuthUser = '';
		basicAuthPass = '';
	}

	function handleDeleteDomain(id: string) {
		dataStore.deleteDomain(id);
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- DNS Guidance Helper Banner -->
	<div
		class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)]/60 p-3.5 text-xs sm:flex-row sm:items-center"
	>
		<div class="flex items-center gap-2 text-[var(--text-secondary)]">
			<Globe size={16} class="shrink-0 text-[var(--accent)]" />
			<span>
				Point your domain's <strong>A-Record</strong> at your DNS provider (Cloudflare, Namecheap, etc.)
				to the host IP:
			</span>
			<code
				class="rounded border border-[var(--border)] bg-[var(--bg-panel)] px-2 py-0.5 font-[var(--font-mono)] font-semibold text-[var(--text-primary)]"
			>
				{serverIp}
			</code>
		</div>
		<CopyButton text={serverIp} label="Copy Server IP" variant="button" />
	</div>

	<!-- Header -->
	<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
		<div class="flex flex-col gap-0.5">
			<h2 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Public Domains & Routing</h2>
			<span class="text-xs text-[var(--text-tertiary)]">
				Caddy handles reverse-proxy routing, automatic HTTPS (Let's Encrypt), and WebSocket upgrades
				out-of-the-box.
			</span>
		</div>

		{#if !isAdding}
			<Button
				variant="primary"
				size="sm"
				onclick={() => {
					resetForm();
					isAdding = true;
				}}
			>
				<Plus size={13} /> Add Domain
			</Button>
		{/if}
	</div>

	<!-- Domain Add / Edit Form Card -->
	{#if isAdding}
		<div
			class="animate-in fade-in flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 duration-150"
		>
			<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
				<span class="text-xs font-semibold text-[var(--text-primary)]">
					{editingDomainId ? 'Edit Domain Route' : 'Add Domain Route'}
				</span>
			</div>

			<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
				<div class="sm:col-span-2">
					<FormField label="Domain Name" required forId="new-domain-host">
						<Input
							id="new-domain-host"
							bind:value={domainName}
							placeholder="e.g. app.example.com or api.example.com"
							class="text-xs font-[var(--font-mono)]"
						/>
					</FormField>
				</div>

				<FormField label="Target Container Port" forId="new-domain-port">
					<Input
						id="new-domain-port"
						bind:value={targetPort}
						placeholder="3000"
						class="text-xs font-[var(--font-mono)]"
					/>
				</FormField>
			</div>

			<div class="flex items-center gap-3 pt-1">
				<label class="flex cursor-pointer items-center gap-2 text-xs text-[var(--text-secondary)]">
					<input
						type="checkbox"
						bind:checked={autoHttps}
						class="cursor-pointer accent-[var(--accent)]"
					/>
					<ShieldCheck size={14} class="text-[var(--status-green)]" />
					<span>Automatic HTTPS (TLS via Let's Encrypt / ZeroSSL)</span>
				</label>
			</div>

			<!-- Advanced Caddy Settings Accordion -->
			<div class="flex flex-col gap-3 border-t border-[var(--border-subtle)] pt-2">
				<button
					type="button"
					onclick={() => (showAdvanced = !showAdvanced)}
					class="flex w-fit cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
				>
					{#if showAdvanced}
						<CaretUp size={12} />
					{:else}
						<CaretDown size={12} />
					{/if}
					<span>Advanced Caddy Settings (Upload limit, Basic Auth, WWW Redirect)</span>
				</button>

				{#if showAdvanced}
					<div
						class="grid grid-cols-1 gap-4 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]/40 p-3.5 sm:grid-cols-2"
					>
						<FormField label="Max Upload Body Size" forId="adv-body-limit">
							<Input
								id="adv-body-limit"
								bind:value={maxBodySize}
								placeholder="e.g. 50MB or 100MB"
								class="text-xs font-[var(--font-mono)]"
							/>
						</FormField>

						<FormField label="WWW Redirect" forId="adv-redirect">
							<select
								id="adv-redirect"
								bind:value={wwwRedirect}
								class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1.5 text-xs text-[var(--text-primary)]"
							>
								<option value="none">None (Direct)</option>
								<option value="to-non-www">Redirect www → non-www</option>
								<option value="to-www">Redirect non-www → www</option>
							</select>
						</FormField>

						<div
							class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2 sm:col-span-2"
						>
							<label
								class="flex cursor-pointer items-center gap-2 text-xs text-[var(--text-secondary)]"
							>
								<input
									type="checkbox"
									bind:checked={basicAuthEnabled}
									class="cursor-pointer accent-[var(--accent)]"
								/>
								<span>Enable Basic Authentication (Password protect staging/preview)</span>
							</label>

							{#if basicAuthEnabled}
								<div class="grid grid-cols-2 gap-3 pt-1">
									<Input bind:value={basicAuthUser} placeholder="Username" class="text-xs" />
									<Input
										type="password"
										bind:value={basicAuthPass}
										placeholder="Password"
										class="text-xs"
									/>
								</div>
							{/if}
						</div>
					</div>
				{/if}
			</div>

			<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-3">
				<Button variant="ghost" size="sm" onclick={resetForm}>Cancel</Button>
				<Button
					variant="primary"
					size="sm"
					disabled={!domainName.trim()}
					onclick={handleSaveDomain}
				>
					{editingDomainId ? 'Update Route' : 'Save Domain Route'}
				</Button>
			</div>
		</div>
	{/if}

	<!-- Domain Routes List -->
	<div
		class="flex flex-col divide-y divide-[var(--border-subtle)] overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
	>
		{#each serviceDomains as d (d.id)}
			<div
				class="flex items-center justify-between gap-4 p-4 transition-colors hover:bg-[var(--bg-hover)]/30"
			>
				<div class="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center sm:gap-6">
					<!-- Domain Hostname + External Link -->
					<div class="flex min-w-[220px] items-center gap-2">
						<Globe size={15} class="shrink-0 text-[var(--accent)]" />
						<a
							href="http://{d.hostname}"
							target="_blank"
							rel="noreferrer"
							class="flex items-center gap-1 truncate text-sm font-[var(--font-mono)] font-medium text-[var(--text-primary)] hover:underline"
						>
							{d.hostname}
							<ArrowSquareOut size={12} class="text-[var(--text-tertiary)]" />
						</a>
					</div>

					<!-- Route Target -->
					<div
						class="flex min-w-[150px] items-center gap-2 text-xs font-[var(--font-mono)] text-[var(--text-secondary)]"
					>
						<ArrowRight size={12} class="text-[var(--text-tertiary)]" />
						<span>{service.name} :{d.containerPort}</span>
					</div>

					<!-- TLS state -->
					<div class="flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
						{#if d.tls}
							<ShieldCheck size={14} class="text-[var(--status-green)]" />
							<span class="font-medium text-[var(--status-green)]">SSL Active</span>
						{:else}
							<span>HTTP Only</span>
						{/if}
					</div>

					<!-- Real / Honest Status -->
					<div>
						<StatusBadge status={d.status} size="sm" />
					</div>
				</div>

				<div class="flex items-center gap-1">
					<button
						type="button"
						onclick={() => startEdit(d)}
						class="cursor-pointer rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
						title="Edit domain route"
					>
						<PencilSimple size={14} />
					</button>

					<button
						type="button"
						onclick={() => handleDeleteDomain(d.id)}
						class="cursor-pointer rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
						title="Remove domain route"
					>
						<Trash size={14} />
					</button>
				</div>
			</div>
		{/each}

		{#if serviceDomains.length === 0}
			<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
				No public domains configured for this service yet. Add a domain above to expose this service
				via Caddy reverse proxy.
			</div>
		{/if}
	</div>
</div>
