<script lang="ts">
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { Button, Card, Chip } from '$lib/components/primitives';
	import { containers, pods, volumes, networks, server, projects } from '$lib/data';
	import type { WorkloadChoice } from '$lib/components/features/services/create/WorkloadTypeSelector.svelte';
	import {
		TopologyCanvas,
		TopologyInspector,
		computeTopologyGraph,
		type SelectedItem
	} from '$lib/components/features/topology';
	import {
		Plus,
		ArrowRight,
		ArrowUpRight,
		SquaresFour,
		List,
		FileText,
		Stack,
		Database,
		GitBranch,
		ShareNetwork,
		CornersOut
	} from 'phosphor-svelte';
	import type { Service, Project, Domain } from '$lib/types';

	interface Props {
		project: Project;
		services: Service[];
		projectDomains: Domain[];
		oncreateService: (type: WorkloadChoice) => void;
	}

	let { project, services, projectDomains, oncreateService }: Props = $props();

	let viewMode = $state<'cards' | 'table' | 'topology'>('cards');
	let topoSelectedItem: SelectedItem = $state(null);
	let topoZoom = $state(0.9);
	let topoPanX = $state(40);
	let topoPanY = $state(40);

	let projectTopologyGraph = $derived(
		computeTopologyGraph({
			projects,
			services,
			containers,
			pods,
			domains: projectDomains,
			volumes,
			networks,
			server,
			viewMode: project.id
		})
	);

	// ── Service Category Filtering (Dokploy & PaaS Standard) ──
	type ServiceCategory = 'all' | 'application' | 'quadlet' | 'compose' | 'database';
	let selectedCategory = $state<ServiceCategory>('all');

	let appServices = $derived(services.filter((s) => s.type === 'application'));
	let quadletServices = $derived(services.filter((s) => s.type === 'quadlet'));
	let composeServices = $derived(
		services.filter((s) => s.type === 'compose' || s.type === 'kubernetes' || s.type === 'pod')
	);
	let databaseServices = $derived(
		services.filter(
			(s) =>
				s.type === 'database' ||
				(s.type === 'image' &&
					(s.name.includes('redis') ||
						s.name.includes('postgres') ||
						s.name.includes('mysql') ||
						s.name.includes('mongo') ||
						s.name.includes('db')))
		)
	);

	let filteredServices = $derived.by(() => {
		switch (selectedCategory) {
			case 'application':
				return appServices;
			case 'quadlet':
				return quadletServices;
			case 'compose':
				return composeServices;
			case 'database':
				return databaseServices;
			default:
				return services;
		}
	});

	function getCategoryLabel(s: Service): string {
		switch (s.type) {
			case 'application':
				return 'Application';
			case 'compose':
				return s.workloads && s.workloads.length > 1
					? `Compose · ${s.workloads.length}`
					: 'Compose';
			case 'pod':
				return s.workloads && s.workloads.length > 1 ? `Pod · ${s.workloads.length}` : 'Pod';
			case 'kubernetes':
				return 'Kubernetes';
			case 'quadlet':
				return 'Quadlet (Systemd)';
			case 'image':
				return 'Container';
			default:
				return 'Service';
		}
	}

	function formatSource(s: Service): string {
		if (s.type === 'quadlet') {
			return s.source || `${s.name}.container`;
		}
		if (s.type === 'application' && s.source) {
			return s.branch ? `${s.source} · ${s.branch}` : s.source;
		}
		if (s.image) return s.image;
		if (s.source) return s.source;
		return '—';
	}

	function getServiceDomain(s: Service): string | undefined {
		if (s.domain) return `${s.domain} :${s.port}`;
		const match = projectDomains.find((d) => d.serviceId === s.id);
		if (match) return `${match.hostname} :${match.containerPort}`;
		return s.port ? `:${s.port}` : undefined;
	}
</script>

