<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, DataTable } from '$lib/components/ui';
	import { Button, Card, Chip } from '$lib/components/primitives';
	import { getProjectById, getProjectServices, getProjectDomains, getProjectDeployments } from '$lib/data';
	import CreateServiceModal from '$lib/components/features/services/create/CreateServiceModal.svelte';
	import { Plus, ArrowRight, ArrowUpRight, SquaresFour, List } from 'phosphor-svelte';
	import type { Service } from '$lib/types';

	let projectId = $derived(page.params.projectId ?? '');
	let project = $derived(projectId ? getProjectById(projectId) : undefined);
	let services = $derived(projectId ? getProjectServices(projectId) : []);
	let projectDomains = $derived(projectId ? getProjectDomains(projectId) : []);
	let projectDeployments = $derived(projectId ? getProjectDeployments(projectId) : []);

	let isCreateModalOpen = $state(false);
	let viewMode = $state<'cards' | 'table'>('cards');

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
				return 'Quadlet';
			case 'image':
				return 'Container';
			default:
				return 'Service';
		}
	}

	function formatSource(s: Service): string {
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
				<!-- ONLY ONE New Service button in the project header -->
				<Button variant="primary" size="sm" onclick={() => (isCreateModalOpen = true)}>
					<Plus size={13} /> New service
				</Button>
			{/snippet}
		</PageHeader>

		<!-- Services Resource Inventory -->
		<section class="flex flex-col gap-3.5">
			<div class="flex items-center justify-between">
				<div class="flex items-center gap-2.5">
					<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">Services</h2>
					<span class="text-xs text-[var(--text-tertiary)] tabular-nums">{services.length} total</span>
				</div>

				{#if services.length > 0}
					<!-- View Mode Toggle: Cards / Table -->
					<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)]">
						<button
							type="button"
							onclick={() => (viewMode = 'cards')}
							class="flex items-center justify-center w-7 h-7 rounded-[calc(var(--radius-sm)-2px)] transition-colors cursor-pointer border-0 {viewMode === 'cards'
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
							class="flex items-center justify-center w-7 h-7 rounded-[calc(var(--radius-sm)-2px)] transition-colors cursor-pointer border-0 {viewMode === 'table'
								? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
								: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
							title="Table view"
							aria-label="Table view"
						>
							<List size={14} />
						</button>
					</div>
				{/if}
			</div>

			{#if services.length === 0}
				<div class="p-8 text-center text-xs text-[var(--text-tertiary)] border border-dashed border-[var(--border)] rounded-[var(--radius-card)] bg-[var(--bg-panel)] flex flex-col items-center gap-3">
					<span>No services deployed in this project yet.</span>
					<Button variant="secondary" size="sm" onclick={() => (isCreateModalOpen = true)}>
						<Plus size={13} /> Create your first service
					</Button>
				</div>
			{:else if viewMode === 'cards'}
				<!-- Cards View (Default) -->
				<div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3.5">
					{#each services as svc}
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
					{#each services as svc}
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
			{/if}
		</section>

		<!-- Recent Deployments -->
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
