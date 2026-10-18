<script lang="ts">
	import type { Domain, Server } from '$lib/types';
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
	import {
		Globe,
		Lock,
		LockOpen,
		CheckCircle,
		Warning,
		ArrowSquareOut,
		PencilSimple,
		Trash,
		Copy,
		Check,
		ArrowsClockwise
	} from 'phosphor-svelte';
	import { dataStore } from '$lib/data';

	interface Props {
		server: Server;
		caddyHttpPort: number;
		caddyHttpsPort: number;
		onEditDomain: (domain: Domain) => void;
		onDeleteDomain: (id: string) => void;
	}

	let { server, caddyHttpPort, caddyHttpsPort, onEditDomain, onDeleteDomain }: Props = $props();

	const toast = useKumoToastManager();
	let copiedIp = $state(false);
	let revalidatingIds = $state<Set<string>>(new Set());

	function getDomainUrl(d: Domain): string {
		const host = d.hostname
			.trim()
			.replace(/^https?:\/\//, '')
			.replace(/:\d+$/, '');
		const path = d.pathPrefix && d.pathPrefix !== '/' ? d.pathPrefix : '';
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
			const isLoopback =
				updated.hostname.endsWith('.localhost') ||
				updated.hostname.endsWith('.local') ||
				updated.hostname.includes('127.0.0.1') ||
				updated.hostname === 'localhost';
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

<div class="flex flex-col gap-4">
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
							Upstream service
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							DNS status
						</TableHead>
						<TableHead
							class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
						>
							TLS certificate
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
					{#if dataStore.domains.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
								No domain routes configured yet. Click "Add Domain" to create your first Caddy
								route.
							</TableCell>
						</TableRow>
					{:else}
						{#each dataStore.domains as domain, idx (domain.id)}
							<TableRow
								class={idx % 2 === 1
									? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]'
									: 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}
							>
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
											<Badge variant="neutral" class="font-mono text-[10.5px]">WS 101</Badge>
										{/if}
										{#if domain.cors}
											<Badge variant="neutral" class="font-mono text-[10.5px]">CORS</Badge>
										{/if}
										{#if domain.hsts}
											<Badge variant="neutral" class="font-mono text-[10.5px]">HSTS</Badge>
										{/if}
										{#if domain.basicAuth && domain.hasBasicAuth}
											<Badge variant="warning" class="font-mono text-[10.5px]">Basic auth</Badge>
										{/if}
										{#if !domain.websocket && !domain.cors && !(domain.basicAuth && domain.hasBasicAuth) && !domain.hsts}
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
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:opacity-50"
											title="Revalidate DNS Record"
											aria-label="Revalidate DNS Record"
										>
											<ArrowsClockwise
												size={14}
												class={revalidatingIds.has(domain.id) ? 'animate-spin' : ''}
											/>
										</button>
										<button
											type="button"
											onclick={() => onEditDomain(domain)}
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											title="Edit Domain Settings"
											aria-label="Edit Domain Settings"
										>
											<PencilSimple size={14} />
										</button>
										<button
											type="button"
											onclick={() => onDeleteDomain(domain.id)}
											class="flex h-7 min-h-[28px] w-7 min-w-[28px] cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
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
			<span>{dataStore.domains.length} active domain routing configurations</span>
			<span class="font-mono text-[11px]"
				>Engine: Caddy v{server.caddy || '2.8'} (Rootless Ingress)</span
			>
		</div>
	</LayerCard>
</div>
