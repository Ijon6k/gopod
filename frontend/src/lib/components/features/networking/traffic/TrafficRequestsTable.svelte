<script lang="ts">
	import { Button, TablePagination } from '$lib/components/ui';
	import {
		LayerCard,
		Badge,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell
	} from 'kumo-svelte';
	import type { CaddyAccessLog } from '$lib/types';
	import { Copy, Check, Globe } from 'phosphor-svelte';

	interface Props {
		filteredLogs: CaddyAccessLog[];
		paginatedLogs: CaddyAccessLog[];
		currentPage: number;
		pageSize: number;
		hasActiveFilters: boolean;
		onResetFilters: () => void;
	}

	let {
		filteredLogs,
		paginatedLogs,
		currentPage = $bindable(),
		pageSize = $bindable(),
		hasActiveFilters,
		onResetFilters
	}: Props = $props();

	let copiedId = $state<string | null>(null);

	function getStatusVariant(status: number): 'success' | 'neutral' | 'warning' | 'error' {
		if (status >= 200 && status < 300) return 'success';
		if (status === 101 || (status >= 300 && status < 400)) return 'neutral';
		if (status >= 400 && status < 500) return 'warning';
		return 'error';
	}

	function handleCopyUri(uri: string, id: string) {
		navigator.clipboard.writeText(uri);
		copiedId = id;
		setTimeout(() => {
			if (copiedId === id) copiedId = null;
		}, 1500);
	}
</script>

<LayerCard class="overflow-hidden p-0">
	<!-- Table Container with desktop sticky preservation -->
	<div class="w-full overflow-x-auto md:overflow-x-visible">
		<Table class="w-full text-left text-xs">
			<TableHeader
				class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
			>
				<TableRow>
					<TableHead
						class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Status
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Method
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Request URI and host
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Upstream service
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Latency
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Client IP
					</TableHead>
					<TableHead
						class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Time
					</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-[var(--border-subtle)]">
				{#if filteredLogs.length === 0}
					<TableRow>
						<TableCell colspan={7} class="py-14 text-center">
							<div
								class="mx-auto flex max-w-md flex-col items-center justify-center gap-2.5 text-center"
							>
								<div
									class="flex h-10 w-10 items-center justify-center rounded-lg bg-[var(--accent-muted)] text-[var(--accent)]"
								>
									<Globe size={22} />
								</div>
								<h4 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
									{#if hasActiveFilters}
										No matching requests found
									{:else}
										No Caddy reverse proxy requests recorded yet
									{/if}
								</h4>
								<p class="m-0 text-xs leading-relaxed text-[var(--text-tertiary)]">
									{#if hasActiveFilters}
										Clear active search filters to view recorded reverse proxy requests.
									{:else}
										Caddy is listening on port 80 and active domain routes. Incoming HTTP requests
										proxied through Caddy will appear here in real time.
									{/if}
								</p>
								{#if hasActiveFilters}
									<Button variant="secondary" size="sm" onclick={onResetFilters} class="mt-2">
										Clear filters
									</Button>
								{/if}
							</div>
						</TableCell>
					</TableRow>
				{:else}
					{#each paginatedLogs as log, idx (log.id)}
						{@const variant = getStatusVariant(log.status)}
						<TableRow
							class={idx % 2 === 1
								? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]'
								: 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}
						>
							<!-- Status -->
							<TableCell class="px-3.5 py-3 whitespace-nowrap">
								<Badge {variant} class="font-mono text-xs">
									{log.status}
								</Badge>
							</TableCell>

							<!-- Method -->
							<TableCell class="px-3 py-3 whitespace-nowrap">
								<Badge variant="neutral" class="font-mono text-xs">
									{log.method}
								</Badge>
							</TableCell>

							<!-- URI & Host -->
							<TableCell class="min-w-[240px] px-3 py-3">
								<div class="flex flex-col">
									<div class="flex items-center gap-1.5">
										<span
											class="max-w-md truncate font-mono text-[13px] font-medium text-[var(--text-primary)]"
										>
											{log.uri}
										</span>
										<button
											type="button"
											onclick={() => handleCopyUri(log.uri, log.id)}
											class="flex h-7 w-7 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
											aria-label="Copy request URI"
										>
											{#if copiedId === log.id}
												<Check size={14} class="text-[var(--status-green)]" />
											{:else}
												<Copy size={14} />
											{/if}
										</button>
									</div>
									<span class="truncate text-xs text-[var(--text-tertiary)]">
										{log.host}
									</span>
								</div>
							</TableCell>

							<!-- Upstream Service -->
							<TableCell
								class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
							>
								{log.upstream || log.serviceName || '—'}
							</TableCell>

							<!-- Latency -->
							<TableCell
								class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
							>
								<span
									class={(log.durationMs || 0) > 100
										? 'text-[var(--status-amber)]'
										: 'text-[var(--text-primary)]'}
								>
									{(log.durationMs || 0).toFixed(1)}ms
								</span>
								<span class="block text-[11px] text-[var(--text-tertiary)]">
									{log.bytesSent || '—'}
								</span>
							</TableCell>

							<!-- Client IP -->
							<TableCell
								class="px-3 py-3 font-mono text-xs whitespace-nowrap text-[var(--text-secondary)]"
							>
								{log.clientIp}
							</TableCell>

							<!-- Time -->
							<TableCell
								class="px-3.5 py-3 text-right text-xs whitespace-nowrap text-[var(--text-tertiary)]"
							>
								{log.timeAgo}
							</TableCell>
						</TableRow>
					{/each}
				{/if}
			</TableBody>
		</Table>
	</div>

	<!-- Table Footer with Full Pagination (Kumo Parity) -->
	<TablePagination
		bind:page={currentPage}
		bind:pageSize
		totalCount={filteredLogs.length}
		pageSizeOptions={[15, 25, 50, 100]}
		itemName="requests"
	/>
</LayerCard>
