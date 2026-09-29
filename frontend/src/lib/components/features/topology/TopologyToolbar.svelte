<script lang="ts">
	import type { Project } from '$lib/types';
	import {
		MagnifyingGlassPlus,
		MagnifyingGlassMinus,
		ArrowsOut,
		ArrowCounterClockwise,
		Stack,
		Globe
	} from 'phosphor-svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		projects: Project[];
		viewMode: string;
		zoom: number;
		onViewModeChange: (mode: string) => void;
		onZoomIn: () => void;
		onZoomOut: () => void;
		onResetZoom: () => void;
		onFitView: () => void;
	}

	let {
		projects,
		viewMode,
		zoom,
		onViewModeChange,
		onZoomIn,
		onZoomOut,
		onResetZoom,
		onFitView
	}: Props = $props();

	let currentProject = $derived(projects.find((p) => p.id === viewMode));
</script>

<div class="flex items-center justify-between gap-3 w-full flex-wrap">
	<!-- Left: View Mode Selector & Breadcrumb -->
	<div class="flex items-center gap-2">
		<div class="relative flex items-center bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-sm)] px-2.5 py-1.5">
			<span class="text-[var(--text-tertiary)] mr-2 shrink-0">
				{#if viewMode === 'global'}
					<Globe size={14} />
				{:else}
					<Stack size={14} />
				{/if}
			</span>

			<label for="topology-view-select" class="sr-only">Select Topology View</label>
			<select
				id="topology-view-select"
				value={viewMode}
				onchange={(e) => onViewModeChange((e.target as HTMLSelectElement).value)}
				class="bg-transparent border-0 outline-none text-base font-medium text-[var(--text-primary)] cursor-pointer pr-4 font-[var(--font-sans)]"
			>
				<option value="global" class="bg-[var(--bg-shell)] text-[var(--text-primary)]">
					Global Infrastructure (Overview)
				</option>
				<optgroup label="Projects (Territories)" class="bg-[var(--bg-shell)] text-[var(--text-tertiary)]">
					{#each projects as proj}
						<option value={proj.id} class="bg-[var(--bg-shell)] text-[var(--text-primary)]">
							Project: {proj.name}
						</option>
					{/each}
				</optgroup>
			</select>
		</div>

		{#if viewMode !== 'global'}
			<button
				onclick={() => onViewModeChange('global')}
				class="text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)] px-2 py-1 rounded hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
			>
				← Back to Global
			</button>
		{/if}
	</div>

	<!-- Right: Zoom & Fit Controls -->
	<div class="flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border)] rounded-[var(--radius-sm)] p-1">
		<button
			onclick={onZoomOut}
			class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
			title="Zoom Out"
		>
			<MagnifyingGlassMinus size={14} />
		</button>

		<button
			onclick={onResetZoom}
			class="px-2 h-7 rounded flex items-center justify-center text-[11px] font-mono text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
			title="Reset to 100%"
		>
			{Math.round(zoom * 100)}%
		</button>

		<button
			onclick={onZoomIn}
			class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
			title="Zoom In"
		>
			<MagnifyingGlassPlus size={14} />
		</button>

		<div class="w-px h-4 bg-[var(--border-subtle)] mx-0.5"></div>

		<button
			onclick={onFitView}
			class="w-7 h-7 rounded flex items-center justify-center text-[var(--text-tertiary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] border-0 bg-transparent cursor-pointer transition-colors"
			title="Fit to View"
		>
			<ArrowsOut size={14} />
		</button>
	</div>
</div>
