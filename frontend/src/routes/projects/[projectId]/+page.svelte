<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, DataTable, Tabs } from '$lib/components/ui';
	import { Button, Card, Chip } from '$lib/components/primitives';
	import { getProjectById, getProjectServices, getProjectDomains, getProjectDeployments, containers, pods, volumes, networks, server, projects } from '$lib/data';
	import CreateServiceModal from '$lib/components/features/services/create/CreateServiceModal.svelte';
	import { ProjectBackupsView } from '$lib/components/features/projects';
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
	import type { Service } from '$lib/types';

	let projectId = $derived(page.params.projectId ?? '');
	let project = $derived(projectId ? getProjectById(projectId) : undefined);
	let services = $derived(projectId ? getProjectServices(projectId) : []);
	let projectDomains = $derived(projectId ? getProjectDomains(projectId) : []);
	let projectDeployments = $derived(projectId ? getProjectDeployments(projectId) : []);

	let activeTab = $state<'services' | 'backups' | 'deployments'>('services');
	let projectTabs = $derived([
		{ id: 'services', label: `Services (${services.length})` },
		{ id: 'backups', label: 'Volumes & Backups' },
		{ id: 'deployments', label: `Deployments (${projectDeployments.length})` }
	]);

	let isCreateModalOpen = $state(false);
	let modalInitialType = $state<WorkloadChoice>('application');
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
			viewMode: projectId
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

	function openCreateWithCategory(type: WorkloadChoice) {
		modalInitialType = type;
		isCreateModalOpen = true;
	}

	function getCategoryLabel(s: Service): string {
		switch (s.type) {
			case 'application':
				return 'Application';
			case 'compose':
				return s.workloads && s.workloads.length > 1 ? `Compose · ${s.workloads.length}` : 'Compose';
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

<svelte:head>
	<title>{project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

{#if project}
	<div class="w-full flex flex-col gap-8">
		<!-- Project Identity Header -->
		<PageHeader title={project.name} subtitle={project.description}>
			{#snippet meta()}
				<div class="flex items-center gap-3">
					<StatusBadge status={project.status} size="sm" />
					<span class="text-xs text-[var(--text-tertiary)]">
						{services.length} {services.length === 1 ? 'service' : 'services'} · {projectDomains.length} {projectDomains.length === 1 ? 'domain' : 'domains'}
					</span>
				</div>
			{/snippet}
			{#snippet actions()}
				{#if activeTab === 'services'}
					<Button
						variant="primary"
						size="sm"
						onclick={() =>
							openCreateWithCategory(
								selectedCategory !== 'all' ? (selectedCategory as WorkloadChoice) : 'application'
							)}
					>
						<Plus size={13} /> New service
					</Button>
				{/if}
			{/snippet}
		</PageHeader>

		<!-- Project Tabs -->
		<Tabs tabs={projectTabs} bind:active={activeTab} />

		{#if activeTab === 'services'}
			<!-- Services Resource Inventory -->
			<section class="flex flex-col gap-4">
			<!-- Header & Filter Row -->
			<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
				<!-- Category Filter Pills -->
				<div class="inline-flex items-center p-1 rounded-lg border border-[var(--border)] bg-[var(--bg-shell)] gap-1 overflow-x-auto max-w-full">
					<button
						type="button"
						onclick={() => (selectedCategory = 'all')}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {selectedCategory ===
						'all'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						<span>All</span>
						<span class="px-1.5 py-0.2 rounded text-[10px] bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--text-secondary)]">
							{services.length}
						</span>
					</button>

					<button
						type="button"
						onclick={() => (selectedCategory = 'application')}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {selectedCategory ===
						'application'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						<GitBranch size={13} class="opacity-70" />
						<span>Applications</span>
						{#if appServices.length > 0}
							<span class="px-1.5 py-0.2 rounded text-[10px] bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--text-secondary)]">
								{appServices.length}
							</span>
						{/if}
					</button>

					<button
						type="button"
						onclick={() => (selectedCategory = 'quadlet')}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {selectedCategory ===
						'quadlet'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						<FileText size={13} class="text-[var(--accent)]" />
						<span>Quadlet (Systemd)</span>
						{#if quadletServices.length > 0}
							<span class="px-1.5 py-0.2 rounded text-[10px] bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--accent)]">
								{quadletServices.length}
							</span>
						{/if}
					</button>

					<button
						type="button"
						onclick={() => (selectedCategory = 'compose')}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {selectedCategory ===
						'compose'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						<Stack size={13} class="opacity-70" />
						<span>Compose / Stacks</span>
						{#if composeServices.length > 0}
							<span class="px-1.5 py-0.2 rounded text-[10px] bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--text-secondary)]">
								{composeServices.length}
							</span>
						{/if}
					</button>

					<button
						type="button"
						onclick={() => (selectedCategory = 'database')}
						class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {selectedCategory ===
						'database'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
					>
						<Database size={13} class="opacity-70" />
						<span>Databases</span>
						{#if databaseServices.length > 0}
							<span class="px-1.5 py-0.2 rounded text-[10px] bg-[var(--bg-panel)] font-[var(--font-mono)] text-[var(--text-secondary)]">
								{databaseServices.length}
							</span>
						{/if}
					</button>
				</div>

				<!-- View Mode Toggle: Cards / Table / Topology -->
				{#if services.length > 0}
					<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] self-end sm:self-auto">
						<button
							type="button"
							onclick={() => (viewMode = 'cards')}
							class="flex items-center justify-center w-7 h-7 rounded-[calc(var(--radius-sm)-2px)] transition-colors cursor-pointer border-0 {viewMode ===
							'cards'
								? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
								: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
							title="Cards view"
							aria-label="Cards view"
						>
							<SquaresFour size={14} />
						</button>
						<button
							type="button"
							onclick={() => (viewMode = 'table')}
							class="flex items-center justify-center w-7 h-7 rounded-[calc(var(--radius-sm)-2px)] transition-colors cursor-pointer border-0 {viewMode ===
							'table'
								? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
								: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
							title="Table view"
							aria-label="Table view"
						>
							<List size={14} />
						</button>
						<button
							type="button"
							onclick={() => (viewMode = 'topology')}
							class="flex items-center justify-center w-7 h-7 rounded-[calc(var(--radius-sm)-2px)] transition-colors cursor-pointer border-0 {viewMode ===
							'topology'
								? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
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
				<div class="p-8 text-center border border-dashed border-[var(--border)] rounded-[var(--radius-card)] bg-[var(--bg-panel)] flex flex-col items-center justify-center gap-3">
					{#if selectedCategory === 'quadlet'}
						<div class="flex items-center justify-center w-10 h-10 rounded-full bg-[rgba(105,115,168,0.12)] text-[var(--accent)] mb-1">
							<FileText size={20} />
						</div>
						<div class="flex flex-col gap-1 max-w-md">
							<span class="text-sm font-semibold text-[var(--text-primary)]">No Quadlet services found</span>
							<p class="text-xs text-[var(--text-tertiary)] m-0 leading-relaxed">
								Quadlet enables declarative systemd container services (<code class="text-[var(--accent)]">.container</code>) that auto-start on host boot, restart cleanly, and integrate natively into systemd journal logging.
							</p>
						</div>
						<Button variant="primary" size="sm" onclick={() => openCreateWithCategory('quadlet')} class="mt-1">
							<Plus size={13} /> Deploy Quadlet Service
						</Button>
					{:else if selectedCategory === 'compose'}
						<div class="flex items-center justify-center w-10 h-10 rounded-full bg-[var(--bg-surface)] text-[var(--text-secondary)] mb-1">
							<Stack size={20} />
						</div>
						<div class="flex flex-col gap-1 max-w-md">
							<span class="text-sm font-semibold text-[var(--text-primary)]">No Compose stacks found</span>
							<p class="text-xs text-[var(--text-tertiary)] m-0 leading-relaxed">
								Deploy multi-container workloads managed seamlessly via Compose YAML or Kubernetes pod manifests.
							</p>
						</div>
						<Button variant="primary" size="sm" onclick={() => openCreateWithCategory('compose')} class="mt-1">
							<Plus size={13} /> Deploy Compose Stack
						</Button>
					{:else if selectedCategory === 'database'}
						<div class="flex items-center justify-center w-10 h-10 rounded-full bg-[var(--bg-surface)] text-[var(--text-secondary)] mb-1">
							<Database size={20} />
						</div>
						<div class="flex flex-col gap-1 max-w-md">
							<span class="text-sm font-semibold text-[var(--text-primary)]">No Databases deployed</span>
							<p class="text-xs text-[var(--text-tertiary)] m-0 leading-relaxed">
								Launch dedicated PostgreSQL, Redis, MySQL, or MongoDB engines with isolated persistent volumes in 1 click.
							</p>
						</div>
						<Button variant="primary" size="sm" onclick={() => openCreateWithCategory('database')} class="mt-1">
							<Plus size={13} /> Deploy Database
						</Button>
					{:else}
						<div class="flex flex-col gap-1 max-w-md">
							<span class="text-sm font-semibold text-[var(--text-primary)]">No services found</span>
							<p class="text-xs text-[var(--text-tertiary)] m-0 leading-relaxed">
								Deploy your first service to this project.
							</p>
						</div>
						<Button variant="secondary" size="sm" onclick={() => openCreateWithCategory('application')} class="mt-1">
							<Plus size={13} /> Create your first service
						</Button>
					{/if}
				</div>
			{:else if viewMode === 'cards'}
				<!-- Cards View (Default) -->
				<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3.5">
					{#each filteredServices as svc}
						{@const domainInfo = getServiceDomain(svc)}
						{@const sourceInfo = formatSource(svc)}
						{@const categoryLabel = getCategoryLabel(svc)}
						<Card
							interactive
							padding="md"
							onclick={() => goto(`/projects/${project.id}/services/${svc.id}`)}
							class="flex flex-col justify-between gap-4 h-full"
						>
							<div class="flex flex-col gap-2.5">
								<!-- Top Row: Name + Status on left, Category Chip on right -->
								<div class="flex items-start justify-between gap-2">
									<div class="flex items-center gap-2 min-w-0">
										<span class="text-sm font-medium text-[var(--text-primary)] group-hover:text-[var(--accent)] transition-colors truncate">
											{svc.name}
										</span>
										<StatusBadge status={svc.status} size="sm" />
									</div>
									<Chip size="sm" variant={svc.type === 'quadlet' || svc.type === 'kubernetes' ? 'mono' : 'default'}>
										{categoryLabel}
									</Chip>
								</div>

								<!-- Middle: Description or Source snippet -->
								{#if svc.description}
									<p class="text-xs text-[var(--text-secondary)] line-clamp-2 m-0 leading-relaxed">
										{svc.description}
									</p>
								{:else if sourceInfo && sourceInfo !== '—'}
									<p class="text-xs font-[var(--font-mono)] text-[var(--text-tertiary)] truncate m-0">
										{sourceInfo}
									</p>
								{/if}
							</div>

							<!-- Bottom Row: Domain / Port info on left, Container info / Arrow on right -->
							<div class="flex items-center justify-between pt-2.5 border-t border-[var(--border-subtle)] text-xs text-[var(--text-tertiary)]">
								{#if domainInfo}
									<div class="flex items-center gap-1 font-[var(--font-mono)] text-[11px] text-[var(--text-secondary)] group-hover:text-[var(--text-primary)] truncate">
										<ArrowUpRight size={12} class="text-[var(--text-tertiary)] shrink-0" />
										<span class="truncate">{domainInfo}</span>
									</div>
								{:else if sourceInfo && sourceInfo !== '—'}
									<span class="font-[var(--font-mono)] text-[11px] text-[var(--text-tertiary)] truncate">
										{sourceInfo}
									</span>
								{:else}
									<span class="text-[11px] text-[var(--text-tertiary)]">Internal service</span>
								{/if}

								<div class="flex items-center gap-1 shrink-0 text-[11px] text-[var(--text-tertiary)]">
									{#if svc.replicas && svc.replicas > 1}
										<span>{svc.replicas} replicas</span>
									{:else if svc.workloads && svc.workloads.length > 1}
										<span>{svc.workloads.length} containers</span>
									{/if}
									<ArrowRight size={12} class="opacity-0 group-hover:opacity-100 transition-opacity ml-0.5 text-[var(--text-tertiary)]" />
								</div>
							</div>
						</Card>
					{/each}
				</div>
			{:else}
				<!-- Table / List View -->
				<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-card)] bg-[var(--bg-panel)] overflow-hidden">
					{#each filteredServices as svc}
						{@const domainInfo = getServiceDomain(svc)}
						{@const sourceInfo = formatSource(svc)}
						{@const categoryLabel = getCategoryLabel(svc)}
						<button
							type="button"
							onclick={() => goto(`/projects/${project.id}/services/${svc.id}`)}
							class="group w-full flex items-center justify-between p-4 bg-transparent hover:bg-[var(--bg-hover)] transition-colors text-left cursor-pointer border-0"
						>
							<div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-6 min-w-0 flex-1">
								<!-- Name + Status -->
								<div class="flex items-center gap-2.5 min-w-[160px]">
									<span class="text-sm font-medium text-[var(--text-primary)] group-hover:text-[var(--accent)] transition-colors">
										{svc.name}
									</span>
									<StatusBadge status={svc.status} size="sm" />
								</div>

								<!-- Category Chip -->
								<div class="min-w-[110px]">
									<Chip size="sm" variant={svc.type === 'quadlet' || svc.type === 'kubernetes' ? 'mono' : 'default'}>
										{categoryLabel}
									</Chip>
								</div>

								<!-- Source / Image -->
								<div class="min-w-[160px] max-w-[260px] overflow-hidden text-ellipsis whitespace-nowrap">
									<span class="font-[var(--font-mono)] text-[11px] text-[var(--text-tertiary)]">
										{sourceInfo}
									</span>
								</div>

								<!-- Public Endpoint / Port -->
								{#if domainInfo}
									<div class="flex items-center gap-1 text-xs text-[var(--text-secondary)] font-[var(--font-mono)]">
										<ArrowUpRight size={13} class="text-[var(--text-tertiary)]" />
										<span>{domainInfo}</span>
									</div>
								{/if}
							</div>

							<span class="text-[var(--text-tertiary)] opacity-0 group-hover:opacity-100 transition-opacity pl-3">
								<ArrowRight size={14} />
							</span>
						</button>
					{/each}
				</div>
			{:else if viewMode === 'topology'}
				<!-- In-Page Project Topology Architecture Studio -->
				<div class="relative w-full h-[620px] rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-canvas)] overflow-hidden shadow-xs flex select-none">
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
					<div class="absolute top-3 left-3 z-10 flex items-center gap-2 pointer-events-auto">
						<div class="flex items-center gap-2 px-3 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]/90 backdrop-blur-md text-xs shadow-xs">
							<ShareNetwork size={14} class="text-[var(--accent)]" />
							<span class="font-semibold text-[var(--text-primary)]">{project.name} Architecture</span>
							<span class="text-[var(--text-tertiary)]">·</span>
							<span class="text-[var(--text-tertiary)] text-[11px]">{projectTopologyGraph.nodes.length} nodes · {projectTopologyGraph.edges.length} links</span>
						</div>
					</div>

					<!-- Floating Controls in Project Canvas (Top-Right) -->
					<div class="absolute top-3 right-3 z-10 flex items-center gap-2 pointer-events-auto">
						<a
							href="/topology?project={project.id}"
							class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]/90 backdrop-blur-md text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:border-[var(--accent)] shadow-xs transition-colors no-underline"
						>
							<CornersOut size={13} />
							<span>Fullscreen</span>
						</a>
					</div>

					<!-- Inspector Drawer if node is clicked -->
					{#if topoSelectedItem}
						<div class="absolute right-0 top-0 bottom-0 z-20 w-80 sm:w-88 shadow-2xl h-full flex flex-col pointer-events-auto">
							<TopologyInspector
								selected={topoSelectedItem}
								onclose={() => (topoSelectedItem = null)}
							/>
						</div>
					{/if}
				</div>
			{/if}
		</section>

		<!-- Recent Deployments (shown on services tab) -->
		{#if projectDeployments.length > 0}
			<section class="flex flex-col gap-3">
				<div class="flex items-center justify-between">
					<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">Recent deployments</h2>
					<span class="text-xs text-[var(--text-tertiary)] tabular-nums">{projectDeployments.length} total</span>
				</div>

				<DataTable
					columns={[
						{ key: 'serviceName', label: 'Service' },
						{ key: 'version', label: 'Version', mono: true },
						{ key: 'commit', label: 'Commit', mono: true },
						{ key: 'status', label: 'Status', render: depStatusSnippet },
						{ key: 'duration', label: 'Duration' },
						{ key: 'timeAgo', label: 'When' }
					]}
					rows={projectDeployments.slice(0, 5)}
					keyExtractor={(d) => d.id}
					onRowClick={(d) => goto(`/projects/${project.id}/services/${d.serviceId}`)}
				/>
			</section>
		{/if}
	{:else if activeTab === 'backups'}
		<ProjectBackupsView projectId={project.id} />
	{:else if activeTab === 'deployments'}
		<section class="flex flex-col gap-3">
			<div class="flex items-center justify-between">
				<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">All Project Deployments</h2>
				<span class="text-xs text-[var(--text-tertiary)] tabular-nums">{projectDeployments.length} total</span>
			</div>

			<DataTable
				columns={[
					{ key: 'serviceName', label: 'Service' },
					{ key: 'version', label: 'Version', mono: true },
					{ key: 'commit', label: 'Commit', mono: true },
					{ key: 'status', label: 'Status', render: depStatusSnippet },
					{ key: 'duration', label: 'Duration' },
					{ key: 'timeAgo', label: 'When' }
				]}
				rows={projectDeployments}
				keyExtractor={(d) => d.id}
				onRowClick={(d) => goto(`/projects/${project.id}/services/${d.serviceId}`)}
			/>
		</section>
	{/if}
	</div>

	<!-- Create Service Modal -->
	<CreateServiceModal
		projectId={project.id}
		bind:open={isCreateModalOpen}
		initialType={modalInitialType}
		onclose={() => (isCreateModalOpen = false)}
	/>
{:else}
	<div class="text-[var(--text-tertiary)] py-12 text-center text-xs">
		Project not found.
	</div>
{/if}

{#snippet depStatusSnippet(row: typeof projectDeployments[0])}
	<StatusBadge status={row.status} size="sm" />
{/snippet}
