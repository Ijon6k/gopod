<script lang="ts">
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/ui';
	import { LayerCard, Empty } from 'kumo-svelte';
	import { projects, getProjectServices, getProjectDomains } from '$lib/data';
	import { formatMemory, pluralize } from '$lib/utils/format';
	import { Plus, ArrowRight, FolderOpen } from 'phosphor-svelte';

	let search = $state('');

	let filtered = $derived(
		projects.filter((p) =>
			`${p.name ?? ''} ${p.description ?? ''}`.toLowerCase().includes(search.toLowerCase())
		)
	);
</script>

<svelte:head>
	<title>Projects — GOPOD</title>
</svelte:head>

<div class="flex w-full flex-col gap-6">
	<PageHeader
		title="Projects"
		subtitle="Logical workspaces for the services you deploy and manage."
	>
		{#snippet meta()}
			<span class="ml-0.5 text-xs text-[var(--text-tertiary)]">{projects.length} active</span>
		{/snippet}
		{#snippet actions()}
			<Button variant="secondary" size="sm" onclick={() => goto('/projects/new')}>
				<Plus size={13} /> New project
			</Button>
			<Button variant="primary" size="sm" onclick={() => goto('/projects/new-service')}>
				<Plus size={13} /> New service
			</Button>
		{/snippet}
	</PageHeader>

	<!-- Search & Meta Toolbar -->
	<div class="flex items-center justify-between gap-4 text-xs text-[var(--text-tertiary)]">
		<SearchInput bind:value={search} placeholder="Search projects…" class="w-[min(320px,100%)]" />
		<span>{filtered.length} {filtered.length === 1 ? 'project' : 'projects'}</span>
	</div>

	<!-- Project List using Kumo LayerCard -->
	{#if filtered.length > 0}
		<div class="flex flex-col gap-3">
			{#each filtered as project (project.id)}
				{@const svcCount = project.serviceCount ?? (project.services?.length || getProjectServices(project.id).length)}
				{@const domCount = project.domainCount ?? (project.domains?.length || getProjectDomains(project.id).length)}
				{@const cpuTotal = project.cpu || 0}
				{@const memTotal = project.memory || 0}
				{@const memory = formatMemory(memTotal)}

				<LayerCard
					class="group cursor-pointer p-4 transition-all hover:border-[var(--accent)] hover:shadow-xs"
					onclick={() => goto(`/projects/${project.id}`)}
				>
					<div class="flex items-center justify-between gap-4">
						<div class="flex min-w-0 flex-1 items-center gap-3.5">
							<div
								class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-[var(--color-kumo-fill)] text-[var(--accent)]"
							>
								<FolderOpen size={18} />
							</div>
							<div class="flex min-w-0 flex-col gap-0.5">
								<div class="flex items-center gap-2.5">
									<strong class="text-sm font-semibold text-[var(--text-primary)]">
										{project.name}
									</strong>
									<StatusBadge status={project.status || 'healthy'} size="sm" />
								</div>
								<p class="m-0 truncate text-xs text-[var(--text-tertiary)]">
									{project.description || 'No description provided'}
								</p>
							</div>
						</div>

						<div class="flex shrink-0 items-center gap-4 text-xs text-[var(--text-secondary)]">
							<div class="hidden sm:flex items-center gap-3 text-[var(--text-tertiary)]">
								<span class="rounded bg-[var(--color-kumo-fill)] px-2 py-0.5 font-mono text-[11px]">
									{svcCount} {pluralize(svcCount, 'service')}
								</span>
								<span class="rounded bg-[var(--color-kumo-fill)] px-2 py-0.5 font-mono text-[11px]">
									{domCount} {pluralize(domCount, 'domain')}
								</span>
								<span class="font-mono text-[11px] tabular-nums">
									{cpuTotal.toFixed(1)}% CPU · {memory}
								</span>
							</div>

							<span
								class="text-[var(--text-tertiary)] transition-transform duration-150 group-hover:translate-x-1 group-hover:text-[var(--text-primary)]"
							>
								<ArrowRight size={15} />
							</span>
						</div>
					</div>
				</LayerCard>
			{/each}
		</div>
	{:else}
		<Empty
			title={`No projects match "${search}"`}
			description="Clear your search term or create a new project to get started."
		>
			<Button variant="secondary" size="sm" onclick={() => (search = '')}>
				Clear search
			</Button>
		</Empty>
	{/if}
</div>
