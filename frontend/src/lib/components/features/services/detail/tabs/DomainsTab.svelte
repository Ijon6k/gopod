<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import {
		Button,
		CopyButton,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		Badge,
		LayerCard
	} from '$lib/components/ui';
	import { dataStore, server } from '$lib/data';
	import { api } from '$lib/api';
	import {
		Globe,
		Plus,
		Trash,
		ShieldCheck,
		ArrowSquareOut,
		PencilSimple,
		LockOpen,
		CheckCircle,
		Warning,
		HardDrive,
		ArrowsClockwise
	} from 'phosphor-svelte';
	import { DomainFormCard } from '../networking';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let caddyHttpPort = $state(80);
	let caddyHttpsPort = $state(443);
	let revalidatingIds = $state<Set<string>>(new Set());

	$effect(() => {
		api.domains
			.getCaddyfile()
			.then((res) => {
				if (res.httpPort) caddyHttpPort = res.httpPort;
				if (res.httpsPort) caddyHttpsPort = res.httpsPort;
			})
			.catch(() => {});
	});

	let serviceDomains = $derived(dataStore.domains.filter((d) => d.serviceId === service.id));

	let isAdding = $state(false);
	let editingDomainId = $state<string | null>(null);
	let editingDomain = $derived(
		editingDomainId ? dataStore.domains.find((d) => d.id === editingDomainId) || null : null
	);

	let serverIp = $derived(server.ip || '127.0.0.1');

	function getDomainUrl(d: Domain): string {
		const host = d.hostname
			.trim()
			.replace(/^https?:\/\//, '')
			.replace(/:\d+$/, '');
		const isLocal =
			host.endsWith('.localhost') ||
			host.endsWith('.local') ||
			host.includes('127.0.0.1') ||
			host === 'localhost';
		if (isLocal) {
			const port = d.tls
				? caddyHttpsPort !== 443
					? `:${caddyHttpsPort}`
					: ''
				: caddyHttpPort !== 80
					? `:${caddyHttpPort}`
					: '';
			const proto = d.tls ? 'https' : 'http';
			return `${proto}://${host}${port}`;
		}
		return `${d.tls ? 'https' : 'http'}://${host}`;
	}

	async function handleRevalidate(domainId: string) {
		const next = new Set(revalidatingIds);
		next.add(domainId);
		revalidatingIds = next;
		try {
			await dataStore.revalidateDomain(domainId);
		} catch (err) {
			console.warn('Failed to revalidate domain:', err);
		} finally {
			const done = new Set(revalidatingIds);
			done.delete(domainId);
			revalidatingIds = done;
		}
	}

	function handleSaveDomain(domain: Domain) {
		if (editingDomainId) {
			dataStore.updateDomain(domain);
		} else {
			dataStore.addDomain(domain);
		}
		isAdding = false;
		editingDomainId = null;
	}

	function handleDeleteDomain(id: string) {
		dataStore.deleteDomain(id);
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- DNS Guidance Helper Banner -->
	<div
		class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-3.5 text-xs sm:flex-row sm:items-center"
	>
		<div class="flex items-center gap-2 text-[var(--text-secondary)]">
			<Globe size={16} class="shrink-0 text-[var(--accent)]" />
			<span>
				Point your domain's <strong>A-Record</strong> at your DNS provider (Cloudflare, Namecheap, etc.)
				to the host IP:
			</span>
			<code
				class="rounded border border-[var(--border)] bg-[var(--bg-table-header)] px-2 py-0.5 font-[var(--font-mono)] font-semibold text-[var(--text-primary)]"
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

		{#if !isAdding && !editingDomainId}
			<Button
				variant="primary"
				size="sm"
				onclick={() => {
					editingDomainId = null;
					isAdding = true;
				}}
			>
				<Plus size={13} /> Add Domain
			</Button>
		{/if}
	</div>

	<!-- Domain Add / Edit Form Card -->
	{#if isAdding || editingDomainId}
		<DomainFormCard
			{service}
			{editingDomain}
			onSave={handleSaveDomain}
			onCancel={() => {
				isAdding = false;
				editingDomainId = null;
			}}
		/>
	{/if}

	<!-- Domain Routes Data Table (Kumo Standard Compliant) -->
	<LayerCard class="overflow-hidden p-0">
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-xs">
				<TableHeader
					class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
				>
					<TableRow>
						<TableHead
							class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							Domain
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							Upstream Route
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							DNS Status
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							TLS Certificate
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							Features
						</TableHead>
						<TableHead
							class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							Actions
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if serviceDomains.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
								No public domain routes configured for this service yet. Add a domain above to
								expose this workload via Caddy reverse proxy.
							</TableCell>
						</TableRow>
					{:else}
						{#each serviceDomains as d, idx (d.id)}
							<TableRow
								class={idx % 2 === 1
									? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]'
									: 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}
							>
								<!-- Domain Hostname + Link -->
								<TableCell class="px-3.5 py-3 whitespace-nowrap">
									<div class="flex items-center gap-2">
										<Globe size={15} class="shrink-0 text-[var(--accent)]" />
										<a
											href={getDomainUrl(d)}
											target="_blank"
											rel="noreferrer"
											class="flex items-center gap-1 text-[13px] font-[var(--font-mono)] font-medium text-[var(--text-primary)] hover:underline"
										>
											{d.hostname}
											<ArrowSquareOut size={12} class="text-[var(--text-tertiary)]" />
										</a>
										{#if d.pathPrefix && d.pathPrefix !== '/'}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
											>
												{d.pathPrefix}{d.stripPathPrefix ? ' (stripped)' : ''}
											</span>
										{/if}
									</div>
								</TableCell>

								<!-- Upstream Route: Target Container / Service ➜ Port -->
								<TableCell class="px-3 py-3 whitespace-nowrap">
									<div class="flex items-center gap-2">
										<div
											class="flex items-center gap-1.5 text-[13px] font-[var(--font-mono)] font-medium text-[var(--text-primary)]"
										>
											<HardDrive size={14} class="text-[var(--text-tertiary)]" />
											<span>{d.targetContainer || service.name}</span>
											<span class="text-[var(--text-tertiary)]">:</span>
											<span class="font-semibold text-[var(--accent)]">{d.containerPort}</span>
										</div>
										{#if d.targetContainer}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10.5px] font-[var(--font-mono)] text-[var(--text-secondary)]"
											>
												upstream: {d.targetContainer}
											</span>
										{/if}
									</div>
								</TableCell>

								<!-- DNS Status -->
								<TableCell class="px-3 py-3 whitespace-nowrap">
									{#if d.hostname.endsWith('.localhost') || d.hostname.endsWith('.local') || d.hostname.includes('127.0.0.1')}
										<Badge variant="success" class="gap-1 text-xs">
											<CheckCircle size={12} /> Local Loopback
										</Badge>
									{:else if d.dnsStatus === 'valid'}
										<Badge variant="success" class="gap-1 text-xs">
											<CheckCircle size={12} /> Resolves ({serverIp})
										</Badge>
									{:else}
										<Badge variant="warning" class="gap-1 text-xs">
											<Warning size={12} /> Pending DNS
										</Badge>
									{/if}
								</TableCell>

								<!-- TLS Certificate -->
								<TableCell class="px-3 py-3 whitespace-nowrap">
									{#if d.tls}
										<span
											class="inline-flex items-center gap-1.5 text-xs text-[var(--status-green)]"
										>
											<ShieldCheck size={14} /> SSL Active (ACME)
										</span>
									{:else}
										<span
											class="inline-flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]"
										>
											<LockOpen size={14} /> Plain HTTP
										</span>
									{/if}
								</TableCell>

								<!-- Features -->
								<TableCell class="px-3 py-3">
									<div class="flex flex-wrap items-center gap-1">
										{#if d.websocket}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
												>WS</span
											>
										{/if}
										{#if d.cors}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
												>CORS</span
											>
										{/if}
										{#if d.basicAuth}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10px] font-[var(--font-mono)] text-[var(--status-amber)]"
												>Auth</span
											>
										{/if}
										{#if d.hsts}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
												>HSTS</span
											>
										{/if}
										{#if !d.websocket && !d.cors && !d.basicAuth && !d.hsts}
											<span class="text-[11px] text-[var(--text-muted)]">—</span>
										{/if}
									</div>
								</TableCell>

								<!-- Actions (Hit Target Sizing >= 28x28px) -->
								<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1">
										<button
											type="button"
											onclick={() => handleRevalidate(d.id)}
											disabled={revalidatingIds.has(d.id)}
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:opacity-50"
											title="Revalidate DNS record"
											aria-label="Revalidate DNS record"
										>
											<ArrowsClockwise
												size={14}
												class={revalidatingIds.has(d.id) ? 'animate-spin' : ''}
											/>
										</button>

										<button
											type="button"
											onclick={() => {
												editingDomainId = d.id;
												isAdding = false;
											}}
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											title="Edit domain route"
											aria-label="Edit domain route"
										>
											<PencilSimple size={14} />
										</button>

										<button
											type="button"
											onclick={() => handleDeleteDomain(d.id)}
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
											title="Remove domain route"
											aria-label="Remove domain route"
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
	</LayerCard>
</div>
