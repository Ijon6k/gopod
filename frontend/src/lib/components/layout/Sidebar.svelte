<script lang="ts">
	import { Sidebar as KumoSidebar, useSidebar } from 'kumo-svelte';
	import { page } from '$app/state';
	import { dataStore } from '$lib/stores/data.svelte';
	import { engineLabel, engineVersionLabel } from '$lib/utils/engine';
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
		GearSix
	} from 'phosphor-svelte';

	// Engine identity drives naming everywhere: "Podman" on podman hosts,
	// "Docker" on docker hosts, "Runtime" until system info arrives.
	let engineName = $derived(engineLabel(dataStore.server));
	let engineVersion = $derived(engineVersionLabel(dataStore.server));
	let serverHostname = $derived(dataStore.server?.hostname ?? 'cpx41-edge');
	const sidebar = useSidebar();

	function isActive(path: string): boolean {
		const currentPath = page.url.pathname;
		if (path === '/') return currentPath === '/';
		return currentPath === path || currentPath.startsWith(path + '/');
	}

	// Close the mobile offcanvas drawer when navigation changes it. Guarded on
	// kumo's documented pattern: untouched on desktop (the collapsed state is
	// user-controlled and must survive navigation), only dismissed when the
	// mobile drawer is actually open on an actual mobile viewport.
	$effect(() => {
		page.url.pathname;
		if (!sidebar.isMobile || !sidebar.openMobile) return;
		sidebar.setOpenMobile(false);
	});
</script>

<KumoSidebar>
	<!-- Header: GOPOD Brand & Collapse Trigger (Top-Right) -->
	<KumoSidebar.Header
		class="flex h-12 items-center justify-between border-b border-[var(--border)] px-3"
	>
		{#if sidebar.open}
			<!-- Expanded State: Logo + Title on left, Collapse Trigger on right -->
			<a
				href="/"
				class="group text-decoration-none flex min-w-0 cursor-pointer items-center gap-2.5 overflow-hidden"
				title="GOPOD Dashboard"
			>
				<div
					class="flex h-6 w-6 shrink-0 items-center justify-center rounded-[5px] bg-[var(--accent)] text-white shadow-xs"
				>
					<svg width="12" height="12" viewBox="0 0 14 14" fill="none">
						<path
							d="M7 1L12.5 4V10L7 13L1.5 10V4L7 1Z"
							stroke="currentColor"
							stroke-width="1.6"
							stroke-linejoin="round"
						/>
					</svg>
				</div>
				<div class="flex min-w-0 flex-col">
					<span
						class="truncate text-sm leading-tight font-semibold tracking-normal text-[var(--text-primary)]"
					>
						GOPOD
					</span>
					<span class="truncate font-mono text-[10px] leading-tight text-[var(--text-tertiary)]">
						{engineName} v{engineVersion}
					</span>
				</div>
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
				class="mx-auto flex h-8 w-8 cursor-pointer items-center justify-center rounded-lg border-0 bg-transparent p-0 outline-none hover:bg-[var(--bg-hover)]"
				title="Expand sidebar"
				aria-label="Expand sidebar"
			>
				<div
					class="flex h-6 w-6 items-center justify-center rounded-[5px] bg-[var(--accent)] text-white shadow-xs"
				>
					<svg width="12" height="12" viewBox="0 0 14 14" fill="none">
						<path
							d="M7 1L12.5 4V10L7 13L1.5 10V4L7 1Z"
							stroke="currentColor"
							stroke-width="1.6"
							stroke-linejoin="round"
						/>
					</svg>
				</div>
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

	<!-- Footer: Server status indicator -->
	<KumoSidebar.Footer
		class="flex items-center justify-center border-t border-[var(--border)] px-3 group-not-data-[state=collapsed]/sidebar:justify-start"
	>
		<div class="flex items-center gap-1.5 font-mono text-[11px] text-[var(--text-muted)]">
			<span class="inline-block h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--status-green)]"></span>
			<span class="truncate group-data-[state=collapsed]/sidebar:hidden">{serverHostname}</span>
		</div>
	</KumoSidebar.Footer>
</KumoSidebar>
