<script lang="ts">
	import { CaretLeft, CaretRight } from 'phosphor-svelte';
	import { Select } from 'kumo-svelte';

	interface Props {
		page?: number;
		pageSize?: number;
		totalCount: number;
		pageSizeOptions?: number[];
		itemName?: string;
		showRowsPerPage?: boolean;
		onPageChange?: (page: number) => void;
		onPageSizeChange?: (pageSize: number) => void;
		class?: string;
	}

	let {
		page = $bindable(1),
		pageSize = $bindable(10),
		totalCount,
		pageSizeOptions = [10, 25, 50, 100],
		itemName = 'items',
		showRowsPerPage = true,
		onPageChange,
		onPageSizeChange,
		class: className = ''
	}: Props = $props();

	let totalPages = $derived(Math.max(1, Math.ceil(totalCount / (pageSize || 10))));

	let resolvedOptions = $derived(
		Array.from(new Set([pageSize, ...pageSizeOptions]))
			.filter((s) => typeof s === 'number' && s > 0)
			.sort((a, b) => a - b)
			.map((size) => ({ label: String(size), value: size }))
	);

	let startItem = $derived(totalCount === 0 ? 0 : (page - 1) * pageSize + 1);
	let endItem = $derived(Math.min(page * pageSize, totalCount));

	function goToPage(newPage: number) {
		const target = Math.max(1, Math.min(totalPages, newPage));
		if (target !== page) {
			page = target;
			onPageChange?.(target);
		}
	}

	function handleSizeChange(val: unknown) {
		if (typeof val === 'number') {
			pageSize = val;
			page = 1;
			onPageSizeChange?.(val);
			onPageChange?.(1);
		}
	}
</script>

<!-- Table Footer with Full Pagination (Kumo Parity) -->
<div
	class="flex flex-col items-center justify-between gap-3 border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-3 text-xs text-[var(--text-secondary)] sm:flex-row {className}"
>
	<!-- Left: Entry Counts -->
	<div class="flex items-center gap-2">
		{#if totalCount > 0}
			<span>
				Showing <strong>{startItem}</strong>–<strong>{endItem}</strong> of <strong>{totalCount}</strong> {itemName}
			</span>
		{:else}
			<span>No {itemName} to display</span>
		{/if}
	</div>

	<!-- Right: Page Controls & Size Selector -->
	<div class="flex items-center gap-4">
		{#if showRowsPerPage}
			<!-- Rows per page selector -->
			<div class="flex items-center gap-2">
				<span class="text-[var(--text-tertiary)]">Rows:</span>
				<div class="w-20">
					<Select
						value={pageSize}
						onValueChange={handleSizeChange}
						options={resolvedOptions}
						size="sm"
					/>
				</div>
			</div>
		{/if}

		<!-- Page Navigation Buttons -->
		<div class="flex items-center gap-1.5">
			<button
				type="button"
				disabled={page <= 1}
				onclick={() => goToPage(page - 1)}
				class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40 transition-colors"
				aria-label="Previous page"
			>
				<CaretLeft size={13} />
			</button>

			<span class="px-2 text-xs font-medium text-[var(--text-primary)] tabular-nums">
				{page} / {totalPages}
			</span>

			<button
				type="button"
				disabled={page >= totalPages}
				onclick={() => goToPage(page + 1)}
				class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border border-[var(--border)] bg-[var(--bg-surface)] text-[var(--text-secondary)] hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-40 transition-colors"
				aria-label="Next page"
			>
				<CaretRight size={13} />
			</button>
		</div>
	</div>
</div>
