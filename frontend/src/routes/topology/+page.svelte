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

	// Query param support (e.g., /topology?project=aerochat)
	let initialProject = page.url.searchParams.get('project');
	let viewMode = $state(
		initialProject && projects.some((p) => p.id === initialProject) ? initialProject : 'global'
	);

	let selectedItem: SelectedItem = $state(null);
	let zoom = $state(0.9);
	let panX = $state(40);
	let panY = $state(40);

	// Compute graph deterministically whenever inputs or viewMode change
	let graph = $derived(
		computeTopologyGraph({
			projects,
			services,
			containers,
			pods,
			domains,
			volumes,
			networks,
			server,
			viewMode
		})
	);

	let currentProject = $derived(projects.find((p) => p.id === viewMode));

	function setViewMode(mode: string) {
		viewMode = mode;
		selectedItem = null;
		if (mode === 'global') {
			goto('/topology', { replaceState: true, noScroll: true });
			panX = 40;
			panY = 40;
			zoom = 0.9;
		} else {
			goto(`/topology?project=${mode}`, { replaceState: true, noScroll: true });
			panX = 40;
			panY = 40;
			zoom = 1;
		}
	}

	function handleZoomIn() {
		zoom = Math.min(2.2, Math.round(zoom * 1.15 * 100) / 100);
	}

	function handleZoomOut() {
		zoom = Math.max(0.3, Math.round(zoom * 0.85 * 100) / 100);
	}

	function handleResetZoom() {
		zoom = 1;
		panX = 40;
		panY = 40;
	}

	function handleFitView() {
		const bounds = graph.bounds;
		const viewportWidth = window.innerWidth || 1200;
		const viewportHeight = window.innerHeight || 800;

		const scaleX = (viewportWidth - 200) / bounds.width;
		const scaleY = (viewportHeight - 200) / bounds.height;
		const fitZoom = Math.max(0.4, Math.min(1.15, Math.min(scaleX, scaleY)));

		zoom = Math.round(fitZoom * 100) / 100;
		panX = Math.round(60 - bounds.minX * zoom);
		panY = Math.round(60 - bounds.minY * zoom);
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
		{graph}
		{selectedItem}
		onselect={(item) => (selectedItem = item)}
		bind:zoom
		bind:panX
		bind:panY
		onViewportChange={(v) => {
			zoom = v.zoom;
			panX = v.panX;
			panY = v.panY;
		}}
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

			<!-- Dropdown Selector -->
			<div class="relative flex items-center">
				<select
					id="topology-scope-select"
					value={viewMode}
					onchange={(e) => setViewMode(e.currentTarget.value)}
					class="cursor-pointer appearance-none border-0 bg-transparent pr-5 text-xs font-[var(--font-sans)] font-medium text-[var(--text-primary)] outline-none"
				>
					<option value="global" class="bg-[var(--bg-panel)] text-[var(--text-primary)]">
						Global Infrastructure (Overview)
					</option>
					<optgroup label="Projects" class="bg-[var(--bg-panel)] text-[var(--text-tertiary)]">
						{#each projects as proj (proj.id)}
							<option value={proj.id} class="bg-[var(--bg-panel)] text-[var(--text-primary)]">
								{proj.name} (Project)
							</option>
						{/each}
					</optgroup>
				</select>
				<CaretDown
					size={11}
					class="pointer-events-none absolute right-0 text-[var(--text-tertiary)]"
				/>
			</div>

			{#if viewMode !== 'global'}
				<button
					type="button"
					onclick={() => setViewMode('global')}
					class="ml-1 flex shrink-0 cursor-pointer items-center gap-1 rounded border border-[var(--accent)]/20 bg-[var(--accent)]/10 px-1.5 py-0.5 text-[11px] text-[var(--accent)] hover:underline"
					title="Switch back to global overview"
				>
					<CaretLeft size={11} /> All
				</button>
			{/if}
		</div>

		<!-- Quick Metadata Bar -->
		<div
			class="flex w-fit items-center gap-2 rounded-full border border-[var(--border)] bg-[var(--bg-panel)]/85 px-3 py-1 text-[11px] whitespace-nowrap text-[var(--text-tertiary)] shadow-xs backdrop-blur-xs"
		>
			<span class="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--status-green)]"></span>
			<span>{graph.nodes.length} nodes · {graph.edges.length} connections</span>
			{#if currentProject}
				<span class="opacity-50">·</span>
				<span class="font-medium text-[var(--text-secondary)]">{currentProject.name}</span>
			{/if}
		</div>
	</div>

	<!-- Floating HUD Controls (Top-Right) -->
	<div class="pointer-events-auto absolute top-3.5 right-3.5 z-20 flex items-center gap-2">
		<div
			class="flex items-center gap-1 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]/90 p-1 shadow-lg backdrop-blur-md"
		>
			<button
				type="button"
				onclick={handleZoomOut}
				class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Zoom Out"
				aria-label="Zoom out"
			>
				<Minus size={13} />
			</button>

			<button
				type="button"
				onclick={handleResetZoom}
				class="flex h-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent px-2 font-mono text-[11px] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Reset to 100%"
			>
				{Math.round(zoom * 100)}%
			</button>

			<button
				type="button"
				onclick={handleZoomIn}
				class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Zoom In"
				aria-label="Zoom in"
			>
				<Plus size={13} />
			</button>

			<div class="mx-0.5 h-3.5 w-px bg-[var(--border)]"></div>

			<button
				type="button"
				onclick={handleFitView}
				class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Fit View"
				aria-label="Fit view"
			>
				<CornersOut size={13} />
			</button>
		</div>
	</div>

	<!-- Floating Legend & Navigation Hint (Bottom-Left) -->
	<div
		class="pointer-events-none absolute bottom-3.5 left-3.5 z-10 hidden items-center gap-2.5 select-none lg:flex"
	>
		<div
			class="flex items-center gap-3 rounded-full border border-[var(--border)] bg-[var(--bg-panel)]/85 px-3 py-1.5 text-[11px] text-[var(--text-secondary)] shadow-xs backdrop-blur-xs"
		>
			<span class="flex items-center gap-1.5"
				><Globe size={12} class="text-[var(--accent)]" /> Domain</span
			>
			<span class="flex items-center gap-1.5"
				><ShieldCheck size={12} class="text-[var(--status-green)]" /> Caddy</span
			>
			<span class="flex items-center gap-1.5"
				><Database size={12} class="text-[var(--status-amber)]" /> Database</span
			>
			<span class="flex items-center gap-1.5"
				><FileText size={12} class="text-[var(--accent)]" /> Quadlet</span
			>
			<span class="flex items-center gap-1.5"><Cube size={12} /> Container</span>
			<span class="flex items-center gap-1.5"
				><HardDrive size={12} class="text-[var(--text-tertiary)]" /> Volume</span
			>
		</div>

		<div
			class="rounded-full border border-[var(--border)] bg-[var(--bg-panel)]/75 px-2.5 py-1 text-[11px] text-[var(--text-tertiary)]"
		>
			Drag to pan · Scroll to zoom · Click node to inspect
		</div>
	</div>

	<!-- Slide-Over Right Inspector Drawer -->
	{#if selectedItem}
		<div
			class="animate-in slide-in-from-right pointer-events-auto absolute top-0 right-0 bottom-0 z-30 flex h-full w-80 flex-col shadow-2xl duration-200 sm:w-96"
		>
			<TopologyInspector
				selected={selectedItem}
				onclose={() => (selectedItem = null)}
				onFocusProject={(projId) => setViewMode(projId)}
			/>
		</div>
	{/if}
</div>
