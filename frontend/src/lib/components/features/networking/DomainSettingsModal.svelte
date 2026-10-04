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
		class="fixed inset-0 z-50 flex items-center justify-center p-3 sm:p-5 bg-[rgba(5,6,7,0.82)] backdrop-blur-[3px]"
		role="dialog"
		aria-modal="true"
	>
		<div
			class="w-full max-w-[620px] max-h-[90vh] flex flex-col rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden shadow-2xl animate-in fade-in zoom-in-95 duration-150"
		>
			<!-- Header -->
			<div class="flex items-center justify-between px-5 py-4 border-b border-[var(--border)] bg-[var(--bg-panel)] shrink-0">
				<div class="flex items-center gap-2.5">
					<div class="w-8 h-8 rounded-lg bg-[var(--accent-muted)] text-[var(--accent)] flex items-center justify-center shrink-0">
						<Globe size={18} />
					</div>
					<div class="flex flex-col">
						<h3 class="text-sm font-semibold text-[var(--text-primary)] font-[var(--font-sans)]">
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
					class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
				>
					<X size={15} />
				</button>
			</div>

			<!-- Form Content (Scrollable) -->
			<div class="p-5 flex flex-col gap-5 overflow-y-auto text-left text-xs">
				<!-- 1. Hostname & Upstream Service -->
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
					<div class="sm:col-span-2 flex flex-col gap-1.5">
						<label for="domain-hostname" class="text-[11px] font-medium text-[var(--text-secondary)]">
							Domain Name / Hostname
						</label>
						<input
							id="domain-hostname"
							type="text"
							bind:value={hostname}
							placeholder="e.g. app.yourdomain.com"
							class="w-full px-3 py-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] font-mono text-xs outline-none focus:border-[var(--accent)]"
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
							class="w-full px-3 py-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] font-mono text-xs outline-none focus:border-[var(--accent)]"
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
						class="w-full px-3 py-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] text-xs outline-none cursor-pointer"
					>
						{#each services as svc}
							<option value={svc.id}>
								{svc.name} ({svc.type} · port {svc.port || 3000}) — {svc.projectId}
							</option>
						{/each}
					</select>
				</div>

				<!-- 2. DNS Resolution Diagnostic Box -->
				<div class="p-3.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-2.5">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-2">
							<span class="text-[11.5px] font-semibold text-[var(--text-primary)]">DNS A-Record Verification</span>
							{#if dnsVerified}
								<span class="inline-flex items-center gap-1 text-[10.5px] font-mono text-[var(--status-green)] bg-[var(--status-green-muted)] px-1.5 py-0.5 rounded font-medium">
									<CheckCircle size={12} /> Resolves to {server.ip}
								</span>
							{:else}
								<span class="inline-flex items-center gap-1 text-[10.5px] font-mono text-[var(--status-amber)] bg-[var(--status-amber-muted)] px-1.5 py-0.5 rounded font-medium">
									<Warning size={12} /> Pending propagation
								</span>
							{/if}
						</div>

						<button
							type="button"
							onclick={verifyDNS}
							disabled={isCheckingDNS || !hostname}
							class="text-[11px] text-[var(--accent)] hover:underline inline-flex items-center gap-1 cursor-pointer bg-transparent border-0 p-0 disabled:opacity-50"
						>
							{#if isCheckingDNS}
								<CircleNotch size={12} class="animate-spin" /> Checking...
							{:else}
								Verify DNS Now 🔄
							{/if}
						</button>
					</div>

					<p class="text-[11px] text-[var(--text-tertiary)] leading-relaxed m-0">
						Point your domain's <strong class="text-[var(--text-secondary)]">A record</strong> to your server IP: <code class="font-mono text-[11px] text-[var(--text-primary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border-subtle)]">{server.ip}</code>. Caddy automatically requests Let's Encrypt once the record is active.
					</p>
				</div>

				<!-- 3. TLS & Ingress Options -->
				<div class="flex flex-col gap-2.5 pt-1">
					<span class="text-[11.5px] font-semibold text-[var(--text-primary)]">
						TLS & Routing Configuration
					</span>

					<div class="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
						<!-- Toggle: Auto HTTPS -->
						<label class="flex items-center gap-2.5 p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] cursor-pointer">
							<input type="checkbox" bind:checked={tls} class="accent-[var(--accent)] rounded" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">Automatic HTTPS</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">Let's Encrypt / ZeroSSL ACME</span>
							</div>
						</label>

						<!-- Toggle: HTTP to HTTPS Redirect -->
						<label class="flex items-center gap-2.5 p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] cursor-pointer">
							<input type="checkbox" bind:checked={httpsRedirect} class="accent-[var(--accent)] rounded" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">HTTP ➔ HTTPS Redirect</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">Forces secure 308 redirect</span>
							</div>
						</label>

						<!-- Toggle: WebSocket Upgrades -->
						<label class="flex items-center gap-2.5 p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] cursor-pointer">
							<input type="checkbox" bind:checked={websocket} class="accent-[var(--accent)] rounded" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">WebSocket Upgrades</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">Pass 101 Switching Protocols</span>
							</div>
						</label>

						<!-- Toggle: CORS Headers -->
						<label class="flex items-center gap-2.5 p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] cursor-pointer">
							<input type="checkbox" bind:checked={cors} class="accent-[var(--accent)] rounded" />
							<div class="flex flex-col">
								<span class="font-medium text-[var(--text-primary)]">Enable CORS Headers</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]">Allow Cross-Origin Requests</span>
							</div>
						</label>
					</div>
				</div>

				<!-- 4. Path Routing Prefix -->
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-1">
					<div class="flex flex-col gap-1.5">
						<label for="domain-path-prefix" class="text-[11px] font-medium text-[var(--text-secondary)]">
							Path Routing Prefix
						</label>
						<input
							id="domain-path-prefix"
							type="text"
							bind:value={pathPrefix}
							placeholder="/"
							class="w-full px-3 py-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-primary)] font-mono text-xs outline-none focus:border-[var(--accent)]"
						/>
					</div>

					<div class="flex items-center gap-2.5 pt-5">
						<label class="flex items-center gap-2 text-[11.5px] text-[var(--text-secondary)] cursor-pointer">
							<input type="checkbox" bind:checked={stripPathPrefix} class="accent-[var(--accent)] rounded" />
							<span>Strip path prefix before upstream</span>
						</label>
					</div>
				</div>

				<!-- 5. Basic Authentication (Staging Protection) -->
				<div class="p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-2.5">
					<label class="flex items-center justify-between cursor-pointer">
						<div class="flex flex-col">
							<span class="font-medium text-[var(--text-primary)]">Basic HTTP Authentication</span>
							<span class="text-[10.5px] text-[var(--text-tertiary)]">Password protect staging or preview environments</span>
						</div>
						<input type="checkbox" bind:checked={basicAuth} class="accent-[var(--accent)] rounded" />
					</label>

					{#if basicAuth}
						<div class="pt-2 border-t border-[var(--border-subtle)] grid grid-cols-2 gap-2">
							<input
								type="text"
								bind:value={basicAuthUser}
								placeholder="Username"
								class="px-2.5 py-1.5 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-primary)]"
							/>
							<input
								type="password"
								placeholder="Password"
								value="••••••••••••"
								class="px-2.5 py-1.5 rounded border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-primary)]"
							/>
						</div>
					{/if}
				</div>
			</div>

			<!-- Footer -->
			<div class="flex items-center justify-between px-5 py-3.5 border-t border-[var(--border)] bg-[var(--bg-panel)] shrink-0">
				<Button variant="secondary" size="sm" onclick={onclose}>
					Cancel
				</Button>

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
