<script lang="ts">
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
	import { dataStore } from '$lib/data';
</script>

<LayerCard class="overflow-hidden p-0">
	<div class="w-full overflow-x-auto md:overflow-x-visible">
		<Table class="w-full text-left text-xs">
			<TableHeader
				class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
			>
				<TableRow>
					<TableHead
						class="px-3.5 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Host port
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Protocol
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Target service / container
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Bind address
					</TableHead>
					<TableHead
						class="px-3.5 py-2.5 text-right text-[13px] font-semibold text-[var(--text-secondary)] normal-case"
					>
						Status
					</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-[var(--border-subtle)]">
				{#if dataStore.ports.length === 0}
					<TableRow>
						<TableCell colspan={5} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
							No host port bindings currently active. Host port bindings appear when services or
							Quadlet containers specify published ports (e.g., -p 8080:80 or PublishPort=...).
						</TableCell>
					</TableRow>
				{:else}
					{#each dataStore.ports as p, idx (p.id)}
						<TableRow
							class={idx % 2 === 1
								? 'bg-[var(--bg-table-row-alt)] hover:bg-[var(--bg-table-row-hover)]'
								: 'bg-[var(--bg-table-row)] hover:bg-[var(--bg-table-row-hover)]'}
						>
							<!-- Host Port -->
							<TableCell class="px-3.5 py-3 whitespace-nowrap">
								<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
									{p.bindAddress === 'Internal network'
										? `Internal (${p.containerPort})`
										: `:${p.hostPort}`}
								</span>
							</TableCell>

							<!-- Protocol -->
							<TableCell class="px-3 py-3 whitespace-nowrap">
								<Badge variant="neutral" class="font-mono text-[10.5px] uppercase">
									{p.protocol}
								</Badge>
							</TableCell>

							<!-- Target Service -->
							<TableCell class="px-3 py-3 whitespace-nowrap">
								<div class="flex flex-col">
									<span class="font-medium text-[var(--text-primary)]">{p.serviceName}</span>
									<span class="font-mono text-[11px] text-[var(--text-tertiary)]"
										>{p.containerName} ➔ :{p.containerPort}</span
									>
								</div>
							</TableCell>

							<!-- Binding Scope -->
							<TableCell class="px-3 py-3 whitespace-nowrap">
								{#if p.isPublic}
									<span
										class="inline-flex items-center gap-1 font-mono text-[11px] font-medium text-[var(--accent)]"
									>
										{p.bindAddress} (Public ingress)
									</span>
								{:else}
									<span
										class="inline-flex items-center gap-1 font-mono text-[11px] text-[var(--text-tertiary)]"
									>
										{p.bindAddress} (Local / Bridge)
									</span>
								{/if}
							</TableCell>

							<!-- Status -->
							<TableCell class="px-3.5 py-3 text-right whitespace-nowrap">
								<Badge
									variant={p.status === 'listening' ? 'success' : 'neutral'}
									class="gap-1.5 text-xs capitalize"
								>
									{p.status || 'Listening'}
								</Badge>
							</TableCell>
						</TableRow>
					{/each}
				{/if}
			</TableBody>
		</Table>
	</div>

	<!-- Footer -->
	<div
		class="flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-table-header)] px-4 py-2.5 text-xs text-[var(--text-tertiary)]"
	>
		<span>{dataStore.ports.length} ports mapped on host network</span>
		<span class="font-mono text-[11px]">Kernel Netfilter / pasta rootless bridge</span>
	</div>
</LayerCard>
