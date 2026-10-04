<script lang="ts">
	import type { Service, Deployment } from '$lib/types';
	import { Button, Input, Chip } from '$lib/components/primitives';
	import { SearchInput } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { DeploymentCard, DeploymentLogsModal, WebhookDeployModal } from '$lib/components/features/deployments';
	import {
		ArrowClockwise,
		Trash,
		Globe,
		MagnifyingGlass,
		X,
		Funnel,
		RocketLaunch
	} from 'phosphor-svelte';

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
	let statusFilter = $state<'all' | 'running' | 'building' | 'failed'>('all');
	let searchQuery = $state('');

	// Deployments from store
	let rawDeployments = $derived.by(() => {
		const list = dataStore.getServiceDeployments(service.id);
		if (list.length > 0) return list;
		// Fallback mock history if empty
		return [
			{
				id: `dep-${service.id}-1`,
				projectId: service.projectId,
				projectName: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				number: 24,
				version: service.image?.split(':').pop() ?? 'v1.8.0',
				commit: '4b89c02',
				commitMessage: 'feat: add thread reactions and emoji picker',
				branch: service.branch ?? 'main',
				status: 'running',
				duration: '1m 42s',
				timeAgo: '8 min ago',
				startedAt: new Date(Date.now() - 8 * 60 * 1000).toISOString(),
				finishedAt: new Date().toISOString(),
				trigger: 'git-push',
				author: 'pixy',
				isCurrent: true
			},
			{
				id: `dep-${service.id}-2`,
				projectId: service.projectId,
				projectName: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				number: 23,
				version: 'v1.7.9',
				commit: 'f92c10b',
				commitMessage: 'perf: optimize cache eviction and connection pool',
				branch: service.branch ?? 'main',
				status: 'running',
				duration: '48s',
				timeAgo: '1 d ago',
				startedAt: new Date(Date.now() - 24 * 3600 * 1000).toISOString(),
				finishedAt: new Date(Date.now() - 24 * 3600 * 1000 + 48000).toISOString(),
				trigger: 'git-push',
				author: 'pixy'
			},
			{
				id: `dep-${service.id}-3`,
				projectId: service.projectId,
				projectName: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				number: 22,
				version: 'v1.7.8',
				commit: '8b31ea4',
				commitMessage: 'fix: resolve connection timeout during quadlet reload',
				branch: service.branch ?? 'main',
				status: 'failed',
				duration: '45s',
				timeAgo: '2 d ago',
				startedAt: new Date(Date.now() - 48 * 3600 * 1000).toISOString(),
				finishedAt: new Date(Date.now() - 48 * 3600 * 1000 + 45000).toISOString(),
				trigger: 'manual',
				author: 'pixy'
			}
		] as Deployment[];
	});

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
		const running = rawDeployments.filter((d) => d.status === 'running' || d.status === 'healthy').length;
		const building = rawDeployments.filter((d) => d.status === 'building' || d.status === 'deploying').length;
		const failed = rawDeployments.filter((d) => d.status === 'failed').length;
		return { total, running, building, failed };
	});

	function handleTriggerDeploy() {
		if (onRedeploy) {
			onRedeploy();
			return;
		}

		// Built-in deployment trigger
		const newNumber = (rawDeployments[0]?.number ?? 0) + 1;
		const newDep: Deployment = {
			id: `dep-${service.id}-${Date.now()}`,
			projectId: service.projectId,
			projectName: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			number: newNumber,
			version: service.image?.split(':').pop() ?? `v1.${newNumber}.0`,
			commit: Math.random().toString(16).substring(2, 9),
			commitMessage: 'Triggered manual rollout via dashboard',
			branch: service.branch ?? 'main',
			status: 'deploying',
			duration: 'Building…',
			timeAgo: 'Just now',
			startedAt: new Date().toISOString(),
			finishedAt: '',
			trigger: 'manual',
			author: 'operator'
		};

		dataStore.deployments.unshift(newDep);

		setTimeout(() => {
			newDep.status = 'running';
			newDep.duration = '32s';
			newDep.finishedAt = new Date().toISOString();
		}, 3000);
	}

	function handleRollback(dep: Deployment) {
		const newNumber = (rawDeployments[0]?.number ?? 0) + 1;
		const rollbackDep: Deployment = {
			id: `dep-${service.id}-${Date.now()}`,
			projectId: service.projectId,
			projectName: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			number: newNumber,
			version: dep.version,
			commit: dep.commit,
			commitMessage: `Rollback to #${dep.number} (${dep.commit.substring(0, 7)})`,
			branch: dep.branch,
			status: 'deploying',
			duration: 'Rolling back…',
			timeAgo: 'Just now',
			startedAt: new Date().toISOString(),
			finishedAt: '',
			trigger: 'rollback',
			author: 'operator'
		};

		dataStore.deployments.unshift(rollbackDep);

		setTimeout(() => {
			rollbackDep.status = 'running';
			rollbackDep.duration = '24s';
			rollbackDep.finishedAt = new Date().toISOString();
		}, 2500);
	}

	function handleCancel(dep: Deployment) {
		dataStore.cancelDeployment(dep.id);
	}

	function handleDelete(dep: Deployment) {
		dataStore.deleteDeployment(dep.id);
	}

	function handleClearHistory() {
		if (confirm('Clear old deployments from history? (The currently active production deployment will be preserved)')) {
			dataStore.clearServiceDeployments(service.id, true);
		}
	}

	function openLogs(dep: Deployment) {
		selectedLogDep = dep;
		isLogsModalOpen = true;
	}
