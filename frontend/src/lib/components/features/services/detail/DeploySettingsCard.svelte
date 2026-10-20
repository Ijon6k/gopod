<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { LayerCard, Badge, useKumoToastManager } from 'kumo-svelte';
	import { Sparkle, CheckCircle, Info } from 'phosphor-svelte';
	import DeployActionsBar from './deploy/DeployActionsBar.svelte';
	import DeployPodStrategyBanner from './deploy/DeployPodStrategyBanner.svelte';

	interface Props {
		service: Service;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, onTerminalClick, onRedeploy }: Props = $props();

	const toast = useKumoToastManager();

	let autoDeploy = $state(true);
	let inPod = $state(false);

	$effect(() => {
		autoDeploy = service.autoDeploy ?? true;
		inPod = service.inPod || service.runtimeTarget === 'pod';
	});

	let isDeploying = $state(false);
	let isRebuilding = $state(false);
	let isFreshingVolumes = $state(false);
	let isStopping = $state(false);
	let isStarting = $state(false);
	let isReloadingDaemon = $state(false);
	let isCancelling = $state(false);
	let feedbackMessage = $state<{ text: string; type: 'success' | 'info' } | null>(null);
	let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;

	function showFeedback(text: string, type: 'success' | 'info' = 'info', notifyToast = true) {
		if (feedbackTimeout) clearTimeout(feedbackTimeout);
		feedbackMessage = { text, type };
		feedbackTimeout = setTimeout(() => {
			feedbackMessage = null;
		}, 3500);

		if (notifyToast) {
			toast.add({
				title: type === 'success' ? 'Success' : 'Notice',
				description: text,
				variant: type === 'success' ? 'success' : 'info'
			});
		}
	}

	async function handleReloadDaemon() {
		isReloadingDaemon = true;
		try {
			await api.runtime.quadlets.reload();
			showFeedback('Systemd user daemon reloaded successfully', 'success');
		} catch (err: any) {
			toast.add({
				title: 'Reload Failed',
				description: err?.message || 'Failed to reload systemd daemon',
				variant: 'error'
			});
		} finally {
			isReloadingDaemon = false;
		}
	}

	function handleScrollToUnit() {
		const el = document.getElementById('quadlet-editor-section');
		if (el) {
			el.scrollIntoView({ behavior: 'smooth' });
		} else {
			showFeedback(
				`Unit file located at ~/.config/containers/systemd/${service.name}.container`,
				'info'
			);
		}
	}

	function handleToggleInPod() {
		inPod = !inPod;
		service.inPod = inPod;
		if (service.type !== 'quadlet') {
			service.runtimeTarget = inPod ? 'pod' : 'standalone';
		}
		if (!inPod) {
			service.podId = '';
		}
		dataStore.updateService(service);
		showFeedback(
			inPod
				? 'Podman Pod mode enabled: services will share localhost network namespace.'
				: 'Standalone mode enabled: services will run as individual containers with bridge DNS.',
			'info'
		);
	}

	function handleToggleAutodeploy(checked?: boolean) {
		autoDeploy = checked !== undefined ? checked : !autoDeploy;
		service.autoDeploy = autoDeploy;
		dataStore.updateService(service);
		showFeedback(
			autoDeploy
				? 'Autodeploy enabled: webhooks and registry updates will auto-deploy.'
				: 'Autodeploy paused: deployments must be triggered manually.',
			'info'
		);
	}

	let typeLabel = $derived.by(() => {
		switch (service.type) {
			case 'quadlet':
				return 'Quadlet';
			case 'compose':
				return 'Compose';
			case 'kubernetes':
				return 'Kubernetes';
			case 'pod':
				return 'Pod';
			case 'database':
				return 'Database';
			case 'image':
				return 'Container';
			default:
				return 'Application';
		}
	});

	let subtitle = $derived.by(() => {
		switch (service.type) {
			case 'quadlet':
				return 'Deploy and supervise native systemd Quadlet service unit (~/.config/containers/systemd)';
			case 'compose':
				return 'Create a compose file to deploy your multi-container compose stack';
			case 'kubernetes':
				return 'Execute declarative Kubernetes manifests via podman kube play';
			case 'pod':
				return 'Manage Podman pod infra, host port forwarding, and member containers';
			case 'database':
				return 'Manage containerized database engine with dedicated volume mounts';
			case 'image':
				return 'Run and supervise your standalone container workload';
			default:
				return 'Build and deploy your containerized application';
		}
	});

	let isRunning = $derived(service.status === 'running' || service.status === 'healthy');

	async function handleDeploy() {
		if (isDeploying) return;
		isDeploying = true;
		showFeedback(
			service.type === 'quadlet'
				? 'Compiling Quadlet unit & generating systemd service...'
				: 'Deployment initiated. Building and rolling out workload...',
			'info'
		);
		try {
			await dataStore.deployService(service.id, 'manual');
			service.status = 'running';
			showFeedback('Deployment completed successfully.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Deployment error', 'info');
		} finally {
			isDeploying = false;
		}
	}

	async function handleRebuild() {
		if (isRebuilding) return;
		isRebuilding = true;
		showFeedback('Pulling latest base images and initiating clean build...', 'info');
		try {
			await dataStore.deployService(service.id, 'manual');
			showFeedback('Rebuild and deployment complete.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Rebuild error', 'info');
		} finally {
			isRebuilding = false;
		}
	}

	async function handleFreshVolumes() {
		if (isFreshingVolumes) return;
		if (
			!confirm(
				'Are you sure you want to deploy with fresh volumes? Existing container volume mounts will be recreated.'
			)
		) {
			return;
		}
		isFreshingVolumes = true;
		showFeedback('Purging volume mounts and recreating workload container...', 'info');
		try {
			await dataStore.deployService(service.id, 'manual');
			showFeedback('Workload redeployed with fresh storage volumes.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Failed to deploy with fresh volumes', 'info');
		} finally {
			isFreshingVolumes = false;
		}
	}

	async function handleTogglePower() {
		if (isRunning) {
			isStopping = true;
			try {
				await dataStore.stopService(service.id);
				service.status = 'stopped';
				showFeedback('Workload stopped.', 'info');
			} catch (err: any) {
				showFeedback(err?.message || 'Failed to stop service', 'info');
			} finally {
				isStopping = false;
			}
		} else {
			isStarting = true;
			try {
				await dataStore.startService(service.id);
				service.status = 'running';
				showFeedback('Workload started.', 'success');
			} catch (err: any) {
				showFeedback(err?.message || 'Failed to start service', 'info');
			} finally {
				isStarting = false;
			}
		}
	}

	async function handleCancelDeployment() {
		if (isCancelling) return;
		isCancelling = true;
		showFeedback('Cancelling active deployment...', 'info');
		try {
			await dataStore.cancelServiceDeployment(service.id);
			service.status = 'stopped';
			isDeploying = false;
			showFeedback('Deployment cancelled successfully.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Failed to cancel deployment', 'info');
		} finally {
			isCancelling = false;
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- 1. Card Header -->
	<LayerCard.Secondary
		class="flex items-center justify-between gap-4 border-b border-kumo-hairline px-5 py-3.5"
	>
		<div class="flex flex-col gap-0.5">
			<div class="flex items-center gap-2">
				<h3 class="m-0 text-sm font-semibold text-kumo-default">Deploy settings</h3>
			</div>
			<p class="m-0 text-xs text-kumo-subtle">{subtitle}</p>
		</div>

		<!-- Workload Type Badge -->
		<Badge variant="neutral" class="shrink-0 px-2.5 py-1 font-mono text-xs">
			{#if service.type === 'quadlet'}
				<Sparkle size={13} class="mr-1 inline text-[var(--accent)]" />
			{/if}
			{typeLabel}
		</Badge>
	</LayerCard.Secondary>

	<!-- 2. Action Buttons Bar -->
	<DeployActionsBar
		{service}
		{isRunning}
		{isDeploying}
		{isCancelling}
		{isRebuilding}
		{isFreshingVolumes}
		{isStopping}
		{isStarting}
		{isReloadingDaemon}
		{autoDeploy}
		onDeploy={handleDeploy}
		onCancelDeployment={handleCancelDeployment}
		onReloadDaemon={handleReloadDaemon}
		onScrollToUnit={handleScrollToUnit}
		onTogglePower={handleTogglePower}
		onRebuild={handleRebuild}
		onFreshVolumes={handleFreshVolumes}
		{onTerminalClick}
		onToggleAutodeploy={handleToggleAutodeploy}
	/>

	<!-- 3. Podman Execution & Pod Strategy -->
	<DeployPodStrategyBanner {service} {inPod} onToggleInPod={handleToggleInPod} />

	<!-- 4. Instant Feedback Banner -->
	{#if feedbackMessage}
		<div
			class="flex items-center justify-between border-t border-kumo-hairline px-5 py-2.5 text-xs {feedbackMessage.type ===
			'success'
				? 'bg-kumo-success-tint text-kumo-success'
				: 'bg-[var(--accent-muted)] text-[var(--status-blue)]'}"
		>
			<div class="flex items-center gap-2">
				{#if feedbackMessage.type === 'success'}
					<CheckCircle size={15} weight="fill" />
				{:else}
					<Info size={15} weight="fill" />
				{/if}
				<span>{feedbackMessage.text}</span>
			</div>
			<button
				type="button"
				onclick={() => (feedbackMessage = null)}
				class="cursor-pointer border-0 bg-transparent text-[11px] text-inherit underline opacity-70 hover:opacity-100"
			>
				Dismiss
			</button>
		</div>
	{/if}
</LayerCard>
