<script lang="ts" generics="T">
	import { cn } from '$lib/utils/cn';
	import { untrack } from 'svelte';
	import type { Snippet } from 'svelte';
	import {
		SkeletonLine,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		LayerCard
	} from 'kumo-svelte';
	import TablePagination from './TablePagination.svelte';

	interface Column {
		key: string;
		label: string;
		render?: Snippet<[T]>;
		mono?: boolean;
		width?: string;
		align?: 'left' | 'right' | 'center';
	}

	interface Props {
		columns: Column[];
		rows: T[];
		keyExtractor: (row: T) => string;
		onRowClick?: (row: T) => void;
		emptyMessage?: string;
		actions?: Snippet<[T]>;
		/**
		 * Pin the actions column to the right edge of the horizontal scroll
		 * container (kumo sticky-column pattern) so row actions stay reachable
		 * no matter how wide the table is.
		 */
		stickyActions?: boolean;
		/** While true the table body renders skeleton rows instead of data. */
		loading?: boolean;
		/** Number of skeleton rows to display while loading. */
		skeletonRows?: number;
		class?: string;
		striped?: boolean;
		pageSize?: number;
		page?: number;
		pageSizeOptions?: number[];
		itemName?: string;
		showRowsPerPage?: boolean;
		onPageChange?: (page: number) => void;
		onPageSizeChange?: (pageSize: number) => void;
	}

	let {
		columns,
		rows,
		keyExtractor,
		onRowClick,
		emptyMessage = 'No data available',
		actions,
		stickyActions = false,
		loading = false,
		skeletonRows = 8,
		class: className = '',
		striped = true,
		pageSize = $bindable(),
		page = $bindable(1),
		pageSizeOptions = [10, 25, 50, 100],
		itemName = 'items',
		showRowsPerPage = true,
		onPageChange,
		onPageSizeChange
	}: Props = $props();

	// Clamp when pagination inputs change — the bound page itself is compared
	// untracked so the correction does not re-entry the same effect.
	$effect(() => {
		const effectivePageSize = pageSize;
		if (!effectivePageSize || rows.length === 0) return;
		untrack(() => {
			const maxPage = Math.max(1, Math.ceil(rows.length / effectivePageSize));
			if (page > maxPage) {
				page = 1;
				onPageChange?.(1);
			}
		});
	});

	let displayedRows = $derived.by(() => {
		if (!pageSize || pageSize <= 0) return rows;
		const start = (page - 1) * pageSize;
		return rows.slice(start, start + pageSize);
	});

	// Keyed row list for the skeleton phase; re-derives only if skeletonRows
	// changes, so the DOM does not thrash between ticks of any parent loop.
	const skeletonKeys = $derived.by(() =>
		Array.from({ length: skeletonRows }, (_, i) => `skeleton-${i}`)
	);

	// Width recipe per column index reproduces natural data variance without
	// swallowing random values in the component tree.
	function widthClass(i: number, isLast: boolean): number {
		if (isLast) return 45;
		return 66 + ((i * 61) % 129) / 5.0;
	}
</script>

<!-- Kumo Table Container: LayerCard wrapper with smooth rounded corners -->
<!-- Horizontal scrolling lives on this inner container on every breakpoint
     (no vertical inner scroll: height stays content-driven; sticky top-0 row
     header + optional sticky-right actions require this scroll context). -->
