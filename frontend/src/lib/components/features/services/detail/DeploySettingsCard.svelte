<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { Button } from '$lib/components/ui';
	import { LayerCard, Switch, Badge, useKumoToastManager } from 'kumo-svelte';
	import {
		RocketLaunch,
		ArrowClockwise,
		Stop,
		Play,
		Terminal,
		Database,
		CheckCircle,
		Info,
		Sparkle,
		FileText,
		Scroll,
		Package
	} from 'phosphor-svelte';

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
	let isTogglingPower = $state(false);
	let isReloadingDaemon = $state(false);
	let feedbackMessage = $state<{ text: string; type: 'success' | 'info' } | null>(null);
	let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;

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
			showFeedback(`Unit file located at ~/.config/containers/systemd/${service.name}.container`, 'info');
		}
	}

	function showFeedback(text: string, type: 'success' | 'info' = 'info') {
		if (feedbackTimeout) clearTimeout(feedbackTimeout);
		feedbackMessage = { text, type };
		feedbackTimeout = setTimeout(() => {
			feedbackMessage = null;
		}, 3500);

		toast.add({
			title: type === 'success' ? 'Success' : 'Notice',
			description: text,
			variant: type === 'success' ? 'success' : 'info'
		});
	}

	function handleToggleInPod() {
		inPod = !inPod;
		service.inPod = inPod;
		if (service.type !== 'quadlet') {
			service.runtimeTarget = inPod ? 'pod' : 'standalone';
		}
		dataStore.updateService(service);
		showFeedback(
			inPod
				? 'Podman Pod mode enabled: services will share localhost network namespace.'
				: 'Standalone mode enabled: services will run as individual containers with bridge DNS.',
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
			if (onRedeploy) {
				await onRedeploy();
			} else {
				await dataStore.deployService(service.id, 'manual');
			}
			showFeedback('Deployment queued and running. Live output streaming in Logs.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Deployment failed to initiate', 'info');
		} finally {
			isDeploying = false;
		}
	}

	async function handleRebuild() {
		if (isRebuilding) return;
		isRebuilding = true;
		showFeedback('Pulling fresh image layers and initiating rebuild...', 'info');

		try {
			if (onRedeploy) {
				await onRedeploy();
			} else {
				await dataStore.deployService(service.id, 'rebuild');
			}
			showFeedback('Rebuild rollout started.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Rebuild failed', 'info');
		} finally {
			isRebuilding = false;
		}
	}

	async function handleFreshVolumes() {
		if (isFreshingVolumes) return;
		isFreshingVolumes = true;
		showFeedback('Purging ephemeral volumes and triggering clean rollout...', 'info');

		try {
			await dataStore.deployService(service.id, 'fresh-volumes');
			showFeedback('Fresh volumes rollout started.', 'success');
		} catch (err: any) {
			showFeedback(err?.message || 'Fresh volumes rollout failed', 'info');
		} finally {
			isFreshingVolumes = false;
		}
	}

	async function handleTogglePower() {
		if (isTogglingPower) return;
		isTogglingPower = true;

		try {
			if (isRunning) {
				showFeedback('Stopping service workload...', 'info');
				const updated = await dataStore.stopService(service.id);
				service.status = updated?.status || 'stopped';
				showFeedback('Workload stopped successfully.', 'info');
			} else {
				showFeedback('Starting service workload...', 'info');
				const updated = await dataStore.startService(service.id);
				service.status = updated?.status || 'running';
				showFeedback('Workload started successfully.', 'success');
			}
		} catch (err: any) {
			showFeedback(err?.message || 'Failed to update service state', 'info');
		} finally {
			isTogglingPower = false;
		}
	}

	let isCancelling = $state(false);

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
</script>

