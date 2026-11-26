<script lang="ts">
	import type { DNSCheckResult, Domain } from '$lib/types';
	import { services } from '$lib/data';
	import { Banner, Button, Field, Input, Modal, Select, Switch } from '$lib/components/ui';
	import DomainDnsValidation from './DomainDnsValidation.svelte';
	import { parseComposeServices } from '$lib/parsers/compose';
	import { Globe } from 'phosphor-svelte';

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
	let pathPrefix = $state('/');
	let stripPathPrefix = $state(false);
	let cors = $state(false);
	let hsts = $state(true);
	let basicAuth = $state(false);
	let basicAuthUser = $state('admin');
	let basicAuthPass = $state('');

	let selectedService = $derived(services.find((s) => s.id === selectedServiceId));
	// Edited rules can reference a service that has since been deleted; the
	// Select then shows a stale entry while Save would silently do nothing.
	// Surfacing that state beats a dead button.
	let staleServiceSelected = $derived(selectedServiceId !== '' && !selectedService);
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

	// Latest DNS check from DomainDnsValidation (when triggered in modal)
	let dnsCheckResult = $state<DNSCheckResult | null>(null);

	$effect(() => {
		if (domain) {
			hostname = domain.hostname;
			selectedServiceId = domain.serviceId;
			targetContainer = domain.targetContainer || '';
			containerPort = domain.containerPort || 3000;
			tls = domain.tls !== false;
			pathPrefix = domain.pathPrefix || '/';
			stripPathPrefix = !!domain.stripPathPrefix;
			cors = !!domain.cors;
			hsts = domain.hsts !== false;
			basicAuth = !!domain.basicAuth;
			basicAuthUser = domain.basicAuthUser || 'admin';
			basicAuthPass = '';
			dnsCheckResult = domain.dnsCheck || null;
		} else {
			hostname = '';
			selectedServiceId = services[0]?.id || '';
			targetContainer = '';
			containerPort = services[0]?.port || 3000;
			tls = true;
			pathPrefix = '/';
			stripPathPrefix = false;
			cors = false;
			hsts = true;
			basicAuth = false;
			basicAuthUser = 'admin';
			basicAuthPass = '';
			dnsCheckResult = null;
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

	function handleDnsValidated(res: DNSCheckResult) {
		dnsCheckResult = res;
	}

	function handleSave() {
		if (!hostname.trim() || !selectedServiceId || basicAuthIncomplete) return;
		const svc = services.find((s) => s.id === selectedServiceId);
		// A missing service means stale store data. Posting a rule anyway with a
		// fabricated project id ('aerochat' was a leftover from a foreign
		// template) would only fail the projects FK server-side. Bail out —
		// the caller just re-opens on refreshed data.
		if (!svc) return;

		let cleanHost = hostname.trim().toLowerCase();
		cleanHost = cleanHost.replace(/^https?:\/\//, '');
		cleanHost = cleanHost.replace(/:\d+$/, '');

		const isLocal =
			cleanHost.endsWith('.localhost') ||
			cleanHost.endsWith('.local') ||
			cleanHost.includes('127.0.0.1') ||
			cleanHost === 'localhost';
		let dnsStatus: Domain['dnsStatus'] = isLocal ? 'valid' : domain?.dnsStatus || 'pending';
		if (!isLocal && dnsCheckResult && dnsCheckResult.hostname === cleanHost) {
			if (dnsCheckResult.valid) dnsStatus = 'valid';
			else if (dnsCheckResult.status === 'mismatch') dnsStatus = 'mismatch';
			else dnsStatus = 'pending';
		}

		const portNum = parseInt(String(containerPort), 10) || 3000;

		const updated: Domain = {
			id: domain?.id || crypto.randomUUID(),
			hostname: cleanHost,
			projectId: svc.projectId,
			serviceId: selectedServiceId,
			serviceName: svc.name,
			targetContainer: targetContainer.trim() || undefined,
			tls,
			status: 'active',
			proxyPort: domain?.proxyPort || 8300 + Math.floor(Math.random() * 90),
			containerPort: portNum,
			publishedPort: portNum,
			dnsStatus,
			resolvedIp: dnsCheckResult?.resolvedIp || domain?.resolvedIp,
			behindCloudflare: dnsCheckResult?.behindCloudflare ?? domain?.behindCloudflare,
			pathPrefix: pathPrefix.trim() || '/',
			stripPathPrefix,
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
</script>

<Modal
	{open}
	{onclose}
	size="lg"
	icon={Globe}
	title={domain ? 'Edit domain configuration' : 'Add domain route'}
	subtitle="Configure Caddy reverse proxy, TLS certificates, and path routing."
>
	<!-- 1. Hostname & Upstream Service -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
		<div class="sm:col-span-2">
			<Field label="Domain name / hostname">
				<Input
					id="domain-hostname"
					bind:value={hostname}
					placeholder="e.g. app.yourdomain.com"
					class="text-xs"
				/>
			</Field>
		</div>

		<div>
			<Field label="Container port">
				<Input
					id="domain-port"
					type="number"
					bind:value={containerPort}
					placeholder="3000"
					class="text-xs tabular-nums"
				/>
			</Field>
		</div>
	</div>

	<Select
		id="domain-service"
		label="Target service workload"
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

	{#if staleServiceSelected}
		<Banner variant="alert" size="sm">
			<div class="text-xs">
				<span class="font-medium">Target service no longer exists:</span>
				<span>refresh the services list and pick a workload before saving.</span>
			</div>
		</Banner>
	{/if}

	{#if detectedTargets.length > 0}
		<Select
			id="domain-target-container"
			label="Target container / upstream host"
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

	<!-- 2. Minimalist DNS Validation (turns green when valid or behind Cloudflare) -->
	<DomainDnsValidation
		{hostname}
		domainId={domain?.id}
		initialDnsStatus={domain?.dnsStatus}
		onvalidated={handleDnsValidated}
	/>

	<!-- 3. TLS & Ingress Options -->
	<div class="flex flex-col gap-2">
		<span class="text-xs font-medium text-[var(--text-primary)]">
			TLS &amp; routing options
		</span>

		<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
			<div
				class="flex items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2.5"
			>
				<div class="flex flex-col">
					<span class="text-xs font-medium text-[var(--text-primary)]">Automatic HTTPS</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Let's Encrypt / ZeroSSL ACME</span>
				</div>
				<Switch bind:checked={tls} size="sm" />
			</div>

			<div
				class="flex items-center justify-between rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-3 py-2.5"
			>
				<div class="flex flex-col">
					<span class="text-xs font-medium text-[var(--text-primary)]">Enable CORS headers</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Allow cross-origin requests</span>
				</div>
				<Switch bind:checked={cors} size="sm" />
			</div>
		</div>
	</div>

	<!-- 4. Path Routing Prefix -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
		<Field label="Path routing prefix">
			<Input id="domain-path-prefix" bind:value={pathPrefix} placeholder="/" class="text-xs" />
		</Field>

		<div class="flex items-center pt-5">
			<Switch
				bind:checked={stripPathPrefix}
				size="sm"
				label="Strip path prefix before upstream"
			/>
		</div>
	</div>

	<!-- 5. Basic Authentication (Staging Protection) -->
	<div
		class="flex flex-col gap-2.5 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] p-3"
	>
		<div class="flex items-center justify-between">
			<div class="flex flex-col">
				<span class="text-xs font-medium text-[var(--text-primary)]">
					Basic HTTP authentication
				</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Password protect staging or preview environments
				</span>
			</div>
			<Switch bind:checked={basicAuth} size="sm" />
		</div>

		{#if basicAuth}
			<div class="flex flex-col gap-1.5 border-t border-[var(--border-subtle)] pt-2.5">
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
					<span class="text-[11px] text-[var(--status-amber)]">
						Set a username and password to protect this route.
					</span>
				{/if}
			</div>
		{/if}
	</div>

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>
		<Button
			variant="primary"
			size="sm"
			disabled={!hostname.trim() || !selectedServiceId || !selectedService || basicAuthIncomplete}
			onclick={handleSave}
		>
			Save domain configuration
		</Button>
	{/snippet}
</Modal>
