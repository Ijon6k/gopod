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

	let {
		container: c,
		isTree = false,
		rowIndex = 0,
		onOpenTerminal,
		onOpenLogs
	}: Props = $props();

	let limit = $derived(c.memoryLimit || 512);
	let memPct = $derived(getMemPercent(c.memory, limit));
</script>

<tr
	class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] group {rowIndex % 2 === 1
		? 'bg-[var(--bg-table-row-alt)]'
		: 'bg-[var(--bg-table-row)]'}"
>
	<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
		<div class="flex items-center justify-between min-w-0 pr-2">
			{#if isTree}
				<div class="flex items-center gap-2 pl-14 min-w-0">
					<span class="text-[var(--text-tertiary)] font-mono select-none text-[11px]">└─</span>
					<Cube size={15} class="text-[var(--accent)] shrink-0" />
					<div class="flex items-baseline gap-2 min-w-0">
						<span class="font-mono text-[13px] text-[var(--text-primary)] font-medium truncate" title={c.name}>
							{c.name}
						</span>
						<span class="text-[11px] text-[var(--text-tertiary)] truncate font-mono" title={c.image}>
							{c.image}
						</span>
					</div>
				</div>
			{:else}
				<div class="flex items-center gap-2 min-w-0">
					<Cube size={15} class="text-[var(--accent)] shrink-0" />
					<div class="flex items-baseline gap-2 min-w-0">
						<span class="font-mono text-[13px] font-medium text-[var(--text-primary)] truncate" title={c.name}>
							{c.name}
						</span>
						<span class="text-[11px] text-[var(--text-tertiary)] font-mono truncate">
							{c.projectName} / {c.serviceName} • {c.image}
						</span>
					</div>
				</div>
			{/if}

			<!-- Quick Actions (28px hit target, 14px icon) -->
			<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1.5 shrink-0 ml-2">
				<button
					type="button"
					onclick={() => onOpenTerminal?.(c.name)}
					class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
					title="Open terminal console"
					aria-label="Open terminal console"
				>
					<Terminal size={14} />
				</button>
				<button
					type="button"
					onclick={() => onOpenLogs?.(c.serviceId, c.name)}
					class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
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
	<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
		<div class="flex items-center justify-end gap-2">
			<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
				<div
					class="h-full rounded-full transition-all duration-300 {getCpuColor(c.cpu)}"
					style="width: {Math.min(c.cpu * 12, 100)}%;"
				></div>
			</div>
			<span class="text-[13px] font-semibold tabular-nums text-[var(--text-primary)]">
				{c.cpu.toFixed(1)}%
			</span>
		</div>
	</td>

	<!-- Memory -->
	<td class="px-3.5 py-2 text-right font-mono align-middle whitespace-nowrap">
		<div class="flex items-center justify-end gap-2">
			<div class="w-14 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
				<div
					class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
					style="width: {memPct}%;"
				></div>
			</div>
			<span class="text-[13px] tabular-nums text-[var(--text-primary)]">
				{c.memory} <span class="text-[10px] text-[var(--text-tertiary)]">/ {limit} MB</span>
			</span>
		</div>
	</td>

	<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
		{c.netTx || c.ports || '—'}
	</td>

	<td class="px-3.5 py-2 text-center text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
		{c.pids || 1}
	</td>

	<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
		{c.uptime || 'Active'}
	</td>
</tr>
