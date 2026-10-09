<script lang="ts">
	import { PageHeader, SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import type { Deployment } from '$lib/types';
	import { DeploymentCard, DeploymentLogsModal } from '$lib/components/features/deployments';
	import { RocketLaunch } from 'phosphor-svelte';

	let selectedLogDep = $state<Deployment | null>(null);
	let isLogsModalOpen = $state(false);

	let statusFilter = $state<'all' | 'running' | 'building' | 'failed'>('all');
	let searchQuery = $state('');

	let allDeployments = $derived(dataStore.deployments);

	let filteredDeployments = $derived.by(() => {
		let list = allDeployments;

		if (statusFilter !== 'all') {
			if (statusFilter === 'running') {
				list = list.filter((d) => d.status === 'running' || d.status === 'healthy');
			} else if (statusFilter === 'building') {
				list = list.filter((d) => d.status === 'building' || d.status === 'deploying');
			} else if (statusFilter === 'failed') {
				list = list.filter((d) => d.status === 'failed');
			}
		}

		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			list = list.filter(
				(d) =>
					d.commitMessage.toLowerCase().includes(q) ||
					d.projectName.toLowerCase().includes(q) ||
					d.serviceName.toLowerCase().includes(q) ||
					d.commit?.toLowerCase().includes(q) ||
					d.version?.toLowerCase().includes(q) ||
					d.branch?.toLowerCase().includes(q)
			);
		}

		return list;
	});

	let counts = $derived.by(() => {
		const total = allDeployments.length;
		const running = allDeployments.filter(
			(d) => d.status === 'running' || d.status === 'healthy'
		).length;
		const building = allDeployments.filter(
			(d) => d.status === 'building' || d.status === 'deploying'
		).length;
		const failed = allDeployments.filter((d) => d.status === 'failed').length;
		return { total, running, building, failed };
	});

	function openLogs(dep: Deployment) {
		selectedLogDep = dep;
		isLogsModalOpen = true;
	}

	async function handleRollback(dep: Deployment) {
		try {
			const targetCommit = dep.commit || dep.commitHash;
			await dataStore.deployService(dep.serviceId, 'rollback', targetCommit, dep.image);
		} catch (err) {
			console.error('Rollback deploy error:', err);
		}
	}

	function handleCancel(dep: Deployment) {
		dataStore.cancelDeployment(dep.id);
	}

	function handleDelete(dep: Deployment) {
		dataStore.deleteDeployment(dep.id);
	}
</script>

<svelte:head>
	<title>Deployments — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="Deployments"
		subtitle="Comprehensive deployment history, audit traces, and container rollout status across all projects."
	/>

	<!-- Toolbar & Filters -->
	<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
		<!-- Status Filters -->
		<div
			class="flex items-center gap-1 overflow-x-auto rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5 text-xs"
		>
			<button
				type="button"
				onclick={() => (statusFilter = 'all')}
				class="cursor-pointer rounded border-0 px-2.5 py-1 transition-colors {statusFilter === 'all'
					? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				All ({counts.total})
			</button>

			<button
				type="button"
				onclick={() => (statusFilter = 'running')}
				class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 transition-colors {statusFilter ===
				'running'
					? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span class="h-1.5 w-1.5 rounded-full bg-[var(--status-green)]"></span>
				Success ({counts.running})
			</button>

			{#if counts.building > 0}
				<button
					type="button"
					onclick={() => (statusFilter = 'building')}
					class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 transition-colors {statusFilter ===
					'building'
						? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					<span class="h-1.5 w-1.5 animate-pulse rounded-full bg-[var(--accent)]"></span>
					Building ({counts.building})
				</button>
			{/if}

			{#if counts.failed > 0}
				<button
					type="button"
					onclick={() => (statusFilter = 'failed')}
					class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 transition-colors {statusFilter ===
					'failed'
						? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					<span class="h-1.5 w-1.5 rounded-full bg-[var(--status-red)]"></span>
					Failed ({counts.failed})
				</button>
			{/if}
		</div>

		<!-- Search Input -->
		<SearchInput
			bind:value={searchQuery}
			placeholder="Search by project, service, commit..."
			class="w-full max-w-[320px]"
		/>
	</div>

	<!-- Long Component List ("Komponen Panjang") -->
	{#if filteredDeployments.length === 0}
		<div
			class="flex w-full flex-col items-center justify-center gap-2.5 rounded-[var(--radius-card)] border border-dashed border-[var(--border)] bg-[var(--bg-panel)] py-16 text-center"
		>
			<div class="rounded-full bg-[var(--bg-surface)] p-2.5 text-[var(--text-tertiary)]">
				<RocketLaunch size={20} />
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">No deployments found</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					{searchQuery ? 'Try clearing your search query' : 'No deployments have occurred yet.'}
				</span>
			</div>
			{#if searchQuery}
				<Button variant="secondary" size="sm" onclick={() => (searchQuery = '')} class="mt-1">
					Clear Filter
				</Button>
			{/if}
		</div>
	{:else}
		<div class="flex flex-col gap-2.5">
			{#each filteredDeployments as dep, i (dep.id)}
				<div class="flex flex-col gap-1">
					<!-- Project & Service breadcrumb badge for global view -->
					<div class="flex items-center gap-1.5 px-1 text-[11px] text-[var(--text-tertiary)]">
						<span class="font-medium text-[var(--text-secondary)]">{dep.projectName}</span>
						<span>/</span>
						<span class="font-mono text-[var(--accent)]">{dep.serviceName}</span>
					</div>

					<DeploymentCard
						deployment={dep}
						isCurrent={i === 0 && (dep.status === 'running' || dep.status === 'healthy')}
						onViewLogs={openLogs}
						onRedeploy={handleRollback}
						onCancel={handleCancel}
						onDelete={handleDelete}
					/>
				</div>
			{/each}
		</div>
	{/if}
</div>

<!-- Deployment Logs Modal -->
<DeploymentLogsModal
	deployment={selectedLogDep}
	open={isLogsModalOpen}
	onclose={() => {
		isLogsModalOpen = false;
		selectedLogDep = null;
	}}
/>
