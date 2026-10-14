<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { Switch } from 'kumo-svelte';
	import {
		RocketLaunch,
		ArrowClockwise,
		Stop,
		Play,
		Terminal,
		Database,
		FileText
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		isRunning: boolean;
		isDeploying: boolean;
		isCancelling: boolean;
		isRebuilding: boolean;
		isFreshingVolumes: boolean;
		isStopping: boolean;
		isStarting: boolean;
		isReloadingDaemon: boolean;
		autoDeploy: boolean;
		onDeploy: () => void;
		onCancelDeployment: () => void;
		onReloadDaemon: () => void;
		onScrollToUnit: () => void;
		onTogglePower: () => void;
		onRebuild: () => void;
		onFreshVolumes: () => void;
		onTerminalClick?: () => void;
		onToggleAutodeploy: (checked?: boolean) => void;
	}

	let {
		service,
		isRunning,
		isDeploying,
		isCancelling,
		isRebuilding,
		isFreshingVolumes,
		isStopping,
		isStarting,
		isReloadingDaemon,
		autoDeploy,
		onDeploy,
		onCancelDeployment,
		onReloadDaemon,
		onScrollToUnit,
		onTogglePower,
		onRebuild,
		onFreshVolumes,
		onTerminalClick,
		onToggleAutodeploy
	}: Props = $props();
</script>

<div class="flex flex-wrap items-center justify-between gap-4 bg-kumo-base p-5">
	<!-- Left: Cluster of Action Buttons -->
	<div class="flex flex-wrap items-center gap-2.5">
		{#if service.type === 'quadlet'}
			<!-- 1. Deploy / Reload (Primary for Quadlet) -->
			<Button
				variant="primary"
				size="sm"
				onclick={onDeploy}
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
					onclick={onCancelDeployment}
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
				onclick={onReloadDaemon}
				disabled={isReloadingDaemon || isDeploying}
				class="gap-1.5 shadow-xs"
			>
				<ArrowClockwise size={13} class={isReloadingDaemon ? 'animate-spin' : ''} />
				<span>Daemon Reload</span>
			</Button>

			<!-- 3. View Unit File -->
			<Button variant="secondary" size="sm" onclick={onScrollToUnit} class="gap-1.5 shadow-xs">
				<FileText size={13} />
				<span>View Unit File</span>
			</Button>

			<!-- 4. Stop / Start -->
			{#if isRunning || isStopping || service.status === 'stopping'}
				<Button
					variant="secondary-destructive"
					size="sm"
					onclick={onTogglePower}
					disabled={isStopping || isStarting || service.status === 'stopping'}
					class="gap-1.5 shadow-xs"
				>
					{#if isStopping || service.status === 'stopping'}
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
					onclick={onTogglePower}
					disabled={isStopping ||
						isStarting ||
						service.status === 'deploying' ||
						service.status === 'starting'}
					class="hover:bg-kumo-success-tint gap-1.5 text-kumo-success shadow-xs hover:ring-kumo-success/30"
				>
					{#if isStarting || service.status === 'starting'}
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
				onclick={onDeploy}
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
					onclick={onCancelDeployment}
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
				onclick={onFreshVolumes}
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
				onclick={onRebuild}
				disabled={isRebuilding || isDeploying || service.status === 'deploying'}
				class="gap-1.5 shadow-xs"
			>
				<ArrowClockwise size={13} class={isRebuilding ? 'animate-spin' : ''} />
				<span>Rebuild</span>
			</Button>

			<!-- 4. Stop / Start Button -->
			{#if isRunning || isStopping || service.status === 'stopping'}
				<Button
					variant="secondary-destructive"
					size="sm"
					onclick={onTogglePower}
					disabled={isStopping || isStarting || service.status === 'stopping'}
					class="gap-1.5 shadow-xs"
				>
					{#if isStopping || service.status === 'stopping'}
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
					onclick={onTogglePower}
					disabled={isStopping ||
						isStarting ||
						service.status === 'deploying' ||
						service.status === 'starting'}
					class="hover:bg-kumo-success-tint gap-1.5 text-kumo-success shadow-xs hover:ring-kumo-success/30"
				>
					{#if isStarting || service.status === 'starting'}
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

	<!-- Right: Autodeploy Toggle Switch -->
	<div class="flex items-center gap-3 pl-2 sm:border-l sm:border-kumo-line">
		<span class="text-xs font-medium text-kumo-subtle select-none">Autodeploy</span>
		<Switch
			size="sm"
			checked={autoDeploy}
			onCheckedChange={onToggleAutodeploy}
			aria-label="Toggle automatic deployment on webhook or registry push"
		/>
	</div>
</div>
