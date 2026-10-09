<script lang="ts">
	import type { ServiceType } from '$lib/types';
	import { GitBranch, Database, Stack, FileText, Check } from 'phosphor-svelte';

	export type WorkloadChoice = 'application' | 'quadlet' | 'compose' | 'database';

	interface Props {
		selected: WorkloadChoice;
		onselect: (choice: WorkloadChoice) => void;
	}

	let { selected, onselect }: Props = $props();

	const options = [
		{
			id: 'application' as const,
			title: 'Application',
			description:
				'Deploy web apps, APIs, or static sites from Git repository, Docker Image, or Drag & Drop.',
			icon: GitBranch
		},
		{
			id: 'quadlet' as const,
			title: 'Quadlet (Systemd)',
			description:
				'Declarative systemd .container unit with auto-restart on host boot & native systemd supervision.',
			icon: FileText
		},
		{
			id: 'compose' as const,
			title: 'Stack / Compose',
			description:
				'Multi-container workloads defined via Compose YAML or Kubernetes Pod manifests.',
			icon: Stack
		},
		{
			id: 'database' as const,
			title: 'Database',
			description:
				'1-click deployment for PostgreSQL, Redis, MySQL, or MongoDB with persistent volume storage.',
			badge: 'Preset',
			icon: Database
		}
	];
</script>

<div class="flex flex-col gap-2">
	<span class="text-xs font-medium text-[var(--text-secondary)]"> Select Service Type </span>

	<!-- Vertical Stack Layout (Spacious, Clear Hierarchy, No Cramped Columns) -->
	<div class="flex flex-col gap-2.5">
		{#each options as opt}
			{@const Icon = opt.icon}
			{@const isSelected = selected === opt.id}
			<button
				type="button"
				onclick={() => onselect(opt.id)}
				class="group relative flex cursor-pointer items-center justify-between rounded-[var(--radius-card)] border p-3.5 text-left transition-all {isSelected
					? 'border-[var(--accent)] bg-[var(--bg-surface)] shadow-xs ring-1 ring-[var(--accent)]/30'
					: 'border-[var(--border)] bg-[var(--bg-panel)] hover:border-[var(--border-subtle)] hover:bg-[var(--bg-hover)]'}"
			>
				<div class="flex min-w-0 items-start gap-3.5 pr-3">
					<!-- Icon Box -->
					<div
						class="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors {isSelected
							? 'bg-[var(--accent)] text-white shadow-xs'
							: 'border border-[var(--border-subtle)] bg-[rgba(255,255,255,0.05)] text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]'}"
					>
						<Icon size={18} weight={isSelected ? 'fill' : 'regular'} />
					</div>

					<!-- Text Block -->
					<div class="flex min-w-0 flex-col gap-1">
						<div class="flex flex-wrap items-center gap-2">
							<span class="text-sm font-semibold text-[var(--text-primary)]">{opt.title}</span>
							{#if opt.badge}
								<span
									class="rounded-full px-2 py-0.5 text-[10px] font-semibold {isSelected
										? 'border border-[var(--accent)]/30 bg-[var(--accent)]/15 text-[var(--accent)]'
										: 'border border-[var(--border-subtle)] bg-[rgba(255,255,255,0.06)] text-[var(--text-tertiary)]'}"
								>
									{opt.badge}
								</span>
							{/if}
						</div>
						<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
							{opt.description}
						</p>
					</div>
				</div>

				<!-- Radio Indicator -->
				<div
					class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors {isSelected
						? 'border-[var(--accent)] bg-[var(--accent)] text-white shadow-xs'
						: 'border-[var(--border)] bg-transparent group-hover:border-[var(--border-hover)]'}"
				>
					{#if isSelected}
						<Check size={12} weight="bold" />
					{/if}
				</div>
			</button>
		{/each}
	</div>
</div>
