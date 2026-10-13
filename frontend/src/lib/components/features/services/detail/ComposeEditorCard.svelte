<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Input } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import {
		Stack,
		Eye,
		Check,
		FileCode,
		GitBranch,
		GithubLogo,
		GitlabLogo,
		ShareNetwork,
		HardDrives,
		Info,
		ArrowsSplit
	} from 'phosphor-svelte';

	import { getDefaultComposeYaml } from '../manifests/defaults';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let providerMode = $state<'raw' | 'git'>('raw');
	let gitProvider = $state<'github' | 'gitlab' | 'custom'>('github');
	let gitRepoUrl = $state('');
	let gitBranch = $state('main');
	let gitComposePath = $state('docker-compose.yml');
	let composeYaml = $state('');

	let previewOpen = $state(false);
	let saveStatus = $state<'idle' | 'saving' | 'saved'>('idle');
	let lastLoadedServiceId = $state<string | null>(null);
	let inPod = $state(false);

	$effect(() => {
		if (service.id !== lastLoadedServiceId) {
			lastLoadedServiceId = service.id;
			const isGit =
				service.sourceType === 'git' ||
				Boolean(service.source && /^(https?:\/\/|git@|ssh:\/\/|.*\.git$)/.test(service.source.trim()));
			gitRepoUrl = isGit ? service.source || '' : '';
			gitBranch = service.branch || 'main';
			gitComposePath = 'docker-compose.yml';
			composeYaml = service.composeYaml || service.k8sYaml || getDefaultComposeYaml(service);
			inPod = service.inPod || service.runtimeTarget === 'pod';
			providerMode = isGit ? 'git' : 'raw';
		}
	});

	function toggleInPod(checked?: boolean) {
		inPod = checked !== undefined ? checked : !inPod;
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		dataStore.updateService(service);
		toast.add({
			title: inPod ? 'Podman pod mode enabled' : 'Standalone bridge mode enabled',
			description: inPod
				? 'Services will share localhost network namespace in an isolated Pod.'
				: 'Services will run as independent containers with bridge DNS and custom network isolation.',
			variant: 'info'
		});
	}

	interface ParsedServiceItem {
		name: string;
		image?: string;
		ports: string[];
		networks: string[];
		dependsOn: string[];
	}

	interface ParsedNetworkItem {
		name: string;
		driver?: string;
		external?: boolean;
	}

	interface ParsedComposeStack {
		services: ParsedServiceItem[];
		networks: ParsedNetworkItem[];
		volumes: string[];
	}

	// Declarative parser for inspecting Compose architecture (workloads, networks, volumes)
	let parsedStack = $derived.by<ParsedComposeStack>(() => {
		try {
			const lines = composeYaml.split('\n');
			const services: ParsedServiceItem[] = [];
			const networks: ParsedNetworkItem[] = [];
			const volumes: string[] = [];

			let currentSection: 'services' | 'networks' | 'volumes' | null = null;
			let currentSvc: ParsedServiceItem | null = null;
			let currentNet: ParsedNetworkItem | null = null;
			let subArrayField: 'ports' | 'networks' | 'depends_on' | null = null;

			for (const rawLine of lines) {
				const trimmed = rawLine.trim();
				if (!trimmed || trimmed.startsWith('#')) continue;

				// Root keys (no leading indentation)
				if (!rawLine.startsWith(' ') && !rawLine.startsWith('\t')) {
					if (currentSvc) {
						services.push(currentSvc);
						currentSvc = null;
					}
					if (currentNet) {
						networks.push(currentNet);
						currentNet = null;
					}
					subArrayField = null;

					if (trimmed.startsWith('services:')) {
						currentSection = 'services';
					} else if (trimmed.startsWith('networks:')) {
						currentSection = 'networks';
					} else if (trimmed.startsWith('volumes:')) {
						currentSection = 'volumes';
					} else {
						currentSection = null;
					}
					continue;
				}

				if (currentSection === 'services') {
					const svcMatch = rawLine.match(/^ {2}([a-zA-Z0-9_.-]+):/);
					if (svcMatch) {
						if (currentSvc) services.push(currentSvc);
						currentSvc = {
							name: svcMatch[1],
							ports: [],
							networks: [],
							dependsOn: []
						};
						subArrayField = null;
						continue;
					}

					if (currentSvc) {
						const imgMatch = trimmed.match(/^image:\s*([^\s#]+)/);
						if (imgMatch) {
							currentSvc.image = imgMatch[1];
							subArrayField = null;
							continue;
						}

						if (trimmed.startsWith('ports:')) {
							subArrayField = 'ports';
							continue;
						}
						if (trimmed.startsWith('networks:')) {
							subArrayField = 'networks';
							continue;
						}
						if (trimmed.startsWith('depends_on:')) {
							subArrayField = 'depends_on';
							continue;
						}

						const itemMatch = trimmed.match(/^-\s+["']?([^"'#]+)["']?/);
						if (itemMatch && subArrayField) {
							const val = itemMatch[1].trim();
							if (subArrayField === 'ports') currentSvc.ports.push(val);
							else if (subArrayField === 'networks') currentSvc.networks.push(val);
							else if (subArrayField === 'depends_on') currentSvc.dependsOn.push(val);
							continue;
						}

						if (!trimmed.startsWith('-')) {
							subArrayField = null;
						}
					}
				} else if (currentSection === 'networks') {
					const netMatch = rawLine.match(/^ {2}([a-zA-Z0-9_.-]+):/);
					if (netMatch) {
						if (currentNet) networks.push(currentNet);
						currentNet = { name: netMatch[1] };
						continue;
					}
					if (currentNet) {
						const driverMatch = trimmed.match(/^driver:\s*([^\s#]+)/);
						if (driverMatch) currentNet.driver = driverMatch[1];
						const extMatch = trimmed.match(/^external:\s*(true|false)/);
						if (extMatch) currentNet.external = extMatch[1] === 'true';
					}
				} else if (currentSection === 'volumes') {
					const volMatch = rawLine.match(/^ {2}([a-zA-Z0-9_.-]+):?/);
					if (volMatch && volMatch[1]) {
						const volName = volMatch[1];
						if (!volumes.includes(volName)) {
							volumes.push(volName);
						}
					}
				}
			}

			if (currentSvc) services.push(currentSvc);
			if (currentNet) networks.push(currentNet);

			return {
				services:
					services.length > 0
						? services
						: [
								{
									name: service.name,
									image: service.image || 'nginx:alpine',
									ports: [`${service.port || 8080}:80`],
									networks: [],
									dependsOn: []
								}
							],
				networks,
				volumes
			};
		} catch {
			return {
				services: [
					{
						name: service.name,
						image: service.image || 'nginx:alpine',
						ports: [`${service.port || 8080}:80`],
						networks: [],
						dependsOn: []
					}
				],
				networks: [],
				volumes: []
			};
		}
	});

	async function handleSave() {
		saveStatus = 'saving';
		if (service.type === 'kubernetes') {
			service.k8sYaml = composeYaml;
		} else {
			service.composeYaml = composeYaml;
		}
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		if (providerMode === 'git') {
			service.source = gitRepoUrl.trim();
			service.branch = gitBranch.trim();
			service.sourceType = 'git';
		} else {
			service.source = '';
			service.sourceType = 'compose';
		}

		try {
			await dataStore.updateService(service);
			saveStatus = 'saved';
			toast.add({
				title: 'Compose manifest saved',
				description: `Configuration and network strategy for ${service.name} have been updated.`,
				variant: 'success'
			});
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err: any) {
			console.error('Failed to save compose config:', err);
			saveStatus = 'idle';
			toast.add({
				title: 'Save failed',
				description: err?.message || 'Failed to save compose configuration',
				variant: 'error'
			});
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) -->
	<LayerCard.Secondary class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3">
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring ring-kumo-line"
			>
				<FileCode size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<h3 class="m-0 text-sm font-semibold text-kumo-default">Compose specification</h3>
				<span class="text-[13px] text-kumo-subtle">
					Multi-container orchestration definition for Podman & Docker Compose
				</span>
			</div>
		</div>

		<!-- Right Controls: Source Mode Selector + Preview Button -->
		<div class="flex items-center gap-2.5">
			<!-- Compact Source Segmented Pill -->
			<div
				class="inline-flex items-center rounded-md bg-kumo-base p-0.5 ring ring-kumo-hairline"
				role="tablist"
			>
				<button
					type="button"
					onclick={() => (providerMode = 'raw')}
					class="cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors {providerMode ===
					'raw'
						? 'bg-kumo-elevated text-kumo-default shadow-xs ring ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					Inline YAML
				</button>
				<button
					type="button"
					onclick={() => (providerMode = 'git')}
					class="cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors {providerMode ===
					'git'
						? 'bg-kumo-elevated text-kumo-default shadow-xs ring ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					Git repository
				</button>
			</div>

			<!-- Inspect Stack Button -->
			<Button
				variant="secondary"
				size="sm"
				onclick={() => (previewOpen = !previewOpen)}
				class="gap-1.5"
			>
				<Eye size={14} />
				<span>{previewOpen ? 'Hide inspection' : 'Inspect stack'}</span>
				<span
					class="ml-1 inline-flex items-center rounded bg-kumo-recessed px-1.5 py-0.2 font-mono text-[11px] text-kumo-subtle"
				>
					{parsedStack.services.length}
				</span>
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Main Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-4">
		<!-- Git Source Inputs (Shown only if Git mode selected) -->
		{#if providerMode === 'git'}
			<div
				class="flex flex-col gap-3 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line"
			>
				<!-- Quick Provider selector -->
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (gitProvider = 'github')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
						'github'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GithubLogo size={14} /> GitHub
					</button>
					<button
						type="button"
						onclick={() => (gitProvider = 'gitlab')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
						'gitlab'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GitlabLogo size={14} /> GitLab
					</button>
					<button
						type="button"
						onclick={() => (gitProvider = 'custom')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2.5 py-1 text-xs transition-colors {gitProvider ===
						'custom'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GitBranch size={14} /> Custom Git / SSH
					</button>
				</div>

				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1 sm:col-span-2">
						<label for="compose-repo" class="text-xs font-medium text-kumo-subtle">
							Repository URL
						</label>
						<Input
							id="compose-repo"
							bind:value={gitRepoUrl}
							placeholder="https://github.com/user/stack-repo.git"
							class="font-mono text-xs"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<label for="compose-branch" class="text-xs font-medium text-kumo-subtle">
							Branch
						</label>
						<Input
							id="compose-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="font-mono text-xs"
						/>
					</div>
				</div>

				<div class="flex flex-col gap-1">
					<label for="compose-path" class="text-xs font-medium text-kumo-subtle">
						Compose file path in repository
					</label>
					<Input
						id="compose-path"
						bind:value={gitComposePath}
						placeholder="docker-compose.yml"
						class="font-mono text-xs"
					/>
				</div>
			</div>
		{/if}

		<!-- Podman Pod Encapsulation Strategy Banner -->
		<div
			class="flex flex-col justify-between gap-3 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line sm:flex-row sm:items-center"
		>
			<div class="flex items-start gap-3">
				<Switch
					size="sm"
					id="compose-in-pod"
					bind:checked={inPod}
					onCheckedChange={toggleInPod}
					aria-label="Encapsulate stack inside Podman pod"
					class="mt-0.5"
				/>
				<label for="compose-in-pod" class="flex cursor-pointer flex-col select-none">
					<span class="text-sm font-semibold text-kumo-default">
						Encapsulate stack inside Podman pod
					</span>
					<span class="text-[13px] text-kumo-subtle leading-normal">
						Creates an isolated Pod with shared localhost. Uncheck to run services as independent containers with bridge DNS and custom network isolation.
					</span>
				</label>
			</div>

			<span
				class="shrink-0 rounded border px-2.5 py-1 font-mono text-xs {inPod
					? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent-text)]'
					: 'border-kumo-line bg-kumo-recessed text-kumo-subtle'}"
			>
				{inPod ? '--in-pod=true' : '--in-pod=false'}
			</span>
		</div>

		<!-- Monospace Code Editor -->
		<div class="overflow-hidden rounded-md ring ring-kumo-line">
			<CodeEditor bind:value={composeYaml} language="yaml" height="auto" />
		</div>

		<!-- Workload & Architecture Inspection Drawer -->
		{#if previewOpen}
			<div
				class="flex flex-col gap-4 rounded-md bg-kumo-base p-4 ring ring-kumo-line"
			>
				<!-- Section 1: Detected Workload Containers -->
				<div class="flex flex-col gap-2">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
							<Stack size={14} class="text-[var(--accent)]" /> Detected workloads ({parsedStack.services.length})
						</span>
						<span class="text-[11px] text-kumo-subtle">
							Auto-assigned service alias and Caddy proxy endpoints
						</span>
					</div>

					<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 md:grid-cols-3">
						{#each parsedStack.services as svc}
							<div
								class="flex flex-col gap-1.5 rounded bg-kumo-elevated p-3 ring ring-kumo-hairline"
							>
								<div class="flex items-center justify-between gap-1">
									<span class="font-mono text-xs font-bold text-kumo-default truncate">
										{svc.name}
									</span>
									{#if svc.networks.length > 0}
										<span
											class="inline-flex items-center gap-1 rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle"
										>
											<ShareNetwork size={10} /> {svc.networks[0]}
										</span>
									{/if}
								</div>

								{#if svc.image}
									<span class="truncate font-mono text-[11px] text-kumo-subtle">
										{svc.image}
									</span>
								{/if}

								{#if svc.ports.length > 0}
									<div class="flex flex-wrap gap-1 pt-0.5">
										{#each svc.ports as port}
											<span class="rounded bg-[var(--accent-muted)] px-1.5 py-0.2 font-mono text-[10px] text-[var(--accent-text)]">
												{port}
											</span>
										{/each}
									</div>
								{/if}

								{#if svc.dependsOn.length > 0}
									<div class="flex items-center gap-1 pt-0.5 text-[10px] text-kumo-subtle">
										<ArrowsSplit size={11} class="shrink-0" />
										<span class="truncate">depends: {svc.dependsOn.join(', ')}</span>
									</div>
								{/if}
							</div>
						{/each}
					</div>
				</div>

				<!-- Section 2: Stack Networking Architecture -->
				<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
					<div class="flex items-center justify-between">
						<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
							<ShareNetwork size={14} class="text-[var(--status-blue)]" /> Stack networking architecture
						</span>
						<span class="text-[11px] text-kumo-subtle">
							{parsedStack.networks.length} declared + 1 ingress bridge
						</span>
					</div>

					<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
						<!-- User Declared Networks -->
						{#if parsedStack.networks.length > 0}
							{#each parsedStack.networks as net}
								<div class="flex items-center justify-between rounded bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline">
									<div class="flex items-center gap-2">
										<ShareNetwork size={13} class="text-[var(--text-secondary)]" />
										<span class="font-mono text-xs font-medium text-kumo-default">{net.name}</span>
									</div>
									<span class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle">
										{net.driver || 'bridge'}
									</span>
								</div>
							{/each}
						{:else}
							<div class="flex items-center justify-between rounded bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline">
								<div class="flex items-center gap-2">
									<ShareNetwork size={13} class="text-[var(--text-secondary)]" />
									<span class="font-mono text-xs font-medium text-kumo-default">default</span>
								</div>
								<span class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle">
									bridge (project isolated)
								</span>
							</div>
						{/if}

						<!-- Ingress Reverse Proxy Bridge -->
						<div class="flex items-center justify-between rounded bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline border-l-2 border-l-[var(--accent)]">
							<div class="flex items-center gap-2">
								<ShareNetwork size={13} class="text-[var(--accent)]" />
								<span class="font-mono text-xs font-medium text-kumo-default">gopod-net</span>
							</div>
							<span class="rounded bg-[var(--accent-muted)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent-text)]">
								Caddy ingress bridge
							</span>
						</div>
					</div>

					<!-- Informational note for peace of mind -->
					<div class="flex items-center gap-2 rounded bg-kumo-recessed px-3 py-2 text-[12px] text-kumo-subtle">
						<Info size={14} class="shrink-0 text-kumo-default" />
						<span>
							GoPod merges <code class="font-mono text-kumo-default">gopod-net</code> into the stack so Caddy reverse proxy can resolve containers via internal DNS while preserving custom network isolation.
						</span>
					</div>
				</div>

				<!-- Section 3: Storage Volumes (if any declared) -->
				{#if parsedStack.volumes.length > 0}
					<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
						<div class="flex items-center justify-between">
							<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
								<HardDrives size={14} class="text-[var(--status-purple)]" /> Persistent storage volumes ({parsedStack.volumes.length})
							</span>
						</div>

						<div class="flex flex-wrap gap-2">
							{#each parsedStack.volumes as vol}
								<span class="inline-flex items-center gap-1.5 rounded bg-kumo-elevated px-2.5 py-1 font-mono text-xs text-kumo-default ring ring-kumo-hairline">
									<HardDrives size={12} class="text-kumo-subtle" /> {vol}
								</span>
							{/each}
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<!-- Footer with Save CTA -->
		<div
			class="flex items-center justify-between border-t border-kumo-hairline pt-3"
		>
			<span class="text-xs text-kumo-subtle">
				Configuration is synchronized with the GoPod runtime engine.
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="flex items-center gap-1 text-xs text-kumo-success">
						<Check size={14} weight="bold" /> Saved
					</span>
				{/if}
				<Button
					variant="primary"
					size="sm"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
				>
					{saveStatus === 'saving' ? 'Saving…' : 'Save compose specification'}
				</Button>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
