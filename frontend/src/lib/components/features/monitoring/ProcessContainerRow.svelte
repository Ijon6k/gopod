<script lang="ts">
	import type { Container } from '$lib/types';
	import { StatusBadge } from '$lib/components/ui';
	import { Cube, Terminal, FileText } from 'phosphor-svelte';
	import { getCpuColor, getMemPercent, getMemColor } from './processManagerHelpers';

	interface Props {
		container: Container;
		isTree?: boolean;
		rowIndex?: number;
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (serviceId: string, containerName?: string) => void;
	}

	let { container: c, isTree = false, rowIndex = 0, onOpenTerminal, onOpenLogs }: Props = $props();

	let limit = $derived(c.memoryLimit || 512);
	let memPct = $derived(getMemPercent(c.memory, limit));
</script>

<tr
	class="group border-b border-[var(--border-subtle)] transition-colors hover:bg-[var(--bg-table-row-hover)] {rowIndex %
		2 ===
	1
		? 'bg-[var(--bg-table-row-alt)]'
		: 'bg-[var(--bg-table-row)]'}"
>
	<td class="px-3.5 py-2 align-middle text-[13px] whitespace-nowrap text-[var(--text-primary)]">
		<div class="flex min-w-0 items-center justify-between pr-2">
			{#if isTree}
				<div class="flex min-w-0 items-center gap-2 pl-14">
					<span class="font-mono text-[11px] text-[var(--text-tertiary)] select-none">└─</span>
					<Cube size={15} class="shrink-0 text-[var(--accent)]" />
					<div class="flex min-w-0 items-baseline gap-2">
						<span
							class="truncate font-mono text-[13px] font-medium text-[var(--text-primary)]"
							title={c.name}
						>
							{c.name}
						</span>
						<span
							class="truncate font-mono text-[11px] text-[var(--text-tertiary)]"
							title={c.image}
						>
							{c.image}
						</span>
					</div>
				</div>
			{:else}
				<div class="flex min-w-0 items-center gap-2">
					<Cube size={15} class="shrink-0 text-[var(--accent)]" />
					<div class="flex min-w-0 items-baseline gap-2">
						<span
							class="truncate font-mono text-[13px] font-medium text-[var(--text-primary)]"
							title={c.name}
						>
							{c.name}
						</span>
						<span class="truncate font-mono text-[11px] text-[var(--text-tertiary)]">
							{c.projectName} / {c.serviceName} • {c.image}
						</span>
					</div>
				</div>
			{/if}

			<!-- Quick Actions (28px hit target, 14px icon) -->
			<div
				class="ml-2 flex shrink-0 items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100"
			>
				<button
					type="button"
					onclick={() => onOpenTerminal?.(c.name)}
					class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1.5 text-[var(--text-secondary)] shadow-xs transition-colors hover:bg-[var(--accent)] hover:text-black"
					title="Open terminal console"
					aria-label="Open terminal console"
				>
					<Terminal size={14} />
				</button>
				<button
					type="button"
					onclick={() => onOpenLogs?.(c.serviceId, c.name)}
					class="cursor-pointer rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1.5 text-[var(--text-secondary)] shadow-xs transition-colors hover:bg-[var(--accent)] hover:text-black"
					title="View container logs"
					aria-label="View container logs"
				>
					<FileText size={14} />
				</button>
			</div>
		</div>
	</td>

	<td class="px-3.5 py-2 align-middle whitespace-nowrap">
		<StatusBadge status={c.status} size="sm" />
	</td>

	<!-- CPU Sparkbar -->
	<td class="px-3.5 py-2 text-right align-middle font-mono whitespace-nowrap">
		<div class="flex items-center justify-end gap-2">
			<div
				class="hidden h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-[var(--bg-surface)] sm:block"
			>
				<div
					class="h-full rounded-full transition-all duration-300 {getCpuColor(c.cpu)}"
					style="width: {Math.min(c.cpu * 12, 100)}%;"
				></div>
			</div>
			<span class="text-[13px] font-semibold text-[var(--text-primary)] tabular-nums">
				{c.cpu.toFixed(1)}%
			</span>
		</div>
	</td>

	<!-- Memory -->
	<td class="px-3.5 py-2 text-right align-middle font-mono whitespace-nowrap">
		<div class="flex items-center justify-end gap-2">
			<div
				class="hidden h-1.5 w-14 shrink-0 overflow-hidden rounded-full bg-[var(--bg-surface)] sm:block"
			>
				<div
					class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
					style="width: {memPct}%;"
				></div>
			</div>
			<span class="text-[13px] text-[var(--text-primary)] tabular-nums">
				{c.memory} <span class="text-[10px] text-[var(--text-tertiary)]">/ {limit} MB</span>
			</span>
		</div>
	</td>

	<td
		class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)] tabular-nums"
	>
		{c.netTx || c.ports || '—'}
	</td>

	<td
		class="px-3.5 py-2 text-center align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)] tabular-nums"
	>
		{c.pids || 1}
	</td>

	<td
		class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)] tabular-nums"
	>
		{c.uptime || 'Active'}
	</td>
</tr>
