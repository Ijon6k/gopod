<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
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

<section class={cn(className)}>
	<div class="flex items-center justify-between gap-4 min-h-[22px] mb-4">
		<span class="text-[10px] font-medium text-[var(--text-tertiary)] tracking-[0.1em] uppercase">
			Projects
		</span>
		<button
			onclick={() => goto('/projects')}
			class="inline-flex items-center gap-[5px] p-0 border-0 bg-transparent text-[var(--text-secondary)] text-xs font-[var(--font-sans)] cursor-pointer whitespace-nowrap hover:text-[var(--text-primary)]"
		>
			View all <ArrowRight size={12} />
		</button>
	</div>
	<div class="flex flex-col">
		{#each projects as project}
			{@const serviceCount = getProjectServices(project.id).length}
			{@const domainCount = getProjectDomains(project.id).length}
			{@const memory = formatMemory(project.memory)}
			<button
				onclick={() => goto(`/projects/${project.id}`)}
				class="group flex items-center gap-3.5 min-h-[56px] py-2 px-3 -mx-3 border-0 rounded-[var(--radius-sm)] bg-transparent text-[var(--text-primary)] cursor-pointer text-left font-[var(--font-sans)] hover:bg-[var(--bg-panel)]"
			>
				<span class="flex flex-col gap-[3px] min-w-0 flex-1">
					<strong class="text-base font-medium">{project.name}</strong>
					<small class="text-[var(--text-tertiary)] text-[11.5px]">
						{serviceCount} {pluralize(serviceCount, 'service')} · {domainCount}
						{pluralize(domainCount, 'domain')}
					</small>
				</span>
				<span
					class="text-[var(--text-tertiary)] text-[11px] whitespace-nowrap tabular-nums max-sm:hidden"
				>
					{project.cpu.toFixed(1)}% · {memory}
				</span>
				<StatusBadge status={project.status} size="sm" />
				<span
					class="text-[var(--text-tertiary)] shrink-0 opacity-0 -translate-x-[3px] transition-all duration-150 group-hover:opacity-100 group-hover:translate-x-0"
				>
					<CaretRight size={14} />
				</span>
			</button>
		{/each}
	</div>
</section>
