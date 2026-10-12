<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { Badge } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import {
		Terminal,
		ArrowCounterClockwise,
		Trash,
		GitBranch,
		GitCommit,
		User,
		Timer,
		Clock,
		Globe,
		StopCircle,
		Copy,
		Check,
		FolderSimple
	} from 'phosphor-svelte';

	interface Props {
		deployment: Deployment;
		isCurrent?: boolean;
		showProjectContext?: boolean;
		onViewLogs: (dep: Deployment) => void;
		onRedeploy: (dep: Deployment) => void;
		onCancel?: (dep: Deployment) => void;
		onDelete: (dep: Deployment) => void;
	}

	let {
		deployment,
		isCurrent = false,
		showProjectContext = true,
		onViewLogs,
		onRedeploy,
		onCancel,
		onDelete
	}: Props = $props();

	let copiedCommit = $state(false);

	let resolvedProjectName = $derived(
		deployment.projectName ||
		dataStore.getProjectById(deployment.projectId)?.name ||
		deployment.projectId ||
		'Default'
	);

	let resolvedServiceName = $derived(
		deployment.serviceName ||
		dataStore.getServiceById(deployment.serviceId)?.name ||
		deployment.serviceId ||
		'Service'
	);

	async function copyCommit(e: MouseEvent) {
		e.stopPropagation();
		if (!deployment.commit || deployment.commit === '—') return;
		try {
			await navigator.clipboard.writeText(deployment.commit);
			copiedCommit = true;
			setTimeout(() => {
				copiedCommit = false;
			}, 1500);
		} catch (err) {
			console.error('Failed to copy commit', err);
		}
	}

	const isSuccess = $derived(
		deployment.status === 'success' ||
		deployment.status === 'running' ||
		deployment.status === 'healthy'
	);

	const isBuilding = $derived(
		deployment.status === 'deploying' || deployment.status === 'building'
	);

	const isFailed = $derived(deployment.status === 'failed');

	const badgeVariant = $derived(
		isSuccess
			? 'success'
			: isFailed
				? 'error'
				: isBuilding
					? 'warning'
					: 'neutral'
	);

	const statusLabel = $derived(
		isSuccess
			? 'Success'
			: isBuilding
				? 'Building'
				: isFailed
					? 'Failed'
					: deployment.status
	);
</script>

<div
	class="group relative flex w-full flex-col justify-between gap-3.5 rounded-lg bg-kumo-base p-4 ring ring-kumo-line transition-colors hover:bg-kumo-tint md:flex-row md:items-center"
