<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, Tabs, ConfirmDialog } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import {
		getProjectById,
		getProjectServices,
		getProjectDomains,
		getProjectDeployments,
		dataStore
	} from '$lib/data';
	import CreateServiceModal from '$lib/components/features/services/create/CreateServiceModal.svelte';
	import {
		ProjectBackupsView,
		ProjectServicesTab,
		ProjectDeploymentsTab
	} from '$lib/components/features/projects';
	import type { WorkloadChoice } from '$lib/components/features/services/create/WorkloadTypeSelector.svelte';
	import { Plus, SpinnerGap, Trash, WarningCircle } from 'phosphor-svelte';
	import type { Project } from '$lib/types';
	import { untrack } from 'svelte';
	import { api } from '$lib/api';

	let projectId = $derived(page.params.projectId ?? '');
	let directProject = $state<Project | null>(null);
	let isProjectLoading = $state(true);
	let lastResolvedProjectId = '';

	let project = $derived(directProject ?? (projectId ? getProjectById(projectId) : undefined));
	let services = $derived(projectId ? getProjectServices(projectId) : []);
	let projectDomains = $derived(projectId ? getProjectDomains(projectId) : []);
	let projectDeployments = $derived(projectId ? getProjectDeployments(projectId) : []);

	$effect(() => {
		const pId = projectId;
		if (!pId) return;

		let active = true;

		untrack(() => {
			if (lastResolvedProjectId === pId) return;
			lastResolvedProjectId = pId;

			async function resolveProject() {
				const existing = getProjectById(pId);
				if (existing) {
					directProject = existing;
					isProjectLoading = false;
					return;
				}

				isProjectLoading = true;
				try {
					const p = await api.projects.get(pId);
					if (!active) return;
					if (p) {
						const fullProj: Project = {
							id: p.id,
							name: p.name,
							description: p.description || '',
							status: 'healthy',
							services: [],
							domains: [],
							cpu: 0,
							memory: 0,
							memoryTotal: 0,
							createdAt: p.createdAt || new Date().toISOString()
						};
						directProject = fullProj;
						const idx = dataStore.projects.findIndex((item) => item.id === fullProj.id);
						if (idx >= 0) {
							dataStore.projects[idx] = fullProj;
						} else {
							dataStore.projects.push(fullProj);
						}
					}
				} catch {
					// Silently handle
				} finally {
					if (active) isProjectLoading = false;
				}
			}

			resolveProject();
		});

		return () => {
			active = false;
		};
	});

	let activeTab = $state<'services' | 'backups' | 'deployments'>('services');
	let projectTabs = $derived([
		{ id: 'services', label: `Services (${services.length})` },
		{ id: 'backups', label: 'Volumes & Backups' },
		{ id: 'deployments', label: `Deployments (${projectDeployments.length})` }
	]);

	let isCreateModalOpen = $state(false);
	let isDeleteProjectOpen = $state(false);
	let modalInitialType = $state<WorkloadChoice>('application');

	function handleCreateService(type: WorkloadChoice = 'application') {
		modalInitialType = type;
		isCreateModalOpen = true;
	}
</script>

<svelte:head>
	<title>{project?.name ?? 'Project'} — GOPOD</title>
</svelte:head>

{#if isProjectLoading && !project}
	<div class="flex flex-col items-center justify-center gap-3 py-24 text-[var(--text-tertiary)]">
		<SpinnerGap size={24} class="animate-spin text-[var(--accent)]" />
		<span class="text-xs">Loading project…</span>
	</div>
{:else if project}
	<div class="flex w-full flex-col gap-8">
		<!-- Project Identity Header -->
		<PageHeader title={project.name} subtitle={project.description}>
			{#snippet meta()}
				<div class="flex items-center gap-3">
					<StatusBadge status={project.status} size="sm" />
					<span class="text-xs text-[var(--text-tertiary)]">
						{services.length}
						{services.length === 1 ? 'service' : 'services'} · {projectDomains.length}
						{projectDomains.length === 1 ? 'domain' : 'domains'}
					</span>
				</div>
			{/snippet}
			{#snippet actions()}
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (isDeleteProjectOpen = true)}
						class="cursor-pointer rounded-md border border-[var(--border)] bg-[var(--bg-surface)] p-2 text-[var(--text-tertiary)] transition-colors hover:border-red-500/30 hover:bg-red-500/10 hover:text-red-400"
						title="Delete project"
						aria-label="Delete project"
					>
						<Trash size={15} />
					</button>
					{#if activeTab === 'services'}
						<Button variant="primary" size="sm" onclick={() => handleCreateService('application')}>
							<Plus size={13} /> New service
						</Button>
					{/if}
				</div>
			{/snippet}
		</PageHeader>

		<!-- Clean, Modular Confirm Dialog for Project Deletion -->
		<ConfirmDialog
			open={isDeleteProjectOpen}
			title={`Delete Project "${project.name}"?`}
			description={services.length === 0
				? 'This action cannot be undone. This will permanently delete the project record and its configurations.'
				: undefined}
			confirmText="Delete Project"
			confirmDisabled={services.length > 0}
			oncancel={() => (isDeleteProjectOpen = false)}
			onconfirm={async () => {
				await dataStore.deleteProject(project.id);
				isDeleteProjectOpen = false;
				goto('/projects');
			}}
		>
			{#if services.length > 0}
				<div
					class="flex items-start gap-3 rounded-md border border-amber-500/20 bg-amber-500/10 p-3 text-xs leading-relaxed text-amber-400"
				>
					<WarningCircle size={18} class="mt-0.5 shrink-0" />
					<div>
						<strong class="font-semibold">Active services detected ({services.length})</strong>
						<p class="m-0 mt-1 text-[var(--text-secondary)]">
							Dokploy safety policy: You have active services in this project. Please terminate and
							delete all services first before deleting the project.
						</p>
					</div>
				</div>
			{/if}
		</ConfirmDialog>

		<!-- Project Tabs -->
		<Tabs tabs={projectTabs} bind:active={activeTab} />

		{#if activeTab === 'services'}
			<!-- Services Inventory -->
			<ProjectServicesTab
				{project}
				{services}
				{projectDomains}
				oncreateService={handleCreateService}
			/>

			<!-- Recent Deployments Preview -->
			{#if projectDeployments.length > 0}
				<ProjectDeploymentsTab
					{project}
					deployments={projectDeployments}
					title="Recent deployments"
					limit={5}
				/>
			{/if}
		{:else if activeTab === 'backups'}
			<ProjectBackupsView projectId={project.id} />
		{:else if activeTab === 'deployments'}
			<ProjectDeploymentsTab
				{project}
				deployments={projectDeployments}
				title="All Project Deployments"
			/>
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
	<div class="py-12 text-center text-xs text-[var(--text-tertiary)]">Project not found.</div>
{/if}
