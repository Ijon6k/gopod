<script lang="ts">
	import type { Deployment } from '$lib/types';
	import { Button, Chip } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
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
		Check
	} from 'phosphor-svelte';

	interface Props {
		deployment: Deployment;
		isCurrent?: boolean;
		onViewLogs: (dep: Deployment) => void;
		onRedeploy: (dep: Deployment) => void;
		onCancel?: (dep: Deployment) => void;
		onDelete: (dep: Deployment) => void;
	}

	let {
		deployment,
		isCurrent = false,
		onViewLogs,
		onRedeploy,
		onCancel,
		onDelete
	}: Props = $props();

	let copiedCommit = $state(false);

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

	const isBuilding = $derived(
		deployment.status === 'deploying' || deployment.status === 'building'
	);
</script>

<div
	class="group relative flex w-full flex-col justify-between gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4 transition-all hover:bg-[var(--bg-hover)] md:flex-row md:items-center"
>
	<!-- Left / Main: Status, Commit Info, Branch, Trigger -->
	<div class="flex min-w-0 flex-1 items-start gap-3.5">
		<!-- Status Indicator Icon -->
		<div class="mt-0.5 flex shrink-0 items-center justify-center">
			<StatusBadge status={deployment.status} size="sm" />
		</div>

		<div class="flex min-w-0 flex-1 flex-col gap-1.5">
			<!-- Row 1: Number, Current live badge, Commit message -->
			<div class="flex flex-wrap items-center gap-2">
				<span class="text-xs font-[var(--font-mono)] font-semibold text-[var(--text-primary)]">
					#{deployment.number}
				</span>

				{#if isCurrent}
					<Chip variant="accent" size="sm" class="text-[10px] font-medium">Live</Chip>
				{/if}

				<span
					class="max-w-[420px] truncate text-xs font-medium text-[var(--text-primary)]"
					title={deployment.commitMessage}
				>
					{deployment.commitMessage}
				</span>
			</div>

			<!-- Row 2: Commit SHA, Branch, Trigger, Author -->
			<div class="flex flex-wrap items-center gap-2 text-[11px] text-[var(--text-tertiary)]">
				{#if deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== ''}
					<button
						type="button"
						onclick={copyCommit}
						class="inline-flex cursor-pointer items-center gap-1 rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 text-[10.5px] font-[var(--font-mono)] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
						title="Copy commit hash"
					>
						<GitCommit size={11} />
						<span>{deployment.commit.substring(0, 7)}</span>
						{#if copiedCommit}
							<Check size={10} class="text-[var(--status-green)]" />
						{:else}
							<Copy size={10} class="opacity-60" />
						{/if}
					</button>
				{/if}

				{#if deployment.branch && deployment.branch !== '—' && deployment.branch.trim() !== ''}
					<span
						class="inline-flex items-center gap-1 font-[var(--font-mono)] text-[var(--text-secondary)]"
					>
						<GitBranch size={11} class="text-[var(--text-tertiary)]" />
						{deployment.branch}
					</span>
				{/if}

				{#if (deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== '') || (deployment.branch && deployment.branch !== '—' && deployment.branch.trim() !== '')}
					<span class="text-[var(--border)]">•</span>
				{/if}

				<!-- Trigger Type -->
				<span class="inline-flex items-center gap-1">
					{#if deployment.trigger === 'rollback'}
						<ArrowCounterClockwise size={11} class="text-[var(--status-amber)]" />
						<span>Rollback</span>
					{:else if deployment.trigger === 'webhook'}
						<Globe size={11} class="text-[var(--accent)]" />
						<span>Webhook</span>
					{:else if deployment.trigger === 'compose'}
						<Terminal size={11} class="text-[var(--accent)]" />
						<span>Compose</span>
					{:else if deployment.trigger === 'quadlet'}
						<Terminal size={11} class="text-[var(--text-secondary)]" />
						<span>Quadlet</span>
					{:else if deployment.trigger === 'git-push' || (deployment.commit && deployment.commit !== '—' && deployment.commit.trim() !== '')}
						<GitCommit size={11} />
						<span>Git Push</span>
					{:else}
						<User size={11} />
						<span>Manual</span>
					{/if}
				</span>

				{#if deployment.author}
					<span class="text-[var(--border)]">•</span>
					<span>by {deployment.author}</span>
				{/if}
			</div>
		</div>
	</div>

	<!-- Middle: Image Tag & Duration/Timestamp -->
	<div
		class="flex shrink-0 flex-wrap items-center gap-4 pl-7 text-xs text-[var(--text-secondary)] sm:flex-nowrap md:gap-6 md:pl-0"
	>
		<!-- Version or Image Pill -->
		<div class="flex flex-col gap-0.5">
			<span class="text-[10.5px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
				>Artifact</span
			>
			<span
				class="max-w-[190px] truncate rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-0.5 text-[11.5px] font-[var(--font-mono)] text-[var(--text-primary)]"
				title={deployment.version}
			>
				{deployment.version}
			</span>
		</div>

		<!-- Duration & Time -->
		<div class="flex min-w-[90px] flex-col gap-0.5">
			<span class="text-[10.5px] font-medium tracking-wider text-[var(--text-tertiary)] uppercase"
				>Timing</span
			>
			<div class="flex items-center gap-2 text-[11px] text-[var(--text-secondary)]">
				<span class="inline-flex items-center gap-1" title="Execution duration">
					<Timer size={11} class="text-[var(--text-tertiary)]" />
					{deployment.duration}
				</span>
				<span class="text-[var(--border)]">•</span>
				<span class="inline-flex items-center gap-1" title={deployment.startedAt}>
					<Clock size={11} class="text-[var(--text-tertiary)]" />
					{deployment.timeAgo}
				</span>
			</div>
		</div>
	</div>

	<!-- Right: Actions -->
	<div
		class="flex shrink-0 items-center gap-2 border-t border-[var(--border-subtle)] pt-3 pl-7 md:border-t-0 md:pt-0 md:pl-0"
	>
		<!-- View Logs Button (Dokploy / Coolify style) -->
		<Button
			variant="secondary"
			size="sm"
			onclick={() => onViewLogs(deployment)}
			class="h-8 gap-1.5"
		>
			<Terminal size={13} />
			<span>View Logs</span>
		</Button>

		<!-- Rollback / Redeploy Button -->
		{#if !isBuilding}
			<Button
				variant="ghost"
				size="sm"
				onclick={() => onRedeploy(deployment)}
				class="h-8 gap-1 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
				title={isCurrent ? 'Redeploy this version' : 'Rollback to this deployment'}
			>
				{#if isCurrent}
					<ArrowCounterClockwise size={13} />
					<span class="hidden sm:inline">Redeploy</span>
				{:else}
					<ArrowCounterClockwise size={13} />
					<span class="hidden sm:inline">Rollback</span>
				{/if}
			</Button>
		{:else if onCancel}
			<!-- Cancel Active Deployment Button -->
			<Button
				variant="ghost"
				size="sm"
				onclick={() => onCancel(deployment)}
				class="h-8 gap-1 text-[var(--status-red)] hover:bg-[rgba(184,84,84,0.12)]"
				title="Cancel active deployment"
			>
				<StopCircle size={13} />
				<span class="hidden sm:inline">Cancel</span>
			</Button>
		{/if}

		<!-- Delete from history -->
		<button
			type="button"
			onclick={() => onDelete(deployment)}
			class="cursor-pointer rounded-[var(--radius-sm)] border-0 bg-transparent p-2 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-surface)] hover:text-[var(--status-red)]"
			title="Delete deployment record"
			aria-label="Delete deployment"
		>
			<Trash size={14} />
		</button>
	</div>
</div>
