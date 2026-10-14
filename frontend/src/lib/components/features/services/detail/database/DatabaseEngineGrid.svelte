<script lang="ts">
	import { DatabaseIcon } from '$lib/components/ui';
	import { DATABASE_ENGINES, type DatabaseEngine } from '../../database/database-utils';
	import { CheckCircle } from 'phosphor-svelte';

	interface Props {
		selectedEngineId: DatabaseEngine;
		onSelectEngine: (engineId: DatabaseEngine) => void;
	}

	let { selectedEngineId, onSelectEngine }: Props = $props();
</script>

<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
	{#each DATABASE_ENGINES as eng (eng.id)}
		{@const isSelected = selectedEngineId === eng.id}
		<button
			type="button"
			onclick={() => onSelectEngine(eng.id)}
			class="group relative flex cursor-pointer flex-col items-start rounded-[var(--radius-card)] border p-4 text-left transition-all {isSelected
				? 'border-[var(--accent)] bg-[var(--bg-surface)] shadow-sm ring-1 ring-[var(--accent)]'
				: 'border-[var(--border)] bg-[var(--bg-panel)] hover:border-[var(--border-strong)] hover:bg-[var(--bg-surface)]'}"
		>
			<!-- Top Bar: Internet Photo/Logo & Badges -->
			<div class="flex w-full items-start justify-between gap-2">
				<div
					class="flex size-14 shrink-0 items-center justify-center rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-2 shadow-2xs transition-transform group-hover:scale-105"
				>
					<DatabaseIcon engine={eng.id} size={36} useInternetImage={true} />
				</div>

				<div class="flex flex-col items-end gap-1">
					{#if isSelected}
						<span
							class="flex items-center gap-1 rounded-full bg-[var(--accent)]/15 px-2 py-0.5 text-[10px] font-semibold text-[var(--accent)]"
						>
							<CheckCircle size={12} weight="fill" /> Selected
						</span>
					{/if}
					<span
						class="rounded bg-[var(--bg-elevated)] px-1.5 py-0.5 font-mono text-[10px] font-medium text-[var(--text-tertiary)]"
					>
						Port {eng.defaultPort}
					</span>
				</div>
			</div>

			<!-- Database Info -->
			<div class="mt-3 flex w-full flex-col">
				<div class="flex items-center justify-between">
					<span class="text-sm font-semibold text-[var(--text-primary)]">
						{eng.name}
					</span>
					<span class="font-mono text-[10px] text-[var(--text-tertiary)] uppercase">
						{eng.id === 'redis'
							? 'Cache / KV'
							: eng.id === 'mongodb'
								? 'NoSQL'
								: eng.id === 'clickhouse'
									? 'OLAP'
									: eng.id === 'libsql'
										? 'SQLite'
										: eng.id === 'custom'
											? 'Docker'
											: 'SQL'}
					</span>
				</div>
				<p class="m-0 mt-1 line-clamp-2 text-xs leading-relaxed text-[var(--text-secondary)]">
					{eng.desc}
				</p>
			</div>
		</button>
	{/each}
</div>
