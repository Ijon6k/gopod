<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { ui } from '$lib/stores/ui.svelte';
	import {
		SquaresFour,
		FolderOpen,
		Stack,
		Cube,
		ImageSquare,
		Database,
		WifiHigh,
		Globe,
		TreeStructure as NetworkIcon,
		TreeStructure,
		ChartLineUp,
		GitBranch,
		ChartBar,
		HardDrives,
		ShareNetwork,
		GearSix,
		Key,
		ClockCounterClockwise,
		CaretDown,
		SignOut,
		X
	} from 'phosphor-svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import type { NavItem } from '$lib/types';

	interface Props {
		onclose?: () => void;
	}

	let { onclose }: Props = $props();

	let isMobile = $derived(!!onclose);
	let isMinimized = $derived(ui.sidebarCollapsed && !isMobile);

	const nav: NavItem[] = [
		{ label: 'Home', path: '/', icon: 'SquaresFour' },
		{ label: 'Projects', path: '/projects', icon: 'FolderOpen' },
		{
			label: 'Runtime',
			icon: 'Stack',
			children: [
				{ label: 'Containers', path: '/runtime/containers', icon: 'Cube' },
				{ label: 'Pods', path: '/runtime/pods', icon: 'Stack' },
				{ label: 'Images', path: '/runtime/images', icon: 'ImageSquare' },
				{ label: 'Volumes', path: '/runtime/volumes', icon: 'Database' },
				{ label: 'Networks', path: '/runtime/networks', icon: 'WifiHigh' }
			]
		},
		{
			label: 'Networking',
			icon: 'Globe',
			children: [
				{ label: 'Domains', path: '/networking/domains', icon: 'Globe' },
				{ label: 'Traffic & Logs', path: '/networking/requests', icon: 'ChartLineUp' },
				{ label: 'Ports', path: '/networking/ports', icon: 'NetworkIcon' }
			]
		},
		{ label: 'Deployments', path: '/deployments', icon: 'GitBranch' },
		{ label: 'Credentials', path: '/credentials', icon: 'Key' },
		{ label: 'Monitoring', path: '/monitoring', icon: 'ChartBar' },
		{ label: 'Servers', path: '/servers', icon: 'HardDrives' },
		{ label: 'Topology', path: '/topology', icon: 'ShareNetwork' },
		{ label: 'Audit Log', path: '/audit-log', icon: 'ClockCounterClockwise' },
		{ label: 'Settings', path: '/settings', icon: 'GearSix' }
	];

	let collapsed: Record<string, boolean> = $state({});

	const iconMap: Record<string, typeof SquaresFour> = {
		SquaresFour,
		FolderOpen,
		Stack,
		Cube,
		ImageSquare,
		Database,
		WifiHigh,
		Globe,
		NetworkIcon,
		TreeStructure,
		ChartLineUp,
		GitBranch,
		Key,
		ChartBar,
		HardDrives,
		ShareNetwork,
		ClockCounterClockwise,
		GearSix
	};

	function isActive(path: string): boolean {
		const currentPath = page.url.pathname;
		const currentSearch = page.url.search;
		const currentFull = currentPath + currentSearch;

		if (path.includes('?')) {
			return currentFull === path;
		}

		if (path === '/monitoring') {
			return currentPath.startsWith('/monitoring');
		}

		if (path === '/') return currentPath === '/';
		return currentPath.startsWith(path);
	}

	function toggle(label: string) {
		collapsed[label] = !collapsed[label];
	}

	function navigate(path: string) {
		goto(path);
		onclose?.();
	}
</script>

<aside
	class={cn(
		'flex h-full shrink-0 flex-col bg-[var(--bg-outer)] transition-[width] duration-200 select-none',
		isMinimized ? 'w-[58px]' : 'w-[var(--sidebar-width)]'
	)}
