<script lang="ts">
	import { Stack, ShareNetwork, HardDrives, Info, ArrowsSplit } from 'phosphor-svelte';

	export interface ParsedServiceItem {
		name: string;
		image?: string;
		ports: string[];
		networks: string[];
		dependsOn: string[];
	}

	export interface ParsedNetworkItem {
		name: string;
		driver?: string;
		external?: boolean;
	}

	export interface ParsedComposeStack {
		services: ParsedServiceItem[];
		networks: ParsedNetworkItem[];
		volumes: string[];
	}

	interface Props {
		parsedStack: ParsedComposeStack;
	}

	let { parsedStack }: Props = $props();
</script>

<div class="flex flex-col gap-4 rounded-md bg-kumo-base p-4 ring ring-kumo-line">
	<!-- Section 1: Detected Workload Containers -->
	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between">
			<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
				<Stack size={14} class="text-[var(--accent)]" /> Detected workloads ({parsedStack.services
					.length})
			</span>
			<span class="text-[11px] text-kumo-subtle">
				Auto-assigned service alias and Caddy proxy endpoints
			</span>
		</div>

		<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 md:grid-cols-3">
			{#each parsedStack.services as svc}
				<div class="flex flex-col gap-1.5 rounded bg-kumo-elevated p-3 ring ring-kumo-hairline">
					<div class="flex items-center justify-between gap-1">
						<span class="truncate font-mono text-xs font-semibold text-kumo-default">
							{svc.name}
						</span>
						{#if svc.networks.length > 0}
							<span
								class="inline-flex items-center gap-1 rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle"
							>
								<ShareNetwork size={10} />
								{svc.networks[0]}
							</span>
						{/if}
					</div>

					{#if svc.image}
						<span class="truncate font-mono text-[11px] text-kumo-subtle">
							{svc.image}
						</span>
					{/if}

					{#if svc.ports.length > 0}
						<div class="flex flex-wrap gap-1 pt-0.5">
							{#each svc.ports as port}
								<span
									class="py-0.2 rounded bg-[var(--accent-muted)] px-1.5 font-mono text-[10px] text-[var(--accent-text)]"
								>
									{port}
								</span>
							{/each}
						</div>
					{/if}

					{#if svc.dependsOn.length > 0}
						<div class="flex items-center gap-1 pt-0.5 text-[10px] text-kumo-subtle">
							<ArrowsSplit size={11} class="shrink-0" />
							<span class="truncate">depends: {svc.dependsOn.join(', ')}</span>
						</div>
					{/if}
				</div>
			{/each}
		</div>
	</div>

	<!-- Section 2: Stack Networking Architecture -->
	<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
		<div class="flex items-center justify-between">
			<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
				<ShareNetwork size={14} class="text-[var(--status-blue)]" /> Stack networking architecture
			</span>
			<span class="text-[11px] text-kumo-subtle">
				{parsedStack.networks.length} declared + 1 ingress bridge
			</span>
		</div>

		<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
			<!-- User Declared Networks -->
			{#if parsedStack.networks.length > 0}
				{#each parsedStack.networks as net}
					<div
						class="flex items-center justify-between rounded bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline"
					>
						<div class="flex items-center gap-2">
							<ShareNetwork size={13} class="text-[var(--text-secondary)]" />
							<span class="font-mono text-xs font-medium text-kumo-default">{net.name}</span>
						</div>
						<span
							class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle"
						>
							{net.driver || 'bridge'}
						</span>
					</div>
				{/each}
			{:else}
				<div
					class="flex items-center justify-between rounded bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline"
				>
					<div class="flex items-center gap-2">
						<ShareNetwork size={13} class="text-[var(--text-secondary)]" />
						<span class="font-mono text-xs font-medium text-kumo-default">default</span>
					</div>
					<span
						class="rounded bg-kumo-recessed px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle"
					>
						bridge (project isolated)
					</span>
				</div>
			{/if}

			<!-- Ingress Reverse Proxy Bridge -->
			<div
				class="flex items-center justify-between rounded border-l-2 border-l-[var(--accent)] bg-kumo-elevated px-3 py-2 ring ring-kumo-hairline"
			>
				<div class="flex items-center gap-2">
					<ShareNetwork size={13} class="text-[var(--accent)]" />
					<span class="font-mono text-xs font-medium text-kumo-default">gopod-net</span>
				</div>
				<span
					class="rounded bg-[var(--accent-muted)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent-text)]"
				>
					Caddy ingress bridge
				</span>
			</div>
		</div>

		<div
			class="flex items-center gap-2 rounded bg-kumo-recessed px-3 py-2 text-[12px] text-kumo-subtle"
		>
			<Info size={14} class="shrink-0 text-kumo-default" />
			<span>
				GoPod merges <code class="font-mono text-kumo-default">gopod-net</code> into the stack so Caddy
				reverse proxy can resolve containers via internal DNS while preserving custom network isolation.
			</span>
		</div>
	</div>

	<!-- Section 3: Storage Volumes (if any declared) -->
	{#if parsedStack.volumes.length > 0}
		<div class="flex flex-col gap-2 border-t border-kumo-hairline pt-3">
			<div class="flex items-center justify-between">
				<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
					<HardDrives size={14} class="text-[var(--status-purple)]" /> Persistent storage volumes ({parsedStack
						.volumes.length})
				</span>
			</div>

			<div class="flex flex-wrap gap-2">
				{#each parsedStack.volumes as vol}
					<span
						class="inline-flex items-center gap-1.5 rounded bg-kumo-elevated px-2.5 py-1 font-mono text-xs text-kumo-default ring ring-kumo-hairline"
					>
						<HardDrives size={12} class="text-kumo-subtle" />
						{vol}
					</span>
				{/each}
			</div>
		</div>
	{/if}
</div>
