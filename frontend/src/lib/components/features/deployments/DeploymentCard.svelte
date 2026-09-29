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

	const isBuilding = $derived(deployment.status === 'deploying' || deployment.status === 'building');
</script>

<div
	class="group relative w-full p-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] hover:bg-[var(--bg-hover)] transition-all flex flex-col md:flex-row md:items-center justify-between gap-4"
>
	<!-- Left / Main: Status, Commit Info, Branch, Trigger -->
	<div class="flex items-start gap-3.5 min-w-0 flex-1">
		<!-- Status Indicator Icon -->
		<div class="mt-0.5 shrink-0 flex items-center justify-center">
			<StatusBadge status={deployment.status} size="sm" />
		</div>

		<div class="flex flex-col gap-1.5 min-w-0 flex-1">
			<!-- Row 1: Number, Current live badge, Commit message -->
			<div class="flex items-center flex-wrap gap-2">
				<span class="text-xs font-semibold font-[var(--font-mono)] text-[var(--text-primary)]">
					#{deployment.number}
				</span>

				{#if isCurrent}
					<Chip variant="accent" size="sm" class="font-medium text-[10px]">
						Live
					</Chip>
				{/if}

				<span
					class="text-xs font-medium text-[var(--text-primary)] truncate max-w-[420px]"
					title={deployment.commitMessage}
				>
					{deployment.commitMessage}
				</span>
			</div>

			<!-- Row 2: Commit SHA, Branch, Trigger, Author -->
			<div class="flex items-center flex-wrap gap-2 text-[11px] text-[var(--text-tertiary)]">
				{#if deployment.commit && deployment.commit !== '—'}
					<button
						type="button"
						onclick={copyCommit}
						class="inline-flex items-center gap-1 px-1.5 py-0.5 rounded bg-[var(--bg-surface)] hover:bg-[var(--bg-panel)] border border-[var(--border)] font-[var(--font-mono)] text-[10.5px] text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors cursor-pointer"
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

				{#if deployment.branch && deployment.branch !== '—'}
					<span class="inline-flex items-center gap-1 font-[var(--font-mono)] text-[var(--text-secondary)]">
						<GitBranch size={11} class="text-[var(--text-tertiary)]" />
						{deployment.branch}
					</span>
				{/if}

				<span class="text-[var(--border)]">•</span>

				<!-- Trigger Type -->
				<span class="inline-flex items-center gap-1">
					{#if deployment.trigger === 'rollback'}
						<ArrowCounterClockwise size={11} class="text-[var(--status-amber)]" />
						<span>Rollback</span>
					{:else if deployment.trigger === 'webhook'}
						<Globe size={11} class="text-[var(--accent)]" />
						<span>Webhook</span>
					{:else if deployment.trigger === 'manual'}
						<User size={11} />
						<span>Manual</span>
					{:else}
						<GitCommit size={11} />
						<span>Git Push</span>
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
	<div class="flex items-center flex-wrap sm:flex-nowrap gap-4 md:gap-6 text-xs text-[var(--text-secondary)] shrink-0 pl-7 md:pl-0">
		<!-- Version or Image Pill -->
		<div class="flex flex-col gap-0.5">
			<span class="text-[10.5px] text-[var(--text-tertiary)] uppercase tracking-wider font-medium">Artifact</span>
			<span class="font-[var(--font-mono)] text-[11.5px] text-[var(--text-primary)] bg-[var(--bg-surface)] px-2 py-0.5 rounded border border-[var(--border-subtle)] truncate max-w-[190px]" title={deployment.version}>
				{deployment.version}
			</span>
		</div>

		<!-- Duration & Time -->
		<div class="flex flex-col gap-0.5 min-w-[90px]">
			<span class="text-[10.5px] text-[var(--text-tertiary)] uppercase tracking-wider font-medium">Timing</span>
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
	<div class="flex items-center gap-2 shrink-0 pl-7 md:pl-0 border-t md:border-t-0 pt-3 md:pt-0 border-[var(--border-subtle)]">
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
			class="p-2 rounded-[var(--radius-sm)] text-[var(--text-tertiary)] hover:text-[var(--status-red)] hover:bg-[var(--bg-surface)] transition-colors cursor-pointer border-0 bg-transparent"
			title="Delete deployment record"
			aria-label="Delete deployment"
		>
			<Trash size={14} />
		</button>
	</div>
</div>
