<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { projects, services } from '$lib/data';
	import { Input, Kbd } from '$lib/components/primitives';
	import { ui } from '$lib/stores/ui.svelte';
	import { MagnifyingGlass, Plus, SidebarSimple, List, CaretDown, Check } from 'phosphor-svelte';

	let search = $state('');
	let projectMenuOpen = $state(false);
	let serviceMenuOpen = $state(false);

	function handleClickOutside(e: MouseEvent) {
		const target = e.target as HTMLElement;
		if (!target.closest('.dropdown-container')) {
			projectMenuOpen = false;
			serviceMenuOpen = false;
		}
	}

	// Breadcrumb logic
	const labels: Record<string, string[]> = {
		'/': ['Home'],
		'/projects': ['Projects'],
		'/projects/new': ['Projects', 'New project'],
		'/projects/new-service': ['Projects', 'New service'],
		'/runtime': ['Runtime'],
		'/networking': ['Networking'],
		'/deployments': ['Deployments'],
		'/monitoring': ['Monitoring'],
		'/servers': ['Servers'],
		'/topology': ['Topology'],
		'/audit-log': ['Audit Log'],
		'/settings': ['Settings']
	};

	let crumbs = $derived.by(() => {
		const pathname = page.url.pathname;
		const parts = pathname.split('/').filter(Boolean);
		const projectId =
			parts[0] === 'projects' && !['new', 'new-service'].includes(parts[1] ?? '')
				? parts[1]
				: undefined;
		const project = projects.find((p) => p.id === projectId);
		const serviceId = parts[2] === 'services' ? parts[3] : undefined;
		const service = services.find((s) => s.id === serviceId);

		if (labels[pathname])
			return { items: labels[pathname], project: undefined, service: undefined };

		if (project) {
			const items = [
				'Projects',
				project.name,
				...(parts[2] === 'new-service' ? ['New service'] : service ? [service.name] : [])
			];
			return { items, project, service };
		}

		return { items: parts.length ? [parts[0]] : ['Home'], project: undefined, service: undefined };
	});

	let projectServices = $derived(
		crumbs.project ? services.filter((s) => s.projectId === crumbs.project?.id) : []
	);
</script>

<svelte:window onclick={handleClickOutside} />

<div
	class="flex h-[var(--topbar-height)] shrink-0 items-center gap-2.5 border-b border-[var(--border)] bg-[var(--bg-shell)] px-3.5"
