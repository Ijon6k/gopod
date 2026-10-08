<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import {
		RocketLaunch,
		ArrowClockwise,
		Stop,
		Play,
		Terminal,
		Database,
		CheckCircle,
		Info,
		Sparkle
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, onTerminalClick, onRedeploy }: Props = $props();

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
	let feedbackMessage = $state<{ text: string; type: 'success' | 'info' } | null>(null);
	let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;

	function showFeedback(text: string, type: 'success' | 'info' = 'info') {
		if (feedbackTimeout) clearTimeout(feedbackTimeout);
		feedbackMessage = { text, type };
		feedbackTimeout = setTimeout(() => {
			feedbackMessage = null;
		}, 3500);
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

	async function handleDeploy() {
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
		const isRunning = service.status === 'running' || service.status === 'healthy';

		try {
			if (isRunning) {
				showFeedback('Stopping service workload...', 'info');
				const updated = await dataStore.stopService(service.id);
				service.status = updated.status || 'stopped';
				showFeedback('Workload stopped successfully.', 'info');
			} else {
				showFeedback('Starting service workload...', 'info');
				const updated = await dataStore.startService(service.id);
				service.status = updated.status || 'running';
				showFeedback('Workload started successfully.', 'success');
			}
		} catch (err: any) {
			showFeedback(err?.message || 'Failed to update service state', 'info');
		} finally {
			isTogglingPower = false;
		}
	}

	function handleToggleAutodeploy() {
		autoDeploy = !autoDeploy;
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

<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col shadow-xs">
	<!-- Card Header (Dokploy style) -->
	<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between gap-4">
		<div class="flex flex-col gap-0.5">
			<div class="flex items-center gap-2">
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Deploy Settings</h3>
			</div>
			<p class="text-xs text-[var(--text-tertiary)] m-0">{subtitle}</p>
		</div>

		<!-- Workload Type Badge -->
		<span class="inline-flex items-center gap-1.5 text-xs font-semibold px-2.5 py-1 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-primary)] font-[var(--font-mono)] shrink-0">
			{#if service.type === 'quadlet'}
				<Sparkle size={13} class="text-[var(--accent)]" />
			{/if}
			{typeLabel}
		</span>
	</div>

	<!-- Action Buttons Bar (Exact Dokploy layout with high visual hierarchy) -->
	<div class="p-5 flex flex-wrap items-center justify-between gap-4">
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- 1. Deploy (High-contrast Primary Button) -->
			<button
				type="button"
				onclick={handleDeploy}
				disabled={isDeploying || service.status === 'deploying'}
				class="flex items-center gap-2 px-4 py-2 rounded-md bg-[var(--accent)] hover:opacity-90 text-[var(--bg-shell)] text-xs font-semibold cursor-pointer border-0 transition-all shadow-xs disabled:opacity-50"
			>
				{#if isDeploying || service.status === 'deploying'}
					<ArrowClockwise size={15} class="animate-spin text-inherit" /> Deploying…
				{:else}
					<RocketLaunch size={15} weight="fill" class="text-inherit" /> Deploy
				{/if}
			</button>

			<!-- 2. Fresh Volumes (Dokploy signature action) -->
			<button
				type="button"
				onclick={handleFreshVolumes}
				disabled={isFreshingVolumes || isDeploying || service.status === 'deploying'}
				class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors disabled:opacity-50"
				title="Purge and mount fresh storage volumes"
			>
				{#if isFreshingVolumes}
					<ArrowClockwise size={14} class="animate-spin text-inherit" /> Purging…
				{:else}
					<Database size={14} /> Fresh Volumes
				{/if}
			</button>

			<!-- 3. Rebuild (Clean rebuild & image re-pull) -->
			<button
				type="button"
				onclick={handleRebuild}
				disabled={isRebuilding || isDeploying || service.status === 'deploying'}
				class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors disabled:opacity-50"
				title="Force rebuild container image and reload unit"
			>
				<ArrowClockwise size={14} class={isRebuilding ? 'animate-spin' : ''} /> Rebuild
			</button>

			<!-- 4. Stop / Start Button (Subtle red destructive / green start) -->
			{#if service.status === 'running' || service.status === 'healthy'}
				<button
					type="button"
					onclick={handleTogglePower}
					disabled={isTogglingPower}
					class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--status-red)]/25 bg-[var(--status-red)]/10 hover:bg-[var(--status-red)]/20 text-xs font-medium text-[var(--status-red)] cursor-pointer transition-colors disabled:opacity-50"
					title="Gracefully stop service workload"
				>
					{#if isTogglingPower}
						<ArrowClockwise size={14} class="animate-spin" /> Stopping…
					{:else}
						<Stop size={14} weight="fill" /> Stop
					{/if}
				</button>
			{:else}
				<button
					type="button"
					onclick={handleTogglePower}
					disabled={isTogglingPower || service.status === 'deploying'}
					class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--status-green)]/25 bg-[var(--status-green)]/10 hover:bg-[var(--status-green)]/20 text-xs font-medium text-[var(--status-green)] cursor-pointer transition-colors disabled:opacity-50"
					title="Start service workload"
				>
					{#if isTogglingPower}
						<ArrowClockwise size={14} class="animate-spin" /> Starting…
					{:else}
						<Play size={14} weight="fill" /> Start
					{/if}
				</button>
			{/if}

			<!-- 5. Open Terminal -->
			<button
				type="button"
				onclick={onTerminalClick}
				class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors font-[var(--font-mono)]"
				title="Open interactive terminal console"
			>
				<Terminal size={14} /> &gt;_ Open Terminal
			</button>
		</div>

		<!-- 6. Autodeploy Toggle Switch (Dokploy style) -->
		<div class="flex items-center gap-3 pl-2 sm:border-l sm:border-[var(--border-subtle)]">
			<span class="text-xs font-medium text-[var(--text-secondary)]">Autodeploy</span>
			<button
				type="button"
				role="switch"
				aria-checked={autoDeploy}
				onclick={handleToggleAutodeploy}
				class="w-9 h-5 rounded-full transition-colors relative cursor-pointer border-0 p-0 {autoDeploy
					? 'bg-[var(--accent)]'
					: 'bg-zinc-700'}"
				title="Toggle automatic deployment on webhook or registry push"
			>
				<span
					class="w-4 h-4 rounded-full bg-white absolute top-0.5 transition-transform {autoDeploy
						? 'left-4.5'
						: 'left-0.5'}"
				></span>
			</button>
		</div>
	</div>

	<!-- Podman Execution & Pod Strategy (UX Heuristics #1: Visibility & User Control) -->
	<div class="px-5 py-3 border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] flex flex-wrap items-center justify-between gap-3 text-xs">
		<div class="flex items-center gap-2.5">
			<label class="flex items-center gap-2 cursor-pointer select-none">
				<input
					type="checkbox"
					checked={inPod}
					onchange={handleToggleInPod}
					class="w-4 h-4 rounded border-[var(--border)] accent-[var(--accent)] cursor-pointer"
				/>
				<span class="font-medium text-[var(--text-primary)]">
					{service.type === 'compose' ? 'Encapsulate stack inside Podman Pod' : 'Run inside Podman Pod'}
				</span>
			</label>
			<span class="text-[11px] text-[var(--text-tertiary)] hidden sm:inline">
				{inPod
					? `(Shared localhost network namespace · pod_${service.id})`
					: '(Independent containers attached to gopod-net bridge)'}
			</span>
		</div>
		<div class="flex items-center gap-2">
			<span class="text-[11px] font-[var(--font-mono)] px-2 py-0.5 rounded border border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)]">
				network: gopod-net
			</span>
			<span class="text-[11px] font-[var(--font-mono)] px-2 py-0.5 rounded border border-[var(--border)] bg-[var(--bg-panel)] {inPod ? 'text-[var(--accent)] border-[var(--accent)]/40' : 'text-[var(--text-secondary)]'}">
				mode: {inPod ? 'podman-pod' : 'standalone'}
			</span>
		</div>
	</div>

	<!-- Instant Feedback Banner (Nielsen #1: Visibility of System Status) -->
	{#if feedbackMessage}
		<div
			class="px-5 py-2.5 border-t border-[var(--border-subtle)] flex items-center justify-between text-xs {feedbackMessage.type === 'success'
				? 'bg-emerald-500/10 text-emerald-400'
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
				class="text-[11px] underline opacity-70 hover:opacity-100 bg-transparent border-0 cursor-pointer text-inherit"
			>
				Dismiss
			</button>
		</div>
	{/if}
</div>
