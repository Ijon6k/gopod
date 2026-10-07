<script lang="ts">
	import { dataStore } from '$lib/data';
	import type { Container } from '$lib/types';
	import { StatusBadge, SearchInput } from '$lib/components/ui';
	import {
		CaretDown,
		CaretRight,
		CaretUp,
		Folder,
		Gear,
		Warning,
		TreeStructure,
		List,
		ArrowsOutSimple,
		ArrowsInSimple
	} from 'phosphor-svelte';
	import { isAnomaly, buildProjectTree } from './processManagerHelpers';
	import ProcessContainerRow from './ProcessContainerRow.svelte';

	interface Props {
		onOpenTerminal?: (containerName: string) => void;
		onOpenLogs?: (serviceId: string, containerName?: string) => void;
		initialViewMode?: 'tree' | 'flat' | 'system';
	}

	let { onOpenTerminal, onOpenLogs, initialViewMode = 'tree' }: Props = $props();

	let viewMode = $state<'tree' | 'flat' | 'system'>('tree');
	let searchQuery = $state('');
	let sortBy = $state<'cpu' | 'memory' | 'name' | 'pids'>('cpu');
	let sortDesc = $state(true);
	let filterAnomaliesOnly = $state(false);

	$effect(() => {
		if (initialViewMode) {
			viewMode = initialViewMode;
		}
	});

	let openProjects = $state<Record<string, boolean>>({});
	let openServices = $state<Record<string, boolean>>({});

	function toggleProject(id: string) {
		openProjects[id] = openProjects[id] === false ? true : false;
	}

	function toggleService(id: string) {
		openServices[id] = openServices[id] === false ? true : false;
	}

	function handleSort(column: 'cpu' | 'memory' | 'name' | 'pids') {
		if (sortBy === column) {
			sortDesc = !sortDesc;
		} else {
			sortBy = column;
		}
	}

	let systemContainers = $derived(
		dataStore.containers.filter(
			(c) =>
				c.projectId === 'system' ||
				c.name.includes('gopod') ||
				c.name.includes('caddy') ||
				c.name.includes('traefik')
		)
	);

	let allContainers = $derived(viewMode === 'system' ? systemContainers : dataStore.containers);
	let anomalyContainers = $derived(allContainers.filter(isAnomaly));

	let filteredContainers = $derived.by(() => {
		let list = [...allContainers];
		if (filterAnomaliesOnly) {
			list = list.filter(isAnomaly);
		}
		if (searchQuery.trim()) {
			const q = searchQuery.toLowerCase().trim();
			list = list.filter(
				(c) =>
					c.name.toLowerCase().includes(q) ||
					c.image.toLowerCase().includes(q) ||
					c.projectName.toLowerCase().includes(q) ||
					c.serviceName.toLowerCase().includes(q)
			);
		}

		list.sort((a, b) => {
			let valA: any = a[sortBy] ?? 0;
			let valB: any = b[sortBy] ?? 0;
			if (typeof valA === 'string') {
				return sortDesc ? valB.localeCompare(valA) : valA.localeCompare(valB);
			}
			return sortDesc ? valB - valA : valA - valB;
		});

		return list;
	});

	let projectTree = $derived(
		buildProjectTree(
			dataStore.projects,
			allContainers,
			(id) => dataStore.getProjectServices(id),
			searchQuery,
			filterAnomaliesOnly,
			sortBy,
			sortDesc
		)
	);

	let isAllExpanded = $derived(
		projectTree.every((p) => openProjects[p.project.id] !== false)
	);

	function toggleAllExpanded() {
		const nextState = !isAllExpanded;
		projectTree.forEach((p) => {
			openProjects[p.project.id] = nextState;
			p.serviceNodes.forEach((s) => (openServices[s.service.id] = nextState));
		});
	}
</script>

