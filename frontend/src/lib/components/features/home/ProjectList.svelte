<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { LayerCard } from 'kumo-svelte';
	import { ArrowRight, CaretRight } from 'phosphor-svelte';
	import { getProjectServices, getProjectDomains } from '$lib/data';
	import { formatMemory, pluralize } from '$lib/utils/format';
	import type { Project } from '$lib/types';

	interface Props {
		projects: Project[];
		class?: string;
	}

	let { projects, class: className = '' }: Props = $props();
</script>

<LayerCard class={cn('flex flex-col p-5', className)}>
	<div class="mb-4 flex min-h-[22px] items-center justify-between gap-4">
		<span class="text-xs font-medium text-[var(--text-tertiary)]"> Projects </span>
		<button
			onclick={() => goto('/projects')}
			class="inline-flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 text-xs font-[var(--font-sans)] whitespace-nowrap text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
		>
			View all <ArrowRight size={12} />
		</button>
	</div>
	<div class="flex flex-col divide-y divide-[var(--border-subtle)]">
		{#each projects as project}
			{@const serviceCount = getProjectServices(project.id).length}
			{@const domainCount = getProjectDomains(project.id).length}
			{@const memory = formatMemory(project.memory)}
			<button
				onclick={() => goto(`/projects/${project.id}`)}
				class="group -mx-2 flex min-h-[50px] cursor-pointer items-center gap-3.5 rounded-[var(--radius-sm)] border-0 bg-transparent px-2 py-2 text-left font-[var(--font-sans)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]"
			>
				<span class="flex min-w-0 flex-1 flex-col gap-0.5">
					<strong class="text-sm font-medium">{project.name}</strong>
					<small class="text-xs text-[var(--text-tertiary)]">
						{serviceCount}
						{pluralize(serviceCount, 'service')} · {domainCount}
						{pluralize(domainCount, 'domain')}
					</small>
				</span>
				<span
					class="text-xs whitespace-nowrap text-[var(--text-tertiary)] tabular-nums max-sm:hidden"
				>
					{project.cpu.toFixed(1)}% · {memory}
				</span>
				<StatusBadge status={project.status} size="sm" />
				<span
					class="shrink-0 -translate-x-[3px] text-[var(--text-tertiary)] opacity-0 transition-all duration-150 group-hover:translate-x-0 group-hover:opacity-100"
				>
					<CaretRight size={14} />
				</span>
			</button>
		{/each}
	</div>
</LayerCard>
