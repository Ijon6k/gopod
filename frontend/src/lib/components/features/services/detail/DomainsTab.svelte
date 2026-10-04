<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
	import { dataStore, server } from '$lib/data';
	import { Globe, Plus, Trash, ShieldCheck, ArrowRight, ArrowSquareOut, Copy, Check, CaretDown, CaretUp, PencilSimple } from 'phosphor-svelte';

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

	let ipCopied = $state(false);
	let serverIp = $derived(server.ip || '49.12.84.112');

	function copyIp() {
		navigator.clipboard.writeText(serverIp);
		ipCopied = true;
		setTimeout(() => (ipCopied = false), 2000);
	}

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

<div class="w-full flex flex-col gap-5">
	<!-- DNS Guidance Helper Banner -->
	<div class="p-3.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-surface)]/60 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
		<div class="flex items-center gap-2 text-[var(--text-secondary)]">
			<Globe size={16} class="text-[var(--accent)] shrink-0" />
			<span>
				Arahkan <strong>A-Record</strong> domain Anda di penyedia DNS (Cloudflare, Namecheap, dll) ke IP Host:
			</span>
			<code class="px-2 py-0.5 rounded bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--text-primary)] font-semibold border border-[var(--border)]">
				{serverIp}
			</code>
		</div>
		<button
			type="button"
			onclick={copyIp}
			class="flex items-center gap-1.5 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)] cursor-pointer text-[11px] font-medium transition-colors w-fit"
		>
			{#if ipCopied}
				<Check size={12} class="text-[var(--status-green)]" /> Copied!
			{:else}
				<Copy size={12} /> Copy Server IP
			{/if}
		</button>
	</div>

	<!-- Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
		<div class="flex flex-col gap-0.5">
			<h2 class="text-sm font-semibold text-[var(--text-primary)] m-0">Public Domains & Routing</h2>
			<span class="text-xs text-[var(--text-tertiary)]">
				Caddy handles reverse-proxy routing, automatic HTTPS (Let's Encrypt), and WebSocket upgrades out-of-the-box.
			</span>
		</div>

		{#if !isAdding}
			<Button variant="primary" size="sm" onclick={() => { resetForm(); isAdding = true; }}>
				<Plus size={13} /> Add Domain
			</Button>
		{/if}
	</div>

	<!-- Domain Add / Edit Form Card -->
	{#if isAdding}
		<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-4 animate-in fade-in duration-150">
			<div class="flex items-center justify-between pb-3 border-b border-[var(--border-subtle)]">
				<span class="text-xs font-semibold text-[var(--text-primary)]">
					{editingDomainId ? 'Edit Domain Route' : 'Add Domain Route'}
				</span>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
				<div class="sm:col-span-2 flex flex-col gap-1.5">
					<label for="new-domain-host" class="text-xs text-[var(--text-secondary)] font-medium">
						Domain Name <span class="text-[var(--status-red)]">*</span>
					</label>
					<Input
						id="new-domain-host"
						bind:value={domainName}
						placeholder="e.g. app.example.com or api.example.com"
						class="font-[var(--font-mono)] text-xs"
					/>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="new-domain-port" class="text-xs text-[var(--text-secondary)] font-medium">Target Container Port</label>
					<Input
						id="new-domain-port"
						bind:value={targetPort}
						placeholder="3000"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex items-center gap-3 pt-1">
				<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
					<input type="checkbox" bind:checked={autoHttps} class="accent-[var(--accent)] cursor-pointer" />
					<ShieldCheck size={14} class="text-[var(--status-green)]" />
					<span>Automatic HTTPS (TLS via Let's Encrypt / ZeroSSL)</span>
				</label>
			</div>

			<!-- Advanced Caddy Settings Accordion -->
			<div class="pt-2 border-t border-[var(--border-subtle)] flex flex-col gap-3">
				<button
					type="button"
					onclick={() => (showAdvanced = !showAdvanced)}
					class="flex items-center gap-1.5 text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer p-0 w-fit"
				>
					{#if showAdvanced}
						<CaretUp size={12} />
					{:else}
						<CaretDown size={12} />
					{/if}
					<span>Advanced Caddy Settings (Upload limit, Basic Auth, WWW Redirect)</span>
				</button>

				{#if showAdvanced}
					<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 p-3.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]/40">
						<div class="flex flex-col gap-1.5">
							<label for="adv-body-limit" class="text-xs font-medium text-[var(--text-secondary)]">Max Upload Body Size</label>
							<Input
								id="adv-body-limit"
								bind:value={maxBodySize}
								placeholder="e.g. 50MB or 100MB"
								class="text-xs font-[var(--font-mono)]"
							/>
						</div>

						<div class="flex flex-col gap-1.5">
							<label for="adv-redirect" class="text-xs font-medium text-[var(--text-secondary)]">WWW Redirect</label>
							<select
								id="adv-redirect"
								bind:value={wwwRedirect}
								class="px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-xs text-[var(--text-primary)]"
							>
								<option value="none">None (Direct)</option>
								<option value="to-non-www">Redirect www → non-www</option>
								<option value="to-www">Redirect non-www → www</option>
							</select>
						</div>

						<div class="sm:col-span-2 pt-2 border-t border-[var(--border-subtle)] flex flex-col gap-2">
							<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
								<input type="checkbox" bind:checked={basicAuthEnabled} class="accent-[var(--accent)] cursor-pointer" />
								<span>Enable Basic Authentication (Password protect staging/preview)</span>
							</label>

							{#if basicAuthEnabled}
								<div class="grid grid-cols-2 gap-3 pt-1">
									<Input
										bind:value={basicAuthUser}
										placeholder="Username"
										class="text-xs"
									/>
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

			<div class="flex items-center justify-end gap-2 pt-3 border-t border-[var(--border-subtle)]">
				<Button variant="ghost" size="sm" onclick={resetForm}>Cancel</Button>
				<Button variant="primary" size="sm" disabled={!domainName.trim()} onclick={handleSaveDomain}>
					{editingDomainId ? 'Update Route' : 'Save Domain Route'}
				</Button>
			</div>
		</div>
	{/if}

	<!-- Domain Routes List -->
	<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-card)] bg-[var(--bg-panel)] overflow-hidden">
		{#each serviceDomains as d}
			<div class="flex items-center justify-between p-4 gap-4 hover:bg-[var(--bg-hover)]/30 transition-colors">
				<div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-6 min-w-0 flex-1">
					<!-- Domain Hostname + External Link -->
					<div class="flex items-center gap-2 min-w-[220px]">
						<Globe size={15} class="text-[var(--accent)] shrink-0" />
						<a
							href="http://{d.hostname}"
							target="_blank"
							rel="noreferrer"
							class="text-sm font-medium text-[var(--text-primary)] font-[var(--font-mono)] hover:underline flex items-center gap-1 truncate"
						>
							{d.hostname}
							<ArrowSquareOut size={12} class="text-[var(--text-tertiary)]" />
						</a>
					</div>

					<!-- Route Target -->
					<div class="flex items-center gap-2 text-xs text-[var(--text-secondary)] font-[var(--font-mono)] min-w-[150px]">
						<ArrowRight size={12} class="text-[var(--text-tertiary)]" />
						<span>{service.name} :{d.containerPort}</span>
					</div>

					<!-- TLS state -->
					<div class="flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
						{#if d.tls}
							<ShieldCheck size={14} class="text-[var(--status-green)]" />
							<span class="text-[var(--status-green)] font-medium">SSL Active</span>
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
						class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] p-1.5 bg-transparent border-0 cursor-pointer rounded hover:bg-[var(--bg-hover)] transition-colors"
						title="Edit domain route"
					>
						<PencilSimple size={14} />
					</button>

					<button
						type="button"
						onclick={() => handleDeleteDomain(d.id)}
						class="text-[var(--text-tertiary)] hover:text-[var(--status-red)] p-1.5 bg-transparent border-0 cursor-pointer rounded hover:bg-[var(--bg-hover)] transition-colors"
						title="Remove domain route"
					>
						<Trash size={14} />
					</button>
				</div>
			</div>
		{/each}

		{#if serviceDomains.length === 0}
			<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
				Belum ada domain publik untuk service ini. Tambahkan domain di atas untuk mengekspos service ke internet via Caddy.
			</div>
		{/if}
	</div>
</div>
