<script lang="ts" generics="T">
	import { cn } from '$lib/utils/cn';
	import type { Snippet } from 'svelte';
	import {
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
		class?: string;
		striped?: boolean;
		pageSize?: number;
		page?: number;
		pageSizeOptions?: number[];
		itemName?: string;
		showRowsPerPage?: boolean;
		onPageChange?: (page: number) => void;
		onPageSizeChange?: (pageSize: number) => void;
		paginationControls?: 'full' | 'simple';
	}

	let {
		columns,
		rows,
		keyExtractor,
		onRowClick,
		emptyMessage = 'No data available',
		actions,
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

	$effect(() => {
		if (pageSize && rows.length > 0) {
			const maxPage = Math.max(1, Math.ceil(rows.length / pageSize));
			if (page > maxPage) {
				page = 1;
				onPageChange?.(1);
			}
		}
	});

	let displayedRows = $derived.by(() => {
		if (!pageSize || pageSize <= 0) return rows;
		const start = (page - 1) * pageSize;
		return rows.slice(start, start + pageSize);
	});
</script>

<!-- Kumo Table Container: LayerCard wrapper with smooth rounded corners & sticky preservation -->
<LayerCard class={cn('w-full overflow-hidden p-0', className)}>
	<div class="w-full overflow-x-auto md:overflow-x-visible">
		<Table class="w-full min-w-[560px] text-left">
			<TableHeader>
				<!-- Solid Opaque Header: Sticky Top-0 with Z-20, Natural Sentence Case (No All-Caps) -->
				<TableRow class="border-b border-kumo-line bg-kumo-base">
					{#each columns as col (col.key)}
						<TableHead
							class={cn(
								'sticky top-0 z-20 bg-kumo-base px-4 py-3 text-[12px] font-semibold whitespace-nowrap text-kumo-subtle select-none normal-case',
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
						<TableHead class="sticky top-0 z-20 w-12 bg-kumo-base px-4 py-3"></TableHead>
					{/if}
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-kumo-hairline">
				{#if displayedRows.length === 0}
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
								'transition-colors hover:bg-[var(--bg-table-row-hover)]',
								striped
									? i % 2 === 1
										? 'bg-[var(--bg-table-row-alt)]'
										: 'bg-[var(--bg-table-row)]'
									: 'bg-[var(--bg-table-row)]',
								onRowClick && 'cursor-pointer'
							)}
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
									class="px-4 py-3 text-right align-middle"
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

	{#if pageSize}
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
