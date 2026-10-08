<script lang="ts">
	import type { Service, Container } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { StatusBadge } from '$lib/components/ui';
	import { Chip } from '$lib/components/primitives';
	import { getCpuColor, getMemPercent, getMemColor } from '$lib/utils/format';
	import { Cube, Terminal, FileText } from 'phosphor-svelte';

	interface Props {
		service: Service;
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (containerName: string) => void;
	}

	let { service, onOpenTerminal, onOpenLogs }: Props = $props();

	// Match containers from dataStore for this service (serviceId, compose project, prefix, or service name)
	let serviceContainers = $derived.by<Container[]>(() => {
		const storeContainers = dataStore.containers.filter(
			(c) =>
				c.serviceId === service.id ||
				(c.labels &&
					(c.labels['io.gopod.service'] === service.id ||
						c.labels['com.docker.compose.project'] === service.id ||
						c.labels['io.podman.compose.project'] === service.id ||
						c.labels['com.docker.compose.project'] === `${service.projectId}-${service.name}` ||
						c.labels['io.podman.compose.project'] === `${service.projectId}-${service.name}`)) ||
				c.name === service.id ||
				c.name === service.name ||
				(service.id && (c.name.startsWith(`${service.id}-`) || c.name.startsWith(`${service.id}_`)))
		);
		if (storeContainers.length > 0) return storeContainers;

		// Fallback from workloads or compose YAML or service identity
		let workloads = service.workloads;
		if (!workloads && service.composeYaml) {
			try {
				const lines = service.composeYaml.split('\n');
				const detected: { name: string; image: string; status: string; cpu?: number; memory?: number }[] = [];
				let inServices = false;
				let currentName = '';
				let currentImg = '';
				for (const line of lines) {
					const trimmed = line.trim();
					if (trimmed.startsWith('services:')) {
						inServices = true;
						continue;
					}
					if (inServices) {
						if (/^[a-zA-Z0-9_-]+:/.test(trimmed) && !trimmed.startsWith('image:') && !trimmed.startsWith('ports:') && !trimmed.startsWith('volumes:') && !trimmed.startsWith('environment:')) {
							if (currentName) {
								detected.push({ name: currentName, image: currentImg || '—', status: service.status || 'stopped', cpu: 0, memory: 0 });
							}
							currentName = trimmed.replace(':', '');
							currentImg = '';
						} else if (trimmed.startsWith('image:')) {
							currentImg = trimmed.replace('image:', '').trim().replace(/['"]/g, '');
						}
					}
				}
				if (currentName) {
					detected.push({ name: currentName, image: currentImg || '—', status: service.status || 'stopped', cpu: 0, memory: 0 });
				}
				if (detected.length > 0) {
					workloads = detected as any;
				}
			} catch (_) {}
		}

		if (!workloads || workloads.length === 0) {
			workloads = [
				{ name: service.name, image: service.image || '—', status: service.status || 'stopped', cpu: 0, memory: 0 }
			];
		}

		return workloads.map((w, idx) => ({
			id: `c-${service.id}-${idx}`,
			name: w.name,
			projectId: service.projectId,
			projectName: service.projectId,
			serviceId: service.id,
			serviceName: service.name,
			image: w.image,
			status: service.status === 'running' ? (w.status || 'running') : (service.status || 'stopped'),
			cpu: service.status === 'running' ? (w.cpu ?? 0) : 0,
			memory: service.status === 'running' ? (w.memory ?? 0) : 0,
			memoryLimit: 512,
			ports: w.ports || (service.port ? `${service.port}:${service.port}` : '—'),
			startedAt: service.status === 'running' ? 'Active' : '—',
			netRx: '0 B',
			netTx: '0 B',
			blockRead: '0 B',
			blockWrite: '0 B',
			pids: service.status === 'running' ? 1 : 0,
			restarts: 0,
			uptime: service.status === 'running' ? 'Active' : 'Stopped'
		}));
	});
</script>

<div class="w-full flex flex-col gap-3">
	<!-- Section Header -->
	<div class="flex items-center justify-between pb-1 border-b border-[var(--border)]">
		<div class="flex items-center gap-2">
			<span class="text-xs font-semibold text-[var(--text-primary)]">
				Micro-Workloads & Container Cgroups
			</span>
			<Chip variant="mono" size="sm" class="text-[10px]">
				{serviceContainers.length} {serviceContainers.length > 1 ? 'containers' : 'container'}
			</Chip>
		</div>

		<div class="flex items-center gap-2 text-[11px] text-[var(--text-tertiary)] font-mono">
			<span class="text-[var(--status-green)]">● cgroups v2 active</span>
			<span>•</span>
			<span>userns: {service.advanced?.runtime.userNamespace || 'keep-id'}</span>
		</div>
	</div>

	<!-- Container Table with Inline Visual Sparkbars (Table Design Standards & Zero Slop) -->
	<div class="w-full overflow-x-auto md:overflow-x-visible rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs">
		<table class="w-full border-collapse min-w-[720px]">
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
									<div class="p-1.5 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] text-[var(--accent)] shrink-0">
										<Cube size={15} />
									</div>
									<div class="flex flex-col min-w-0">
										<span class="font-mono text-[13px] font-medium text-[var(--text-primary)] truncate">
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
											class="p-1.5 min-w-[28px] min-h-[28px] rounded-[var(--radius-sm)] bg-[var(--bg-panel)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs flex items-center justify-center"
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
											class="p-1.5 min-w-[28px] min-h-[28px] rounded-[var(--radius-sm)] bg-[var(--bg-panel)] hover:bg-[var(--accent)] hover:text-black border border-[var(--border)] text-[var(--text-secondary)] cursor-pointer transition-colors shadow-xs flex items-center justify-center"
											title="View container logs"
											aria-label="View container logs"
										>
											<FileText size={14} />
										</button>
									{/if}
								</div>
							</div>
						</td>

						<!-- Status Badge -->
						<td class="px-3.5 py-2.5 text-[13px] align-middle whitespace-nowrap">
							<StatusBadge status={c.status} size="sm" />
						</td>

						<!-- CPU Metric + Linear Progress Bar -->
						<td class="px-4 py-2.5 text-[13px] align-middle text-right whitespace-nowrap">
							<div class="flex items-center justify-end gap-2.5">
								<div class="w-16 h-1 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block">
									<div
										class="h-full rounded-full transition-all duration-300 {getCpuColor(c.cpu)}"
										style="width: {Math.min((c.cpu || 0) * 10, 100)}%;"
									></div>
								</div>
								<span class="font-mono text-[12px] font-semibold text-[var(--text-primary)] tabular-nums min-w-[42px]">
									{(c.cpu || 0).toFixed(1)}%
								</span>
							</div>
						</td>

						<!-- Memory Metric + Limit Sparkbar -->
						<td class="px-4 py-2.5 text-[13px] align-middle text-right whitespace-nowrap">
							<div class="flex items-center justify-end gap-2.5">
								<div class="w-16 h-1 rounded-full bg-[var(--bg-surface)] overflow-hidden hidden sm:block">
									<div
										class="h-full rounded-full transition-all duration-300 {getMemColor(memPct)}"
										style="width: {memPct}%;"
									></div>
								</div>
								<span class="font-mono text-[12px] font-medium text-[var(--text-secondary)] tabular-nums">
									<strong class="text-[var(--text-primary)] font-semibold">{c.memory || 0}</strong> / {limit} MB
								</span>
							</div>
						</td>

						<!-- PIDs -->
						<td class="px-3 py-2.5 text-[13px] align-middle text-center whitespace-nowrap font-mono text-[12px] text-[var(--text-secondary)]">
							{c.pids ?? (c.status === 'running' ? 1 : 0)}
						</td>

						<!-- Uptime -->
						<td class="px-4 py-2.5 text-[13px] align-middle text-right whitespace-nowrap font-mono text-[11px] text-[var(--text-tertiary)]">
							{c.uptime || c.startedAt || 'Active'}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