</script>

<div class="w-full flex flex-col gap-5">
	<!-- ══════════════════════════════════════════════════════════════
	     1. HEADER & GLOBAL ACTIONS (Dokploy / Coolify Inspired)
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-[var(--border)]">
		<div class="flex flex-col gap-0.5">
			<div class="flex items-center gap-2">
				<h2 class="text-sm font-medium text-[var(--text-primary)] m-0">Deployment History</h2>
				<span class="text-xs text-[var(--text-tertiary)] tabular-nums font-mono">
					({counts.total})
				</span>
			</div>
			<p class="text-xs text-[var(--text-tertiary)] m-0">
				Rolling container updates, systemd Quadlet daemon reloads, and audit logs.
			</p>
		</div>

		<!-- Action Buttons -->
		<div class="flex items-center flex-wrap gap-2">
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
					class="gap-1.5 text-[var(--text-secondary)] hover:text-[var(--status-red)]"
					title="Clear finished deployment history (preserves active live deployment)"
				>
					<Trash size={13} />
					<span>Prune History</span>
				</Button>
			{/if}

			<!-- Trigger Deploy Now -->
			<Button
				variant="primary"
				size="sm"
				onclick={handleTriggerDeploy}
				class="gap-1.5"
			>
				<ArrowClockwise size={13} />
				<span>Deploy Now</span>
			</Button>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     2. FILTERS & SEARCH TOOLBAR
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
		<!-- Status Filter Pills -->
		<div class="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-x-auto text-xs">
			<button
				type="button"
				onclick={() => (statusFilter = 'all')}
				class="px-2.5 py-1 rounded transition-colors border-0 cursor-pointer {statusFilter === 'all'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				All ({counts.total})
			</button>

			<button
				type="button"
				onclick={() => (statusFilter = 'running')}
				class="px-2.5 py-1 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {statusFilter === 'running'
					? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-green)]"></span>
				Success ({counts.running})
			</button>

			{#if counts.building > 0}
				<button
					type="button"
					onclick={() => (statusFilter = 'building')}
					class="px-2.5 py-1 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {statusFilter === 'building'
						? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					<span class="w-1.5 h-1.5 rounded-full bg-[var(--accent)] animate-pulse"></span>
					Building ({counts.building})
				</button>
			{/if}

			{#if counts.failed > 0}
				<button
					type="button"
					onclick={() => (statusFilter = 'failed')}
					class="px-2.5 py-1 rounded transition-colors border-0 cursor-pointer flex items-center gap-1.5 {statusFilter === 'failed'
						? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					<span class="w-1.5 h-1.5 rounded-full bg-[var(--status-red)]"></span>
					Failed ({counts.failed})
				</button>
			{/if}
		</div>

		<!-- Search Input -->
		<SearchInput
			bind:value={searchQuery}
			placeholder="Search commit, message, version..."
			class="max-w-[280px] w-full"
		/>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     3. DEPLOYMENT LIST (Komponen Panjang / Horizontal Cards)
	     ══════════════════════════════════════════════════════════════ -->
	{#if filteredDeployments.length === 0}
		<div class="w-full py-14 flex flex-col items-center justify-center gap-2.5 rounded-[var(--radius-card)] border border-dashed border-[var(--border)] bg-[var(--bg-panel)] text-center">
			<div class="p-2.5 rounded-full bg-[var(--bg-surface)] text-[var(--text-tertiary)]">
				<RocketLaunch size={20} />
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">No deployments match your criteria</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					{searchQuery ? 'Try clearing your search query' : 'Trigger your first deployment to build and reload this service'}
				</span>
			</div>
			{#if searchQuery}
				<Button variant="secondary" size="sm" onclick={() => (searchQuery = '')} class="mt-1">
					Clear Filter
				</Button>
			{:else}
				<Button variant="primary" size="sm" onclick={handleTriggerDeploy} class="mt-1">
					Deploy Now
				</Button>
			{/if}
		</div>
	{:else}
		<div class="flex flex-col gap-2.5">
			{#each filteredDeployments as dep, i (dep.id)}
				{@const isCurrent = dep.isCurrent ?? (i === 0 && (dep.status === 'running' || dep.status === 'healthy'))}
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
	deployment={selectedLogDep}
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