<LayerCard class={cn('w-full overflow-hidden p-0', className)} aria-busy={loading}>
	<div class="w-full overflow-x-auto">
		<Table class="w-full min-w-[560px] text-left">
			<TableHeader>
				<!-- Solid Opaque Header: Sticky Top-0 with Z-20, Natural Sentence Case (No All-Caps) -->
				<TableRow class="border-b border-kumo-line bg-kumo-base">
					{#each columns as col (col.key)}
						<TableHead
							class={cn(
								'sticky top-0 z-20 bg-kumo-base px-4 py-3 text-[12px] font-semibold whitespace-nowrap text-kumo-subtle normal-case select-none',
								col.align === 'right'
									? 'text-right'
									: col.align === 'center'
										? 'text-center'
										: 'text-left'
							)}
							style={col.width ? `width: ${col.width}` : ''}
						>
							{col.label}
						</TableHead>
					{/each}
					{#if actions}
						<TableHead
							sticky={stickyActions ? 'right' : undefined}
							class="sticky top-0 z-20 w-12 bg-kumo-base px-4 py-3"
						>
							<span class="sr-only">Actions</span>
						</TableHead>
					{/if}
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-kumo-hairline">
				{#if loading}
					<!-- Skeleton phase: mirrors the live row silhouette -->
					{#each skeletonKeys as sk (sk)}
						<TableRow
							class="bg-[var(--bg-table-row)]"
							style="--kumo-table-row-bg: var(--bg-table-row);"
						>
							{#each columns as col, ci (col.key)}
								<TableCell class="px-4 py-3.5 align-middle whitespace-nowrap">
									<div style:width={`${widthClass(ci, ci === columns.length - 1)}%`}>
										<SkeletonLine minWidth={100} maxWidth={100} class="h-3 w-full" />
									</div>
								</TableCell>
							{/each}
							{#if actions}
								<TableCell
									sticky="right"
									class="bg-(--kumo-table-row-bg) px-4 py-3.5 align-middle"
								></TableCell>
							{/if}
						</TableRow>
					{/each}
				{:else if displayedRows.length === 0}
					<TableRow>
						<TableCell
							colspan={columns.length + (actions ? 1 : 0)}
							class="px-4 py-12 text-center text-sm text-kumo-subtle"
						>
							{emptyMessage}
						</TableCell>
					</TableRow>
				{:else}
					{#each displayedRows as row, i (keyExtractor(row))}
						<TableRow
							class={cn(
								'hover:bg-[var(--bg-table-row-hover)]',
								striped
									? i % 2 === 1
										? 'bg-[var(--bg-table-row-alt)]'
										: 'bg-[var(--bg-table-row)]'
									: 'bg-[var(--bg-table-row)]',
								onRowClick && 'cursor-pointer'
							)}
							style={stickyActions
								? `--kumo-table-row-bg: ${striped && i % 2 === 1 ? 'var(--bg-table-row-alt)' : 'var(--bg-table-row)'};`
								: ''}
							onclick={() => onRowClick?.(row)}
						>
							{#each columns as col (col.key)}
								{@const val = (row as Record<string, unknown>)[col.key]}
								<TableCell
									class={cn(
										'px-4 py-3 align-middle text-[13px] whitespace-nowrap text-kumo-default',
										col.align === 'right'
											? 'text-right'
											: col.align === 'center'
												? 'text-center'
												: 'text-left',
										col.mono && 'font-mono text-xs text-kumo-subtle'
									)}
								>
									{#if col.render}
										{@render col.render(row)}
									{:else}
										{String(val ?? '—')}
									{/if}
								</TableCell>
							{/each}
							{#if actions}
								<TableCell
									sticky={stickyActions ? 'right' : undefined}
									class={cn(
										stickyActions && 'bg-(--kumo-table-row-bg)',
										'px-4 py-3 text-right align-middle'
									)}
									onclick={(e: MouseEvent) => e.stopPropagation()}
								>
									<div class="flex items-center justify-end gap-1">
										{@render actions(row)}
									</div>
								</TableCell>
							{/if}
						</TableRow>
					{/each}
				{/if}
			</TableBody>
		</Table>
	</div>

	{#if pageSize && !loading}
		<TablePagination
			bind:page
			bind:pageSize
			totalCount={rows.length}
			{pageSizeOptions}
			{itemName}
			{showRowsPerPage}
			onPageChange={(p) => onPageChange?.(p)}
			onPageSizeChange={(s) => onPageSizeChange?.(s)}
		/>
	{/if}
</LayerCard>
