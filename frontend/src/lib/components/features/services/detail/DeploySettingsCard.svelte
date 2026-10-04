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
	$effect(() => {
		autoDeploy = service.autoDeploy ?? true;
	});
	let isDeploying = $state(false);
	let isRebuilding = $state(false);
	let feedbackMessage = $state<{ text: string; type: 'success' | 'info' } | null>(null);
	let feedbackTimeout: ReturnType<typeof setTimeout> | null = null;

	function showFeedback(text: string, type: 'success' | 'info' = 'info') {
		if (feedbackTimeout) clearTimeout(feedbackTimeout);
		feedbackMessage = { text, type };
		feedbackTimeout = setTimeout(() => {
			feedbackMessage = null;
		}, 3500);
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

	function handleDeploy() {
		isDeploying = true;
		service.status = 'deploying';

		if (onRedeploy) {
			onRedeploy();
		} else {
			const newDep: import('$lib/types').Deployment = {
				id: `dep-${service.id}-${Date.now()}`,
				projectId: service.projectId,
				projectName: service.projectId,
				serviceId: service.id,
				serviceName: service.name,
				number: (service.deployments?.length ?? 0) + 1,
				version: `v1.${(service.deployments?.length ?? 0) + 1}.0`,
				commit: Math.random().toString(16).substring(2, 9),
				commitMessage:
					service.type === 'quadlet'
						? 'Triggered Quadlet systemd service reload & start'
						: 'Triggered deployment from Deploy Settings',
				branch: service.branch ?? 'main',
				status: 'deploying',
				duration: 'Running…',
				timeAgo: 'Just now',
				startedAt: new Date().toISOString(),
				finishedAt: ''
			};
			dataStore.deployments.unshift(newDep);
		}

		showFeedback(
			service.type === 'quadlet'
				? 'Compiling unit & executing systemctl --user start...'
				: 'Deployment initiated. Monitoring build logs...',
			'info'
		);

		setTimeout(() => {
			service.status = 'running';
			isDeploying = false;
			dataStore.updateService(service);
			showFeedback(
				service.type === 'quadlet'
					? `Quadlet unit ${service.name}.service is now active (running)`
					: 'Deployment completed successfully. Service is healthy.',
				'success'
			);
		}, 1600);
	}

	function handleRebuild() {
		isRebuilding = true;
		service.status = 'building';
		showFeedback('Pulling fresh image and rebuilding service...', 'info');

		setTimeout(() => {
			isRebuilding = false;
			handleDeploy();
		}, 1200);
	}

	function handleTogglePower() {
		if (service.status === 'running' || service.status === 'healthy') {
			service.status = 'stopped';
			showFeedback(
				service.type === 'quadlet'
					? `Executed: systemctl --user stop ${service.name}`
					: 'Container workload stopped.',
				'info'
			);
		} else {
			service.status = 'running';
			showFeedback(
				service.type === 'quadlet'
					? `Executed: systemctl --user start ${service.name}`
					: 'Container workload started.',
				'success'
			);
		}
		dataStore.updateService(service);
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

	function handleFreshVolumes() {
		showFeedback(
			'Fresh Volumes: storage caches queued for reset. Next deployment will mount clean volumes.',
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
				class="flex items-center gap-2 px-4 py-2 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold cursor-pointer border-0 transition-all shadow-xs disabled:opacity-50"
			>
				{#if isDeploying || service.status === 'deploying'}
					<ArrowClockwise size={15} class="animate-spin text-zinc-950" /> Deploying…
				{:else}
					<RocketLaunch size={15} weight="fill" class="text-zinc-950" /> Deploy
				{/if}
			</button>

			<!-- 2. Fresh Volumes (Dokploy signature action) -->
			<button
				type="button"
				onclick={handleFreshVolumes}
				class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
				title="Purge and mount fresh storage volumes"
			>
				<Database size={14} /> Fresh Volumes
			</button>

			<!-- 3. Rebuild (Clean rebuild & image re-pull) -->
			<button
				type="button"
				onclick={handleRebuild}
				disabled={isRebuilding}
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
					class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-red-500/25 bg-red-500/10 hover:bg-red-500/20 text-xs font-medium text-red-400 cursor-pointer transition-colors"
					title="Gracefully stop service container"
				>
					<Stop size={14} weight="fill" /> Stop
				</button>
			{:else}
				<button
					type="button"
					onclick={handleTogglePower}
					class="flex items-center gap-1.5 px-3.5 py-2 rounded-md border border-emerald-500/25 bg-emerald-500/10 hover:bg-emerald-500/20 text-xs font-medium text-emerald-400 cursor-pointer transition-colors"
					title="Start service container"
				>
					<Play size={14} weight="fill" /> Start
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
