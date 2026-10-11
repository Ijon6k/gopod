<script lang="ts">
	import type { Project } from '$lib/types';
	import {
		MagnifyingGlassPlus,
		MagnifyingGlassMinus,
		ArrowsOut,
		ArrowCounterClockwise,
		Stack,
		Globe,
		CaretDown
	} from 'phosphor-svelte';
	import { DropdownMenu } from 'kumo-svelte';
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
		<!-- Kumo DropdownMenu Selector -->
		<DropdownMenu>
			<DropdownMenu.Trigger
				class="flex cursor-pointer items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs font-[var(--font-sans)] font-medium text-[var(--text-primary)] hover:bg-[var(--bg-hover)] outline-none"
			>
				<span class="shrink-0 text-[var(--text-tertiary)]">
					{#if viewMode === 'global'}
						<Globe size={14} />
					{:else}
						<Stack size={14} />
					{/if}
				</span>
				<span class="max-w-[200px] truncate">
					{#if viewMode === 'global'}
						Global Infrastructure (Overview)
					{:else}
						{currentProject ? `Project: ${currentProject.name}` : viewMode}
					{/if}
				</span>
				<CaretDown size={11} class="text-[var(--text-tertiary)] shrink-0" />
			</DropdownMenu.Trigger>
			<DropdownMenu.Content class="w-64" align="start">
				<DropdownMenu.Item
					selected={viewMode === 'global'}
					onclick={() => onViewModeChange('global')}
				>
					<span class="text-xs">Global Infrastructure (Overview)</span>
				</DropdownMenu.Item>
				<DropdownMenu.Separator />
				<div class="px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">
					Projects (Territories)
				</div>
				{#each projects as proj}
					<DropdownMenu.Item
						selected={viewMode === proj.id}
						onclick={() => onViewModeChange(proj.id)}
					>
						<span class="truncate text-xs">Project: {proj.name}</span>
					</DropdownMenu.Item>
				{/each}
			</DropdownMenu.Content>
		</DropdownMenu>

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
