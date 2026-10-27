<script lang="ts">
	import { untrack } from 'svelte';
	import { Sidebar as KumoSidebar, useSidebar, DropdownMenu } from 'kumo-svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { dataStore } from '$lib/stores/data.svelte';
	import { authStore } from '$lib/stores/auth.svelte';
	import { engineLabel } from '$lib/utils/engine';
	import { cn } from '$lib/utils/cn';
	import {
		SquaresFour,
		FolderOpen,
		Stack,
		Globe,
		GitBranch,
		Key,
		ChartBar,
		ShareNetwork,
		ClockCounterClockwise,
		GearSix,
		SignOut,
		CaretDown
	} from 'phosphor-svelte';

	// Engine identity drives naming: "Podman" on podman hosts, "Docker" on
	// docker hosts, "Runtime" until system info arrives.
	let engineName = $derived(engineLabel(dataStore.server));
	const sidebar = useSidebar();

	// Session profile, mirrored in the sidebar footer.
	let userName = $derived(authStore.user?.name || 'Administrator');
	let userInitial = $derived((authStore.user?.name || 'A').charAt(0).toUpperCase());

	function isActive(path: string): boolean {
		const currentPath = page.url.pathname;
		if (path === '/') return currentPath === '/';
		return currentPath === path || currentPath.startsWith(path + '/');
	}

	// Close the mobile offcanvas drawer only when navigation actually changes route.
	// Tracking previous pathname prevents an immediate re-render close loop when the drawer opens.
	let previousPath = page.url.pathname;
	$effect(() => {
		const currentPath = page.url.pathname;
		if (currentPath !== previousPath) {
			previousPath = currentPath;
			if (sidebar.isMobile && untrack(() => sidebar.openMobile)) {
				sidebar.setOpenMobile(false);
			}
		}
	});
</script>