>
	<!-- Mobile Burger Button -->
	<button
		aria-label="Open navigation"
		onclick={() => ui.toggleSidebar()}
		class="-ml-1 flex cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] md:hidden"
	>
		<List size={20} weight="bold" />
	</button>

	<!-- Desktop Sidebar Toggle Button -->
	<button
		aria-label="Toggle sidebar"
		title={ui.sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
		onclick={() => ui.toggleSidebar()}
		class="hidden cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] md:flex"
	>
		<SidebarSimple size={18} />
	</button>

	<!-- Breadcrumbs -->
	<div class="flex min-w-0 flex-1 items-center gap-1.5">
		{#each crumbs.items as crumb, index}
			<span
				class={cn(
					'flex items-center gap-1.5 text-base whitespace-nowrap text-[var(--text-tertiary)]',
					index === crumbs.items.length - 1 && 'font-medium text-[var(--text-primary)]'
				)}
			>
				{#if index > 0}
					<i class="text-sm text-[var(--text-tertiary)] not-italic">/</i>
				{/if}
				{#if crumbs.project && crumb === crumbs.project.name}
					<span class="dropdown-container relative">
						<button
							onclick={() => {
								projectMenuOpen = !projectMenuOpen;
								serviceMenuOpen = false;
							}}
							aria-expanded={projectMenuOpen}
							class="flex cursor-pointer items-center gap-[3px] border-0 bg-transparent px-1 py-[3px] text-base font-[var(--font-sans)] font-medium text-[var(--text-primary)] transition-colors hover:text-[var(--accent)]"
						>
							{crumbs.project.name}
							<CaretDown size={13} />
						</button>
						{#if projectMenuOpen}
							<span
								class="absolute top-[calc(100%+7px)] left-[-4px] z-70 w-[196px] rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1 shadow-lg"
							>
								{#each projects as item}
									<button
										onclick={() => {
											projectMenuOpen = false;
											goto(`/projects/${item.id}`);
										}}
										class="flex w-full cursor-pointer items-center justify-between gap-2 rounded border-0 bg-transparent px-2 py-[7px] text-left text-xs font-[var(--font-sans)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
									>
										<span>{item.name}</span>
										{#if item.id === crumbs.project?.id}
											<Check size={13} class="shrink-0 text-[var(--accent)]" />
										{/if}
									</button>
								{/each}
								<hr class="mx-0.5 my-1 h-px border-0 bg-[var(--border-subtle)]" />
								<button
									onclick={() => {
										projectMenuOpen = false;
										goto('/projects');
									}}
									class="flex w-full cursor-pointer items-center gap-2 rounded border-0 bg-transparent px-2 py-[7px] text-left text-xs font-[var(--font-sans)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
								>
									All projects
								</button>
								<button
									onclick={() => {
										projectMenuOpen = false;
										goto('/projects/new');
									}}
									class="flex w-full cursor-pointer items-center gap-2 rounded border-0 bg-transparent px-2 py-[7px] text-left text-xs font-[var(--font-sans)] text-[var(--accent)] hover:bg-[var(--bg-hover)]"
								>
									<Plus size={13} />
									New project
								</button>
							</span>
						{/if}
					</span>
				{:else if crumbs.service && crumb === crumbs.service.name}
					<span class="dropdown-container relative">
						<button
							onclick={() => {
								serviceMenuOpen = !serviceMenuOpen;
								projectMenuOpen = false;
							}}
							aria-expanded={serviceMenuOpen}
							class="flex cursor-pointer items-center gap-[3px] border-0 bg-transparent px-1 py-[3px] text-base font-[var(--font-sans)] font-medium text-[var(--text-primary)] transition-colors hover:text-[var(--accent)]"
						>
							{crumbs.service.name}
							<CaretDown size={13} />
						</button>
						{#if serviceMenuOpen}
							<span
								class="absolute top-[calc(100%+7px)] left-[-4px] z-70 w-[210px] rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1 shadow-lg"
							>
								{#each projectServices as item}
									<button
										onclick={() => {
											serviceMenuOpen = false;
											goto(`/projects/${crumbs.project?.id}/services/${item.id}`);
										}}
										class="flex w-full cursor-pointer items-center justify-between gap-2 rounded border-0 bg-transparent px-2 py-[7px] text-left text-xs font-[var(--font-sans)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
									>
										<div class="flex items-center gap-1.5 truncate">
											<span class="truncate">{item.name}</span>
											<span
												class="rounded border border-[var(--border-subtle)] bg-[var(--bg-panel)] px-1 font-mono text-[10px] text-[var(--text-tertiary)]"
											>
												{item.type}
											</span>
										</div>
										{#if item.id === crumbs.service?.id}
											<Check size={13} class="shrink-0 text-[var(--accent)]" />
										{/if}
									</button>
								{/each}
								{#if projectServices.length === 0}
									<div class="px-2 py-1.5 font-mono text-xs text-[var(--text-tertiary)]">
										No services in project
									</div>
								{/if}
								<hr class="mx-0.5 my-1 h-px border-0 bg-[var(--border-subtle)]" />
								<button
									onclick={() => {
										serviceMenuOpen = false;
										goto(`/projects/${crumbs.project?.id}/new-service`);
									}}
									class="flex w-full cursor-pointer items-center gap-2 rounded border-0 bg-transparent px-2 py-[7px] text-left text-xs font-[var(--font-sans)] text-[var(--accent)] hover:bg-[var(--bg-hover)]"
								>
									<Plus size={13} />
									New service
								</button>
							</span>
						{/if}
					</span>
				{:else}
					<span>{crumb}</span>
				{/if}
			</span>
		{/each}
	</div>

	<!-- Search only on the right -->
	<label
		class="flex w-[140px] shrink-0 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-[5px] text-[var(--text-tertiary)] sm:w-[180px] md:w-[220px]"
	>
		<MagnifyingGlass size={13} class="shrink-0" />
		<span class="sr-only">Search</span>
		<Input bind:value={search} placeholder="Search…" class="text-xs" />
		<Kbd class="hidden sm:inline-flex">⌘K</Kbd>
	</label>
</div>
