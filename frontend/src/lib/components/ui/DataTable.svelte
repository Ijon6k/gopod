<script lang="ts" generics="T">
	import { cn } from '$lib/utils/cn';
	import type { Snippet } from 'svelte';

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
	}

	let {
		columns,
		rows,
		keyExtractor,
		onRowClick,
		emptyMessage = 'No data',
		actions,
		class: className = ''
	}: Props = $props();
</script>

<div class={cn('overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)]', className)}>
	<table class="w-full border-collapse min-w-[560px]">
		<thead>
			<tr class="border-b border-[var(--border)]">
				{#each columns as col}
					<th
						class={cn(
							'px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)]',
							col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left'
						)}
						style={col.width ? `width: ${col.width}` : ''}
					>
						{col.label}
					</th>
				{/each}
				{#if actions}
					<th class="px-3.5 py-2.5 bg-[var(--bg-panel)] w-10"></th>
				{/if}
			</tr>
		</thead>
		<tbody>
			{#if rows.length === 0}
				<tr>
					<td
						colspan={columns.length + (actions ? 1 : 0)}
						class="px-3.5 py-8 text-center text-[var(--text-tertiary)] text-base"
					>
						{emptyMessage}
					</td>
				</tr>
			{:else}
				{#each rows as row, i (keyExtractor(row))}
					<tr
						class={cn(
							'transition-colors duration-100',
							i < rows.length - 1 && 'border-b border-[var(--border-subtle)]',
							onRowClick && 'cursor-pointer hover:bg-[var(--bg-hover)]'
						)}
						onclick={() => onRowClick?.(row)}
					>
						{#each columns as col}
							{@const val = (row as Record<string, unknown>)[col.key]}
							<td
								class={cn(
									'px-3.5 py-2.5 text-base text-[var(--text-primary)] align-middle whitespace-nowrap',
									col.align === 'right' ? 'text-right' : col.align === 'center' ? 'text-center' : 'text-left',
									col.mono && 'font-[var(--font-mono)]'
								)}
							>
								{#if col.render}
									{@render col.render(row)}
								{:else}
									{String(val ?? '—')}
								{/if}
							</td>
						{/each}
						{#if actions}
							<td class="px-3.5 py-2.5" onclick={(e) => e.stopPropagation()}>
								{@render actions(row)}
							</td>
						{/if}
					</tr>
				{/each}
			{/if}
		</tbody>
	</table>
</div>