<KumoSidebar>
	<!-- Header: GOPOD Logo + Collapse Trigger (Top-Right) -->
	<KumoSidebar.Header
		class="flex h-12 items-center justify-between border-b border-[var(--border)] px-3"
	>
		{#if sidebar.open || sidebar.isMobile}
			<!-- Expanded State: Logo Icon + Title on left, Collapse Trigger on right -->
			<a
				href="/"
				class="group text-decoration-none flex min-w-0 cursor-pointer items-center gap-2 overflow-hidden"
				title="GOPOD Dashboard"
			>
				<img
					src="/logo.png"
					alt=""
					draggable="false"
					class="h-7 w-7 shrink-0 rounded-md select-none"
				/>
				<span
					class="truncate text-sm leading-tight font-semibold tracking-normal text-[var(--text-primary)]"
				>
					GOPOD
				</span>
			</a>

			<KumoSidebar.Trigger
				class="shrink-0 text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
				title="Collapse sidebar"
			/>
		{:else}
			<!-- Collapsed State: Static GOPOD Logo, click to expand manually -->
			<button
				type="button"
				onclick={() => sidebar.toggleSidebar()}
				class="mx-auto flex h-9 w-9 cursor-pointer items-center justify-center rounded-lg border-0 bg-transparent p-0 outline-none hover:bg-[var(--bg-hover)]"
				title="Expand sidebar"
				aria-label="Expand sidebar"
			>
				<img
					src="/logo.png"
					alt=""
					draggable="false"
					class="h-7 w-7 shrink-0 rounded-md select-none"
				/>
			</button>
		{/if}
	</KumoSidebar.Header>

	<!-- Content Navigation Groups -->
	<KumoSidebar.Content>
		<!-- Overview Group -->
		<KumoSidebar.Group>
			<KumoSidebar.GroupLabel>Overview</KumoSidebar.GroupLabel>
			<KumoSidebar.Menu>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton href="/" active={isActive('/')} icon={SquaresFour} tooltip="Home">
						Home
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/projects"
						active={isActive('/projects')}
						icon={FolderOpen}
						tooltip="Projects"
					>
						Projects
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
			</KumoSidebar.Menu>
		</KumoSidebar.Group>

		<!-- Compute & Network Group -->
		<KumoSidebar.Group>
			<KumoSidebar.GroupLabel>Compute & Network</KumoSidebar.GroupLabel>
			<KumoSidebar.Menu>
				<!-- Engine runtime: flat entry, tabs live inside the page -->
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/runtime/containers"
						active={isActive('/runtime')}
						icon={Stack}
						tooltip={engineName}
					>
						{engineName}
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<!-- Networking: flat entry, tabs live inside the page -->
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/networking/domains"
						active={isActive('/networking')}
						icon={Globe}
						tooltip="Networking"
					>
						Networking
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
			</KumoSidebar.Menu>
		</KumoSidebar.Group>

		<!-- Operations Group -->
		<KumoSidebar.Group>
			<KumoSidebar.GroupLabel>Operations</KumoSidebar.GroupLabel>
			<KumoSidebar.Menu>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/deployments"
						active={isActive('/deployments')}
						icon={GitBranch}
						tooltip="Deployments"
					>
						Deployments
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/monitoring"
						active={isActive('/monitoring')}
						icon={ChartBar}
						tooltip="Monitoring"
					>
						Monitoring
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/topology"
						active={isActive('/topology')}
						icon={ShareNetwork}
						tooltip="Topology"
					>
						Topology
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
			</KumoSidebar.Menu>
		</KumoSidebar.Group>

		<!-- Platform Group -->
		<KumoSidebar.Group>
			<KumoSidebar.GroupLabel>Platform</KumoSidebar.GroupLabel>
			<KumoSidebar.Menu>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/credentials"
						active={isActive('/credentials')}
						icon={Key}
						tooltip="Credentials"
					>
						Credentials
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/audit-log"
						active={isActive('/audit-log')}
						icon={ClockCounterClockwise}
						tooltip="Audit Log"
					>
						Audit Log
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
				<KumoSidebar.MenuItem>
					<KumoSidebar.MenuButton
						href="/settings"
						active={isActive('/settings')}
						icon={GearSix}
						tooltip="Settings"
					>
						Settings
					</KumoSidebar.MenuButton>
				</KumoSidebar.MenuItem>
			</KumoSidebar.Menu>
		</KumoSidebar.Group>
	</KumoSidebar.Content>

	<!-- Footer: Account Profile -->
	<KumoSidebar.Footer class="flex flex-col items-center border-t border-[var(--border)] px-2 py-2 pb-6 md:pb-2">
		<DropdownMenu>
			<DropdownMenu.Trigger
				title="Account profile"
				class={cn(
					'flex w-full cursor-pointer items-center gap-2 rounded-md border-0 bg-transparent',
					'p-1.5 outline-none hover:bg-[var(--bg-hover)]',
					'group-not-data-[state=collapsed]/sidebar:justify-start',
					'group-data-[state=collapsed]/sidebar:justify-center'
				)}
			>
				<div
					class="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[var(--accent-muted)] text-[11px] font-semibold text-[var(--accent)]"
				>
					{userInitial}
				</div>
				<span
					class="min-w-0 flex-1 truncate text-left text-xs font-medium text-[var(--text-primary)] group-data-[state=collapsed]/sidebar:hidden"
				>
					{userName}
				</span>
				<CaretDown
					size={11}
					class="shrink-0 text-[var(--text-tertiary)] group-data-[state=collapsed]/sidebar:hidden"
				/>
			</DropdownMenu.Trigger>
			<DropdownMenu.Content class="w-52" side="top" align="start">
				<div class="mb-1 border-b border-[var(--border-subtle)] px-2 py-1.5">
					<p class="truncate text-xs font-semibold text-[var(--text-primary)]">
						{userName}
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
	</KumoSidebar.Footer>
</KumoSidebar>
