<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import { Button, Input, Field, Select, Switch, LayerCard } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { parseComposeServices, extractPortNumber } from '$lib/parsers/compose';
	import { Globe, CaretDown, CaretUp } from 'phosphor-svelte';
	import { untrack } from 'svelte';

	export interface UpstreamTargetChoice {
		value: string;
		label: string;
		badge: string;
		ports: number[];
	}

	interface Props {
		service: Service;
		editingDomain: Domain | null;
		onSave: (domainData: Domain) => void;
		onCancel: () => void;
	}

	let { service, editingDomain, onSave, onCancel }: Props = $props();

	let domainName = $state('');
	let targetContainer = $state('');
	let isCustomTarget = $state(false);
	let customTargetName = $state('');
	let targetPort = $state('3000');
	let autoHttps = $state(true);
	let pathPrefix = $state('/');
	let stripPath = $state(false);
	let websocket = $state(false);
	let cors = $state(false);
	let hsts = $state(true);

	// Advanced Caddy settings
	let showAdvanced = $state(false);
	let maxBodySize = $state('');
	let wwwRedirect = $state<'none' | 'to-www' | 'to-non-www'>('none');
	let basicAuthEnabled = $state(false);
	let basicAuthUser = $state('');
	let basicAuthPass = $state('');

	// Basic auth needs a username and a password. On edit, an already stored hash
	// allows leaving the password blank (empty = keep, enforced by the backend).
	let basicAuthIncomplete = $derived(
		basicAuthEnabled &&
			(!basicAuthUser.trim() || (!editingDomain?.hasBasicAuth && !basicAuthPass.trim()))
	);

	// Extract candidate targets: member services of this workload
	let detectedTargets = $derived.by<UpstreamTargetChoice[]>(() => {
		const list: UpstreamTargetChoice[] = [];
		const seen = new Set<string>();

		// 1. Compose Workload: member services declared in compose.yaml
		if (service.composeYaml) {
			const composeServices = parseComposeServices(service.composeYaml);
			for (const cs of composeServices) {
				const targetHost = cs.containerName || cs.name;
				if (!targetHost || seen.has(targetHost)) continue;
				seen.add(targetHost);

				let label = cs.name;
				if (cs.containerName && cs.containerName !== cs.name) {
					label = `${cs.name} (${cs.containerName})`;
				}

				list.push({
					value: targetHost,
					label,
					badge: 'Compose Service',
					ports: cs.ports
				});
			}
			if (list.length > 0) {
				return list;
			}
		}

		// 2. Podman Pod Workload: member services and member containers in this pod
		if (service.type === 'pod') {
			const podKey = service.id;
			const podNameKey = service.name;
			const podPrefixedKey = `pod_${service.name}`;

			// A. Member services in dataStore
			const memberSvcs = dataStore.services.filter(
				(s) =>
					s.projectId === service.projectId &&
					s.id !== service.id &&
					(s.podId === podKey ||
						s.podId === podNameKey ||
						s.podId === podPrefixedKey ||
						(s.inPod && s.type !== 'pod'))
			);

			for (const m of memberSvcs) {
				const val = m.name;
				if (!seen.has(val)) {
					seen.add(val);
					list.push({
						value: val,
						label: `${m.name} (member container · :${m.port || 80})`,
						badge: 'Pod Member',
						ports: m.port ? [m.port] : [80]
					});
				}
			}

			// B. Live containers running in this pod (excluding infra pause container)
			const podContainers = dataStore.containers.filter(
				(c) =>
					(c.pod === podPrefixedKey ||
						c.pod === podNameKey ||
						c.pod === podKey ||
						(c.labels && c.labels['io.gopod.project'] === service.projectId)) &&
					!c.name.endsWith('-infra')
			);

			for (const c of podContainers) {
				if (!seen.has(c.name)) {
					seen.add(c.name);
					const ports: number[] = [];
					if (c.ports) {
						const parts = c.ports.split(/[,;\s]+/);
						for (const p of parts) {
							const num = extractPortNumber(p);
							if (num && !ports.includes(num)) ports.push(num);
						}
					}
					list.push({
						value: c.name,
						label: `${c.name} (live container)`,
						badge: 'Pod Container',
						ports: ports.length > 0 ? ports : [80]
					});
				}
			}

			if (list.length > 0) {
				return list;
			}
		}

		// 3. Fallback live containers belonging to this service
		const matchingContainers = dataStore.containers.filter(
			(c) =>
				(c.serviceId && c.serviceId === service.id) ||
				(c.labels &&
					(c.labels['io.gopod.service'] === service.id ||
						c.labels['com.docker.compose.project'] === service.id ||
						c.labels['io.podman.compose.project'] === service.id)) ||
				(service.id && (c.name.startsWith(`${service.id}-`) || c.name.startsWith(`${service.id}_`)))
		);

		for (const c of matchingContainers) {
			if (!seen.has(c.name)) {
				seen.add(c.name);
				const ports: number[] = [];
				if (c.ports) {
					const parts = c.ports.split(/[,;\s]+/);
					for (const p of parts) {
						const num = extractPortNumber(p);
						if (num && !ports.includes(num)) ports.push(num);
					}
				}
				list.push({
					value: c.name,
					label: c.name,
					badge: 'Container',
					ports
				});
			}
		}

		if (list.length > 0) {
			return list;
		}

		// 3. Standalone / Quadlet fallback
		const defaultName = service.id || service.name;
		list.push({
			value: defaultName,
			label: service.name || defaultName,
			badge: 'Service',
			ports: service.port ? [service.port] : [3000]
		});

		return list;
	});

	// Detected port suggestions for quick click
	let portSuggestions = $derived.by<number[]>(() => {
		const portSet = new Set<number>();
		const currentTargetVal = isCustomTarget ? customTargetName : targetContainer;
		const matched = detectedTargets.find((t) => t.value === currentTargetVal);
		if (matched && matched.ports.length > 0) {
			matched.ports.forEach((p) => portSet.add(p));
		} else {
			for (const t of detectedTargets) {
				t.ports.forEach((p) => portSet.add(p));
			}
			if (service.port) portSet.add(service.port);
		}
		return Array.from(portSet);
	});

	function initFormDefaults() {
		if (detectedTargets.length > 0) {
			const first = detectedTargets[0];
			targetContainer = first.value;
			isCustomTarget = false;
			customTargetName = '';
			if (first.ports.length > 0) {
				targetPort = String(first.ports[0]);
			} else if (service.port) {
				targetPort = String(service.port);
			}
		} else {
			targetContainer = service.id || service.name;
			targetPort = String(service.port || 3000);
		}
	}

	let initializedFor = $state('');

	// Initialize once per editing session, keyed by the edited domain id only.
	// Target and port candidates refresh live in the dropdown; re-syncing the
	// whole form from them on every runtime poll would discard in-progress edits.
	$effect(() => {
		const key = editingDomain ? `edit:${editingDomain.id}` : 'new';
		if (initializedFor === key) return;
		initializedFor = key;
		untrack(() => {
			if (editingDomain) {
				domainName = editingDomain.hostname;
				const targetVal = editingDomain.targetContainer || '';
				const matched = detectedTargets.find((t) => t.value === targetVal);
				if (matched) {
					targetContainer = targetVal;
					isCustomTarget = false;
					customTargetName = '';
				} else if (targetVal) {
					isCustomTarget = true;
					customTargetName = targetVal;
					targetContainer = '__custom__';
				} else {
					targetContainer = detectedTargets[0]?.value || service.id;
					isCustomTarget = false;
				}

				targetPort = String(editingDomain.containerPort || service.port || 3000);
				autoHttps = editingDomain.tls;
				pathPrefix = editingDomain.pathPrefix || '/';
				stripPath = !!editingDomain.stripPathPrefix;
				websocket = !!editingDomain.websocket;
				cors = !!editingDomain.cors;
				hsts = editingDomain.hsts !== false;
				basicAuthEnabled = !!editingDomain.basicAuth;
				basicAuthUser = editingDomain.basicAuthUser || '';
				// The stored hash is write-only: never prefill, blank means keep.
				basicAuthPass = '';
			} else {
				initFormDefaults();
			}
		});
	});

	function handleTargetChange(val: string) {
		if (val === '__custom__') {
			isCustomTarget = true;
			customTargetName = '';
		} else {
			isCustomTarget = false;
			targetContainer = val;
			const matched = detectedTargets.find((t) => t.value === val);
			if (matched && matched.ports.length > 0) {
				targetPort = String(matched.ports[0]);
			}
		}
	}

	function handleSave() {
		if (!domainName.trim() || basicAuthIncomplete) return;

		const portNum = parseInt(targetPort, 10) || service.port || 3000;
		let cleanHost = domainName.trim().toLowerCase();
		cleanHost = cleanHost.replace(/^https?:\/\//, '');
		cleanHost = cleanHost.replace(/:\d+$/, '');

		const isLocal =
			cleanHost.endsWith('.localhost') ||
			cleanHost.endsWith('.local') ||
			cleanHost.includes('127.0.0.1') ||
			cleanHost === 'localhost';
		const effectiveTarget = isCustomTarget ? customTargetName.trim() : targetContainer.trim();

		const payload: Domain = {
			id: editingDomain ? editingDomain.id : crypto.randomUUID(),
			hostname: cleanHost,
			projectId: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			targetContainer: effectiveTarget || undefined,
			tls: autoHttps,
			status: 'active',
			proxyPort: editingDomain?.proxyPort || 8000 + Math.floor(Math.random() * 900),
			containerPort: portNum,
			publishedPort: portNum,
			pathPrefix: pathPrefix.trim() || '/',
			stripPathPrefix: stripPath,
			websocket,
			cors,
			hsts,
			basicAuth: basicAuthEnabled,
			basicAuthUser: basicAuthEnabled ? basicAuthUser : undefined,
			basicAuthPass: basicAuthEnabled ? basicAuthPass : undefined,
			// Keep the optimistic UI state truthful until the next data refresh.
			hasBasicAuth: basicAuthEnabled
				? editingDomain?.hasBasicAuth || !!basicAuthPass.trim()
				: undefined,
			dnsStatus: isLocal ? 'valid' : editingDomain?.dnsStatus || 'pending'
		};

		onSave(payload);
	}
</script>

<LayerCard
	class="animate-in fade-in flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 duration-150"
>
	<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
		<div class="flex items-center gap-2">
			<Globe size={16} class="text-[var(--accent)]" />
			<span class="text-xs font-semibold text-[var(--text-primary)]">
				{editingDomain ? 'Edit Domain Route' : 'Add Domain Route'}
			</span>
		</div>
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
		<!-- 1. Domain Name -->
		<div class="sm:col-span-2">
			<Field
				label="Domain Name"
				required
				description="Hostname only (e.g. app.localhost or api.example.com), without http:// or port."
			>
				<Input
					id="new-domain-host"
					bind:value={domainName}
					placeholder="e.g. app.localhost or api.example.com"
					class="text-xs font-[var(--font-mono)]"
				/>
			</Field>
		</div>

		<!-- 2. Target Container / Service Selector -->
		<div>
			<Field
				label="Target Upstream Workload"
				required
				description="Destination service or container to receive proxy traffic."
			>
				<Select
					id="target-container-select"
					value={isCustomTarget ? '__custom__' : targetContainer}
					onValueChange={(val: any) => {
						if (typeof val === 'string') handleTargetChange(val);
					}}
					options={[
						...detectedTargets.map((t) => ({
							label: `${t.label}${t.ports.length ? ' (:' + t.ports.join(', :') + ')' : ''}`,
							value: t.value
						})),
						{
							label: 'Custom Hostname / Container DNS...',
							value: '__custom__'
						}
					]}
					size="sm"
				/>
			</Field>
		</div>

		<!-- 2b. Custom Container Name (conditional) -->
		{#if isCustomTarget}
			<div class="sm:col-span-2">
				<Field
					label="Custom Upstream Hostname"
					required
					description="Enter the container name or Podman network alias (e.g. uptime-kuma, arch-sandbox)."
				>
					<Input
						id="custom-target-name"
						bind:value={customTargetName}
						placeholder="e.g. uptime-kuma or arch-sandbox"
						class="text-xs font-[var(--font-mono)]"
					/>
				</Field>
			</div>
		{/if}

		<!-- 3. Container Port & Suggestions -->
		<div>
			<Field
				label="Target Container Port"
				required
				description="Internal port where your service listens."
			>
				<Input
					id="new-domain-port"
					bind:value={targetPort}
					placeholder="3000"
					class="text-xs font-[var(--font-mono)]"
				/>
			</Field>

			{#if portSuggestions.length > 0}
				<div class="mt-1.5 flex flex-wrap items-center gap-1.5">
					<span class="text-[11px] text-[var(--text-tertiary)]">Detected ports:</span>
					{#each portSuggestions as p}
						<button
							type="button"
							onclick={() => (targetPort = String(p))}
							class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-table-header)] px-1.5 py-0.5 text-[10.5px] font-[var(--font-mono)] font-medium text-[var(--text-primary)] transition-colors hover:border-[var(--accent)] hover:text-[var(--accent)]"
						>
							:{p}
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- 4. Path Prefix -->
		<div>
			<Field label="Path Prefix (Optional)" description="Route specific path prefix (default: /).">
				<Input
					id="new-domain-path"
					bind:value={pathPrefix}
					placeholder="/"
					class="text-xs font-[var(--font-mono)]"
				/>
			</Field>
		</div>
	</div>

	<div class="flex flex-col gap-2 pt-1 sm:flex-row sm:items-center sm:gap-6">
		<Switch
			bind:checked={autoHttps}
			size="sm"
			label="Automatic HTTPS (TLS via Let's Encrypt / ZeroSSL)"
		/>

		{#if pathPrefix && pathPrefix !== '/'}
			<Switch bind:checked={stripPath} size="sm" label="Strip Path Prefix before proxying" />
		{/if}
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
			<span>Advanced Caddy Settings (Upload limit, Basic Auth, WebSocket, CORS, HSTS)</span>
		</button>

		{#if showAdvanced}
			<div
				class="grid grid-cols-1 gap-4 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-table-header)] p-4 sm:grid-cols-2"
			>
				<Field label="Max Upload Body Size">
					<Input
						id="adv-body-limit"
						bind:value={maxBodySize}
						placeholder="e.g. 50MB or 100MB"
						class="text-xs font-[var(--font-mono)]"
					/>
				</Field>

				<Select
					label="WWW Redirect"
					value={wwwRedirect}
					onValueChange={(val: any) => {
						if (typeof val === 'string') wwwRedirect = val as any;
					}}
					options={[
						{ label: 'None (Direct)', value: 'none' },
						{ label: 'Redirect www → non-www', value: 'to-non-www' },
						{ label: 'Redirect non-www → www', value: 'to-www' }
					]}
					size="sm"
				/>

				<div class="flex flex-col gap-2 sm:col-span-2">
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
						<Switch bind:checked={websocket} size="sm" label="WebSocket Upgrade" />
						<Switch bind:checked={cors} size="sm" label="Allow CORS (*)" />
						<Switch bind:checked={hsts} size="sm" label="HSTS Security Preload" />
					</div>
				</div>

				<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-3 sm:col-span-2">
					<Switch
						bind:checked={basicAuthEnabled}
						size="sm"
						label="Enable Basic Authentication (Password protect preview/staging)"
					/>

					{#if basicAuthEnabled}
						<div class="flex flex-col gap-1.5 pt-1">
							<div class="grid grid-cols-2 gap-3">
								<Input bind:value={basicAuthUser} placeholder="Username" class="text-xs" />
								<Input
									type="password"
									bind:value={basicAuthPass}
									placeholder={editingDomain?.hasBasicAuth
										? 'Leave blank to keep current'
										: 'Password'}
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
		{/if}
	</div>

	<!-- Caddy Ingress Upstream Preview -->
	<div class="rounded-md border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] p-3 text-xs">
		<div class="flex items-center gap-2 text-[var(--color-kumo-subtle)]">
			<Globe size={14} class="text-[var(--accent)]" />
			<span class="font-semibold text-[var(--color-kumo-default)]">Caddy Ingress Route Preview:</span>
		</div>
		<div class="mt-1.5 font-mono text-[11px] text-[var(--color-kumo-default)]">
			{domainName.trim() || 'your-domain.com'}{pathPrefix !== '/' ? pathPrefix : ''} ➔
			{#if service.type === 'pod'}
				<span class="text-[var(--accent)] font-semibold">pod_{service.name.replace(/^pod_/, '')}:{targetPort || '3000'}</span>
				<span class="text-[var(--color-kumo-subtle)] font-sans ml-1">
					(internal Netavark bridge route to member container)
				</span>
			{:else}
				<span class="text-[var(--accent)] font-semibold">{isCustomTarget ? customTargetName : targetContainer || service.name}:{targetPort || '3000'}</span>
				<span class="text-[var(--color-kumo-subtle)] font-sans ml-1">(direct container upstream)</span>
			{/if}
		</div>
	</div>

	<div class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-3">
		<Button variant="ghost" size="sm" onclick={onCancel}>Cancel</Button>
		<Button
			variant="primary"
			size="sm"
			disabled={!domainName.trim() || (isCustomTarget && !customTargetName.trim()) || basicAuthIncomplete}
			onclick={handleSave}
		>
			{editingDomain ? 'Update Route' : 'Save Domain Route'}
		</Button>
	</div>
</LayerCard>
