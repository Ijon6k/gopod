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
		'h-full bg-[var(--bg-outer)] flex flex-col shrink-0 transition-[width] duration-200 select-none',
		isMinimized ? 'w-[58px]' : 'w-[var(--sidebar-width)]'
	)}
>
	<!-- Product mark -->
	<div
		class={cn(
			'pt-3 pb-2 flex items-center',
			isMinimized ? 'justify-center px-1' : 'px-3 gap-2.5'
		)}
	>
		<button
			class="w-7 h-7 rounded-[7px] bg-[var(--accent)] flex items-center justify-center shrink-0 cursor-pointer border-0 p-0"
			onclick={() => navigate('/')}
			title="GOPOD Dashboard"
		>
			<svg width="14" height="14" viewBox="0 0 14 14" fill="none">
				<path
					d="M7 1L12.5 4V10L7 13L1.5 10V4L7 1Z"
					stroke="#0D0F12"
					stroke-width="1.6"
					stroke-linejoin="round"
				/>
			</svg>
		</button>

		{#if !isMinimized}
			<div class="text-sm font-semibold text-[var(--text-primary)] tracking-[0.2px] truncate">
				GOPOD
			</div>
			{#if isMobile}
				<button
					onclick={onclose}
					aria-label="Close sidebar"
					class="ml-auto p-1.5 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] bg-transparent border-0 cursor-pointer rounded flex items-center justify-center transition-colors"
				>
					<X size={18} />
				</button>
			{/if}
		{/if}
	</div>

	<!-- Nav -->
	<nav class={cn('flex-1 pt-1 overflow-y-auto', isMinimized ? 'px-1' : 'px-1.5')}>
		{#each nav as item}
			{#if isMinimized}
				<!-- Minimized (Icon Rail) Mode -->
				{@const hasChildren = item.children && item.children.length > 0}
				{@const active = hasChildren
					? item.children!.some((c) => c.path && isActive(c.path))
					: item.path ? isActive(item.path) : false}
				{@const targetPath = hasChildren ? item.children![0].path! : item.path!}

				<button
					onclick={() => navigate(targetPath)}
					title={item.label}
					class={cn(
						'w-9 h-9 mx-auto my-1 rounded-[var(--radius-sm)] flex items-center justify-center border-0 cursor-pointer transition-colors',
						active
							? 'bg-[var(--accent-muted)] text-[var(--text-primary)] font-medium'
							: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
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
								'w-full flex items-center justify-between px-2.5 py-[7px] rounded-[var(--radius-sm)] bg-transparent border-none cursor-pointer text-base font-[var(--font-sans)] text-left transition-colors',
								anyActive
									? 'text-[var(--text-primary)] font-medium'
									: 'text-[var(--text-secondary)] font-normal hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
							)}
						>
							<span class="flex items-center gap-2.5 min-w-0">
								<span class="opacity-75 shrink-0">
									{#if item.icon && iconMap[item.icon]}
										{@const ItemIcon = iconMap[item.icon]}
										<ItemIcon size={16} />
									{/if}
								</span>
								<span class="truncate">{item.label}</span>
							</span>
							<span
								class={cn(
									'text-[var(--text-tertiary)] transition-transform duration-150 shrink-0',
									open && 'rotate-180'
								)}
							>
								<CaretDown size={12} />
							</span>
						</button>
						{#if open}
							{#each item.children as child}
								{@const active = child.path ? isActive(child.path) : false}
								<button
									onclick={() => navigate(child.path!)}
									class={cn(
										'w-full flex items-center gap-2.5 pl-[33px] pr-2.5 py-1.5 rounded-[var(--radius-sm)] border-none cursor-pointer text-base font-[var(--font-sans)] text-left transition-colors',
										active
											? 'bg-[var(--accent-muted)] text-[var(--text-primary)] font-medium'
											: 'bg-transparent text-[var(--text-tertiary)] font-normal hover:text-[var(--text-secondary)] hover:bg-[var(--bg-hover)]'
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
							'w-full flex items-center gap-2.5 px-2.5 py-[7px] rounded-[var(--radius-sm)] mb-px border-none cursor-pointer text-base font-[var(--font-sans)] text-left transition-colors',
							active
								? 'bg-[var(--accent-muted)] text-[var(--text-primary)] font-medium'
								: 'bg-transparent text-[var(--text-secondary)] font-normal hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
						)}
					>
						<span class={cn('opacity-75 shrink-0', active && 'opacity-100 text-[var(--accent)]')}>
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
			'pb-2.5 pt-2 flex flex-col border-t border-[var(--border-subtle)]',
			isMinimized ? 'items-center px-1 gap-2' : 'px-2.5 gap-2'
		)}
	>
		{#if isMinimized}
			<button
				onclick={() => navigate('/settings')}
				title="{authStore.user?.name || 'Administrator'} ({authStore.user?.email || 'admin@gopod.dev'})"
				class="w-8 h-8 rounded-full bg-[var(--bg-surface)] border border-[var(--border)] text-[var(--accent)] font-semibold text-xs flex items-center justify-center cursor-pointer hover:border-[var(--accent)] transition-colors relative"
			>
				{authStore.user?.name ? authStore.user.name.charAt(0).toUpperCase() : 'A'}
				<span
					class="absolute -bottom-0.5 -right-0.5 w-2 h-2 rounded-full bg-[var(--status-green)] border-2 border-[var(--bg-outer)]"
				></span>
			</button>
			<button
				onclick={async () => {
					await authStore.logout();
					goto('/login');
				}}
				title="Sign Out"
				class="w-8 h-8 rounded-[var(--radius-sm)] flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--status-red)] hover:bg-[var(--bg-hover)] transition-colors border-0 bg-transparent cursor-pointer"
			>
				<SignOut size={16} />
			</button>
		{:else}
			<div class="flex items-center justify-between gap-1 w-full">
				<button
					onclick={() => navigate('/settings')}
					class="flex-1 flex items-center gap-2.5 min-w-0 p-1.5 rounded-[var(--radius-sm)] bg-transparent border-0 hover:bg-[var(--bg-hover)] transition-colors cursor-pointer text-left group"
					title="Account Settings"
				>
					<div
						class="w-7 h-7 rounded-full bg-[var(--bg-surface)] border border-[var(--border)] text-[var(--accent)] font-semibold text-xs flex items-center justify-center shrink-0 group-hover:border-[var(--accent)]"
					>
						{authStore.user?.name ? authStore.user.name.charAt(0).toUpperCase() : 'A'}
					</div>
					<div class="flex flex-col min-w-0">
						<span class="text-sm font-medium text-[var(--text-primary)] truncate leading-tight">
							{authStore.user?.name || 'Administrator'}
						</span>
						<span class="text-xs text-[var(--text-tertiary)] truncate">
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
					class="p-1.5 rounded-[var(--radius-sm)] text-[var(--text-tertiary)] hover:text-[var(--status-red)] hover:bg-[var(--bg-hover)] transition-colors border-0 bg-transparent cursor-pointer shrink-0"
				>
					<SignOut size={16} />
				</button>
			</div>
			<div class="flex items-center justify-between px-2 text-[11px] text-[var(--text-muted)] font-mono">
				<span>cpx41-edge</span>
				<span>v0.1.0</span>
			</div>
		{/if}
	</div>
</aside>
