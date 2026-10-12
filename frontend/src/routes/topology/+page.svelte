<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import {
		projects,
		services,
		containers,
		pods,
		domains,
		volumes,
		networks,
		server
	} from '$lib/data';
	import {
		TopologyCanvas,
		TopologyInspector,
		computeTopologyGraph,
		type TopologyGraph,
		type SelectedItem
	} from '$lib/components/features/topology';
	import {
		ShareNetwork,
		Globe,
		Minus,
		Plus,
		CornersOut,
		Database,
		ShieldCheck,
		FileText,
		Cube,
		HardDrive,
		CaretDown,
		CaretLeft
	} from 'phosphor-svelte';
	import { onMount } from 'svelte';
	import { Button, Badge, DropdownMenu } from 'kumo-svelte';

	// Query param support (e.g., /topology?project=aerochat)
	let initialProject = page.url.searchParams.get('project');
	let viewMode = $state(
		initialProject && projects.some((p) => p.id === initialProject) ? initialProject : 'global'
	);

	let selectedItem: SelectedItem = $state(null);

	// Memoize topology graph based on structural identities so continuous telemetry ticks don't re-create nodes
	let topologyStructureKey = $derived.by(() => {
		const s1 = projects.map((p) => p.id + p.name).join(';');
		const s2 = services.map((s) => s.id + s.status + s.name).join(';');
		const s3 = containers.map((c) => c.id + c.status + c.name).join(';');
		const s4 = pods.map((p) => p.id + p.status).join(';');
		const s5 = domains.map((d) => d.id + (d.hostname || '')).join(';');
		const s6 = volumes.map((v) => v.id + v.name).join(';');
		return `${viewMode}:${s1}:${s2}:${s3}:${s4}:${s5}:${s6}`;
	});

	let cachedGraph: TopologyGraph | null = null;
	let lastStructureKey = '';

	let graph = $derived.by(() => {
		const key = topologyStructureKey;
		if (key !== lastStructureKey || !cachedGraph) {
			lastStructureKey = key;
			cachedGraph = computeTopologyGraph({
				projects,
				services,
				containers,
				pods,
				domains,
				volumes,
				networks,
				server,
				viewMode
			});
		}
		return cachedGraph;
	});

	let currentProject = $derived(projects.find((p) => p.id === viewMode));

	function setViewMode(mode: string) {
		viewMode = mode;
		selectedItem = null;
		if (mode === 'global') {
			goto('/topology', { replaceState: true, noScroll: true });
		} else {
			goto(`/topology?project=${mode}`, { replaceState: true, noScroll: true });
		}
	}

	let canvasRef = $state<any>(null);

	function handleZoomIn() {
		canvasRef?.zoomIn?.();
	}

	function handleZoomOut() {
		canvasRef?.zoomOut?.();
	}

	function handleResetZoom() {
		canvasRef?.resetZoom?.();
	}

	function handleFitView() {
		canvasRef?.fitView?.();
	}

	// Keyboard shortcut: Esc to close inspector
	function handleKeyDown(e: KeyboardEvent) {
		if (e.key === 'Escape' && selectedItem) {
			selectedItem = null;
		}
	}

	onMount(() => {
		window.addEventListener('keydown', handleKeyDown);
		return () => {
			window.removeEventListener('keydown', handleKeyDown);
		};
	});
</script>

<svelte:head>
	<title>Topology — GOPOD</title>
</svelte:head>

