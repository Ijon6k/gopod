<script lang="ts">
	import { onMount } from 'svelte';
	import { cn } from '$lib/utils/cn';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { projects, services } from '$lib/data';
	import { dataStore } from '$lib/stores/data.svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { ui } from '$lib/stores/ui.svelte';
	import { DropdownMenu } from 'kumo-svelte';
	import { Kbd } from '$lib/components/ui';
	import CommandPaletteModal from './CommandPaletteModal.svelte';
	import { MagnifyingGlass, Plus, CaretDown, Sun, Moon, GearSix, SignOut } from 'phosphor-svelte';

	let openCommandPalette = $state(false);
	let isMac = $state(false);

	onMount(() => {
		isMac =
			typeof navigator !== 'undefined' &&
			/(Mac|iPhone|iPod|iPad)/i.test(navigator.userAgent || navigator.platform || '');
	});

	function handleKeydown(e: KeyboardEvent) {
		if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
			e.preventDefault();
			openCommandPalette = true;
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
		'/audit-log': ['Audit log'],
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

	let podmanStatus = $derived(dataStore.server?.status ?? 'online');
	let podmanVersion = $derived(dataStore.server?.podmanVersion ?? '5.0');
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- Level 1: Global Context - Sticky Docked Topbar (Flush Top) -->
<header
	class="flex h-12 w-full shrink-0 items-center justify-between border-b border-[var(--border)] bg-[var(--bg-panel)] px-3 text-xs md:px-4"
>
	<!-- Left section: Context Switcher & Breadcrumbs -->
	<div class="flex min-w-0 items-center gap-2 sm:gap-3">
		<!-- Breadcrumbs & Dynamic Switcher with Kumo DropdownMenu -->
		<nav class="flex min-w-0 items-center gap-1 overflow-hidden" aria-label="Breadcrumbs">
			{#each crumbs.items as crumb, index}
				<span
					class={cn(
						'flex items-center gap-1 text-[13px] whitespace-nowrap text-[var(--text-tertiary)]',
						index === crumbs.items.length - 1 && 'font-medium text-[var(--text-primary)]'
					)}
				>
					{#if index > 0}
						<span class="text-[var(--text-muted)] select-none">/</span>
					{/if}

					{#if crumbs.project && crumb === crumbs.project.name}
						<DropdownMenu>
							<DropdownMenu.Trigger
								class="flex cursor-pointer items-center gap-1 rounded border-0 bg-transparent px-1 py-0.5 text-[13px] font-medium text-[var(--text-primary)] outline-none hover:bg-[var(--bg-hover)] hover:text-[var(--accent)]"
							>
								<span class="max-w-[120px] truncate sm:max-w-[160px]">{crumbs.project.name}</span>
								<CaretDown size={11} class="text-[var(--text-tertiary)]" />
							</DropdownMenu.Trigger>
							<DropdownMenu.Content class="w-48">
								{#each projects as item}
									<DropdownMenu.Item
										onclick={() => goto(`/projects/${item.id}`)}
										selected={item.id === crumbs.project?.id}
									>
										<span class="truncate text-xs">{item.name}</span>
									</DropdownMenu.Item>
								{/each}
								<DropdownMenu.Separator />
								<DropdownMenu.Item onclick={() => goto('/projects')}>
									<span class="text-xs">All projects</span>
								</DropdownMenu.Item>
								<DropdownMenu.Item onclick={() => goto('/projects/new')}>
									<span class="flex items-center gap-1.5 text-xs font-medium text-[var(--accent)]">
										<Plus size={12} /> New project
									</span>
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu>
					{:else if crumbs.service && crumb === crumbs.service.name}
						<DropdownMenu>
							<DropdownMenu.Trigger
								class="flex cursor-pointer items-center gap-1 rounded border-0 bg-transparent px-1 py-0.5 text-[13px] font-medium text-[var(--text-primary)] outline-none hover:bg-[var(--bg-hover)] hover:text-[var(--accent)]"
							>
								<span class="max-w-[120px] truncate sm:max-w-[160px]">{crumbs.service.name}</span>
								<CaretDown size={11} class="text-[var(--text-tertiary)]" />
							</DropdownMenu.Trigger>
							<DropdownMenu.Content class="w-52">
								{#each projectServices as item}
									<DropdownMenu.Item
										onclick={() => goto(`/projects/${crumbs.project?.id}/services/${item.id}`)}
										selected={item.id === crumbs.service?.id}
									>
										<div class="flex items-center gap-1.5 truncate text-xs">
											<span class="truncate">{item.name}</span>
											<span
												class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1 font-mono text-[10px] text-[var(--text-muted)]"
											>
												{item.type}
											</span>
										</div>
									</DropdownMenu.Item>
								{/each}
								{#if projectServices.length === 0}
									<div class="px-2 py-1.5 text-xs text-[var(--text-muted)]">
										No services in project
									</div>
								{/if}
								<DropdownMenu.Separator />
								<DropdownMenu.Item
									onclick={() => goto(`/projects/${crumbs.project?.id}/new-service`)}
								>
									<span class="flex items-center gap-1.5 text-xs font-medium text-[var(--accent)]">
										<Plus size={12} /> New service
									</span>
								</DropdownMenu.Item>
							</DropdownMenu.Content>
						</DropdownMenu>
					{:else}
						<span class="truncate">{crumb}</span>
					{/if}
				</span>
			{/each}
		</nav>
	</div>

	<!-- Right section: Quick Search (Cmd+K), Engine Status, Theme Toggle & User Profile -->
	<div class="flex items-center gap-2">
		<!-- Podman Engine Status Pill Badge -->
		<div
			class="hidden shrink-0 items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[11px] text-[var(--text-secondary)] select-none lg:flex"
			title="Podman local engine status"
		>
			<span
				class={cn(
					'h-1.5 w-1.5 rounded-full',
					podmanStatus === 'online' ? 'bg-[var(--status-green)]' : 'bg-[var(--status-red)]'
				)}
			></span>
			<span class="font-medium text-[var(--text-primary)]">Podman</span>
			<span class="text-[var(--text-muted)]">v{podmanVersion}</span>
		</div>

		<!-- Unified Search Bar & Command Palette Trigger (Cmd+K / Ctrl+K) -->
		<button
			type="button"
			onclick={() => (openCommandPalette = true)}
			title="Search GOPOD ({isMac ? '⌘K' : 'Ctrl K'})"
			aria-label="Search GOPOD"
			class="group hover:border-kumo-focus flex h-7.5 w-[130px] cursor-pointer items-center justify-between rounded-md border border-kumo-line bg-kumo-control px-2.5 text-xs text-kumo-subtle shadow-xs transition-colors hover:bg-kumo-base hover:text-kumo-default sm:w-[170px] md:w-[210px]"
		>
			<div class="flex items-center gap-2 truncate">
				<MagnifyingGlass
					size={13}
					class="shrink-0 text-kumo-muted transition-colors group-hover:text-kumo-default"
				/>
				<span class="truncate text-xs font-normal">Search…</span>
			</div>
			<Kbd
				class="shrink-0 border-kumo-hairline bg-kumo-recessed text-[10px] text-kumo-subtle transition-colors group-hover:border-kumo-line group-hover:text-kumo-default"
			>
				{isMac ? '⌘K' : 'Ctrl K'}
			</Kbd>
		</button>

		<!-- Theme Toggle Button -->
		<button
			type="button"
			onclick={() => ui.toggleTheme()}
			title="Switch to {ui.theme === 'dark' ? 'Light' : 'Dark'} mode"
			aria-label="Toggle color theme"
			class="flex h-7 w-7 cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
		>
			{#if ui.theme === 'dark'}
				<Sun size={14} />
			{:else}
				<Moon size={14} />
			{/if}
		</button>

		<!-- User Profile Dropdown using Kumo DropdownMenu -->
		<DropdownMenu>
			<DropdownMenu.Trigger
				class="flex cursor-pointer items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--bg-surface)] p-0.5 pr-1.5 outline-none hover:border-[var(--accent)]"
				title="Account profile"
			>
				<div
					class="flex h-6 w-6 items-center justify-center rounded-full bg-[var(--accent-muted)] text-[11px] font-semibold text-[var(--accent)]"
				>
					{authStore.user?.name ? authStore.user.name.charAt(0).toUpperCase() : 'A'}
				</div>
				<span
					class="hidden max-w-[80px] truncate text-xs font-medium text-[var(--text-primary)] sm:inline"
				>
					{authStore.user?.name || 'Admin'}
				</span>
				<CaretDown size={10} class="text-[var(--text-tertiary)]" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content class="w-52" align="end">
				<div class="mb-1 border-b border-[var(--border-subtle)] px-2 py-1.5">
					<p class="truncate text-xs font-semibold text-[var(--text-primary)]">
						{authStore.user?.name || 'Administrator'}
					</p>
					<p class="truncate text-[11px] text-[var(--text-tertiary)]">
						{authStore.user?.email || 'admin@gopod.dev'}
					</p>
				</div>
				<DropdownMenu.Item onclick={() => goto('/settings')} icon={GearSix}>
					<span class="text-xs">Settings</span>
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<DropdownMenu.Item
					variant="danger"
					icon={SignOut}
					onclick={async () => {
						await authStore.logout();
						goto('/login');
					}}
				>
					<span class="text-xs">Sign out</span>
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu>
	</div>
</header>

<CommandPaletteModal bind:open={openCommandPalette} />
