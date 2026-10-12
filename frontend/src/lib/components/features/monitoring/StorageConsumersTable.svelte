<script lang="ts">
	import {
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from '$lib/components/ui';

	interface Props {
		topConsumers: {
			name: string;
			type: string;
			size: string;
			detail: string;
			reclaimable: boolean;
		}[];
	}

	let { topConsumers }: Props = $props();
</script>

<div
	class="relative overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
>
	<div
		class="flex items-center justify-between border-b border-[var(--border)] bg-[var(--bg-table-header)] px-5 py-3"
	>
		<h4 class="text-xs font-semibold text-[var(--text-primary)]">Largest disk consumers</h4>
		<span class="font-mono text-[11px] text-[var(--text-tertiary)]">Ranked by size</span>
	</div>

	<div class="overflow-x-auto md:overflow-x-visible">
		<Table class="w-full text-left text-xs">
			<TableHeader>
				<TableRow
					class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)] text-[11px] font-medium text-[var(--text-secondary)]"
				>
					<TableHead class="px-5 py-3 font-medium">Artifact name</TableHead>
					<TableHead class="px-4 py-3 font-medium">Type</TableHead>
					<TableHead class="px-4 py-3 font-medium">Context / owner</TableHead>
					<TableHead class="px-4 py-3 font-medium">Size</TableHead>
					<TableHead class="px-5 py-3 text-right font-medium">Status</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-[var(--border-subtle)]">
				{#each topConsumers as item, idx}
					<TableRow
						class="transition-colors hover:bg-[var(--bg-hover)] {idx % 2 === 1
							? 'bg-[var(--bg-table-row-alt)]'
							: 'bg-[var(--bg-table-row)]'}"
					>
						<TableCell
							class="max-w-[280px] truncate px-5 py-3 font-mono text-[11.5px] text-[var(--text-primary)]"
							title={item.name}
						>
							{item.name}
						</TableCell>
						<TableCell class="px-4 py-3 capitalize">
							<span
								class="rounded bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-secondary)]"
							>
								{item.type}
							</span>
						</TableCell>
						<TableCell class="px-4 py-3 text-[11.5px] text-[var(--text-secondary)]">
							{item.detail}
						</TableCell>
						<TableCell class="px-4 py-3 font-mono font-medium text-[var(--text-primary)]">
							{item.size}
						</TableCell>
						<TableCell class="px-5 py-3 text-right">
							{#if item.reclaimable}
								<span
									class="rounded bg-[var(--status-green-muted)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--status-green)]"
								>
									Reclaimable
								</span>
							{:else}
								<span
									class="rounded bg-[var(--bg-panel)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-tertiary)]"
								>
									In use
								</span>
							{/if}
						</TableCell>
					</TableRow>
				{/each}
			</TableBody>
		</Table>
	</div>
</div>