<section class="flex flex-col gap-4">
	<!-- Header & Filter Row -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<!-- Category Filter Pills -->
		<div
			class="inline-flex max-w-full items-center gap-1 overflow-x-auto rounded-lg border border-[var(--border)] bg-[var(--bg-shell)] p-1"
		>
			<button
				type="button"
				onclick={() => (selectedCategory = 'all')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {selectedCategory ===
				'all'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span>All</span>
				<span
					class="py-0.2 rounded bg-[var(--bg-panel)] px-1.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
				>
					{services.length}
				</span>
			</button>

			<button
				type="button"
				onclick={() => (selectedCategory = 'application')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {selectedCategory ===
				'application'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<GitBranch size={13} class="opacity-70" />
				<span>Applications</span>
				{#if appServices.length > 0}
					<span
						class="py-0.2 rounded bg-[var(--bg-panel)] px-1.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
					>
						{appServices.length}
					</span>
				{/if}
			</button>

			<button
				type="button"
				onclick={() => (selectedCategory = 'quadlet')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {selectedCategory ===
				'quadlet'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<FileText size={13} class="text-[var(--accent)]" />
				<span>Quadlet (Systemd)</span>
				{#if quadletServices.length > 0}
					<span
						class="py-0.2 rounded bg-[var(--bg-panel)] px-1.5 text-[10px] font-[var(--font-mono)] text-[var(--accent)]"
					>
						{quadletServices.length}
					</span>
				{/if}
			</button>

			<button
				type="button"
				onclick={() => (selectedCategory = 'compose')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {selectedCategory ===
				'compose'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<Stack size={13} class="opacity-70" />
				<span>Compose / Stacks</span>
				{#if composeServices.length > 0}
					<span
						class="py-0.2 rounded bg-[var(--bg-panel)] px-1.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
					>
						{composeServices.length}
					</span>
				{/if}
			</button>

			<button
				type="button"
				onclick={() => (selectedCategory = 'database')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {selectedCategory ===
				'database'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<Database size={13} class="opacity-70" />
				<span>Databases</span>
				{#if databaseServices.length > 0}
					<span
						class="py-0.2 rounded bg-[var(--bg-panel)] px-1.5 text-[10px] font-[var(--font-mono)] text-[var(--text-secondary)]"
					>
						{databaseServices.length}
					</span>
				{/if}
			</button>
		</div>

		<!-- View Mode Toggle: Cards / Table / Topology -->
		{#if services.length > 0}
			<div
				class="inline-flex items-center self-end rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] p-0.5 sm:self-auto"
			>
				<button
					type="button"
					onclick={() => (viewMode = 'cards')}
					class="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[calc(var(--radius-sm)-2px)] border-0 transition-colors {viewMode ===
					'cards'
						? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					title="Cards view"
					aria-label="Cards view"
				>
					<SquaresFour size={14} />
				</button>
				<button
					type="button"
					onclick={() => (viewMode = 'table')}
					class="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[calc(var(--radius-sm)-2px)] border-0 transition-colors {viewMode ===
					'table'
						? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					title="Table view"
					aria-label="Table view"
				>
					<List size={14} />
				</button>
				<button
					type="button"
					onclick={() => (viewMode = 'topology')}
					class="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[calc(var(--radius-sm)-2px)] border-0 transition-colors {viewMode ===
					'topology'
						? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					title="Architecture Topology Map"
					aria-label="Architecture Topology Map"
				>
					<ShareNetwork size={14} />
				</button>
			</div>
		{/if}
	</div>

	<!-- Services Content -->
	{#if filteredServices.length === 0}
		<!-- Contextual Empty State -->
		<div
			class="flex flex-col items-center justify-center gap-3 rounded-[var(--radius-card)] border border-dashed border-[var(--border)] bg-[var(--bg-panel)] p-8 text-center"
		>
			{#if selectedCategory === 'quadlet'}
				<div
					class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-[rgba(105,115,168,0.12)] text-[var(--accent)]"
				>
					<FileText size={20} />
				</div>
				<div class="flex max-w-md flex-col gap-1">
					<span class="text-sm font-semibold text-[var(--text-primary)]"
						>No Quadlet services found</span
					>
					<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
						Quadlet enables declarative systemd container services (<code
							class="text-[var(--accent)]">.container</code
						>) that auto-start on host boot, restart cleanly, and integrate natively into systemd
						journal logging.
					</p>
				</div>
				<Button variant="primary" size="sm" onclick={() => oncreateService('quadlet')} class="mt-1">
					<Plus size={13} /> Deploy Quadlet Service
				</Button>
			{:else if selectedCategory === 'compose'}
				<div
					class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-[var(--bg-surface)] text-[var(--text-secondary)]"
				>
					<Stack size={20} />
				</div>
				<div class="flex max-w-md flex-col gap-1">
					<span class="text-sm font-semibold text-[var(--text-primary)]"
						>No Compose stacks found</span
					>
					<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
						Deploy multi-container workloads managed seamlessly via Compose YAML or Kubernetes pod
						manifests.
					</p>
				</div>
				<Button variant="primary" size="sm" onclick={() => oncreateService('compose')} class="mt-1">
					<Plus size={13} /> Deploy Compose Stack
				</Button>
			{:else if selectedCategory === 'database'}
				<div
					class="mb-1 flex h-10 w-10 items-center justify-center rounded-full bg-[var(--bg-surface)] text-[var(--text-secondary)]"
				>
					<Database size={20} />
				</div>
				<div class="flex max-w-md flex-col gap-1">
					<span class="text-sm font-semibold text-[var(--text-primary)]">No Databases deployed</span
					>
					<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
						Launch dedicated PostgreSQL, Redis, MySQL, or MongoDB engines with isolated persistent
						volumes in 1 click.
					</p>
				</div>
				<Button
					variant="primary"
					size="sm"
					onclick={() => oncreateService('database')}
					class="mt-1"
				>
					<Plus size={13} /> Deploy Database
				</Button>
			{:else}
				<div class="flex max-w-md flex-col gap-1">
					<span class="text-sm font-semibold text-[var(--text-primary)]">No services found</span>
					<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
						Deploy your first service to this project.
					</p>
				</div>
				<Button
					variant="secondary"
					size="sm"
					onclick={() => oncreateService('application')}
					class="mt-1"
				>
					<Plus size={13} /> Create your first service
				</Button>
			{/if}
		</div>
	{:else if viewMode === 'cards'}
		<!-- Cards View (Default) -->
		<div class="grid grid-cols-1 gap-3.5 md:grid-cols-2 xl:grid-cols-3">
			{#each filteredServices as svc}
				{@const domainInfo = getServiceDomain(svc)}
				{@const sourceInfo = formatSource(svc)}
				{@const categoryLabel = getCategoryLabel(svc)}
				<Card
					interactive
					padding="md"
					onclick={() => goto(`/projects/${project.id}/services/${svc.id}`)}
					class="flex h-full flex-col justify-between gap-4"
				>
					<div class="flex flex-col gap-2.5">
						<!-- Top Row: Name + Status on left, Category Chip on right -->
						<div class="flex items-start justify-between gap-2">
							<div class="flex min-w-0 items-center gap-2">
								<span
									class="truncate text-sm font-medium text-[var(--text-primary)] transition-colors group-hover:text-[var(--accent)]"
								>
									{svc.name}
								</span>
								<StatusBadge status={svc.status} size="sm" />
							</div>
							<Chip
								size="sm"
								variant={svc.type === 'quadlet' || svc.type === 'kubernetes' ? 'mono' : 'default'}
							>
								{categoryLabel}
							</Chip>
						</div>

						<!-- Middle: Description or Source snippet -->
						{#if svc.description}
							<p class="m-0 line-clamp-2 text-xs leading-relaxed text-[var(--text-secondary)]">
								{svc.description}
							</p>
						{:else if sourceInfo && sourceInfo !== '—'}
							<p class="m-0 truncate text-xs font-[var(--font-mono)] text-[var(--text-tertiary)]">
								{sourceInfo}
							</p>
						{/if}
					</div>

					<!-- Bottom Row: Domain / Port info on left, Container info / Arrow on right -->
					<div
						class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-2.5 text-xs text-[var(--text-tertiary)]"
					>
						{#if domainInfo}
							<div
								class="flex items-center gap-1 truncate text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]"
							>
								<ArrowUpRight size={12} class="shrink-0 text-[var(--text-tertiary)]" />
								<span class="truncate">{domainInfo}</span>
							</div>
						{:else if sourceInfo && sourceInfo !== '—'}
							<span
								class="truncate text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]"
							>
								{sourceInfo}
							</span>
						{:else}
							<span class="text-[11px] text-[var(--text-tertiary)]">Internal service</span>
						{/if}

						<div class="flex shrink-0 items-center gap-1 text-[11px] text-[var(--text-tertiary)]">
							{#if svc.replicas && svc.replicas > 1}
								<span>{svc.replicas} replicas</span>
							{:else if svc.workloads && svc.workloads.length > 1}
								<span>{svc.workloads.length} containers</span>
							{/if}
							<ArrowRight
								size={12}
								class="ml-0.5 text-[var(--text-tertiary)] opacity-0 transition-opacity group-hover:opacity-100"
							/>
						</div>
					</div>
				</Card>
			{/each}
		</div>
	{:else if viewMode === 'table'}
		<!-- Table / List View -->
		<div
			class="flex flex-col divide-y divide-[var(--border-subtle)] overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
		>
			{#each filteredServices as svc}
				{@const domainInfo = getServiceDomain(svc)}
				{@const sourceInfo = formatSource(svc)}
				{@const categoryLabel = getCategoryLabel(svc)}
				<button
					type="button"
					onclick={() => goto(`/projects/${project.id}/services/${svc.id}`)}
					class="group flex w-full cursor-pointer items-center justify-between border-0 bg-transparent p-4 text-left transition-colors hover:bg-[var(--bg-hover)]"
				>
					<div class="flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center sm:gap-6">
						<!-- Name + Status -->
						<div class="flex min-w-[160px] items-center gap-2.5">
							<span
								class="text-sm font-medium text-[var(--text-primary)] transition-colors group-hover:text-[var(--accent)]"
							>
								{svc.name}
							</span>
							<StatusBadge status={svc.status} size="sm" />
						</div>

						<!-- Category Chip -->
						<div class="min-w-[110px]">
							<Chip
								size="sm"
								variant={svc.type === 'quadlet' || svc.type === 'kubernetes' ? 'mono' : 'default'}
							>
								{categoryLabel}
							</Chip>
						</div>

						<!-- Source / Image -->
						<div
							class="max-w-[260px] min-w-[160px] overflow-hidden text-ellipsis whitespace-nowrap"
						>
							<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">
								{sourceInfo}
							</span>
						</div>

						<!-- Public Endpoint / Port -->
						{#if domainInfo}
							<div
								class="flex items-center gap-1 text-xs font-[var(--font-mono)] text-[var(--text-secondary)]"
							>
								<ArrowUpRight size={13} class="text-[var(--text-tertiary)]" />
								<span>{domainInfo}</span>
							</div>
						{/if}
					</div>

					<span
						class="pl-3 text-[var(--text-tertiary)] opacity-0 transition-opacity group-hover:opacity-100"
					>
						<ArrowRight size={14} />
					</span>
				</button>
			{/each}
		</div>
	{:else if viewMode === 'topology'}
		<!-- In-Page Project Topology Architecture Studio -->
		<div
			class="relative flex h-[620px] w-full overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-canvas)] shadow-xs select-none"
		>
			<TopologyCanvas
				graph={projectTopologyGraph}
				selectedItem={topoSelectedItem}
				onselect={(item) => (topoSelectedItem = item)}
				bind:zoom={topoZoom}
				bind:panX={topoPanX}
				bind:panY={topoPanY}
				onViewportChange={(v) => {
					topoZoom = v.zoom;
					topoPanX = v.panX;
					topoPanY = v.panY;
				}}
			/>

			<!-- Floating Controls in Project Canvas (Top-Left) -->
			<div class="pointer-events-auto absolute top-3 left-3 z-10 flex items-center gap-2">
				<div
					class="flex items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]/90 px-3 py-1.5 text-xs shadow-xs backdrop-blur-md"
				>
					<ShareNetwork size={14} class="text-[var(--accent)]" />
					<span class="font-semibold text-[var(--text-primary)]">{project.name} Architecture</span>
					<span class="text-[var(--text-tertiary)]">·</span>
					<span class="text-[11px] text-[var(--text-tertiary)]"
						>{projectTopologyGraph.nodes.length} nodes · {projectTopologyGraph.edges.length} links</span
					>
				</div>
			</div>

			<!-- Floating Controls in Project Canvas (Top-Right) -->
			<div class="pointer-events-auto absolute top-3 right-3 z-10 flex items-center gap-2">
				<a
					href="/topology?project={project.id}"
					class="flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]/90 px-2.5 py-1.5 text-xs text-[var(--text-secondary)] no-underline shadow-xs backdrop-blur-md transition-colors hover:border-[var(--accent)] hover:text-[var(--text-primary)]"
				>
					<CornersOut size={13} />
					<span>Fullscreen</span>
				</a>
			</div>

			<!-- Inspector Drawer if node is clicked -->
			{#if topoSelectedItem}
				<div
					class="pointer-events-auto absolute top-0 right-0 bottom-0 z-20 flex h-full w-80 flex-col shadow-2xl sm:w-88"
				>
					<TopologyInspector
						selected={topoSelectedItem}
						onclose={() => (topoSelectedItem = null)}
					/>
				</div>
			{/if}
		</div>
	{/if}
</section>