>
	<!-- Left / Primary: Status, Number, Commit Message, Metadata -->
	<div class="flex min-w-0 flex-1 items-start gap-3">
		<!-- Status Dot Badge -->
		<div class="mt-0.5 flex shrink-0 items-center justify-center">
			<Badge variant={badgeVariant} appearance="dot">
				{statusLabel}
			</Badge>
		</div>

		<div class="flex min-w-0 flex-1 flex-col gap-1.5">
			<!-- Context Eyebrow: Project & Service Breadcrumb (Global View) -->
			{#if showProjectContext}
				<div class="flex items-center gap-1.5 text-xs text-kumo-subtle">
					<FolderSimple size={13} class="shrink-0 text-kumo-subtle/70" />
					<a
						href="/projects/{deployment.projectId}"
						class="font-medium text-kumo-subtle hover:text-kumo-default transition-colors no-underline"
						title="Go to project {resolvedProjectName}"
					>
						{resolvedProjectName}
					</a>
					<span class="text-kumo-hairline select-none">/</span>
					<a
						href="/projects/{deployment.projectId}/services/{deployment.serviceId}"
						class="font-mono text-xs text-[var(--accent)] hover:underline"
						title="Go to service {resolvedServiceName}"
					>
						{resolvedServiceName}
					</a>
				</div>
			{/if}

			<!-- Line 1: Number, Live badge, Commit message -->
			<div class="flex flex-wrap items-center gap-2">
				<span class="font-mono text-sm font-semibold text-kumo-default">
					#{deployment.number}
				</span>

				{#if isCurrent}
					<Badge variant="secondary" class="text-xs">Live</Badge>
				{/if}

				<span
					class="truncate text-sm font-medium text-kumo-default"
					title={deployment.commitMessage}
				>
					{deployment.commitMessage}
				</span>
			</div>

			<!-- Line 2: Commit SHA, Branch, Trigger, Author -->
			<div class="flex flex-wrap items-center gap-2 text-xs text-kumo-subtle">
				{#if deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== ''}
					<button
						type="button"
						onclick={copyCommit}
						class="inline-flex cursor-pointer items-center gap-1 rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-xs text-kumo-default ring ring-kumo-hairline/60 transition-colors hover:bg-kumo-fill"
						title="Copy commit hash"
					>
						<GitCommit size={12} />
						<span>{deployment.commit.substring(0, 7)}</span>
						{#if copiedCommit}
							<Check size={11} class="text-kumo-success" />
						{:else}
							<Copy size={11} class="opacity-60" />
						{/if}
					</button>
				{/if}

				{#if deployment.branch && deployment.branch !== '—' && deployment.branch.trim() !== ''}
					<span class="inline-flex items-center gap-1 font-mono text-kumo-subtle">
						<GitBranch size={12} />
						{deployment.branch}
					</span>
				{/if}

				{#if (deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== '') || (deployment.branch && deployment.branch !== '—' && deployment.branch.trim() !== '')}
					<span class="text-kumo-hairline">•</span>
				{/if}

				<!-- Trigger Type -->
				<span class="inline-flex items-center gap-1">
					{#if deployment.trigger === 'rollback'}
						<ArrowCounterClockwise size={12} class="text-kumo-warning" />
						<span>Rollback</span>
					{:else if deployment.trigger === 'webhook'}
						<Globe size={12} class="text-[var(--accent)]" />
						<span>Webhook</span>
					{:else if deployment.trigger === 'compose'}
						<Terminal size={12} class="text-[var(--accent)]" />
						<span>Compose</span>
					{:else if deployment.trigger === 'quadlet'}
						<Terminal size={12} class="text-kumo-subtle" />
						<span>Quadlet</span>
					{:else if deployment.trigger === 'git-push' || (deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== '')}
						<GitCommit size={12} />
						<span>Git push</span>
					{:else}
						<User size={12} />
						<span>Manual</span>
					{/if}
				</span>

				{#if deployment.author}
					<span class="text-kumo-hairline">•</span>
					<span>by {deployment.author}</span>
				{/if}
			</div>
		</div>
	</div>

	<!-- Middle: Artifact tag & Timing -->
	<div
		class="flex shrink-0 flex-wrap items-center gap-3 text-xs text-kumo-subtle sm:flex-nowrap md:gap-5"
	>
		<!-- Artifact tag -->
		{#if deployment.version}
			<span
				class="max-w-[190px] truncate rounded bg-kumo-recessed px-2 py-0.5 font-mono text-xs text-kumo-default ring ring-kumo-hairline/60"
				title={deployment.version}
			>
				{deployment.version}
			</span>
		{/if}

		<!-- Timing -->
		<div class="flex items-center gap-1.5 tabular-nums text-kumo-subtle">
			<span class="inline-flex items-center gap-1" title="Duration">
				<Timer size={13} />
				{deployment.duration}
			</span>
			<span class="text-kumo-hairline">•</span>
			<span class="inline-flex items-center gap-1" title={deployment.startedAt}>
				<Clock size={13} />
				{deployment.timeAgo}
			</span>
		</div>
	</div>

	<!-- Right: Actions -->
	<div class="flex shrink-0 items-center gap-2">
		<!-- View Logs Button -->
		<Button
			variant="secondary"
			size="sm"
			onclick={() => onViewLogs(deployment)}
			class="gap-1.5 whitespace-nowrap border border-kumo-line bg-kumo-elevated text-kumo-default shadow-xs hover:bg-kumo-fill"
		>
			<Terminal size={13} />
			<span>View logs</span>
		</Button>

		<!-- Rollback / Redeploy Button -->
		{#if !isBuilding}
			<Button
				variant="secondary"
				size="sm"
				onclick={() => onRedeploy(deployment)}
				class="gap-1.5 whitespace-nowrap border border-kumo-line bg-kumo-elevated text-kumo-default shadow-xs hover:bg-kumo-fill"
				title={isCurrent ? 'Redeploy this version' : 'Rollback to this deployment'}
			>
				<ArrowCounterClockwise size={13} />
				<span>{isCurrent ? 'Redeploy' : 'Rollback'}</span>
			</Button>
		{:else if onCancel}
			<Button
				variant="destructive"
				size="sm"
				onclick={() => onCancel(deployment)}
				class="gap-1.5 whitespace-nowrap shadow-xs"
				title="Cancel active deployment"
			>
				<StopCircle size={13} />
				<span>Cancel</span>
			</Button>
		{/if}

		<!-- Delete Button -->
		<Button
			variant="secondary"
			size="sm"
			shape="square"
			onclick={() => onDelete(deployment)}
			class="border border-kumo-line bg-kumo-elevated shadow-xs hover:bg-kumo-fill hover:text-kumo-danger"
			title="Delete deployment record"
			aria-label="Delete deployment"
		>
			<Trash size={14} class="text-kumo-subtle transition-colors group-hover:text-kumo-danger hover:text-kumo-danger" />
		</Button>
	</div>
</div>
