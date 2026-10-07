<script lang="ts">
	import type { Service, Deployment } from '$lib/types';
	import { Button, Input, Chip } from '$lib/components/primitives';
	import { SearchInput } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
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
		return dataStore.getServiceDeployments(service.id);
	});

	let isDeploying = $state(false);

	async function loadDeployments() {
		try {
			const list = await api.services.deployments(service.id);
			if (Array.isArray(list)) {
				for (const item of list) {
					const existingIndex = dataStore.deployments.findIndex((d) => d.id === item.id);
					const mapped: Deployment = {
						id: item.id,
						projectId: item.projectId || service.projectId,
						projectName: item.projectId || service.projectId,
						serviceId: item.serviceId || service.id,
						serviceName: service.name,
						number: item.number || (dataStore.deployments.length + 1),
						version: item.version || (item.image ? item.image.split(':').pop() : 'latest'),
						commit: item.commitHash || '',
						commitMessage: item.commitMessage || (item.trigger === 'compose' ? 'Docker Compose rollout' : 'Deployment rollout'),
						branch: service.branch || 'main',
						status: item.status || 'running',
						duration: item.duration || '0s',
						timeAgo: item.startedAt ? 'Recently' : 'Just now',
						startedAt: item.startedAt || new Date().toISOString(),
						finishedAt: item.finishedAt || '',
						trigger: item.trigger || 'manual',
						author: 'operator'
					};
					if (existingIndex >= 0) {
						dataStore.deployments[existingIndex] = {
							...dataStore.deployments[existingIndex],
							...mapped
						};
					} else {
						dataStore.deployments.unshift(mapped);
					}
				}
			}
		} catch (err) {
			// ignore
		}
	}

	import { onMount } from 'svelte';
	onMount(() => {
		loadDeployments();
		const interval = setInterval(() => {
			const hasActive = rawDeployments.some((d) => d.status === 'building' || d.status === 'deploying');
			if (hasActive) {
				loadDeployments();
			}
		}, 2500);
		return () => clearInterval(interval);
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

	async function handleTriggerDeploy() {
		if (onRedeploy) {
			onRedeploy();
			return;
		}

		isDeploying = true;
		try {
			await dataStore.deployService(service.id);
			await loadDeployments();
		} catch (err) {
			console.error('Deploy error:', err);
		} finally {
			isDeploying = false;
		}
	}

	async function handleRollback(dep: Deployment) {
		isDeploying = true;
		try {
			await dataStore.deployService(service.id);
			await loadDeployments();
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