>
	<!-- Product mark -->
	<div
		class={cn('flex items-center pt-3 pb-2', isMinimized ? 'justify-center px-1' : 'gap-2.5 px-3')}
	>
		<button
			class="flex h-7 w-7 shrink-0 cursor-pointer items-center justify-center rounded-[7px] border-0 bg-[var(--accent)] p-0"
			onclick={() => navigate('/')}
			title="GOPOD Dashboard"
		>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
				<path
					d="M7 1L12.5 4V10L7 13L1.5 10V4L7 1Z"
					stroke="var(--bg-shell)"
					stroke-width="1.6"
					stroke-linejoin="round"
				/>
			</svg>
		</button>

		{#if !isMinimized}
			<div class="truncate text-sm font-semibold tracking-[0.2px] text-[var(--text-primary)]">
				GOPOD
			</div>
			{#if isMobile}
				<button
					onclick={onclose}
					aria-label="Close sidebar"
					class="ml-auto flex cursor-pointer items-center justify-center rounded border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				>
					<X size={18} />
				</button>
			{/if}
		{/if}
	</div>

	<!-- Nav -->
	<nav class={cn('flex-1 overflow-y-auto pt-1', isMinimized ? 'px-1' : 'px-1.5')}>
		{#each nav as item (item.label)}
			{#if isMinimized}
				<!-- Minimized (Icon Rail) Mode -->
				{@const hasChildren = item.children && item.children.length > 0}
				{@const active = hasChildren
					? item.children!.some((c) => c.path && isActive(c.path))
					: item.path
						? isActive(item.path)
						: false}
				{@const targetPath = hasChildren ? item.children![0].path! : item.path!}

				<button
					onclick={() => navigate(targetPath)}
					title={item.label}
					class={cn(
						'mx-auto my-1 flex h-9 w-9 cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 transition-colors',
						active
							? 'bg-[var(--accent-muted)] font-medium text-[var(--text-primary)]'
							: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
					)}
				>
					{#if item.icon && iconMap[item.icon]}
						{@const Icon = iconMap[item.icon]}
						<Icon size={18} />
					{/if}
				</button>
			{:else}
				<!-- Expanded Mode -->
				{#if item.children}
					{@const open = !collapsed[item.label]}
					{@const anyActive = item.children.some((c) => c.path && isActive(c.path))}
					<div class="mb-px">
						<button
							onclick={() => toggle(item.label)}
							class={cn(
								'flex w-full cursor-pointer items-center justify-between rounded-[var(--radius-sm)] border-none bg-transparent px-2.5 py-[7px] text-left text-base font-[var(--font-sans)] transition-colors',
								anyActive
									? 'font-medium text-[var(--text-primary)]'
									: 'font-normal text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
							)}
						>
							<span class="flex min-w-0 items-center gap-2.5">
								<span class="shrink-0 opacity-75">
									{#if item.icon && iconMap[item.icon]}
										{@const ItemIcon = iconMap[item.icon]}
										<ItemIcon size={16} />
									{/if}
								</span>
								<span class="truncate">{item.label}</span>
							</span>
							<span
								class={cn(
									'shrink-0 text-[var(--text-tertiary)] transition-transform duration-150',
									open && 'rotate-180'
								)}
							>
								<CaretDown size={12} />
							</span>
						</button>
						{#if open}
							{#each item.children as child (child.label)}
								{@const active = child.path ? isActive(child.path) : false}
								<button
									onclick={() => navigate(child.path!)}
									class={cn(
										'flex w-full cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border-none py-1.5 pr-2.5 pl-[33px] text-left text-base font-[var(--font-sans)] transition-colors',
										active
											? 'bg-[var(--accent-muted)] font-medium text-[var(--text-primary)]'
											: 'bg-transparent font-normal text-[var(--text-tertiary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)]'
									)}
								>
									{#if child.icon && iconMap[child.icon]}
										{@const ChildIcon = iconMap[child.icon]}
										<ChildIcon size={14} />
									{/if}
									<span class="truncate">{child.label}</span>
								</button>
							{/each}
						{/if}
					</div>
				{:else}
					{@const active = item.path ? isActive(item.path) : false}
					<button
						onclick={() => navigate(item.path!)}
						class={cn(
							'mb-px flex w-full cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border-none px-2.5 py-[7px] text-left text-base font-[var(--font-sans)] transition-colors',
							active
								? 'bg-[var(--accent-muted)] font-medium text-[var(--text-primary)]'
								: 'bg-transparent font-normal text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
						)}
					>
						<span class={cn('shrink-0 opacity-75', active && 'text-[var(--accent)] opacity-100')}>
							{#if item.icon && iconMap[item.icon]}
								{@const ItemIcon = iconMap[item.icon]}
								<ItemIcon size={16} />
							{/if}
						</span>
						<span class="truncate">{item.label}</span>
					</button>
				{/if}
			{/if}
		{/each}
	</nav>

	<!-- Footer with User Profile & System Status -->
	<div
		class={cn(
			'flex flex-col border-t border-[var(--border-subtle)] pt-2 pb-2.5',
			isMinimized ? 'items-center gap-2 px-1' : 'gap-2 px-2.5'
		)}
	>
		{#if isMinimized}
			<button
				onclick={() => navigate('/settings')}
				title="{authStore.user?.name || 'Administrator'} ({authStore.user?.email ||
					'admin@gopod.dev'})"
				class="relative flex h-8 w-8 cursor-pointer items-center justify-center rounded-full border border-[var(--border)] bg-[var(--bg-surface)] text-xs font-semibold text-[var(--accent)] transition-colors hover:border-[var(--accent)]"
			>
				{authStore.user?.name ? authStore.user.name.charAt(0).toUpperCase() : 'A'}
				<span
					class="absolute -right-0.5 -bottom-0.5 h-2 w-2 rounded-full border-2 border-[var(--bg-outer)] bg-[var(--status-green)]"
				></span>
			</button>
			<button
				onclick={async () => {
					await authStore.logout();
					goto('/login');
				}}
				title="Sign Out"
				class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
			>
				<SignOut size={16} />
			</button>
		{:else}
			<div class="flex w-full items-center justify-between gap-1">
				<button
					onclick={() => navigate('/settings')}
					class="group flex min-w-0 flex-1 cursor-pointer items-center gap-2.5 rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-left transition-colors hover:bg-[var(--bg-hover)]"
					title="Account Settings"
				>
					<div
						class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full border border-[var(--border)] bg-[var(--bg-surface)] text-xs font-semibold text-[var(--accent)] group-hover:border-[var(--accent)]"
					>
						{authStore.user?.name ? authStore.user.name.charAt(0).toUpperCase() : 'A'}
					</div>
					<div class="flex min-w-0 flex-col">
						<span class="truncate text-sm leading-tight font-medium text-[var(--text-primary)]">
							{authStore.user?.name || 'Administrator'}
						</span>
						<span class="truncate text-xs text-[var(--text-tertiary)]">
							{authStore.user?.email || 'admin@gopod.dev'}
						</span>
					</div>
				</button>
				<button
					onclick={async () => {
						await authStore.logout();
						goto('/login');
					}}
					title="Sign Out"
					class="shrink-0 cursor-pointer rounded-[var(--radius-sm)] border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--status-red)]"
				>
					<SignOut size={16} />
				</button>
			</div>
			<div
				class="flex items-center justify-between px-2 font-mono text-[11px] text-[var(--text-muted)]"
			>
				<span>cpx41-edge</span>
				<span>v0.1.0</span>
			</div>
		{/if}
	</div>
</aside>
