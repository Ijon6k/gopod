<script lang="ts">
	import { goto } from '$app/navigation';
	import { dataStore } from '$lib/stores/data.svelte';
	import { ui } from '$lib/stores/ui.svelte';
	import { CommandPalette } from 'kumo-svelte';
	import { Kbd } from '$lib/components/ui';
	import {
		MagnifyingGlass,
		House,
		Folder,
		Rocket,
		Terminal,
		Cube,
		FileCode,
		Disc,
		HardDrive,
		ShareNetwork,
		Globe,
		ArrowsLeftRight,
		Broadcast,
		ChartLineUp,
		Graph,
		Key,
		ShieldCheck,
		GearSix,
		Plus,
		SunDim,
		ArrowClockwise,
		Cpu,
		ArrowRight
	} from 'phosphor-svelte';

	interface Props {
		open: boolean;
	}

	let { open = $bindable(false) }: Props = $props();

	let searchQuery = $state('');
	let inputEl = $state<HTMLInputElement | null>(null);
	let focusTimer: ReturnType<typeof setTimeout> | undefined;

	$effect(() => {
		if (open) {
			focusTimer = setTimeout(() => inputEl?.focus(), 50);
			return () => clearTimeout(focusTimer);
		}
	});

	interface PaletteItem {
		id: string;
		title: string;
		description?: string;
		category: string;
		icon: any;
		action: () => void;
		keywords?: string[];
	}

	const navigationItems: PaletteItem[] = [
		{
			id: 'nav-home',
			title: 'Overview',
			description: 'Platform overview, summary metrics & fleet health',
			category: 'Navigation',
			icon: House,
			action: () => goto('/'),
			keywords: ['home', 'dashboard', 'overview']
		},
		{
			id: 'nav-projects',
			title: 'Projects',
			description: 'Applications, services, and multi-container workloads',
			category: 'Navigation',
			icon: Folder,
			action: () => goto('/projects'),
			keywords: ['projects', 'apps', 'workloads']
		},
		{
			id: 'nav-deployments',
			title: 'Deployments',
			description: 'Deployment history, streaming build logs, and rollbacks',
			category: 'Navigation',
			icon: Rocket,
			action: () => goto('/deployments'),
			keywords: ['deployments', 'builds', 'releases', 'history', 'logs']
		},
		{
			id: 'nav-runtime-containers',
			title: 'Containers',
			description: 'Podman rootless containers, logs, and exec terminal',
			category: 'Navigation',
			icon: Terminal,
			action: () => goto('/runtime/containers'),
			keywords: ['runtime', 'containers', 'docker', 'podman', 'terminal']
		},
		{
			id: 'nav-runtime-quadlets',
			title: 'Quadlets',
			description: 'Declarative systemd-managed container units (~/.config/containers/systemd)',
			category: 'Navigation',
			icon: FileCode,
			action: () => goto('/runtime/quadlets'),
			keywords: ['quadlets', 'systemd', 'units', 'services']
		},
		{
			id: 'nav-runtime-pods',
			title: 'Pods',
			description: 'Podman pods, shared namespaces, and co-located containers',
			category: 'Navigation',
			icon: Cube,
			action: () => goto('/runtime/pods'),
			keywords: ['pods', 'k8s', 'groups']
		},
		{
			id: 'nav-runtime-images',
			title: 'Images',
			description: 'Local container image storage, tags, and registry pulls',
			category: 'Navigation',
			icon: Disc,
			action: () => goto('/runtime/images'),
			keywords: ['images', 'registries', 'tags']
		},
		{
			id: 'nav-runtime-volumes',
			title: 'Volumes',
			description: 'Persistent volumes, host mounts, and storage cgroups',
			category: 'Navigation',
			icon: HardDrive,
			action: () => goto('/runtime/volumes'),
			keywords: ['volumes', 'storage', 'data', 'mounts']
		},
		{
			id: 'nav-runtime-networks',
			title: 'Networks',
			description: 'Software-defined bridge networks, subnets, and DNS',
			category: 'Navigation',
			icon: ShareNetwork,
			action: () => goto('/runtime/networks'),
			keywords: ['networks', 'bridge', 'subnets', 'dns']
		},
		{
			id: 'nav-networking-domains',
			title: 'Domains & SSL',
			description: "Caddy automatic Let's Encrypt HTTPS and custom hostnames",
			category: 'Navigation',
			icon: Globe,
			action: () => goto('/networking/domains'),
			keywords: ['domains', 'caddy', 'https', 'ssl', 'tls', 'proxy']
		},
		{
			id: 'nav-networking-traffic',
			title: 'Traffic & Proxy Logs',
			description: 'Real-time HTTP requests, response latency, and status codes',
			category: 'Navigation',
			icon: ArrowsLeftRight,
			action: () => goto('/networking/traffic'),
			keywords: ['traffic', 'proxy', 'requests', 'logs', 'http']
		},
		{
			id: 'nav-networking-ports',
			title: 'Ingress Ports',
			description: 'Host port bindings, TCP/UDP forwarders, and firewall ingress',
			category: 'Navigation',
			icon: Broadcast,
			action: () => goto('/networking/ports'),
			keywords: ['ports', 'forwarding', 'tcp', 'udp']
		},
		{
			id: 'nav-monitoring',
			title: 'Monitoring & Metrics',
			description: 'CPU, RAM, cgroups v2 resource usage, and host telemetry',
			category: 'Navigation',
			icon: ChartLineUp,
			action: () => goto('/monitoring'),
			keywords: ['monitoring', 'metrics', 'cpu', 'memory', 'ram', 'telemetry']
		},
		{
			id: 'nav-topology',
			title: 'Topology Map',
			description: 'Visual infrastructure network graph and container relationships',
			category: 'Navigation',
			icon: Graph,
			action: () => goto('/topology'),
			keywords: ['topology', 'graph', 'map', 'diagram']
		},
		{
			id: 'nav-credentials',
			title: 'Credentials & SSH Keys',
			description: 'Centralized registry tokens, Git deploy keys, and secrets',
			category: 'Navigation',
			icon: Key,
			action: () => goto('/credentials'),
			keywords: ['credentials', 'keys', 'ssh', 'tokens', 'secrets', 'git']
		},
		{
			id: 'nav-audit-log',
			title: 'Audit Log',
			description: 'Security events, operator activity, and infrastructure audit trail',
			category: 'Navigation',
			icon: ShieldCheck,
			action: () => goto('/audit-log'),
			keywords: ['audit', 'security', 'events', 'activity', 'operator']
		},
		{
			id: 'nav-settings',
			title: 'Settings',
			description: 'Platform preferences, users, updates, and maintenance',
			category: 'Navigation',
			icon: GearSix,
			action: () => goto('/settings'),
			keywords: ['settings', 'preferences', 'config']
		}
	];

	const actionItems: PaletteItem[] = [
		{
			id: 'act-new-project',
			title: 'Create new project',
			description: 'Initialize a new project workspace for microservices or apps',
			category: 'Quick Actions',
			icon: Plus,
			action: () => goto('/projects/new'),
			keywords: ['create', 'new', 'project', 'init']
		},
		{
			id: 'act-new-service',
			title: 'Deploy new service',
			description: 'Create a new container, quadlet, or compose service',
			category: 'Quick Actions',
			icon: Plus,
			action: () => goto('/projects/new-service'),
			keywords: ['deploy', 'new', 'service', 'create']
		},
		{
			id: 'act-add-credential',
			title: 'Add credential / SSH key',
			description: 'Register a Git deploy key, PAT, or container registry login',
			category: 'Quick Actions',
			icon: Key,
			action: () => goto('/credentials'),
			keywords: ['add', 'credential', 'ssh', 'key', 'token']
		},
		{
			id: 'act-toggle-theme',
			title: 'Toggle color theme',
			description: 'Switch between light and dark mode appearance',
			category: 'Quick Actions',
			icon: SunDim,
			action: () => ui.toggleTheme(),
			keywords: ['theme', 'dark', 'light', 'mode', 'color']
		},
		{
			id: 'act-reload-quadlets',
			title: 'Daemon reload (Quadlets)',
			description: 'Trigger systemctl --user daemon-reload for Quadlet generators',
			category: 'Quick Actions',
			icon: ArrowClockwise,
			action: async () => {
				await dataStore.reloadQuadlets();
			},
			keywords: ['reload', 'daemon', 'systemd', 'quadlets']
		}
	];

	let dynamicProjectItems = $derived<PaletteItem[]>(
		(dataStore.projects || []).map((p) => ({
			id: `proj-${p.id}`,
			title: p.name,
			description: `Project • ${p.services?.length ?? 0} services`,
			category: 'Projects',
			icon: Folder,
			action: () => goto(`/projects/${p.id}`),
			keywords: [p.name, 'project', p.id]
		}))
	);

	let dynamicServiceItems = $derived<PaletteItem[]>(
		(dataStore.services || []).map((s) => ({
			id: `srv-${s.id}`,
			title: s.name,
			description: `Service • ${s.type} (${s.status})`,
			category: 'Services',
			icon: Cpu,
			action: () => goto(`/projects/${s.projectId}/services/${s.id}`),
			keywords: [s.name, s.type, s.status, 'service', s.id]
		}))
	);

	let dynamicContainerItems = $derived<PaletteItem[]>(
		(dataStore.containers || []).map((c) => ({
			id: `cnt-${c.id}`,
			title: c.name,
			description: `Container • ${c.image} (${c.status})`,
			category: 'Containers',
			icon: Terminal,
			action: () => goto('/runtime/containers'),
			keywords: [c.name, c.image, c.status, 'container']
		}))
	);

	let dynamicPodItems = $derived<PaletteItem[]>(
		(dataStore.pods || []).map((p) => ({
			id: `pod-${p.id}`,
			title: p.name,
			description: `Pod • ${p.status} • ${p.containers} containers`,
			category: 'Pods',
			icon: Cube,
			action: () => goto('/runtime/pods'),
			keywords: [p.name, p.status, 'pod']
		}))
	);

	let dynamicQuadletItems = $derived<PaletteItem[]>(
		(dataStore.quadlets || []).map((q) => ({
			id: `quad-${q.name}`,
			title: q.name,
			description: `Quadlet • ${q.type} (${q.status})`,
			category: 'Quadlets',
			icon: FileCode,
			action: () => goto('/runtime/quadlets'),
			keywords: [q.name, q.type, q.status, 'quadlet']
		}))
	);

	let allItems = $derived<PaletteItem[]>([
		...actionItems,
		...navigationItems,
		...dynamicProjectItems,
		...dynamicServiceItems,
		...dynamicContainerItems,
		...dynamicPodItems,
		...dynamicQuadletItems
	]);

	let filteredGroups = $derived.by(() => {
		const q = searchQuery.toLowerCase().trim();
		const items = !q
			? allItems
			: allItems.filter((item) => {
					if (item.title.toLowerCase().includes(q)) return true;
					if (item.description && item.description.toLowerCase().includes(q)) return true;
					if (item.category.toLowerCase().includes(q)) return true;
					if (item.keywords?.some((k) => k.toLowerCase().includes(q))) return true;
					return false;
				});

		const groupsMap = new Map<string, PaletteItem[]>();
		for (const item of items) {
			const list = groupsMap.get(item.category) || [];
			list.push(item);
			groupsMap.set(item.category, list);
		}

		return Array.from(groupsMap.entries()).map(([label, groupItems]) => ({
			label,
			items: groupItems
		}));
	});

	function handleSelect(item: PaletteItem) {
		open = false;
		searchQuery = '';
		item.action();
	}
