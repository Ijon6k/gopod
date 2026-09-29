<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { StatusBadge } from '$lib/components/ui';
	import { Chip } from '$lib/components/primitives';
	import { Cube } from 'phosphor-svelte';

	interface Props {
		service: Service;
		onOpenTerminal?: (containerName: string) => void;
	}

	let { service }: Props = $props();

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
</script>

<div class="w-full flex flex-col gap-3">
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
			<span>Cgroups v2: Active</span>
			<span>•</span>
			<span>UserNS: {service.advanced?.runtime.userNamespace || 'keep-id'}</span>
		</div>
	</div>

	<!-- Micro Container Table (Reusable DataTable Style, 14px Font, No Actions, Pure Monitoring) -->
	<div class="w-full overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]">
		<table class="w-full border-collapse min-w-[640px]">
			<thead>
				<tr class="border-b border-[var(--border)]">
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-left">
						Container Workload
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-left w-32">
						Status
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-36">
						CPU Utilization
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-44">
						Memory Allocation
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-center w-20">
						PIDs
					</th>
					<th class="px-3.5 py-2.5 text-[11px] font-medium text-[var(--text-tertiary)] tracking-[0.5px] uppercase whitespace-nowrap bg-[var(--bg-panel)] text-right w-28">
						Uptime
					</th>
				</tr>
			</thead>
			<tbody>
				{#each serviceContainers as c (c.id)}
					{@const limit = c.memoryLimit || 512}

					<tr class="transition-colors duration-100 hover:bg-[var(--bg-hover)] border-b border-[var(--border-subtle)]">
						<!-- Name & Image -->
						<td class="px-3.5 py-2.5 text-[14px] text-[var(--text-primary)] align-middle whitespace-nowrap">
							<div class="flex items-center gap-2 min-w-0">
								<Cube size={15} class="text-[var(--accent)] shrink-0" />
								<div class="flex items-baseline gap-2 min-w-0">
									<span class="font-mono text-[14px] font-medium text-[var(--text-primary)] truncate">
										{c.name}
									</span>
									<span class="font-mono text-[12px] text-[var(--text-tertiary)] truncate" title={c.image}>
										{c.image}
									</span>
								</div>
							</div>
						</td>

						<!-- Status -->
						<td class="px-3.5 py-2.5 text-[14px] align-middle whitespace-nowrap">
							<StatusBadge status={c.status} />
						</td>

						<!-- CPU -->
						<td class="px-3.5 py-2.5 text-right text-[14px] font-mono font-medium text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
							{c.cpu.toFixed(1)}%
						</td>

						<!-- Memory -->
						<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
							{c.memory} <span class="text-[12px] text-[var(--text-tertiary)] font-normal">/ {limit} MB</span>
						</td>

						<!-- PIDs -->
						<td class="px-3.5 py-2.5 text-center text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
							{c.pids || 4}
						</td>

						<!-- Uptime -->
						<td class="px-3.5 py-2.5 text-right text-[14px] font-mono text-[var(--text-secondary)] tabular-nums align-middle whitespace-nowrap">
							{c.uptime || 'Up 8d'}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
</div>
