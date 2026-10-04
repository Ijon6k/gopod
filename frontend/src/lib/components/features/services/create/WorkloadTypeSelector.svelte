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
			description: 'Deploy web apps, APIs, or static sites from Git repository, Docker Image, or Drag & Drop.',
			icon: GitBranch
		},
		{
			id: 'quadlet' as const,
			title: 'Quadlet (Systemd)',
			description: 'Declarative systemd .container unit with auto-restart on host boot & native systemd supervision.',
			icon: FileText
		},
		{
			id: 'compose' as const,
			title: 'Stack / Compose',
			description: 'Multi-container workloads defined via Compose YAML or Kubernetes Pod manifests.',
			icon: Stack
		},
		{
			id: 'database' as const,
			title: 'Database',
			description: '1-click deployment for PostgreSQL, Redis, MySQL, or MongoDB with persistent volume storage.',
			badge: 'Preset',
			icon: Database
		}
	];
</script>

<div class="flex flex-col gap-2">
	<span class="text-xs font-medium text-[var(--text-secondary)]">
		Select Service Type
	</span>

	<!-- Vertical Stack Layout (Spacious, Clear Hierarchy, No Cramped Columns) -->
	<div class="flex flex-col gap-2.5">
		{#each options as opt}
			{@const Icon = opt.icon}
			{@const isSelected = selected === opt.id}
			<button
				type="button"
				onclick={() => onselect(opt.id)}
				class="group relative flex items-center justify-between p-3.5 rounded-[var(--radius-card)] border transition-all cursor-pointer text-left {isSelected
					? 'bg-[var(--bg-surface)] border-[var(--accent)] shadow-xs ring-1 ring-[var(--accent)]/30'
					: 'bg-[var(--bg-panel)] border-[var(--border)] hover:border-[var(--border-subtle)] hover:bg-[var(--bg-hover)]'}"
			>
				<div class="flex items-start gap-3.5 min-w-0 pr-3">
					<!-- Icon Box -->
					<div
						class="flex items-center justify-center w-9 h-9 rounded-lg shrink-0 mt-0.5 transition-colors {isSelected
							? 'bg-[var(--accent)] text-white shadow-xs'
							: 'bg-[rgba(255,255,255,0.05)] text-[var(--text-secondary)] group-hover:text-[var(--text-primary)] border border-[var(--border-subtle)]'}"
					>
						<Icon size={18} weight={isSelected ? 'fill' : 'regular'} />
					</div>

					<!-- Text Block -->
					<div class="flex flex-col gap-1 min-w-0">
						<div class="flex items-center gap-2 flex-wrap">
							<span class="text-sm font-semibold text-[var(--text-primary)]">{opt.title}</span>
							{#if opt.badge}
								<span
									class="text-[10px] font-semibold px-2 py-0.5 rounded-full {isSelected
										? 'bg-[var(--accent)]/15 text-[var(--accent)] border border-[var(--accent)]/30'
										: 'bg-[rgba(255,255,255,0.06)] text-[var(--text-tertiary)] border border-[var(--border-subtle)]'}"
								>
									{opt.badge}
								</span>
							{/if}
						</div>
						<p class="text-xs text-[var(--text-tertiary)] m-0 leading-relaxed">
							{opt.description}
						</p>
					</div>
				</div>

				<!-- Radio Indicator -->
				<div
					class="flex items-center justify-center w-5 h-5 rounded-full border shrink-0 transition-colors {isSelected
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