<!-- Full-bleed Viewport Canvas (No outer margins or boxed clipping) -->
<div class="relative flex h-full w-full flex-1 overflow-hidden bg-[var(--bg-canvas)] select-none">
	<!-- Canvas Interactive Surface -->
	<TopologyCanvas
		bind:this={canvasRef}
		{graph}
		{selectedItem}
		onselect={(item) => (selectedItem = item)}
	/>

	<!-- Floating HUD Header & Scope Selector (Top-Left) -->
	<div
		class="pointer-events-auto absolute top-3.5 left-3.5 z-20 flex w-auto max-w-none flex-col items-start gap-2"
	>
		<div
			class="flex items-center gap-2.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]/95 p-1.5 pl-3 whitespace-nowrap shadow-lg backdrop-blur-md"
		>
			<div class="flex shrink-0 items-center gap-2">
				<ShareNetwork size={16} class="shrink-0 text-[var(--accent)]" />
				<span class="text-xs font-semibold text-[var(--text-primary)]">Topology</span>
			</div>

			<div class="h-4 w-px shrink-0 bg-[var(--border)]"></div>

			<!-- Dropdown Selector using Kumo DropdownMenu -->
			<DropdownMenu>
				<DropdownMenu.Trigger
					class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border-0 bg-transparent px-1.5 py-0.5 text-xs font-[var(--font-sans)] font-medium text-[var(--text-primary)] hover:bg-[var(--bg-hover)] outline-none"
				>
					<span class="max-w-[180px] truncate sm:max-w-[240px]">
						{#if viewMode === 'global'}
							Global Infrastructure (Overview)
						{:else}
							{currentProject?.name ?? viewMode} (Project)
						{/if}
					</span>
					<CaretDown size={11} class="text-[var(--text-tertiary)] shrink-0" />
				</DropdownMenu.Trigger>
				<DropdownMenu.Content class="w-60" align="start">
					<DropdownMenu.Item
						selected={viewMode === 'global'}
						onclick={() => setViewMode('global')}
					>
						<span class="text-xs">Global Infrastructure (Overview)</span>
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					<div class="px-2 py-1 text-[11px] font-medium text-[var(--text-muted)]">
						Projects
					</div>
					{#each projects as proj (proj.id)}
						<DropdownMenu.Item
							selected={viewMode === proj.id}
							onclick={() => setViewMode(proj.id)}
						>
							<span class="truncate text-xs">{proj.name} (Project)</span>
						</DropdownMenu.Item>
					{/each}
				</DropdownMenu.Content>
			</DropdownMenu>

			{#if viewMode !== 'global'}
				<Button
					variant="secondary"
					size="xs"
					onclick={() => setViewMode('global')}
					class="ml-1 shrink-0"
					title="Switch back to global overview"
				>
					<CaretLeft size={11} /> All
				</Button>
			{/if}
		</div>

		<!-- Quick Metadata Bar with Kumo Badge -->
		<Badge
			variant="secondary"
			class="gap-1.5 px-3 py-1 text-xs font-normal shadow-xs backdrop-blur-xs select-none"
		>
			<span class="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--status-green)]"></span>
			<span>{graph.nodes.length} nodes · {graph.edges.length} connections</span>
			{#if currentProject}
				<span class="opacity-40">·</span>
				<span class="font-medium text-[var(--text-primary)]">{currentProject.name}</span>
			{/if}
		</Badge>
	</div>

	<!-- Floating HUD Controls with Kumo Buttons (Top-Right) -->
	<div class="pointer-events-auto absolute top-3.5 right-3.5 z-20 flex items-center gap-2">
		<div
			class="flex items-center gap-1 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]/95 p-1 shadow-lg backdrop-blur-md"
		>
			<Button
				variant="ghost"
				size="xs"
				shape="square"
				onclick={handleZoomOut}
				title="Zoom out"
				aria-label="Zoom out"
			>
				<Minus size={13} />
			</Button>

			<Button
				variant="ghost"
				size="xs"
				onclick={handleResetZoom}
				title="Reset to 100%"
				class="!px-2 font-mono text-xs text-[var(--text-secondary)]"
			>
				100%
			</Button>

			<Button
				variant="ghost"
				size="xs"
				shape="square"
				onclick={handleZoomIn}
				title="Zoom in"
				aria-label="Zoom in"
			>
				<Plus size={13} />
			</Button>

			<div class="mx-0.5 h-3.5 w-px bg-[var(--border)]"></div>

			<Button
				variant="ghost"
				size="xs"
				shape="square"
				onclick={handleFitView}
				title="Fit view"
				aria-label="Fit view"
			>
				<CornersOut size={13} />
			</Button>
		</div>
	</div>

	<!-- Floating Legend & Navigation Hint with Kumo Badges (Bottom-Left) -->
	<div
		class="pointer-events-none absolute bottom-3.5 left-3.5 z-10 hidden items-center gap-2.5 select-none lg:flex"
	>
		<div
			class="flex items-center gap-1.5 rounded-full border border-[var(--border)] bg-[var(--bg-panel)]/90 px-3 py-1.5 shadow-xs backdrop-blur-xs"
		>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<Globe size={12} class="text-[var(--accent)]" /> Domain
			</Badge>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<ShieldCheck size={12} class="text-[var(--status-green)]" /> Caddy
			</Badge>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<Database size={12} class="text-[var(--status-amber)]" /> Database
			</Badge>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<FileText size={12} class="text-[var(--accent)]" /> Quadlet
			</Badge>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<Cube size={12} /> Container
			</Badge>
			<Badge variant="outline" class="gap-1.5 text-xs font-normal">
				<HardDrive size={12} class="text-[var(--text-tertiary)]" /> Volume
			</Badge>
		</div>

		<div
			class="rounded-full border border-[var(--border)] bg-[var(--bg-panel)]/80 px-2.5 py-1 text-xs text-[var(--text-tertiary)]"
		>
			Drag to pan · Scroll to zoom · Click node to inspect
		</div>
	</div>

	<!-- Slide-Over Right Inspector Drawer -->
	{#if selectedItem}
		<!-- Mobile backdrop for easy dismissal -->
		<button
			type="button"
			class="fixed inset-0 z-25 bg-[var(--bg-canvas)]/75 backdrop-blur-2xs transition-opacity sm:hidden cursor-pointer border-0 p-0"
			onclick={() => (selectedItem = null)}
			aria-label="Close drawer overlay"
		></button>

		<div
			class="animate-in slide-in-from-right pointer-events-auto absolute top-0 right-0 bottom-0 z-30 flex h-full w-full max-w-[400px] flex-col shadow-2xl duration-200"
		>
			<TopologyInspector
				selected={selectedItem}
				onclose={() => (selectedItem = null)}
				onFocusProject={(projId) => setViewMode(projId)}
			/>
		</div>
	{/if}
</div>
