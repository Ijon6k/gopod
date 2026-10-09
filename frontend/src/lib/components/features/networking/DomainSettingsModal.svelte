<script lang="ts">
	import type { Domain } from '$lib/types';
	import { services, server } from '$lib/data';
	import { Button } from '$lib/components/primitives';
	import {
		X,
		Globe,
		CheckCircle,
		Warning,
		Lock,
		ArrowsLeftRight,
		ShieldCheck,
		WifiHigh,
		Sparkle,
		ArrowSquareOut,
		CircleNotch
	} from 'phosphor-svelte';

	interface Props {
		open: boolean;
		domain: Domain | null;
		onclose: () => void;
		onsave: (domain: Domain) => void;
	}

	let { open = $bindable(false), domain, onclose, onsave }: Props = $props();

	let hostname = $state('');
	let selectedServiceId = $state('');
	let containerPort = $state(3000);
	let tls = $state(true);
	let httpsRedirect = $state(true);
	let pathPrefix = $state('/');
	let stripPathPrefix = $state(false);
	let websocket = $state(false);
	let cors = $state(false);
	let hsts = $state(true);
	let basicAuth = $state(false);
	let basicAuthUser = $state('admin');

	// DNS verification states
	let isCheckingDNS = $state(false);
	let dnsVerified = $state(true);
	let lastChecked = $state<string | null>(null);

	$effect(() => {
		if (domain) {
			hostname = domain.hostname;
			selectedServiceId = domain.serviceId;
			containerPort = domain.containerPort || 3000;
			tls = domain.tls !== false;
			httpsRedirect = domain.httpsRedirect !== false;
			pathPrefix = domain.pathPrefix || '/';
			stripPathPrefix = !!domain.stripPathPrefix;
			websocket = !!domain.websocket;
			cors = !!domain.cors;
			hsts = domain.hsts !== false;
			basicAuth = !!domain.basicAuth;
			basicAuthUser = domain.basicAuthUser || 'admin';
			dnsVerified = domain.dnsStatus !== 'pending';
		} else {
			hostname = '';
			selectedServiceId = services[0]?.id || '';
			containerPort = services[0]?.port || 3000;
			tls = true;
			httpsRedirect = true;
			pathPrefix = '/';
			stripPathPrefix = false;
			websocket = false;
			cors = false;
			hsts = true;
			basicAuth = false;
			basicAuthUser = 'admin';
			dnsVerified = false;
		}
	});

	function handleServiceChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		const svc = services.find((s) => s.id === target.value);
		if (svc && svc.port) {
			containerPort = svc.port;
		}
	}

	async function verifyDNS() {
		if (!hostname.trim()) return;
		isCheckingDNS = true;
		await new Promise((r) => setTimeout(r, 600));
		isCheckingDNS = false;
		dnsVerified = true;
		lastChecked = 'Just now';
	}

	function handleSave() {
		if (!hostname.trim() || !selectedServiceId) return;
		const svc = services.find((s) => s.id === selectedServiceId);

		const updated: Domain = {
			id: domain?.id || `d-${Date.now()}`,
			hostname: hostname.trim().toLowerCase(),
			projectId: svc?.projectId || 'aerochat',
			serviceId: selectedServiceId,
			serviceName: svc?.name || 'web',
			tls,
			status: 'active',
			proxyPort: domain?.proxyPort || 8300 + Math.floor(Math.random() * 90),
			containerPort,
			publishedPort: containerPort,
			dnsStatus: dnsVerified ? 'valid' : 'pending',
			resolvedIp: server.ip,
			httpsRedirect,
			pathPrefix: pathPrefix.trim() || '/',
			stripPathPrefix,
			websocket,
			cors,
			hsts,
			basicAuth,
			basicAuthUser: basicAuth ? basicAuthUser : undefined
		};

		onsave(updated);
		onclose();
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose();
	}
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-[rgba(5,6,7,0.82)] p-3 backdrop-blur-[3px] sm:p-5"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="animate-in fade-in zoom-in-95 flex max-h-[90vh] w-full max-w-[620px] flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] shadow-2xl duration-150"
		>
			<!-- Header -->
			<div
				class="flex shrink-0 items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-5 py-4"
			>
				<div class="flex items-center gap-2.5">
					<div
						class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]"
					>
						<Globe size={18} />
					</div>
					<div class="flex flex-col">
						<h3 class="text-sm font-[var(--font-sans)] font-semibold text-[var(--text-primary)]">
							{domain ? 'Edit Domain Configuration' : 'Add Domain Route'}
						</h3>
						<span class="text-xs text-[var(--text-tertiary)]">
							Configures Caddy reverse proxy, TLS certificates, and path routing.
						</span>
					</div>
				</div>

				<button
					type="button"
					onclick={onclose}
					class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				>
					<X size={15} />
				</button>
			</div>

			<!-- Form Content (Scrollable) -->
			<div class="flex flex-col gap-5 overflow-y-auto p-5 text-left text-xs">
				<!-- 1. Hostname & Upstream Service -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1.5 sm:col-span-2">
						<label
							for="domain-hostname"
							class="text-[11px] font-medium text-[var(--text-secondary)]"
						>
							Domain Name / Hostname
						</label>
						<input
							id="domain-hostname"
							type="text"
							bind:value={hostname}
							placeholder="e.g. app.yourdomain.com"
							class="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2 font-mono text-xs text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="domain-port" class="text-[11px] font-medium text-[var(--text-secondary)]">
							Container Port
						</label>
						<input
							id="domain-port"
							type="number"
							bind:value={containerPort}
							placeholder="3000"
							class="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2 font-mono text-xs text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
						/>
					</div>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="domain-service" class="text-[11px] font-medium text-[var(--text-secondary)]">
						Target Service Workload
					</label>
					<select
						id="domain-service"
						bind:value={selectedServiceId}
						onchange={handleServiceChange}
						class="w-full cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2 text-xs text-[var(--text-primary)] outline-none"
					>
						{#each services as svc}
							<option value={svc.id}>
								{svc.name} ({svc.type} · port {svc.port || 3000}) — {svc.projectId}
							</option>
						{/each}
					</select>
				</div>

				<!-- 2. DNS Resolution Diagnostic Box -->
				<div
					class="flex flex-col gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3.5"
				>
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							<span class="text-[11.5px] font-semibold text-[var(--text-primary)]"
								>DNS A-Record Verification</span
							>
							{#if dnsVerified}
								<span
									class="inline-flex items-center gap-1 rounded bg-[var(--status-green-muted)] px-1.5 py-0.5 font-mono text-[10.5px] font-medium text-[var(--status-green)]"
								>
									<CheckCircle size={12} /> Resolves to {server.ip}
								</span>
							{:else}
								<span
									class="inline-flex items-center gap-1 rounded bg-[var(--status-amber-muted)] px-1.5 py-0.5 font-mono text-[10.5px] font-medium text-[var(--status-amber)]"
								>
									<Warning size={12} /> Pending propagation
								</span>
							{/if}
						</div>

						<button
							type="button"
							onclick={verifyDNS}
							disabled={isCheckingDNS || !hostname}
							class="inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline disabled:opacity-50"
						>
							{#if isCheckingDNS}
								<CircleNotch size={12} class="animate-spin" /> Checking...
							{:else}
								Verify DNS Now 🔄
							{/if}
						</button>
					</div>

					<p class="m-0 text-[11px] leading-relaxed text-[var(--text-tertiary)]">
						Point your domain's <strong class="text-[var(--text-secondary)]">A record</strong> to
						your server IP:
						<code
							class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--text-primary)]"
							>{server.ip}</code
						>. Caddy automatically requests Let's Encrypt once the record is active.
					</p>
				</div>

				<!-- 3. TLS & Ingress Options -->
				<div class="flex flex-col gap-2.5 pt-1">
					<span class="text-[11.5px] font-semibold text-[var(--text-primary)]">
						TLS & Routing Configuration
					</span>

					<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
						<!-- Toggle: Auto HTTPS -->
						<label
							class="flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<input type="checkbox" bind:checked={tls} class="rounded accent-[var(--accent)]" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">Automatic HTTPS</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Let's Encrypt / ZeroSSL ACME</span
								>
							</div>
						</label>

						<!-- Toggle: HTTP to HTTPS Redirect -->
						<label
							class="flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<input
								type="checkbox"
								bind:checked={httpsRedirect}
								class="rounded accent-[var(--accent)]"
							/>
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">HTTP ➔ HTTPS Redirect</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Forces secure 308 redirect</span
								>
							</div>
						</label>

						<!-- Toggle: WebSocket Upgrades -->
						<label
							class="flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<input
								type="checkbox"
								bind:checked={websocket}
								class="rounded accent-[var(--accent)]"
							/>
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">WebSocket Upgrades</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Pass 101 Switching Protocols</span
								>
							</div>
						</label>

						<!-- Toggle: CORS Headers -->
						<label
							class="flex cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<input type="checkbox" bind:checked={cors} class="rounded accent-[var(--accent)]" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">Enable CORS Headers</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Allow Cross-Origin Requests</span
								>
							</div>
						</label>
					</div>
				</div>

				<!-- 4. Path Routing Prefix -->
				<div class="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-2">
					<div class="flex flex-col gap-1.5">
						<label
							for="domain-path-prefix"
							class="text-[11px] font-medium text-[var(--text-secondary)]"
						>
							Path Routing Prefix
						</label>
						<input
							id="domain-path-prefix"
							type="text"
							bind:value={pathPrefix}
							placeholder="/"
							class="w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2 font-mono text-xs text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
						/>
					</div>

					<div class="flex items-center gap-2.5 pt-5">
						<label
							class="flex cursor-pointer items-center gap-2 text-[11.5px] text-[var(--text-secondary)]"
						>
							<input
								type="checkbox"
								bind:checked={stripPathPrefix}
								class="rounded accent-[var(--accent)]"
							/>
							<span>Strip path prefix before upstream</span>
						</label>
					</div>
				</div>

				<!-- 5. Basic Authentication (Staging Protection) -->
				<div
					class="flex flex-col gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3"
				>
					<label class="flex cursor-pointer items-center justify-between">
						<div class="flex flex-col">
							<span class="font-medium text-[var(--text-primary)]">Basic HTTP Authentication</span>
							<span class="text-[10.5px] text-[var(--text-tertiary)]"
								>Password protect staging or preview environments</span
							>
						</div>
						<input
							type="checkbox"
							bind:checked={basicAuth}
							class="rounded accent-[var(--accent)]"
						/>
					</label>

					{#if basicAuth}
						<div class="grid grid-cols-2 gap-2 border-t border-[var(--border-subtle)] pt-2">
							<input
								type="text"
								bind:value={basicAuthUser}
								placeholder="Username"
								class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)]"
							/>
							<input
								type="password"
								placeholder="Password"
								value="••••••••••••"
								class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)]"
							/>
						</div>
					{/if}
				</div>
			</div>

			<!-- Footer -->
			<div
				class="flex shrink-0 items-center justify-between border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3.5"
			>
				<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>

				<Button
					variant="primary"
					size="sm"
					disabled={!hostname.trim() || !selectedServiceId}
					onclick={handleSave}
				>
					Save Domain Configuration
				</Button>
			</div>
		</div>
	</div>
{/if}
