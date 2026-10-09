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

	let isAllExpanded = $derived(projectTree.every((p) => openProjects[p.project.id] !== false));

	function toggleAllExpanded() {
		const nextState = !isAllExpanded;
		projectTree.forEach((p) => {
			openProjects[p.project.id] = nextState;
			p.serviceNodes.forEach((s) => (openServices[s.service.id] = nextState));
		});
	}
</script>

<div class="flex w-full flex-col gap-3">
	<!-- ══════════════════════════════════════════════════════════════
	     UNIFIED COMPACT TOOLBAR
	     ══════════════════════════════════════════════════════════════ -->
	<div class="flex flex-col justify-between gap-2.5 sm:flex-row sm:items-center">
		<!-- Left: View Mode Toggle & Tree Expander -->
		<div class="flex items-center gap-2">
			{#if initialViewMode === 'system'}
				<span class="text-xs font-semibold text-[var(--text-secondary)]">
					System Daemons ({systemContainers.length})
				</span>
			{:else}
				<div
					class="inline-flex items-center rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-0.5"
				>
					<button
						type="button"
						onclick={() => (viewMode = 'tree')}
						class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 text-xs transition-colors {viewMode ===
						'tree'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
							: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
						title="Hierarchical Project / Service / Container tree"
					>
						<TreeStructure size={13} />
						<span>Tree</span>
					</button>

					<button
						type="button"
						onclick={() => (viewMode = 'flat')}
						class="flex cursor-pointer items-center gap-1.5 rounded border-0 px-2.5 py-1 text-xs transition-colors {viewMode ===
						'flat'
							? 'bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
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
						class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-panel)] hover:text-[var(--text-primary)]"
						title={isAllExpanded
							? 'Collapse all projects & services'
							: 'Expand all projects & services'}
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
					class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border px-2.5 py-1 text-xs transition-colors {filterAnomaliesOnly
						? 'border-[var(--status-amber)] bg-[var(--status-amber-muted)] font-medium text-[var(--status-amber)]'
						: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--status-amber)]'}"
				>
					<Warning size={13} class="text-[var(--status-amber)]" />
					<span>{anomalyContainers.length} Anomalies</span>
				</button>
			{/if}

			<div class="w-56">
				<SearchInput bind:value={searchQuery} placeholder="Filter containers, images..." />
			</div>
		</div>
	</div>

	<!-- ══════════════════════════════════════════════════════════════
	     PROCESS & WORKLOAD TABLE
	     ══════════════════════════════════════════════════════════════ -->
	<div
		class="w-full overflow-x-auto rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs md:overflow-x-visible"
	>
		<table class="w-full min-w-[700px] border-collapse">
			<thead class="sticky top-0 z-20">
				<tr class="border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<th
						class="sticky top-0 z-20 cursor-pointer border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-left text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase transition-colors select-none hover:text-[var(--text-primary)]"
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

					<th
						class="sticky top-0 z-20 w-24 border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-left text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase"
					>
						Status
					</th>

					<th
						class="sticky top-0 z-20 w-36 cursor-pointer border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-right text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase transition-colors select-none hover:text-[var(--text-primary)]"
						onclick={() => handleSort('cpu')}
						title="Click to sort by CPU usage"
					>
						<div class="inline-flex w-full items-center justify-end gap-1.5">
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
						class="sticky top-0 z-20 w-44 cursor-pointer border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-right text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase transition-colors select-none hover:text-[var(--text-primary)]"
						onclick={() => handleSort('memory')}
						title="Click to sort by Memory usage"
					>
						<div class="inline-flex w-full items-center justify-end gap-1.5">
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

					<th
						class="sticky top-0 z-20 w-24 border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-right text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase"
					>
						Net I/O
					</th>

					<th
						class="sticky top-0 z-20 w-16 cursor-pointer border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-center text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase transition-colors select-none hover:text-[var(--text-primary)]"
						onclick={() => handleSort('pids')}
						title="Click to sort by PIDs"
					>
						<div class="inline-flex w-full items-center justify-center gap-1">
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

					<th
						class="sticky top-0 z-20 w-28 border-b border-[var(--border)] bg-[var(--bg-table-header)] px-3.5 py-2.5 text-right text-[11px] font-semibold tracking-wider text-[var(--text-tertiary)] uppercase"
					>
						Uptime
					</th>
				</tr>
			</thead>

			<tbody>
				{#if viewMode === 'tree'}
					<!-- HIERARCHICAL TREE VIEW -->
					{#if projectTree.length === 0}
						<tr>
							<td
								colspan="7"
								class="px-4 py-8 text-center font-mono text-xs text-[var(--text-tertiary)]"
							>
								No processes or workloads match the current filters.
							</td>
						</tr>
					{:else}
						{#each projectTree as pNode (pNode.project.id)}
							{@const isProjOpen = openProjects[pNode.project.id] ?? true}

							<!-- LEVEL 1: PROJECT ROW -->
							<tr
								class="border-b border-[var(--border-subtle)] bg-[var(--bg-table-group)] transition-colors hover:bg-[var(--bg-table-row-hover)]"
							>
								<td
									class="px-3.5 py-2 align-middle text-[13px] whitespace-nowrap text-[var(--text-primary)]"
								>
									<div class="flex min-w-0 items-center gap-2">
										<button
											type="button"
											onclick={() => toggleProject(pNode.project.id)}
											class="-ml-1 cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]"
											title={isProjOpen ? 'Collapse project' : 'Expand project'}
										>
											{#if isProjOpen}
												<CaretDown size={14} />
											{:else}
												<CaretRight size={14} />
											{/if}
										</button>

										<Folder size={16} class="shrink-0 text-[var(--accent)]" />

										<span class="text-[13.5px] font-bold tracking-tight text-[var(--text-primary)]">
											{pNode.project.name}
										</span>

										<span
											class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--text-tertiary)]"
										>
											{pNode.totalContainers}
											{pNode.totalContainers > 1 ? 'containers' : 'container'}
										</span>
									</div>
								</td>

								<td
									class="px-3.5 py-2 align-middle text-xs whitespace-nowrap text-[var(--text-secondary)]"
								>
									<StatusBadge status="running" label="Active" size="sm" />
								</td>

								<td
									class="px-3.5 py-2 text-right align-middle font-mono text-[13.5px] font-bold whitespace-nowrap text-[var(--text-primary)] tabular-nums"
								>
									{pNode.totalCpu.toFixed(1)}%
								</td>

								<td
									class="px-3.5 py-2 text-right align-middle font-mono text-[13.5px] font-bold whitespace-nowrap text-[var(--text-primary)] tabular-nums"
								>
									{pNode.totalMem} MB
								</td>

								<td
									class="px-3.5 py-2 text-right align-middle font-mono text-xs whitespace-nowrap text-[var(--text-tertiary)]"
								>
									Aggregate
								</td>

								<td
									class="px-3.5 py-2 text-center align-middle font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
								>
									—
								</td>

								<td
									class="px-3.5 py-2 text-right align-middle font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
								>
									—
								</td>
							</tr>

							<!-- LEVEL 2: SERVICE ROWS -->
							{#if isProjOpen}
								{#each pNode.serviceNodes as sNode (sNode.service.id)}
									{@const isSvcOpen = openServices[sNode.service.id] ?? true}
									{@const hasMultiple = sNode.containers.length > 1}

									<tr
										class="border-b border-[var(--border-subtle)] bg-[var(--bg-table-subgroup)] transition-colors hover:bg-[var(--bg-table-row-hover)]"
									>
										<td
											class="px-3.5 py-2 align-middle text-[13px] whitespace-nowrap text-[var(--text-primary)]"
										>
											<div class="flex min-w-0 items-center gap-2 pl-7">
												{#if hasMultiple}
													<button
														type="button"
														onclick={() => toggleService(sNode.service.id)}
														class="-ml-1 cursor-pointer border-0 bg-transparent p-1 text-[var(--text-tertiary)] transition-colors hover:text-[var(--text-primary)]"
														title={isSvcOpen ? 'Collapse service' : 'Expand service'}
													>
														{#if isSvcOpen}
															<CaretDown size={14} />
														{:else}
															<CaretRight size={14} />
														{/if}
													</button>
												{:else}
													<span class="inline-block w-4"></span>
												{/if}

												<Gear size={15} class="shrink-0 text-[var(--text-tertiary)]" />

												<span class="truncate text-[13px] font-semibold text-[var(--text-primary)]">
													{sNode.service.name}
												</span>

												<span
													class="rounded border border-[var(--border)] bg-[var(--bg-surface)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-tertiary)]"
												>
													{sNode.service.type}
												</span>
											</div>
										</td>

										<td class="px-3.5 py-2 align-middle whitespace-nowrap">
											<StatusBadge status={sNode.service.status} size="sm" />
										</td>

										<td
											class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-primary)] tabular-nums"
										>
											{sNode.totalCpu.toFixed(1)}%
										</td>

										<td
											class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-primary)] tabular-nums"
										>
											{sNode.totalMem} MB
										</td>

										<td
											class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)]"
										>
											{sNode.service.port ? `:${sNode.service.port}` : '—'}
										</td>

										<td
											class="px-3.5 py-2 text-center align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)]"
										>
											{sNode.totalPids || '—'}
										</td>

										<td
											class="px-3.5 py-2 text-right align-middle font-mono text-[13px] whitespace-nowrap text-[var(--text-secondary)]"
										>
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
							<td
								colspan="7"
								class="px-4 py-8 text-center font-mono text-xs text-[var(--text-tertiary)]"
							>
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