<div class="w-full flex flex-col gap-3">
	<!-- ══════════════════════════════════════════════════════════════
	     UNIFIED COMPACT TOOLBAR
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
		<!-- Left: View Mode Toggle & Tree Expander -->
		<div class="flex items-center gap-2">
			{#if initialViewMode === 'system'}
				<span class="text-xs font-semibold text-[var(--text-secondary)]">
					System Daemons ({systemContainers.length})
				</span>
			{:else}
				<div class="inline-flex items-center p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)]">
					<button
						type="button"
						onclick={() => (viewMode = 'tree')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'tree'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Hierarchical Project / Service / Container tree"
					>
						<TreeStructure size={13} />
						<span>Tree</span>
					</button>

					<button
						type="button"
						onclick={() => (viewMode = 'flat')}
						class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer flex items-center gap-1.5 {viewMode === 'flat'
							? 'bg-[var(--bg-surface)] text-[var(--text-primary)] font-medium shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Flat container process list"
					>
						<List size={13} />
						<span>Flat ({dataStore.containers.length})</span>
					</button>
				</div>

				{#if viewMode === 'tree'}
					<button
						type="button"
						onclick={toggleAllExpanded}
						class="px-2.5 py-1 rounded-[var(--radius-sm)] text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-panel)] border border-[var(--border)] transition-colors cursor-pointer flex items-center gap-1.5"
						title={isAllExpanded ? 'Collapse all projects & services' : 'Expand all projects & services'}
					>
						{#if isAllExpanded}
							<ArrowsInSimple size={13} />
							<span>Collapse</span>
						{:else}
							<ArrowsOutSimple size={13} />
							<span>Expand</span>
						{/if}
					</button>
				{/if}
			{/if}
		</div>

		<!-- Right: Anomalies filter + Search -->
		<div class="flex items-center gap-2">
			{#if anomalyContainers.length > 0}
				<button
					type="button"
					onclick={() => (filterAnomaliesOnly = !filterAnomaliesOnly)}
					class="px-2.5 py-1 rounded-[var(--radius-sm)] text-xs transition-colors border cursor-pointer flex items-center gap-1.5 {filterAnomaliesOnly
						? 'bg-[var(--status-amber-muted)] border-[var(--status-amber)] text-[var(--status-amber)] font-medium'
						: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--status-amber)]'}"
				>
					<Warning size={13} class="text-[var(--status-amber)]" />
					<span>{anomalyContainers.length} Anomalies</span>
				</button>
			{/if}

			<div class="w-56">
				<SearchInput
					bind:value={searchQuery}
					placeholder="Filter containers, images..."
				/>
			</div>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     PROCESS & WORKLOAD TABLE
	     ══════════════════════════════════════════════════════════════ -->
	<div class="w-full overflow-x-auto md:overflow-x-visible rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs">
		<table class="w-full border-collapse min-w-[700px]">
			<thead class="sticky top-0 z-20">
				<tr class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('name')}
						title="Click to sort by name"
					>
						<div class="inline-flex items-center gap-1.5">
							<span>{viewMode === 'tree' ? 'Workload / Entity Tree' : 'Workload Name'}</span>
							{#if sortBy === 'name'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-left w-24">
						Status
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-36 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('cpu')}
						title="Click to sort by CPU usage"
					>
						<div class="inline-flex items-center justify-end gap-1.5 w-full">
							<span>CPU Utilization</span>
							{#if sortBy === 'cpu'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-44 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('memory')}
						title="Click to sort by Memory usage"
					>
						<div class="inline-flex items-center justify-end gap-1.5 w-full">
							<span>Memory</span>
							{#if sortBy === 'memory'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-24">
						Net I/O
					</th>

					<th
						class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-center w-16 cursor-pointer hover:text-[var(--text-primary)] transition-colors select-none"
						onclick={() => handleSort('pids')}
						title="Click to sort by PIDs"
					>
						<div class="inline-flex items-center justify-center gap-1 w-full">
							<span>PIDs</span>
							{#if sortBy === 'pids'}
								{#if sortDesc}
									<CaretDown size={12} class="text-[var(--accent)]" />
								{:else}
									<CaretUp size={12} class="text-[var(--accent)]" />
								{/if}
							{/if}
						</div>
					</th>

					<th class="sticky top-0 z-20 bg-[var(--bg-table-header)] border-b border-[var(--border)] px-3.5 py-2.5 text-[11px] font-semibold text-[var(--text-tertiary)] tracking-wider uppercase text-right w-28">
						Uptime
					</th>
				</tr>
			</thead>

			<tbody>
				{#if viewMode === 'tree'}
					<!-- HIERARCHICAL TREE VIEW -->
					{#if projectTree.length === 0}
						<tr>
							<td colspan="7" class="px-4 py-8 text-center text-xs text-[var(--text-tertiary)] font-mono">
								No processes or workloads match the current filters.
							</td>
						</tr>
					{:else}
						{#each projectTree as pNode (pNode.project.id)}
							{@const isProjOpen = openProjects[pNode.project.id] ?? true}

							<!-- LEVEL 1: PROJECT ROW -->
							<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] bg-[var(--bg-table-group)]">
								<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
									<div class="flex items-center gap-2 min-w-0">
										<button
											type="button"
											onclick={() => toggleProject(pNode.project.id)}
											class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
											title={isProjOpen ? 'Collapse project' : 'Expand project'}
										>
											{#if isProjOpen}
												<CaretDown size={14} />
											{:else}
												<CaretRight size={14} />
											{/if}
										</button>

										<Folder size={16} class="text-[var(--accent)] shrink-0" />

										<span class="font-bold text-[13.5px] text-[var(--text-primary)] tracking-tight">
											{pNode.project.name}
										</span>

										<span class="text-[11px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border)]">
											{pNode.totalContainers} {pNode.totalContainers > 1 ? 'containers' : 'container'}
										</span>
									</div>
								</td>

								<td class="px-3.5 py-2 text-xs text-[var(--text-secondary)] align-middle whitespace-nowrap">
									<StatusBadge status="running" label="Active" size="sm" />
								</td>

								<td class="px-3.5 py-2 text-right text-[13.5px] font-mono font-bold text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalCpu.toFixed(1)}%
								</td>

								<td class="px-3.5 py-2 text-right text-[13.5px] font-mono font-bold text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
									{pNode.totalMem} MB
								</td>

								<td class="px-3.5 py-2 text-right text-xs font-mono text-[var(--text-tertiary)] align-middle whitespace-nowrap">
									Aggregate
								</td>

								<td class="px-3.5 py-2 text-center text-xs font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>

								<td class="px-3.5 py-2 text-right text-xs font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
									—
								</td>
							</tr>

							<!-- LEVEL 2: SERVICE ROWS -->
							{#if isProjOpen}
								{#each pNode.serviceNodes as sNode (sNode.service.id)}
									{@const isSvcOpen = openServices[sNode.service.id] ?? true}
									{@const hasMultiple = sNode.containers.length > 1}

									<tr class="transition-colors hover:bg-[var(--bg-table-row-hover)] border-b border-[var(--border-subtle)] bg-[var(--bg-table-subgroup)]">
										<td class="px-3.5 py-2 text-[13px] text-[var(--text-primary)] align-middle whitespace-nowrap">
											<div class="flex items-center gap-2 pl-7 min-w-0">
												{#if hasMultiple}
													<button
														type="button"
														onclick={() => toggleService(sNode.service.id)}
														class="p-1 -ml-1 text-[var(--text-tertiary)] hover:text-[var(--text-primary)] border-0 bg-transparent cursor-pointer transition-colors"
														title={isSvcOpen ? 'Collapse service' : 'Expand service'}
													>
														{#if isSvcOpen}
															<CaretDown size={14} />
														{:else}
															<CaretRight size={14} />
														{/if}
													</button>
												{:else}
													<span class="w-4 inline-block"></span>
												{/if}

												<Gear size={15} class="text-[var(--text-tertiary)] shrink-0" />

												<span class="font-semibold text-[13px] text-[var(--text-primary)] truncate">
													{sNode.service.name}
												</span>

												<span class="text-[10px] font-mono text-[var(--text-tertiary)] bg-[var(--bg-surface)] px-1.5 py-0.5 rounded border border-[var(--border)]">
													{sNode.service.type}
												</span>
											</div>
										</td>

										<td class="px-3.5 py-2 align-middle whitespace-nowrap">
											<StatusBadge status={sNode.service.status} size="sm" />
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalCpu.toFixed(1)}%
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-primary)] tabular-nums align-middle whitespace-nowrap">
											{sNode.totalMem} MB
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.service.port ? `:${sNode.service.port}` : '—'}
										</td>

										<td class="px-3.5 py-2 text-center text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											{sNode.totalPids || '—'}
										</td>

										<td class="px-3.5 py-2 text-right text-[13px] font-mono text-[var(--text-secondary)] align-middle whitespace-nowrap">
											Active
										</td>
									</tr>

									<!-- LEVEL 3: CONTAINER ROWS -->
									{#if isSvcOpen}
										{#each sNode.containers as c, cIdx (c.id)}
											<ProcessContainerRow
												container={c}
												isTree={true}
												rowIndex={cIdx}
												{onOpenTerminal}
												{onOpenLogs}
											/>
										{/each}
									{/if}
								{/each}
							{/if}
						{/each}
					{/if}
				{:else}
					<!-- FLAT PROCESS LIST -->
					{#if filteredContainers.length === 0}
						<tr>
							<td colspan="7" class="px-4 py-8 text-center text-xs text-[var(--text-tertiary)] font-mono">
								No containers found.
							</td>
						</tr>
					{:else}
						{#each filteredContainers as c, cIdx (c.id)}
							<ProcessContainerRow
								container={c}
								isTree={false}
								rowIndex={cIdx}
								{onOpenTerminal}
								{onOpenLogs}
							/>
						{/each}
					{/if}
				{/if}
			</tbody>
		</table>
	</div>
</div>
