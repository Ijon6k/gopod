<script lang="ts">
	import { goto } from '$app/navigation';
	import { PageHeader, StatusBadge, SearchInput } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import { projects, getProjectServices, getProjectDomains } from '$lib/data';
	import { formatMemory } from '$lib/utils/format';
	import { cn } from '$lib/utils/cn';
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
	<PageHeader title="Projects" subtitle="Logical workspaces for the services you deploy and manage.">
		{#snippet meta()}
			<span class="text-xs text-[var(--text-tertiary)] ml-0.5">{projects.length} active</span>
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
	<div class="flex items-center justify-between gap-4 mb-3 text-[var(--text-tertiary)] text-xs">
		<SearchInput bind:value={search} placeholder="Search projects" class="w-[min(330px,100%)]" />
		<span>{filtered.length} {filtered.length === 1 ? 'project' : 'projects'}</span>
	</div>

	<!-- Project registry -->
	<div class="border-t border-[var(--border)]">
		<!-- Header row -->
		<div
			class="grid grid-cols-[minmax(260px,2.2fr)_minmax(100px,0.8fr)_80px_80px_minmax(160px,1fr)_20px] gap-[18px] items-center px-0.5 py-2.5 text-[var(--text-tertiary)] text-[10px] font-medium tracking-[0.09em] uppercase max-[760px]:hidden"
			aria-hidden="true"
		>
			<span>Project</span><span>Status</span><span>Services</span><span>Domains</span><span
				>Resources</span
			><span></span>
		</div>

		<!-- Rows -->
		{#each filtered as project}
			{@const projServices = getProjectServices(project.id)}
			{@const svcCount = projServices.length}
			{@const domCount = getProjectDomains(project.id).length}
			{@const cpuTotal = projServices.reduce((sum, s) => sum + (s.cpu || 0), 0) || (project.cpu || 0)}
			{@const memTotal = projServices.reduce((sum, s) => sum + (s.memory || 0), 0) || (project.memory || 0)}
			{@const memory = formatMemory(memTotal)}
			<button
				onclick={() => goto(`/projects/${project.id}`)}
				class="group w-full grid grid-cols-[minmax(260px,2.2fr)_minmax(100px,0.8fr)_80px_80px_minmax(160px,1fr)_20px] gap-[18px] items-center min-h-[74px] px-0.5 py-[11px] border-0 border-t border-[var(--border-subtle)] bg-transparent text-[var(--text-secondary)] text-xs font-[var(--font-sans)] text-left cursor-pointer transition-all duration-150 hover:bg-[var(--bg-panel)] hover:px-2 max-[760px]:grid-cols-[minmax(0,1fr)_auto_18px] max-[760px]:min-h-[70px] max-[760px]:gap-3"
			>
				<span class="flex flex-col gap-[3px] min-w-0">
					<strong class="text-[16px] font-medium text-[var(--text-primary)]">{project.name}</strong>
					<small
						class="text-[var(--text-tertiary)] text-sm overflow-hidden text-ellipsis whitespace-nowrap"
					>
						{project.description || 'No description provided'}
					</small>
				</span>
				<StatusBadge status={project.status || 'healthy'} size="sm" />
				<span class="max-[760px]:hidden">{svcCount}</span>
				<span class="max-[760px]:hidden">{domCount}</span>
				<span
					class="text-[var(--text-tertiary)] whitespace-nowrap tabular-nums max-[760px]:hidden"
				>
					<b class="text-[var(--text-secondary)] font-medium">{cpuTotal.toFixed(1)}%</b> CPU · {memory}
				</span>
				<span
					class="text-[var(--text-tertiary)] opacity-0 transition-all duration-150 group-hover:opacity-100 group-hover:translate-x-0.5 max-[760px]:opacity-100"
				>
					<ArrowRight size={15} />
				</span>
			</button>
		{/each}

		{#if filtered.length === 0}
			<div class="flex flex-col gap-1 py-9 text-[var(--text-tertiary)] text-xs">
				<strong class="text-[var(--text-secondary)] font-medium">
					No projects match "{search}".
				</strong>
				<span>Clear the search to see every project.</span>
			</div>
		{/if}
	</div>
</div>
