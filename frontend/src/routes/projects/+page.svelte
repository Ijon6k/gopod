<script lang="ts">
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { projects, getProjectServices, getProjectDomains } from '$lib/data';
	import { formatMemory } from '$lib/utils/format';
	import { Plus, ArrowRight } from 'phosphor-svelte';

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

<div class="w-full">
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

	<!-- Toolbar -->
	<div class="mb-3 flex items-center justify-between gap-4 text-xs text-[var(--text-tertiary)]">
		<SearchInput bind:value={search} placeholder="Search projects" class="w-[min(330px,100%)]" />
		<span>{filtered.length} {filtered.length === 1 ? 'project' : 'projects'}</span>
	</div>

	<!-- Project registry -->
	<div class="border-t border-[var(--border)]">
		<!-- Header row -->
		<div
			class="grid grid-cols-[minmax(260px,2.2fr)_minmax(100px,0.8fr)_80px_80px_minmax(160px,1fr)_20px] items-center gap-[18px] px-0.5 py-2.5 text-[10px] font-medium tracking-[0.09em] text-[var(--text-tertiary)] uppercase max-[760px]:hidden"
			aria-hidden="true"
		>
			<span>Project</span><span>Status</span><span>Services</span><span>Domains</span><span
				>Resources</span
			><span></span>
		</div>

		<!-- Rows -->
		{#each filtered as project (project.id)}
			{@const projServices = getProjectServices(project.id)}
			{@const svcCount = projServices.length}
			{@const domCount = getProjectDomains(project.id).length}
			{@const cpuTotal = projServices.reduce((sum, s) => sum + (s.cpu || 0), 0) || project.cpu || 0}
			{@const memTotal =
				projServices.reduce((sum, s) => sum + (s.memory || 0), 0) || project.memory || 0}
			{@const memory = formatMemory(memTotal)}
			<button
				onclick={() => goto(`/projects/${project.id}`)}
				class="group grid min-h-[74px] w-full cursor-pointer grid-cols-[minmax(260px,2.2fr)_minmax(100px,0.8fr)_80px_80px_minmax(160px,1fr)_20px] items-center gap-[18px] border-0 border-t border-[var(--border-subtle)] bg-transparent px-0.5 py-[11px] text-left text-xs font-[var(--font-sans)] text-[var(--text-secondary)] transition-all duration-150 hover:bg-[var(--bg-panel)] hover:px-2 max-[760px]:min-h-[70px] max-[760px]:grid-cols-[minmax(0,1fr)_auto_18px] max-[760px]:gap-3"
			>
				<span class="flex min-w-0 flex-col gap-[3px]">
					<strong class="text-[16px] font-medium text-[var(--text-primary)]">{project.name}</strong>
					<small
						class="overflow-hidden text-sm text-ellipsis whitespace-nowrap text-[var(--text-tertiary)]"
					>
						{project.description || 'No description provided'}
					</small>
				</span>
				<StatusBadge status={project.status || 'healthy'} size="sm" />
				<span class="max-[760px]:hidden">{svcCount}</span>
				<span class="max-[760px]:hidden">{domCount}</span>
				<span class="whitespace-nowrap text-[var(--text-tertiary)] tabular-nums max-[760px]:hidden">
					<b class="font-medium text-[var(--text-secondary)]">{cpuTotal.toFixed(1)}%</b> CPU · {memory}
				</span>
				<span
					class="text-[var(--text-tertiary)] opacity-0 transition-all duration-150 group-hover:translate-x-0.5 group-hover:opacity-100 max-[760px]:opacity-100"
				>
					<ArrowRight size={15} />
				</span>
			</button>
		{/each}

		{#if filtered.length === 0}
			<div class="flex flex-col gap-1 py-9 text-xs text-[var(--text-tertiary)]">
				<strong class="font-medium text-[var(--text-secondary)]">
					No projects match "{search}".
				</strong>
				<span>Clear the search to see every project.</span>
			</div>
		{/if}
	</div>
</div>
