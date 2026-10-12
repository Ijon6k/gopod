<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		Button
	} from '$lib/components/ui';
	import { ArrowRight, CircleNotch, Trash } from 'phosphor-svelte';

	interface Props {
		storageCategories: any[];
		pruningTarget: string | null;
		onpruneCategory: (type: 'images' | 'containers' | 'volumes') => void;
	}

	let { storageCategories, pruningTarget, onpruneCategory }: Props = $props();
</script>

<div class="overflow-x-auto border-t border-[var(--border)] md:overflow-x-visible">
	<Table class="w-full text-left text-xs">
		<TableHeader>
			<TableRow
				class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)] text-[11px] font-medium text-[var(--text-secondary)]"
			>
				<TableHead class="px-5 py-3 font-medium">Subsystem</TableHead>
				<TableHead class="px-4 py-3 font-medium">Artifacts</TableHead>
				<TableHead class="px-4 py-3 font-medium">Allocated size</TableHead>
				<TableHead class="px-4 py-3 font-medium">Reclaimable</TableHead>
				<TableHead class="px-5 py-3 text-right font-medium">Quick actions</TableHead>
			</TableRow>
		</TableHeader>
		<TableBody class="divide-y divide-[var(--border-subtle)]">
			{#each storageCategories as cat, idx}
				{@const Icon = cat.icon}
				<TableRow
					class="transition-colors hover:bg-[var(--bg-hover)] {idx % 2 === 1
						? 'bg-[var(--bg-table-row-alt)]'
						: 'bg-[var(--bg-table-row)]'}"
				>
					<!-- Subsystem Name with Color Indicator -->
					<TableCell class="px-5 py-3">
						<div class="flex items-center gap-2.5">
							<span class={cat.textClass}>
								<Icon size={16} />
							</span>
							<span class="font-medium text-[var(--text-primary)]">
								{cat.name}
							</span>
						</div>
					</TableCell>

					<!-- Artifact Counts -->
					<TableCell class="px-4 py-3 text-[var(--text-secondary)]">
						<span class="font-mono font-medium text-[var(--text-primary)]">
							{cat.totalCount}
						</span>
						<span class="ml-1 text-[11px] text-[var(--text-tertiary)]">
							({cat.activeText} · {cat.inactiveText})
						</span>
					</TableCell>

					<!-- Allocated Size -->
					<TableCell class="px-4 py-3 font-mono font-semibold text-[var(--text-primary)]">
						{cat.sizeDisplay}
					</TableCell>

					<!-- Reclaimable Space -->
					<TableCell class="px-4 py-3">
						{#if cat.rawReclaimable > 0}
							<span
								class="rounded bg-[var(--status-green-muted)] px-1.5 py-0.5 font-mono text-[11px] font-medium text-[var(--status-green)]"
							>
								~{cat.reclaimableDisplay}
							</span>
						{:else}
							<span class="font-mono text-[11px] text-[var(--text-tertiary)]">—</span>
						{/if}
					</TableCell>

					<!-- Quick Actions: Direct Prune Button + Link -->
					<TableCell class="px-5 py-3 text-right">
						<div class="flex items-center justify-end gap-2">
							<Button
								variant="secondary"
								size="xs"
								disabled={pruningTarget !== null || cat.rawReclaimable === 0}
								onclick={() => onpruneCategory(cat.id as 'images' | 'containers' | 'volumes')}
							>
								{#if pruningTarget === cat.id}
									<CircleNotch size={11} class="animate-spin" />
									<span>Pruning...</span>
								{:else}
									<Trash size={11} />
									<span>Prune</span>
								{/if}
							</Button>

							<button
								type="button"
								onclick={() => goto(cat.viewPath)}
								class="inline-flex cursor-pointer items-center gap-1 border-0 bg-transparent p-1 text-[11.5px] font-medium text-[var(--accent)] hover:underline"
							>
								Manage
								<ArrowRight size={11} />
							</button>
						</div>
					</TableCell>
				</TableRow>
			{/each}
		</TableBody>
	</Table>
</div>