<LayerCard class="w-full shadow-xs">
	<!-- 1. Card Header (Previous commit position, styled with Kumo UI) -->
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
	<div class="flex flex-wrap items-center justify-between gap-4 p-5 bg-kumo-base">
		<!-- Left: Cluster of Action Buttons -->
		<div class="flex flex-wrap items-center gap-2.5">
			{#if service.type === 'quadlet'}
				<!-- 1. Deploy / Reload (Primary for Quadlet) -->
				<Button
					variant="primary"
					size="sm"
					onclick={handleDeploy}
					disabled={isDeploying || service.status === 'deploying'}
					class="gap-2 px-4 py-2 font-medium shadow-xs"
				>
					{#if isDeploying || service.status === 'deploying'}
						<ArrowClockwise size={14} class="animate-spin text-inherit" />
						<span>Reloading…</span>
					{:else}
						<RocketLaunch size={14} weight="fill" class="text-inherit" />
						<span>Deploy / Reload</span>
					{/if}
				</Button>

				{#if isDeploying || service.status === 'deploying'}
					<Button
						variant="secondary-destructive"
						size="sm"
						onclick={handleCancelDeployment}
						disabled={isCancelling}
						class="gap-1.5 shadow-xs"
					>
						{#if isCancelling}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Cancelling…</span>
						{:else}
							<Stop size={13} weight="fill" />
							<span>Cancel</span>
						{/if}
					</Button>
				{/if}

				<!-- 2. Daemon Reload (Dedicated systemctl --user daemon-reload) -->
				<Button
					variant="secondary"
					size="sm"
					onclick={handleReloadDaemon}
					disabled={isReloadingDaemon || isDeploying}
					class="gap-1.5 shadow-xs"
				>
					<ArrowClockwise size={13} class={isReloadingDaemon ? 'animate-spin' : ''} />
					<span>Daemon Reload</span>
				</Button>

				<!-- 3. View Unit File -->
				<Button
					variant="secondary"
					size="sm"
					onclick={handleScrollToUnit}
					class="gap-1.5 shadow-xs"
				>
					<FileText size={13} />
					<span>View Unit File</span>
				</Button>

				<!-- 4. Stop / Start -->
				{#if isRunning}
					<Button
						variant="secondary-destructive"
						size="sm"
						onclick={handleTogglePower}
						disabled={isTogglingPower}
						class="gap-1.5 shadow-xs"
					>
						{#if isTogglingPower}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Stopping…</span>
						{:else}
							<Stop size={13} weight="fill" />
							<span>Stop</span>
						{/if}
					</Button>
				{:else}
					<Button
						variant="secondary"
						size="sm"
						onclick={handleTogglePower}
						disabled={isTogglingPower || service.status === 'deploying'}
						class="gap-1.5 text-kumo-success hover:bg-kumo-success-tint hover:ring-kumo-success/30 shadow-xs"
					>
						{#if isTogglingPower}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Starting…</span>
						{:else}
							<Play size={13} weight="fill" />
							<span>Start</span>
						{/if}
					</Button>
				{/if}

				<!-- 5. Open Terminal -->
				<Button
					variant="secondary"
					size="sm"
					onclick={onTerminalClick}
					class="gap-1.5 font-mono text-xs shadow-xs"
				>
					<Terminal size={13} />
					<span>Terminal</span>
				</Button>
			{:else}
				<!-- 1. Deploy (High-contrast Primary Button) -->
				<Button
					variant="primary"
					size="sm"
					onclick={handleDeploy}
					disabled={isDeploying || service.status === 'deploying'}
					class="gap-2 px-4 py-2 font-medium shadow-xs"
				>
					{#if isDeploying || service.status === 'deploying'}
						<ArrowClockwise size={14} class="animate-spin text-inherit" />
						<span>Deploying…</span>
					{:else}
						<RocketLaunch size={14} weight="fill" class="text-inherit" />
						<span>Deploy</span>
					{/if}
				</Button>

				{#if isDeploying || service.status === 'deploying'}
					<Button
						variant="secondary-destructive"
						size="sm"
						onclick={handleCancelDeployment}
						disabled={isCancelling}
						class="gap-1.5 shadow-xs"
					>
						{#if isCancelling}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Cancelling…</span>
						{:else}
							<Stop size={13} weight="fill" />
							<span>Cancel</span>
						{/if}
					</Button>
				{/if}

				<!-- 2. Fresh Volumes -->
				<Button
					variant="secondary"
					size="sm"
					onclick={handleFreshVolumes}
					disabled={isFreshingVolumes || isDeploying || service.status === 'deploying'}
					class="gap-1.5 shadow-xs"
				>
					{#if isFreshingVolumes}
						<ArrowClockwise size={13} class="animate-spin text-inherit" />
						<span>Purging…</span>
					{:else}
						<Database size={13} />
						<span>Fresh Volumes</span>
					{/if}
				</Button>

				<!-- 3. Rebuild (Clean rebuild & image re-pull) -->
				<Button
					variant="secondary"
					size="sm"
					onclick={handleRebuild}
					disabled={isRebuilding || isDeploying || service.status === 'deploying'}
					class="gap-1.5 shadow-xs"
				>
					<ArrowClockwise size={13} class={isRebuilding ? 'animate-spin' : ''} />
					<span>Rebuild</span>
				</Button>

				<!-- 4. Stop / Start Button -->
				{#if isRunning}
					<Button
						variant="secondary-destructive"
						size="sm"
						onclick={handleTogglePower}
						disabled={isTogglingPower}
						class="gap-1.5 shadow-xs"
					>
						{#if isTogglingPower}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Stopping…</span>
						{:else}
							<Stop size={13} weight="fill" />
							<span>Stop</span>
						{/if}
					</Button>
				{:else}
					<Button
						variant="secondary"
						size="sm"
						onclick={handleTogglePower}
						disabled={isTogglingPower || service.status === 'deploying'}
						class="gap-1.5 text-kumo-success hover:bg-kumo-success-tint hover:ring-kumo-success/30 shadow-xs"
					>
						{#if isTogglingPower}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Starting…</span>
						{:else}
							<Play size={13} weight="fill" />
							<span>Start</span>
						{/if}
					</Button>
				{/if}

				<!-- 5. Open Terminal -->
				<Button
					variant="secondary"
					size="sm"
					onclick={onTerminalClick}
					class="gap-1.5 font-mono text-xs shadow-xs"
				>
					<Terminal size={13} />
					<span>Terminal</span>
				</Button>
			{/if}
		</div>

		<!-- Right: Autodeploy Toggle Switch (Kumo UI Switch) -->
		<div class="flex items-center gap-3 pl-2 sm:border-l sm:border-kumo-line">
			<span class="text-xs font-medium text-kumo-subtle select-none">Autodeploy</span>
			<Switch
				size="sm"
				checked={autoDeploy}
				onCheckedChange={handleToggleAutodeploy}
				aria-label="Toggle automatic deployment on webhook or registry push"
			/>
		</div>
	</div>

	<!-- 3. Podman Execution & Pod Strategy -->
	{#if service.type === 'pod'}
		<!-- Pod Infrastructure Summary (No redundant "Run inside Podman Pod" switch) -->
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-t border-kumo-hairline bg-kumo-elevated px-5 py-3 text-xs"
		>
			<div class="flex items-center gap-2.5 text-kumo-default">
				<Package size={15} class="text-[var(--accent)]" />
				<span class="font-medium">Podman Pod Infrastructure</span>
				<span class="text-[11px] text-kumo-subtle">
					(All member containers share localhost namespace · pod_{service.name})
				</span>
			</div>
			<div class="flex items-center gap-2">
				<Badge variant="neutral" class="font-mono text-[11px]">network: gopod-net</Badge>
				<Badge variant="neutral" class="font-mono text-[11px] text-[var(--accent)]">mode: podman-pod</Badge>
			</div>
		</div>
	{:else if service.type !== 'compose' && service.type !== 'quadlet'}
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-t border-kumo-hairline bg-kumo-elevated px-5 py-3 text-xs"
		>
			<div class="flex items-center gap-3">
				<Switch
					checked={inPod}
					onCheckedChange={handleToggleInPod}
					aria-label="Toggle encapsulation inside Podman Pod"
				/>
				<span class="font-medium text-kumo-default">
					Run inside Podman Pod
				</span>
				<span class="hidden text-[11px] text-kumo-subtle sm:inline">
					{inPod
						? `(Shared localhost network namespace · pod_${service.id})`
						: '(Independent containers attached to gopod-net bridge)'}
				</span>
			</div>
			<div class="flex items-center gap-2">
				<Badge variant="neutral" class="font-mono text-[11px]">
					network: gopod-net
				</Badge>
				<Badge
					variant="neutral"
					class="font-mono text-[11px] {inPod ? 'text-[var(--accent)]' : 'text-kumo-subtle'}"
				>
					mode: {inPod ? 'podman-pod' : 'standalone'}
				</Badge>
			</div>
		</div>
	{/if}

	<!-- 4. Instant Feedback Banner (Nielsen #1: Visibility of System Status) -->
	{#if feedbackMessage}
		<div
			class="flex items-center justify-between border-t border-kumo-hairline px-5 py-2.5 text-xs {feedbackMessage.type ===
			'success'
				? 'bg-kumo-success-tint text-kumo-success'
				: 'bg-blue-500/10 text-blue-400'}"
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
