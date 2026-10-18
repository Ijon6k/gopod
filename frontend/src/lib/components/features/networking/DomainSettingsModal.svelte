<script lang="ts">
	import type { Domain } from '$lib/types';
	import { services, server } from '$lib/data';
	import { api } from '$lib/api';
	import { Button, Input, Select, Switch } from '$lib/components/ui';
	import { parseComposeServices } from '$lib/parsers/compose';
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
	let targetContainer = $state('');
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
	let basicAuthPass = $state('');

	let selectedService = $derived(services.find((s) => s.id === selectedServiceId));
	// Basic auth is only meaningful with a username and a password. On edit, an
	// already stored hash allows leaving the password blank (empty = keep).
	let basicAuthIncomplete = $derived(
		basicAuth &&
			(!basicAuthUser.trim() || (!domain?.hasBasicAuth && !basicAuthPass.trim()))
	);
	let detectedTargets = $derived.by(() => {
		if (!selectedService) return [];
		const list: { value: string; label: string; ports: number[] }[] = [];
		if (selectedService.composeYaml) {
			const csList = parseComposeServices(selectedService.composeYaml);
			for (const cs of csList) {
				let label = cs.name;
				if (cs.containerName && cs.containerName !== cs.name) {
					label = `${cs.name} (${cs.containerName})`;
				}
				if (cs.ports.length > 0) {
					label += ` (:${cs.ports.join(', :')})`;
				}
				list.push({
					value: cs.containerName || cs.name,
					label,
					ports: cs.ports
				});
			}
		}

		// Podman Pod: member services belonging to this Pod
		if (selectedService.type === 'pod') {
			const members = services.filter(
				(s) =>
					s.projectId === selectedService.projectId &&
					s.id !== selectedService.id &&
					(s.podId === selectedService.id ||
						s.podId === selectedService.name ||
						s.podId === `pod_${selectedService.name}` ||
						(s.inPod && s.type !== 'pod'))
			);
			for (const m of members) {
				list.push({
					value: m.name,
					label: `${m.name} (member container · :${m.port || 80})`,
					ports: m.port ? [m.port] : [80]
				});
			}
		}

		return list;
	});

	// DNS verification states
	let isCheckingDNS = $state(false);
	let dnsVerified = $state(true);
	let lastChecked = $state<string | null>(null);

	$effect(() => {
		if (domain) {
			hostname = domain.hostname;
			selectedServiceId = domain.serviceId;
			targetContainer = domain.targetContainer || '';
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
			basicAuthPass = '';
			const isLocal =
				domain.hostname.endsWith('.localhost') ||
				domain.hostname.endsWith('.local') ||
				domain.hostname.includes('127.0.0.1') ||
				domain.hostname === 'localhost';
			dnsVerified = isLocal || domain.dnsStatus !== 'pending';
		} else {
			hostname = '';
			selectedServiceId = services[0]?.id || '';
			targetContainer = '';
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
			basicAuthPass = '';
			dnsVerified = false;
		}
	});

	function updateServicePort(svcId: string) {
		const svc = services.find((s) => s.id === svcId);
		if (svc && svc.port) {
			containerPort = svc.port;
		}
		if (svc?.composeYaml) {
			const csList = parseComposeServices(svc.composeYaml);
			if (csList.length > 0) {
				targetContainer = csList[0].name;
				if (csList[0].ports.length > 0) {
					containerPort = csList[0].ports[0];
				}
			}
		}
	}

	async function verifyDNS() {
		if (!hostname.trim()) return;
		isCheckingDNS = true;
		try {
			const clean = hostname
				.trim()
				.toLowerCase()
				.replace(/^https?:\/\//, '')
				.replace(/:\d+$/, '');
			const isLocal =
				clean.endsWith('.localhost') ||
				clean.endsWith('.local') ||
				clean.includes('127.0.0.1') ||
				clean === 'localhost';
			if (isLocal) {
				dnsVerified = true;
			} else if (domain?.id) {
				const res = await api.domains.revalidate(domain.id);
				dnsVerified = res.dnsStatus === 'valid';
			} else {
				dnsVerified = true;
			}
		} catch {
			dnsVerified = false;
		} finally {
			isCheckingDNS = false;
			lastChecked = 'Just now';
		}
	}

	function handleSave() {
		if (!hostname.trim() || !selectedServiceId || basicAuthIncomplete) return;
		const svc = services.find((s) => s.id === selectedServiceId);

		let cleanHost = hostname.trim().toLowerCase();
		cleanHost = cleanHost.replace(/^https?:\/\//, '');
		cleanHost = cleanHost.replace(/:\d+$/, '');

		const isLocal =
			cleanHost.endsWith('.localhost') ||
			cleanHost.endsWith('.local') ||
			cleanHost.includes('127.0.0.1') ||
			cleanHost === 'localhost';
		const dnsStatus = isLocal || dnsVerified ? 'valid' : 'pending';

		const updated: Domain = {
			id: domain?.id || `d-${Date.now()}`,
			hostname: cleanHost,
			projectId: svc?.projectId || 'aerochat',
			serviceId: selectedServiceId,
			serviceName: svc?.name || 'web',
			targetContainer: targetContainer.trim() || undefined,
			tls,
			status: 'active',
			proxyPort: domain?.proxyPort || 8300 + Math.floor(Math.random() * 90),
			containerPort,
			publishedPort: containerPort,
			dnsStatus,
			resolvedIp: server.ip,
			httpsRedirect,
			pathPrefix: pathPrefix.trim() || '/',
			stripPathPrefix,
			websocket,
			cors,
			hsts,
			basicAuth,
			basicAuthUser: basicAuth ? basicAuthUser : undefined,
			// Empty password on an existing domain keeps the stored hash (backend rule).
			basicAuthPass: basicAuth && basicAuthPass.trim() ? basicAuthPass : undefined,
			// Keep the optimistic UI state truthful until the next data refresh.
			hasBasicAuth: basicAuth ? domain?.hasBasicAuth || !!basicAuthPass.trim() : undefined
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
						<Input
							id="domain-hostname"
							bind:value={hostname}
							placeholder="e.g. app.yourdomain.com"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="domain-port" class="text-[11px] font-medium text-[var(--text-secondary)]">
							Container Port
						</label>
						<Input
							id="domain-port"
							type="number"
							bind:value={containerPort}
							placeholder="3000"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				</div>

				<Select
					id="domain-service"
					label="Target Service Workload"
					value={selectedServiceId}
					onValueChange={(val: any) => {
						if (typeof val === 'string') {
							selectedServiceId = val;
							updateServicePort(val);
						}
					}}
					options={services.map((svc) => ({
						label: `${svc.name} (${svc.type} · port ${svc.port || 3000}) — ${svc.projectId}`,
						value: svc.id
					}))}
					size="sm"
				/>

				{#if detectedTargets.length > 0}
					<Select
						id="domain-target-container"
						label="Target Container / Upstream DNS"
						value={targetContainer || detectedTargets[0]?.value}
						onValueChange={(val: any) => {
							if (typeof val === 'string') {
								targetContainer = val;
								const matched = detectedTargets.find((t) => t.value === val);
								if (matched && matched.ports.length > 0) {
									containerPort = matched.ports[0];
								}
							}
						}}
						options={detectedTargets.map((t) => ({
							label: t.label,
							value: t.value
						}))}
						size="sm"
					/>
				{/if}

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
						<div
							class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<div class="flex flex-col">
								<span class="text-xs font-medium text-[var(--text-primary)]">Automatic HTTPS</span>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Let's Encrypt / ZeroSSL ACME</span
								>
							</div>
							<Switch bind:checked={tls} size="sm" />
						</div>

						<!-- Toggle: HTTP to HTTPS Redirect -->
						<div
							class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<div class="flex flex-col">
								<span class="text-xs font-medium text-[var(--text-primary)]"
									>HTTP ➔ HTTPS Redirect</span
								>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Forces secure 308 redirect</span
								>
							</div>
							<Switch bind:checked={httpsRedirect} size="sm" />
						</div>

						<!-- Toggle: WebSocket Upgrades -->
						<div
							class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<div class="flex flex-col">
								<span class="text-xs font-medium text-[var(--text-primary)]"
									>WebSocket Upgrades</span
								>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Pass 101 Switching Protocols</span
								>
							</div>
							<Switch bind:checked={websocket} size="sm" />
						</div>

						<!-- Toggle: CORS Headers -->
						<div
							class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5"
						>
							<div class="flex flex-col">
								<span class="text-xs font-medium text-[var(--text-primary)]"
									>Enable CORS Headers</span
								>
								<span class="text-[10.5px] text-[var(--text-tertiary)]"
									>Allow Cross-Origin Requests</span
								>
							</div>
							<Switch bind:checked={cors} size="sm" />
						</div>
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
						<Input
							id="domain-path-prefix"
							bind:value={pathPrefix}
							placeholder="/"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>

					<div class="flex items-center gap-2.5 pt-5">
						<Switch
							bind:checked={stripPathPrefix}
							size="sm"
							label="Strip path prefix before upstream"
						/>
					</div>
				</div>

				<!-- 5. Basic Authentication (Staging Protection) -->
				<div
					class="flex flex-col gap-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-3"
				>
					<div class="flex items-center justify-between">
						<div class="flex flex-col">
							<span class="text-xs font-medium text-[var(--text-primary)]"
								>Basic HTTP Authentication</span
							>
							<span class="text-[10.5px] text-[var(--text-tertiary)]"
								>Password protect staging or preview environments</span
							>
						</div>
						<Switch bind:checked={basicAuth} size="sm" />
					</div>

					{#if basicAuth}
						<div class="flex flex-col gap-1.5 border-t border-[var(--border-subtle)] pt-2">
							<div class="grid grid-cols-2 gap-2">
								<Input bind:value={basicAuthUser} placeholder="Username" class="text-xs" />
								<Input
									type="password"
									bind:value={basicAuthPass}
									placeholder={domain?.hasBasicAuth ? 'Leave blank to keep current' : 'Password'}
									class="text-xs"
								/>
							</div>
							{#if basicAuthIncomplete}
								<span class="text-[10.5px] text-[var(--status-amber)]">
									Set a username and password to protect this route.
								</span>
							{/if}
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
					disabled={!hostname.trim() || !selectedServiceId || basicAuthIncomplete}
					onclick={handleSave}
				>
					Save Domain Configuration
				</Button>
			</div>
		</div>
	</div>
{/if}
