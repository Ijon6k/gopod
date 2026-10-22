<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { Button, Empty, LayerCard } from 'kumo-svelte';
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
	<div class="mb-4 flex min-h-[26px] items-center justify-between gap-4">
		<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">Projects</h2>
		{#if projects.length > 0}
			<button
				onclick={() => goto('/projects')}
				class="inline-flex cursor-pointer items-center border-0 bg-transparent p-0 text-[13px] font-[var(--font-sans)] whitespace-nowrap text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
			>
				View all
			</button>
		{/if}
	</div>

	{#if projects.length === 0}
		<Empty
			size="sm"
			title="No projects yet"
			description="Create a project to group services, deployments, and domains."
		>
			{#snippet contents()}
				<Button variant="primary" size="sm" onclick={() => goto('/projects/new')}>New project</Button>
			{/snippet}
		</Empty>
	{:else}
		<div class="flex flex-col divide-y divide-[var(--border-subtle)]">
			{#each projects as project (project.id)}
				{@const serviceCount = getProjectServices(project.id).length}
				{@const domainCount = getProjectDomains(project.id).length}
				{@const memory = formatMemory(project.memory)}
				<button
					onclick={() => goto(`/projects/${project.id}`)}
					class="-mx-2 flex min-h-[48px] cursor-pointer items-center gap-3.5 rounded-[var(--radius-sm)] border-0 bg-transparent px-2 py-2 text-left font-[var(--font-sans)] text-[var(--text-primary)] hover:bg-[var(--bg-hover)]"
				>
					<span class="flex min-w-0 flex-1 flex-col gap-0.5">
						<strong class="text-[13px] font-medium">{project.name}</strong>
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
				</button>
			{/each}
		</div>
	{/if}
</LayerCard>
