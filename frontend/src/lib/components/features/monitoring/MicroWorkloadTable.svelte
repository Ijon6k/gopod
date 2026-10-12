<script lang="ts">
	import type { Service, Container } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { StatusBadge } from '$lib/components/ui';
	import {
		Badge,
		LayerCard,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from '$lib/components/ui';
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
				const detected: {
					name: string;
					image: string;
					status: string;
					cpu?: number;
					memory?: number;
				}[] = [];
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
						if (
							/^[a-zA-Z0-9_-]+:/.test(trimmed) &&
							!trimmed.startsWith('image:') &&
							!trimmed.startsWith('ports:') &&
							!trimmed.startsWith('volumes:') &&
							!trimmed.startsWith('environment:')
						) {
							if (currentName) {
								detected.push({
									name: currentName,
									image: currentImg || '—',
									status: service.status || 'stopped',
									cpu: 0,
									memory: 0
								});
							}
							currentName = trimmed.replace(':', '');
							currentImg = '';
						} else if (trimmed.startsWith('image:')) {
							currentImg = trimmed.replace('image:', '').trim().replace(/['"]/g, '');
						}
					}
				}
				if (currentName) {
					detected.push({
						name: currentName,
						image: currentImg || '—',
						status: service.status || 'stopped',
						cpu: 0,
						memory: 0
					});
				}
				if (detected.length > 0) {
					workloads = detected as any;
				}
			} catch (_) {}
		}

		if (!workloads || workloads.length === 0) {
			workloads = [
				{
					name: service.name,
					image: service.image || '—',
					status: service.status || 'stopped',
					cpu: 0,
					memory: 0
				}
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
			status: service.status === 'running' ? w.status || 'running' : service.status || 'stopped',
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

<div class="flex w-full flex-col gap-3">
	<!-- Section Header -->
	<div class="flex items-center justify-between border-b border-[var(--border)] pb-1">
		<div class="flex items-center gap-2">
			<span class="text-xs font-semibold text-[var(--text-primary)]">
				Micro-Workloads & Container Cgroups
			</span>
			<Badge variant="neutral" size="sm" class="font-mono text-[10px]">
				{serviceContainers.length}
				{serviceContainers.length > 1 ? 'containers' : 'container'}
			</Badge>
		</div>

		<div class="flex items-center gap-2 font-mono text-[11px] text-[var(--text-tertiary)]">
			<span class="text-[var(--status-green)]">● cgroups v2 active</span>
			<span>•</span>
			<span>userns: {service.advanced?.runtime.userNamespace || 'keep-id'}</span>
		</div>
	</div>

	<!-- Container Table with LayerCard wrapper -->
	<LayerCard class="overflow-hidden p-0">
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full min-w-[720px] text-left">
				<TableHeader class="sticky top-0 z-20">
					<TableRow class="border-b border-kumo-line bg-kumo-base">
						<TableHead
							class="sticky top-0 z-20 bg-kumo-base px-4 py-3 text-left text-[12px] font-semibold text-kumo-subtle normal-case"
						>
							Workload Container
						</TableHead>
						<TableHead
							class="sticky top-0 z-20 w-28 bg-kumo-base px-3.5 py-3 text-left text-[12px] font-semibold text-kumo-subtle normal-case"
						>
							Status
						</TableHead>
						<TableHead
							class="sticky top-0 z-20 w-44 bg-kumo-base px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
						>
							CPU
						</TableHead>
						<TableHead
							class="sticky top-0 z-20 w-52 bg-kumo-base px-4 py-3 text-right text-[12px] font-semibold text-kumo-subtle normal-case"
						>
							Memory Allocation
						</TableHead>
						<TableHead
							class="sticky top-0 z-20 w-16 bg-kumo-base px-3 py-3 text-center text-[12px] font-semibold text-kumo-subtle normal-case"
						>
							PIDs
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-kumo-hairline">
				{#each serviceContainers as c, idx (c.id)}
					{@const limit = c.memoryLimit || 512}
					{@const memPct = getMemPercent(c.memory, limit)}

					<!-- Distinct tokenized zebra striping on flat table + prominent hover state -->
					<TableRow
						class="group transition-colors hover:bg-[var(--bg-table-row-hover)] {idx % 2 === 1
							? 'bg-[var(--bg-table-row-alt)]'
							: 'bg-[var(--bg-table-row)]'}"
					>
						<!-- Name & Image with Hover Actions -->
						<TableCell
							class="px-4 py-2.5 align-middle text-[13px] whitespace-nowrap text-[var(--text-primary)]"
						>
							<div class="flex min-w-0 items-center justify-between pr-1">
								<div class="flex min-w-0 items-center gap-2.5">
									<div
										class="shrink-0 rounded-[var(--radius-sm)] bg-[var(--accent-muted)] p-1.5 text-[var(--accent)]"
									>
										<Cube size={15} />
									</div>
									<div class="flex min-w-0 flex-col">
										<span
											class="truncate font-mono text-[13px] font-medium text-[var(--text-primary)]"
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

								<!-- Row Quick Actions on Hover (Refactored: 14px icon, 28px hit target, high contrast) -->
								<div
									class="ml-3 flex shrink-0 items-center gap-1.5 opacity-0 transition-opacity group-hover:opacity-100"
								>
									{#if onOpenTerminal}
										<button
											type="button"
											onclick={() => onOpenTerminal?.(c.name)}
											class="flex min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-1.5 text-[var(--text-secondary)] shadow-xs transition-colors hover:bg-[var(--accent)] hover:text-black"
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
											class="flex min-h-[28px] min-w-[28px] cursor-pointer items-center justify-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-1.5 text-[var(--text-secondary)] shadow-xs transition-colors hover:bg-[var(--accent)] hover:text-black"
											title="View container logs"
											aria-label="View container logs"
										>
											<FileText size={14} />
										</button>
									{/if}
								</div>
							</div>
						</TableCell>

						<!-- Status Badge -->
						<TableCell class="px-3.5 py-2.5 align-middle text-[13px] whitespace-nowrap">
							<StatusBadge status={c.status} size="sm" />
						</TableCell>

						<!-- CPU Metric (numerical only) -->
						<TableCell class="px-4 py-2.5 text-right align-middle text-[13px] whitespace-nowrap">
							<span
								class="font-mono text-[12px] font-medium text-[var(--text-primary)] tabular-nums"
							>
								{(c.cpu || 0).toFixed(1)}%
							</span>
						</TableCell>

						<!-- Memory Metric (usage only) -->
						<TableCell class="px-4 py-2.5 text-right align-middle text-[13px] whitespace-nowrap">
							<span
								class="font-mono text-[12px] font-medium text-[var(--text-primary)] tabular-nums"
							>
								{c.memory || 0} MB
							</span>
						</TableCell>

						<!-- PIDs -->
						<TableCell
							class="px-3 py-2.5 text-center align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)]"
						>
							{c.pids ?? (c.status === 'running' ? 1 : 0)}
						</TableCell>
					</TableRow>
				{/each}
			</TableBody>
		</Table>
	</div>
</LayerCard>
</div>
