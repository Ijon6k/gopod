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

<div class="flex w-full flex-wrap items-center justify-between gap-3">
	<!-- Left: View Mode Selector & Breadcrumb -->
	<div class="flex items-center gap-2">
		<div
			class="relative flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1.5"
		>
			<span class="mr-2 shrink-0 text-[var(--text-tertiary)]">
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
				class="cursor-pointer border-0 bg-transparent pr-4 text-base font-[var(--font-sans)] font-medium text-[var(--text-primary)] outline-none"
			>
				<option value="global" class="bg-[var(--bg-shell)] text-[var(--text-primary)]">
					Global Infrastructure (Overview)
				</option>
				<optgroup
					label="Projects (Territories)"
					class="bg-[var(--bg-shell)] text-[var(--text-tertiary)]"
				>
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
				class="cursor-pointer rounded border-0 bg-transparent px-2 py-1 text-xs text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			>
				← Back to Global
			</button>
		{/if}
	</div>

	<!-- Right: Zoom & Fit Controls -->
	<div
		class="flex items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1"
	>
		<button
			onclick={onZoomOut}
			class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			title="Zoom Out"
		>
			<MagnifyingGlassMinus size={14} />
		</button>

		<button
			onclick={onResetZoom}
			class="flex h-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent px-2 font-mono text-[11px] text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			title="Reset to 100%"
		>
			{Math.round(zoom * 100)}%
		</button>

		<button
			onclick={onZoomIn}
			class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			title="Zoom In"
		>
			<MagnifyingGlassPlus size={14} />
		</button>

		<div class="mx-0.5 h-4 w-px bg-[var(--border-subtle)]"></div>

		<button
			onclick={onFitView}
			class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			title="Fit to View"
		>
			<ArrowsOut size={14} />
		</button>
	</div>
</div>
