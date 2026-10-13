<script lang="ts">
	import type { Service, Domain } from '$lib/types';
	import {
		Button,
		Input,
		CopyButton,
		Field,
		Select,
		Switch,
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
	import { parseComposeServices, extractPortNumber } from '$lib/parsers/compose';
	import {
		Globe,
		Plus,
		Trash,
		ShieldCheck,
		ArrowSquareOut,
		CaretDown,
		CaretUp,
		PencilSimple,
		LockOpen,
		CheckCircle,
		Warning,
		HardDrive,
		ArrowsClockwise
	} from 'phosphor-svelte';

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

	let serverIp = $derived(server.ip || '127.0.0.1');

	interface UpstreamTargetChoice {
		value: string;
		label: string;
		badge: string;
		ports: number[];
	}

	// Extract candidate targets: 1:1 with Dokploy (only member services of this workload)
	let detectedTargets = $derived.by<UpstreamTargetChoice[]>(() => {
		const list: UpstreamTargetChoice[] = [];
		const seen = new Set<string>();

		// 1. Compose Workload: extract strictly the member services declared in compose.yaml
		if (service.composeYaml) {
			const composeServices = parseComposeServices(service.composeYaml);
			for (const cs of composeServices) {
				// In Podman Aardvark-DNS, explicit container_name takes precedence as the primary DNS record
				const targetHost = cs.containerName || cs.name;
				if (!targetHost || seen.has(targetHost)) continue;
				seen.add(targetHost);

				let label = cs.name;
				if (cs.containerName && cs.containerName !== cs.name) {
					label = `${cs.name} (${cs.containerName})`;
				}

				list.push({
					value: targetHost, // Real resolvable container/service DNS in gopod-net
					label,
					badge: 'Compose Service',
					ports: cs.ports
				});
			}
			// When compose services are found, return ONLY them (no ghost parent service wrappers)
			if (list.length > 0) {
				return list;
			}
		}

		// 2. Pod or multi-container runtime: live containers strictly belonging to this service
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

		// 3. Standalone / Quadlet fallback: only if no compose services or containers found
		const defaultName = service.id || service.name;
		list.push({
			value: defaultName,
			label: service.name || defaultName,
			badge: 'Service',
			ports: service.port ? [service.port] : [3000]
		});

		return list;
	});

	// Detected port suggestions for quick click (scoped to selected target)
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

	$effect(() => {
		if (!editingDomainId && !targetContainer) {
			initFormDefaults();
		}
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

	function startEdit(d: Domain) {
		editingDomainId = d.id;
		domainName = d.hostname;
		const targetVal = d.targetContainer || '';
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

		targetPort = String(d.containerPort || service.port || 3000);
		autoHttps = d.tls;
		pathPrefix = d.pathPrefix || '/';
		stripPath = !!d.stripPathPrefix;
		websocket = !!d.websocket;
		cors = !!d.cors;
		hsts = d.hsts !== false;
		basicAuthEnabled = !!d.basicAuth;
		basicAuthUser = d.basicAuthUser || '';
		basicAuthPass = d.basicAuthPass || '';
		isAdding = true;
	}

	function getDomainUrl(d: Domain): string {
		const host = d.hostname.trim().replace(/^https?:\/\//, '').replace(/:\d+$/, '');
		const isLocal = host.endsWith('.localhost') || host.endsWith('.local') || host.includes('127.0.0.1') || host === 'localhost';
		if (isLocal) {
			const port = d.tls ? (caddyHttpsPort !== 443 ? `:${caddyHttpsPort}` : '') : (caddyHttpPort !== 80 ? `:${caddyHttpPort}` : '');
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

	function handleSaveDomain() {
		if (!domainName.trim()) return;

		const portNum = parseInt(targetPort, 10) || service.port || 3000;
		let cleanHost = domainName.trim().toLowerCase();
		cleanHost = cleanHost.replace(/^https?:\/\//, '');
		cleanHost = cleanHost.replace(/:\d+$/, '');

		const isLocal = cleanHost.endsWith('.localhost') || cleanHost.endsWith('.local') || cleanHost.includes('127.0.0.1') || cleanHost === 'localhost';
		const effectiveTarget = isCustomTarget ? customTargetName.trim() : targetContainer.trim();

		if (editingDomainId) {
			const existing = dataStore.domains.find((d) => d.id === editingDomainId);
			if (existing) {
				existing.hostname = cleanHost;
				existing.targetContainer = effectiveTarget || undefined;
				existing.containerPort = portNum;
				existing.publishedPort = portNum;
				existing.tls = autoHttps;
				existing.pathPrefix = pathPrefix.trim() || '/';
				existing.stripPathPrefix = stripPath;
				existing.websocket = websocket;
				existing.cors = cors;
				existing.hsts = hsts;
				existing.basicAuth = basicAuthEnabled;
				existing.basicAuthUser = basicAuthEnabled ? basicAuthUser : undefined;
				existing.basicAuthPass = basicAuthEnabled ? basicAuthPass : undefined;
				if (isLocal) {
					existing.dnsStatus = 'valid';
				}
				dataStore.updateDomain(existing);
			}
		} else {
			const domain: Domain = {
				id: `d-${Date.now()}`,
				hostname: cleanHost,
				projectId: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				targetContainer: effectiveTarget || undefined,
				tls: autoHttps,
				status: 'active',
				proxyPort: 8000 + Math.floor(Math.random() * 900),
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
				dnsStatus: isLocal ? 'valid' : 'pending'
			};
			dataStore.addDomain(domain);
		}

		resetForm();
	}

	function resetForm() {
		isAdding = false;
		editingDomainId = null;
		domainName = '';
		pathPrefix = '/';
		stripPath = false;
		websocket = false;
		cors = false;
		hsts = true;
		showAdvanced = false;
		maxBodySize = '';
		basicAuthEnabled = false;
		basicAuthUser = '';
		basicAuthPass = '';
		initFormDefaults();
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
		<LayerCard
			class="animate-in fade-in flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5 duration-150"
		>
			<div class="flex items-center justify-between border-b border-[var(--border-subtle)] pb-3">
				<div class="flex items-center gap-2">
					<Globe size={16} class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						{editingDomainId ? 'Edit Domain Route' : 'Add Domain Route'}
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
									class="cursor-pointer rounded border border-[var(--border)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10.5px] font-medium text-[var(--text-primary)] transition-colors hover:border-[var(--accent)] hover:text-[var(--accent)]"
								>
									:{p}
								</button>
							{/each}
						</div>
					{/if}
				</div>

				<!-- 4. Path Prefix -->
				<div>
					<Field
						label="Path Prefix (Optional)"
						description="Route specific path prefix (default: /)."
					>
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
					<Switch
						bind:checked={stripPath}
						size="sm"
						label="Strip Path Prefix before proxying"
					/>
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

						<div
							class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-3 sm:col-span-2"
						>
							<Switch
								bind:checked={basicAuthEnabled}
								size="sm"
								label="Enable Basic Authentication (Password protect preview/staging)"
							/>

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
					disabled={!domainName.trim() || (isCustomTarget && !customTargetName.trim())}
					onclick={handleSaveDomain}
				>
					{editingDomainId ? 'Update Route' : 'Save Domain Route'}
				</Button>
			</div>
		</LayerCard>
	{/if}

	<!-- Domain Routes Data Table (Kumo Standard Compliant) -->
	<LayerCard class="overflow-hidden p-0">
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-xs">
				<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<TableRow>
						<TableHead class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Domain
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Upstream Route
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							DNS Status
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							TLS Certificate
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Features
						</TableHead>
						<TableHead class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case">
							Actions
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if serviceDomains.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
								No public domain routes configured for this service yet. Add a domain above to expose this workload via Caddy reverse proxy.
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
											class="flex items-center gap-1 font-[var(--font-mono)] text-[13px] font-medium text-[var(--text-primary)] hover:underline"
										>
											{d.hostname}
											<ArrowSquareOut size={12} class="text-[var(--text-tertiary)]" />
										</a>
										{#if d.pathPrefix && d.pathPrefix !== '/'}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[11px] text-[var(--text-tertiary)]"
											>
												{d.pathPrefix}{d.stripPathPrefix ? ' (stripped)' : ''}
											</span>
										{/if}
									</div>
								</TableCell>

								<!-- Upstream Route: Target Container / Service ➜ Port -->
								<TableCell class="px-3 py-3 whitespace-nowrap">
									<div class="flex items-center gap-2">
										<div class="flex items-center gap-1.5 font-[var(--font-mono)] text-[13px] font-medium text-[var(--text-primary)]">
											<HardDrive size={14} class="text-[var(--text-tertiary)]" />
											<span>{d.targetContainer || service.name}</span>
											<span class="text-[var(--text-tertiary)]">:</span>
											<span class="font-semibold text-[var(--accent)]">{d.containerPort}</span>
										</div>
										{#if d.targetContainer}
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10.5px] text-[var(--text-secondary)]"
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
										<span class="inline-flex items-center gap-1.5 text-xs text-[var(--status-green)]">
											<ShieldCheck size={14} /> SSL Active (ACME)
										</span>
									{:else}
										<span class="inline-flex items-center gap-1.5 text-xs text-[var(--text-tertiary)]">
											<LockOpen size={14} /> Plain HTTP
										</span>
									{/if}
								</TableCell>

								<!-- Features -->
								<TableCell class="px-3 py-3">
									<div class="flex flex-wrap items-center gap-1">
										{#if d.websocket}
											<span class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10px] text-[var(--text-secondary)]">WS</span>
										{/if}
										{#if d.cors}
											<span class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10px] text-[var(--text-secondary)]">CORS</span>
										{/if}
										{#if d.basicAuth}
											<span class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10px] text-[var(--status-amber)]">Auth</span>
										{/if}
										{#if d.hsts}
											<span class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-header)] px-1.5 py-0.5 font-[var(--font-mono)] text-[10px] text-[var(--text-tertiary)]">HSTS</span>
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
											class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:opacity-50"
											title="Revalidate DNS record"
											aria-label="Revalidate DNS record"
										>
											<ArrowsClockwise size={14} class={revalidatingIds.has(d.id) ? 'animate-spin' : ''} />
										</button>

										<button
											type="button"
											onclick={() => startEdit(d)}
											class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
											title="Edit domain route"
											aria-label="Edit domain route"
										>
											<PencilSimple size={14} />
										</button>

										<button
											type="button"
											onclick={() => handleDeleteDomain(d.id)}
											class="flex h-7 w-7 min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
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
