<script lang="ts">
	import { PageHeader, SearchInput, Tabs } from '$lib/components/ui';
	import { Button } from '$lib/components/ui';
	import { Empty } from 'kumo-svelte';
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
				list = list.filter(
					(d) => d.status === 'running' || d.status === 'healthy' || d.status === 'success'
				);
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
					(d.commitMessage || '').toLowerCase().includes(q) ||
					(d.projectName || '').toLowerCase().includes(q) ||
					(d.serviceName || '').toLowerCase().includes(q) ||
					(d.commit || '').toLowerCase().includes(q) ||
					(d.version || '').toLowerCase().includes(q) ||
					(d.branch || '').toLowerCase().includes(q)
			);
		}

		return list;
	});

	let counts = $derived.by(() => {
		const total = allDeployments.length;
		const running = allDeployments.filter(
			(d) => d.status === 'running' || d.status === 'healthy' || d.status === 'success'
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
		<!-- Status Filters (Kumo Segmented Tabs) -->
		<Tabs
			variant="segmented"
			size="sm"
			tabs={[
				{ id: 'all', label: `All (${counts.total})` },
				{ id: 'running', label: `Success (${counts.running})` },
				...(counts.building > 0
					? [{ id: 'building', label: `Building (${counts.building})` }]
					: []),
				...(counts.failed > 0 ? [{ id: 'failed', label: `Failed (${counts.failed})` }] : [])
			]}
			bind:active={statusFilter}
		/>

		<!-- Search Input -->
		<SearchInput
			bind:value={searchQuery}
			placeholder="Search by project, service, commit..."
			class="w-full max-w-[320px]"
		/>
	</div>

	<!-- Deployment List -->
	{#if filteredDeployments.length === 0}
		<div class="py-6">
			<Empty
				size="base"
				title={searchQuery ? 'No matching deployments' : 'No deployments found'}
				description={searchQuery
					? 'Try clearing your search query to see past deployments.'
					: 'No deployments have occurred across your projects yet.'}
			>
				{#snippet icon()}
					<div class="rounded-full bg-kumo-recessed p-2.5 text-kumo-subtle">
						<RocketLaunch size={24} />
					</div>
				{/snippet}
				{#if searchQuery}
					<div class="mt-2">
						<Button variant="secondary" size="sm" onclick={() => (searchQuery = '')}>
							Clear filter
						</Button>
					</div>
				{/if}
			</Empty>
		</div>
	{:else}
		<div class="flex flex-col gap-2.5">
			{#each filteredDeployments as dep, i (dep.serviceId ? `${dep.serviceId}-${dep.id}` : dep.id || i)}
				<DeploymentCard
					deployment={dep}
					showProjectContext={true}
					isCurrent={i === 0 &&
						(dep.status === 'running' || dep.status === 'healthy' || dep.status === 'success')}
					onViewLogs={openLogs}
					onRedeploy={handleRollback}
					onCancel={handleCancel}
					onDelete={handleDelete}
				/>
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
