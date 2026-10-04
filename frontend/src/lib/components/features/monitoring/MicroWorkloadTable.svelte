<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { StatusBadge } from '$lib/components/ui';
	import { Chip } from '$lib/components/primitives';
	import { Cube, Terminal, FileText } from 'phosphor-svelte';

	interface Props {
		service: Service;
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (containerName: string) => void;
	}

	let { service, onOpenTerminal, onOpenLogs }: Props = $props();

	// Match containers from dataStore for this service
	let serviceContainers = $derived.by(() => {
		const storeContainers = dataStore.containers.filter((c) => c.serviceId === service.id);
		if (storeContainers.length > 0) return storeContainers;

		// Fallback from workloads or service identity
		const workloads = service.workloads ?? [
			{ name: service.name, image: service.image || 'app:latest', status: service.status, cpu: service.cpu, memory: service.memory }
		];

		return workloads.map((w, idx) => ({
			id: `c-${service.id}-${idx}`,
			name: w.name,
			projectId: service.projectId,
			projectName: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			image: w.image,
			status: w.status,
			cpu: w.cpu ?? 0.8,
			memory: w.memory ?? 180,
			memoryLimit: 512,
			ports: w.ports || `${service.port || 3000}:${service.port || 3000}`,
			startedAt: '2 hours ago',
			netRx: '4.2 MB',
			netTx: '18.4 MB',
			blockRead: '0.2 MB',
			blockWrite: '0.6 MB',
			pids: 14,
			restarts: 0,
			uptime: '8d 14h'
		}));
	});

	function getCpuColor(cpu: number): string {
		if (cpu >= 5.0) return 'bg-rose-500';
		if (cpu >= 2.0) return 'bg-amber-400';
		return 'bg-[var(--accent)]';
	}

	function getMemPercent(used: number, limit: number): number {
		return Math.min(Math.round((used / (limit || 512)) * 100), 100);
	}

	function getMemColor(percent: number): string {
		if (percent >= 85) return 'bg-rose-500';
		if (percent >= 70) return 'bg-amber-400';
		return 'bg-emerald-400';
	}
</script>

<div class="w-full flex flex-col gap-3">
	<!-- Section Header -->
	<div class="flex items-center justify-between pb-1 border-b border-[var(--border-subtle)]">
		<div class="flex items-center gap-2">
			<span class="text-xs font-semibold text-[var(--text-primary)]">
				Micro-Workloads & Container Cgroups
			</span>
			<Chip variant="mono" size="sm" class="text-[10px]">
				{serviceContainers.length} {serviceContainers.length > 1 ? 'containers' : 'container'}
			</Chip>
		</div>

		<div class="flex items-center gap-2 text-[11px] text-[var(--text-tertiary)] font-mono">
			<span class="text-emerald-400">● cgroups v2 active</span>
			<span>•</span>
			<span>userns: {service.advanced?.runtime.userNamespace || 'keep-id'}</span>
		</div>
	</div>

	<!-- Container Table with Inline Visual Sparkbars (Refactoring UI: Scannable Data & Sub-second Outlier Spotting) -->
	<div class="w-full overflow-x-auto md:overflow-x-visible rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs">
		<table class="w-full border-collapse min-w-[700px]">
			<thead class="sticky top-0 z-20">
				<tr class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<th class="px-4 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left bg-[var(--bg-table-header)] sticky top-0 z-20">
						Workload Container
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left w-28 bg-[var(--bg-table-header)] sticky top-0 z-20">
						Status
					</th>
					<th class="px-4 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-44 bg-[var(--bg-table-header)] sticky top-0 z-20">
						CPU Utilization
					</th>
					<th class="px-4 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-52 bg-[var(--bg-table-header)] sticky top-0 z-20">
						Memory Allocation
					</th>
					<th class="px-3 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-center w-16 bg-[var(--bg-table-header)] sticky top-0 z-20">
						PIDs
					</th>
					<th class="px-4 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-24 bg-[var(--bg-table-header)] sticky top-0 z-20">
						Uptime
					</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-[var(--border-subtle)]">
				{#each serviceContainers as c, idx (c.id)}
					{@const limit = c.memoryLimit || 512}
					{@const memPct = getMemPercent(c.memory, limit)}

					<!-- Distinct tokenized zebra striping on flat table + prominent hover state -->
					<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] group {idx % 2 === 1 ? 'bg-[var(--bg-table-row-alt)]' : 'bg-[var(--bg-table-row)]'}">
						<!-- Name & Image with Hover Actions -->
						<td class="px-4 py-2.5 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
							<div class="flex items-center justify-between min-w-0 pr-1">
								<div class="flex items-center gap-2.5 min-w-0">
									<div class="p-1.5 rounded bg-[var(--accent-muted)] text-[var(--accent)] shrink-0">
										<Cube size={15} />
									</div>
									<div class="flex flex-col min-w-0">
										<span class="font-mono text-[13px] font-semibold text-[var(--text-primary)] truncate">
											{c.name}
										</span>
										<span class="font-mono text-[11px] text-[var(--text-tertiary)] truncate" title={c.image}>
											{c.image}
										</span>
									</div>
								</div>

								<!-- Row Quick Actions on Hover (Refactored: 14px icon, 28px hit target, high contrast) -->
								<div class="opacity-0 group-hover:opacity-100 transition-opacity flex items-center gap-1.5 shrink-0 ml-3">
									{#if onOpenTerminal}
										<button
											type="button"
											onclick={() => onOpenTerminal?.(c.name)}
											class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
											title="Open container terminal"
											aria-label="Open container terminal"
										>
											<Terminal size={14} />
										</button>
									{/if}
									{#if onOpenLogs}
										<button
											type="button"
											onclick={() => onOpenLogs?.(c.name)}
											class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--bg-surface)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs"
											title="View container logs"
											aria-label="View container logs"
										>
											<FileText size={14} />
										</button>
									{/if}
								</div>
							</div>
						</td>

						<!-- Status -->
						<td class="px-3.5 py-2.5 align-middle whitespace-nowrap">
							<StatusBadge status={c.status} size="sm" />
						</td>

						<!-- CPU with Micro Sparkbar (Scannable outlier detection) -->
						<td class="px-4 py-2.5 text-right font-mono align-middle whitespace-nowrap">
							<div class="flex items-center justify-end gap-2.5">
								<div class="w-16 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
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

						<!-- Memory with Micro Sparkbar -->
						<td class="px-4 py-2.5 text-right font-mono align-middle whitespace-nowrap">
							<div class="flex items-center justify-end gap-2.5">
								<div class="w-16 h-1.5 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block shrink-0">
									<div
										class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
										style="width: {memPct}%;"
									></div>
								</div>
								<span class="text-[13px] tabular-nums text-[var(--text-primary)]">
									{c.memory} <span class="text-[11px] text-[var(--text-tertiary)]">/ {limit} MB</span>
								</span>
							</div>
						</td>

						<!-- PIDs -->
						<td class="px-3.5 py-2.5 text-center text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
							{c.pids || 4}
						</td>

						<!-- Uptime -->
						<td class="px-4 py-2.5 text-right text-[13px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
							{c.uptime || 'Up 8d'}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
