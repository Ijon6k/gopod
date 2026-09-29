<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { projects, services } from '$lib/data';
	import { Input, Kbd } from '$lib/components/primitives';
	import { ui } from '$lib/stores/ui.svelte';
	import {
		MagnifyingGlass,
		Plus,
		SidebarSimple,
		List,
		CaretDown,
		Check
	} from 'phosphor-svelte';

	let search = $state('');
	let projectMenuOpen = $state(false);

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
		const service = services.find((s) => s.id === parts[3]);

		if (labels[pathname]) return { items: labels[pathname], project: undefined };

		if (project) {
			const items = [
				'Projects',
				project.name,
				...(parts[2] === 'new-service' ? ['New service'] : service ? [service.name] : [])
			];
			return { items, project };
		}

		return { items: parts.length ? [parts[0]] : ['Home'], project: undefined };
	});
</script>

<div
	class="h-[var(--topbar-height)] flex items-center gap-2.5 px-3.5 border-b border-[var(--border)] bg-[var(--bg-shell)] shrink-0"
>
	<!-- Mobile Burger Button -->
	<button
		aria-label="Open navigation"
		onclick={() => ui.toggleSidebar()}
		class="md:hidden border-0 bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] rounded-[var(--radius-sm)] cursor-pointer p-1.5 -ml-1 flex items-center justify-center transition-colors"
	>
		<List size={20} weight="bold" />
	</button>

	<!-- Desktop Sidebar Toggle Button -->
	<button
		aria-label="Toggle sidebar"
		title={ui.sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
		onclick={() => ui.toggleSidebar()}
		class="hidden md:flex border-0 bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] rounded-[var(--radius-sm)] cursor-pointer p-1.5 items-center justify-center transition-colors"
	>
		<SidebarSimple size={18} />
	</button>

	<!-- Breadcrumbs -->
	<div class="flex items-center gap-1.5 flex-1 min-w-0">
		{#each crumbs.items as crumb, index}
			<span
				class={cn(
					'flex items-center gap-1.5 text-[var(--text-tertiary)] text-base whitespace-nowrap',
					index === crumbs.items.length - 1 && 'text-[var(--text-primary)] font-medium'
				)}
			>
				{#if index > 0}
					<i class="not-italic text-[var(--text-tertiary)] text-sm">/</i>
				{/if}
				{#if crumbs.project && crumb === crumbs.project.name}
					<span class="relative">
						<button
							onclick={() => (projectMenuOpen = !projectMenuOpen)}
							aria-expanded={projectMenuOpen}
							class="flex items-center gap-[3px] border-0 bg-transparent text-[var(--text-primary)] font-[var(--font-sans)] font-medium text-base cursor-pointer px-1 py-[3px]"
						>
							{crumbs.project.name}
							<CaretDown size={13} />
						</button>
						{#if projectMenuOpen}
							<span
								class="absolute top-[calc(100%+7px)] left-[-4px] z-70 w-[196px] p-1 bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-sm)]"
							>
								{#each projects as item}
									<button
										onclick={() => {
											projectMenuOpen = false;
											goto(`/projects/${item.id}`);
										}}
										class="flex w-full items-center justify-between gap-2 px-2 py-[7px] border-0 rounded bg-transparent text-[var(--text-secondary)] text-xs font-[var(--font-sans)] text-left cursor-pointer hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
									>
										<span>{item.name}</span>
										{#if item.id === crumbs.project?.id}
											<Check size={13} />
										{/if}
									</button>
								{/each}
								<hr class="h-px my-1 mx-0.5 border-0 bg-[var(--border-subtle)]" />
								<button
									onclick={() => {
										projectMenuOpen = false;
										goto('/projects');
									}}
									class="flex w-full items-center gap-2 px-2 py-[7px] border-0 rounded bg-transparent text-[var(--text-secondary)] text-xs font-[var(--font-sans)] text-left cursor-pointer hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
								>
									All projects
								</button>
								<button
									onclick={() => {
										projectMenuOpen = false;
										goto('/projects/new');
									}}
									class="flex w-full items-center gap-2 px-2 py-[7px] border-0 rounded bg-transparent text-[var(--accent)] text-xs font-[var(--font-sans)] text-left cursor-pointer hover:bg-[var(--bg-hover)]"
								>
									<Plus size={13} />
									New project
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
		class="flex items-center gap-2 w-[140px] sm:w-[180px] md:w-[220px] px-2.5 py-[5px] border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] text-[var(--text-tertiary)] shrink-0"
	>
		<MagnifyingGlass size={13} class="shrink-0" />
		<span class="sr-only">Search</span>
		<Input bind:value={search} placeholder="Search…" class="text-xs" />
		<Kbd class="hidden sm:inline-flex">⌘K</Kbd>
	</label>
</div>
