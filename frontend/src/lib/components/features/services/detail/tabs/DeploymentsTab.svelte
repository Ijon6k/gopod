<script lang="ts">
	import type { Service, Deployment } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { SearchInput, Tabs } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { Empty } from 'kumo-svelte';
	import {
		DeploymentCard,
		DeploymentLogsModal,
		WebhookDeployModal
	} from '$lib/components/features/deployments';
	import { ArrowClockwise, Trash, Globe, RocketLaunch } from 'phosphor-svelte';

	interface Props {
		service: Service;
		onRedeploy?: () => void;
	}

	let { service, onRedeploy }: Props = $props();

	// Modal states
	let selectedLogDep = $state<Deployment | null>(null);
	let isLogsModalOpen = $state(false);
	let isWebhookModalOpen = $state(false);

	// Filter states
	let statusFilter = $state<string>('all');
	let searchQuery = $state('');

	// Deployments from Svelte 5 reactive dataStore
	let rawDeployments = $derived.by(() => {
		return dataStore.getServiceDeployments(service.id);
	});

	$effect(() => {
		if (service?.id) {
			dataStore.fetchServiceDeployments(service.id);
		}
	});

	let isDeploying = $state(false);

	// Filtered deployments
	let filteredDeployments = $derived.by(() => {
		let list = rawDeployments;

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
					d.commit?.toLowerCase().includes(q) ||
					d.version?.toLowerCase().includes(q) ||
					d.branch?.toLowerCase().includes(q)
			);
		}

		return list;
	});

	// Counts
	let counts = $derived.by(() => {
		const total = rawDeployments.length;
		const running = rawDeployments.filter(
			(d) => d.status === 'running' || d.status === 'healthy'
		).length;
		const building = rawDeployments.filter(
			(d) => d.status === 'building' || d.status === 'deploying'
		).length;
		const failed = rawDeployments.filter((d) => d.status === 'failed').length;
		return { total, running, building, failed };
	});

	async function handleTriggerDeploy() {
		if (onRedeploy) {
			onRedeploy();
			return;
		}

		isDeploying = true;
		try {
			const dep = await dataStore.deployService(service.id);
			if (dep) {
				selectedLogDep = dep;
				isLogsModalOpen = true;
			}
		} catch (err) {
			console.error('Deploy error:', err);
		} finally {
			isDeploying = false;
		}
	}

	async function handleRollback(dep: Deployment) {
		isDeploying = true;
		try {
			const targetCommit = dep.commit || dep.commitHash;
			const newDep = await dataStore.deployService(service.id, 'rollback', targetCommit, dep.image, {
				targetDeploymentId: dep.id
			});
			if (newDep) {
				selectedLogDep = newDep;
				isLogsModalOpen = true;
			}
		} catch (err) {
			console.error('Rollback error:', err);
		} finally {
			isDeploying = false;
		}
	}

	function handleCancel(dep: Deployment) {
		dataStore.cancelDeployment(dep.id);
	}

	function handleDelete(dep: Deployment) {
		dataStore.deleteDeployment(dep.id);
	}

	function handleClearHistory() {
		if (
			confirm(
				'Clear old deployments from history? (The currently active production deployment will be preserved)'
			)
		) {
			dataStore.clearServiceDeployments(service.id, true);
		}
	}

	function openLogs(dep: Deployment) {
		selectedLogDep = dep;
		isLogsModalOpen = true;
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- ══════════════════════════════════════════════════════════════
	     1. HEADER & ACTIONS
	     ══════════════════════════════════════════════════════════════ -->
	<div
		class="flex flex-col justify-between gap-4 border-b border-kumo-hairline pb-4 sm:flex-row sm:items-center"
	>
		<div class="flex flex-col gap-1">
			<div class="flex items-center gap-2">
				<h2 class="m-0 text-base font-semibold text-kumo-default">Deployment history</h2>
				<span class="font-mono text-xs text-kumo-subtle tabular-nums">
					({counts.total})
				</span>
			</div>
			<p class="m-0 text-sm text-kumo-subtle">
				Rolling container updates, systemd Quadlet daemon reloads, and audit logs.
			</p>
		</div>

		<!-- Action Buttons -->
		<div class="flex flex-wrap items-center gap-2">
			<!-- Webhook CI/CD Modal Trigger -->
			<Button
				variant="secondary"
				size="sm"
				onclick={() => (isWebhookModalOpen = true)}
				class="gap-1.5"
			>
				<Globe size={13} />
				<span>Webhook</span>
			</Button>

			<!-- Prune / Clear History -->
			{#if rawDeployments.length > 1}
				<Button
					variant="secondary"
					size="sm"
					onclick={handleClearHistory}
					class="gap-1.5 text-kumo-danger hover:text-kumo-danger"
					title="Clear finished deployment history (preserves active live deployment)"
				>
					<Trash size={13} />
					<span>Prune history</span>
				</Button>
			{/if}

			<!-- Trigger Deploy Now -->
			<Button
				variant="primary"
				size="sm"
				onclick={handleTriggerDeploy}
				loading={isDeploying}
				class="gap-1.5"
			>
				<ArrowClockwise size={13} />
				<span>Deploy now</span>
			</Button>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     2. FILTERS & SEARCH TOOLBAR (Kumo Segmented Tabs)
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
		<!-- Segmented Status Tabs -->
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
			placeholder="Search commit, message, version…"
			class="w-full max-w-[280px]"
		/>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     3. DEPLOYMENT LIST
	     ══════════════════════════════════════════════════════════════ -->
	{#if filteredDeployments.length === 0}
		<div class="py-4">
			<Empty
				size="sm"
				title={searchQuery ? 'No matching deployments' : 'No deployments yet'}
				description={searchQuery
					? 'Try clearing your search query to see previous deployments.'
					: 'Trigger your first deployment to build and reload this service.'}
			>
				{#snippet icon()}
					<div class="rounded-full bg-kumo-recessed p-2.5 text-kumo-subtle">
						<RocketLaunch size={22} />
					</div>
				{/snippet}
				<div class="mt-2">
					{#if searchQuery}
						<Button variant="secondary" size="sm" onclick={() => (searchQuery = '')}>
							Clear filter
						</Button>
					{:else}
						<Button variant="primary" size="sm" onclick={handleTriggerDeploy}>Deploy now</Button>
					{/if}
				</div>
			</Empty>
		</div>
	{:else}
		<div class="flex flex-col gap-2">
			{#each filteredDeployments as dep, i (dep.id)}
				{@const isCurrent =
					dep.isCurrent ?? (i === 0 && (dep.status === 'running' || dep.status === 'healthy'))}
				<DeploymentCard
					deployment={dep}
					{isCurrent}
					onViewLogs={openLogs}
					onRedeploy={handleRollback}
					onCancel={handleCancel}
					onDelete={handleDelete}
				/>
			{/each}
		</div>
	{/if}
</div>

<!-- ══════════════════════════════════════════════════════════════
     MODALS
     ══════════════════════════════════════════════════════════════ -->
<!-- Deployment Logs Modal with Pipeline Stages -->
<DeploymentLogsModal
	deployment={selectedLogDep
		? (rawDeployments.find((d) => d.id === selectedLogDep?.id) ?? selectedLogDep)
		: null}
	open={isLogsModalOpen}
	onclose={() => {
		isLogsModalOpen = false;
		selectedLogDep = null;
	}}
/>

<!-- Webhook CI/CD Modal -->
<WebhookDeployModal
	{service}
	open={isWebhookModalOpen}
	onclose={() => (isWebhookModalOpen = false)}
/>