</script>

<CommandPalette.Root
	bind:open
	shouldFilter={false}
	class="bg-kumo-elevated text-kumo-default shadow-2xl ring-1 ring-kumo-line"
>
	<!-- Integrated Search Input -->
	<div class="relative flex items-center border-b border-kumo-hairline bg-kumo-base px-4 py-3">
		<MagnifyingGlass size={16} class="mr-3 shrink-0 text-kumo-subtle" />
		<input
			bind:this={inputEl}
			type="text"
			bind:value={searchQuery}
			placeholder="Search projects, services, runtime, navigation, or commands…"
			class="h-7 w-full border-none bg-transparent text-sm text-kumo-default placeholder:text-kumo-subtle focus:outline-none"
		/>
		{#if searchQuery}
			<button
				type="button"
				onclick={() => (searchQuery = '')}
				class="cursor-pointer border-0 bg-transparent px-1.5 py-0.5 text-xs text-kumo-subtle hover:text-kumo-default"
			>
				Clear
			</button>
		{/if}
	</div>

	<!-- Results List -->
	<CommandPalette.List class="max-h-[60vh] overflow-y-auto bg-kumo-base p-2">
		<CommandPalette.Results>
			{#each filteredGroups as group (group.label)}
				<CommandPalette.Group value={group.label} class="mb-2">
					<CommandPalette.GroupLabel class="px-2.5 py-1 text-[11px] font-medium text-kumo-subtle">
						{group.label}
					</CommandPalette.GroupLabel>
					<CommandPalette.Items>
						{#each group.items as item (item.id)}
							{@const IconComp = item.icon}
							<CommandPalette.Item
								value={item.id}
								onSelect={() => handleSelect(item)}
								onclick={() => handleSelect(item)}
								class="group flex w-full cursor-pointer items-center justify-between gap-3 rounded-md px-2.5 py-2 text-left transition-colors hover:bg-kumo-fill data-[selected]:bg-kumo-fill"
							>
								<div class="flex min-w-0 items-center gap-2.5">
									<div
										class="flex size-7 shrink-0 items-center justify-center rounded bg-kumo-elevated text-kumo-subtle ring-1 ring-kumo-hairline group-hover:text-kumo-default group-hover:ring-kumo-line"
									>
										<IconComp size={15} />
									</div>
									<div class="flex min-w-0 flex-col">
										<span
											class="truncate text-xs font-medium text-kumo-default group-hover:text-[var(--accent-text)]"
										>
											{item.title}
										</span>
										{#if item.description}
											<span class="truncate text-[11px] text-kumo-subtle">
												{item.description}
											</span>
										{/if}
									</div>
								</div>
								<ArrowRight
									size={13}
									class="shrink-0 text-kumo-muted opacity-0 transition-opacity group-hover:opacity-100 group-data-[selected]:opacity-100"
								/>
							</CommandPalette.Item>
						{/each}
					</CommandPalette.Items>
				</CommandPalette.Group>
			{/each}
		</CommandPalette.Results>

		<CommandPalette.Empty class="py-12 text-center text-xs text-kumo-subtle">
			No matching results for "{searchQuery}".
		</CommandPalette.Empty>
	</CommandPalette.List>

	<!-- Footer with keyboard hints -->
	<CommandPalette.Footer
		class="flex items-center justify-between border-t border-kumo-hairline bg-kumo-elevated px-4 py-2.5 text-[11px] text-kumo-subtle"
	>
		<div class="flex items-center gap-3">
			<span class="inline-flex items-center gap-1">
				<Kbd class="text-[9px]">↑</Kbd>
				<Kbd class="text-[9px]">↓</Kbd>
				<span>Navigate</span>
			</span>
			<span class="inline-flex items-center gap-1">
				<Kbd class="text-[9px]">↵</Kbd>
				<span>Select</span>
			</span>
			<span class="inline-flex items-center gap-1">
				<Kbd class="text-[9px]">Esc</Kbd>
				<span>Close</span>
			</span>
		</div>
		<span class="text-kumo-muted">GOPOD Global Command Palette</span>
	</CommandPalette.Footer>
</CommandPalette.Root>
