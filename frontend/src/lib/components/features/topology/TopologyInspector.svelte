<script lang="ts">
	import type { SelectedItem } from './types';
	import { goto } from '$app/navigation';
	import { Button } from '$lib/components/primitives';
	import { StatusBadge } from '$lib/components/ui';
	import {
		X,
		ArrowRight,
		Globe,
		AppWindow,
		Cube,
		HardDrive,
		TreeStructure,
		ShieldCheck,
		Folder,
		Cpu,
		Database,
		FileText
	} from 'phosphor-svelte';

	interface Props {
		selected: SelectedItem;
		onclose: () => void;
		onFocusProject?: (projectId: string) => void;
	}

	let { selected, onclose, onFocusProject }: Props = $props();

	function getIcon(type: string, isQuadlet?: boolean) {
		if (isQuadlet) return FileText;
		switch (type) {
			case 'domain':
				return Globe;
			case 'service':
				return AppWindow;
			case 'container':
				return Cube;
			case 'volume':
				return HardDrive;
			case 'network':
				return TreeStructure;
			case 'proxy':
				return ShieldCheck;
			case 'project':
				return Folder;
			case 'pod':
				return Cube;
			default:
				return AppWindow;
		}
	}
</script>

{#if selected}
	{@const isQuadlet =
		selected.kind === 'node' &&
		((selected.item.badge || '').toLowerCase() === 'quadlet' ||
			(selected.item.raw as any)?.type === 'quadlet' ||
			(selected.item.raw as any)?.isQuadlet)}
	{@const Icon = getIcon(
		selected.kind === 'node' ? selected.item.type : selected.item.type,
		isQuadlet
	)}
	<aside
		class="z-20 flex h-full w-[330px] shrink-0 flex-col overflow-y-auto border-l border-[var(--border)] bg-[var(--bg-shell)] backdrop-blur-md transition-colors duration-200 select-text"
	>
		<!-- Panel Header -->
		<div class="flex items-center justify-between border-b border-[var(--border)] px-5 py-4">
			<div class="flex items-center gap-2">
				<span class="text-[var(--text-secondary)]">
					<Icon size={16} />
				</span>
				<span
					class="text-xs font-[var(--font-sans)] font-medium text-[var(--text-secondary)] capitalize"
				>
					{selected.kind === 'node'
						? isQuadlet
							? 'Quadlet Unit'
							: selected.item.type
						: `${selected.item.type} territory`}
				</span>
			</div>

			<button
				onclick={onclose}
				class="flex h-6 w-6 cursor-pointer items-center justify-center rounded border-0 bg-transparent text-[var(--text-tertiary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
				title="Close Inspector (Esc)"
			>
				<X size={14} />
			</button>
		</div>

		<!-- Main Content -->
		<div class="flex flex-1 flex-col gap-6 p-5 text-left">
			<!-- Title & Status -->
			<div class="flex flex-col gap-1.5">
				<h3
					class="text-base leading-tight font-[var(--font-sans)] font-medium break-words text-[var(--text-primary)]"
				>
					{selected.kind === 'node' ? selected.item.title : selected.item.label}
				</h3>

				<div class="mt-1 flex items-center gap-2">
					{#if selected.item.status}
						<StatusBadge status={selected.item.status} size="sm" />
					{/if}

					{#if selected.kind === 'node' && selected.item.badge}
						<span
							class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-0.5 font-mono text-[10px] text-[var(--text-tertiary)]"
						>
							{selected.item.badge}
						</span>
					{/if}
				</div>
			</div>

			<!-- Node Specific Inspection -->
			{#if selected.kind === 'node'}
				{@const node = selected.item}
				<div class="flex flex-col gap-4 text-xs">
					<!-- ID -->
					<div class="flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Identifier</span>
						<span
							class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11.5px] break-all text-[var(--text-secondary)]"
						>
							{node.id}
						</span>
					</div>

					<!-- Technical / Mono detail -->
					{#if node.monoDetail}
						<div class="flex flex-col gap-1">
							<span class="text-[11px] text-[var(--text-tertiary)]">Configuration</span>
							<span class="font-mono text-[12px] text-[var(--text-primary)]">
								{node.monoDetail}
							</span>
						</div>
					{/if}

					<!-- Service Details -->
					{#if node.type === 'service' && node.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							{#if node.raw.type === 'quadlet' || node.badge === 'quadlet'}
								<div
									class="flex flex-col gap-1 rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
								>
									<div class="flex items-center justify-between text-[11px]">
										<span class="text-[var(--text-tertiary)]">Quadlet Unit</span>
										<span class="font-mono text-[10px] font-semibold text-[var(--accent)]"
											>systemd --user</span
										>
									</div>
									<span class="font-mono text-[11.5px] text-[var(--text-primary)]">
										{node.raw.source || `${node.title}.container`}
									</span>
								</div>
								{#if node.raw.quadletConfig}
									<div class="flex flex-col gap-1">
										<span class="text-[11px] text-[var(--text-tertiary)]">Quadlet Definition</span>
										<pre
											class="overflow-x-auto rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 font-mono text-[10px] leading-relaxed whitespace-pre text-[var(--text-secondary)]">{node
												.raw.quadletConfig}</pre>
									</div>
								{/if}
							{/if}
							{#if node.raw.source && node.raw.type !== 'quadlet'}
								<div class="flex flex-col gap-0.5">
									<span class="text-[11px] text-[var(--text-tertiary)]">Source Repository</span>
									<span class="font-mono text-[11.5px] text-[var(--text-secondary)]"
										>{node.raw.source}</span
									>
								</div>
							{/if}
							{#if node.raw.image}
								<div class="flex flex-col gap-0.5">
									<span class="text-[11px] text-[var(--text-tertiary)]">Container Image</span>
									<span class="truncate font-mono text-[11px] text-[var(--text-secondary)]"
										>{node.raw.image}</span
									>
								</div>
							{/if}
							{#if node.raw.cpu !== undefined}
								<div class="mt-1 grid grid-cols-2 gap-2">
									<div
										class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
									>
										<div class="flex items-center gap-1 text-[10px] text-[var(--text-tertiary)]">
											<Cpu size={12} /> CPU
										</div>
										<div class="mt-0.5 font-mono text-base text-[var(--text-primary)]">
											{node.raw.cpu}%
										</div>
									</div>
									<div
										class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
									>
										<div class="flex items-center gap-1 text-[10px] text-[var(--text-tertiary)]">
											<Database size={12} /> Memory
										</div>
										<div class="mt-0.5 font-mono text-base text-[var(--text-primary)]">
											{node.raw.memory} MB
										</div>
									</div>
								</div>
							{/if}
						</div>
					{/if}

					<!-- Container Details -->
					{#if node.type === 'container' && node.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							{#if node.raw.isQuadlet || (node.badge && node.badge.toLowerCase() === 'quadlet')}
								<div
									class="flex flex-col gap-1 rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
								>
									<div class="flex items-center justify-between text-[11px]">
										<span class="text-[var(--text-tertiary)]">Systemd Service</span>
										<span class="font-mono text-[10px] font-semibold text-[var(--accent)]"
											>active (running)</span
										>
									</div>
									<span class="font-mono text-[11.5px] text-[var(--text-primary)]">
										{node.title}
									</span>
								</div>
								{#if node.raw.quadletConfig}
									<div class="flex flex-col gap-1">
										<span class="text-[11px] text-[var(--text-tertiary)]"
											>Systemd Unit Configuration</span
										>
										<pre
											class="overflow-x-auto rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 font-mono text-[10px] leading-relaxed whitespace-pre text-[var(--text-secondary)]">{node
												.raw.quadletConfig}</pre>
									</div>
								{/if}
							{/if}
							<div class="flex flex-col gap-0.5">
								<span class="text-[11px] text-[var(--text-tertiary)]">Image</span>
								<span class="font-mono text-[11px] break-all text-[var(--text-secondary)]"
									>{node.raw.image || node.subtitle}</span
								>
							</div>
							{#if node.raw.ports}
								<div class="flex flex-col gap-0.5">
									<span class="text-[11px] text-[var(--text-tertiary)]">Port Forwarding</span>
									<span class="font-mono text-[11.5px] text-[var(--text-primary)]"
										>{node.raw.ports}</span
									>
								</div>
							{/if}
						</div>
					{/if}

					<!-- Volume Details -->
					{#if node.type === 'volume' && node.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							<div class="flex flex-col gap-0.5">
								<span class="text-[11px] text-[var(--text-tertiary)]">Mount Path</span>
								<span class="font-mono text-[11.5px] text-[var(--text-primary)]"
									>{node.raw.mount}</span
								>
							</div>
							<div class="flex flex-col gap-0.5">
								<span class="text-[11px] text-[var(--text-tertiary)]">Storage Size</span>
								<span class="font-mono text-[11.5px] text-[var(--text-secondary)]"
									>{node.raw.size}</span
								>
							</div>
						</div>
					{/if}

					<!-- Network Details -->
					{#if node.type === 'network' && node.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							<div class="flex flex-col gap-0.5">
								<span class="text-[11px] text-[var(--text-tertiary)]">Subnet</span>
								<span class="font-mono text-[11.5px] text-[var(--text-primary)]"
									>{node.raw.subnet}</span
								>
							</div>
							<div class="flex flex-col gap-0.5">
								<span class="text-[11px] text-[var(--text-tertiary)]">Gateway IP</span>
								<span class="font-mono text-[11.5px] text-[var(--text-secondary)]"
									>{node.raw.gateway}</span
								>
							</div>
						</div>
					{/if}
				</div>

				<!-- Action Buttons -->
				<div class="mt-auto flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-4">
					{#if node.type === 'service' && node.projectId && node.serviceId}
						<Button
							variant="primary"
							size="sm"
							onclick={() => goto(`/projects/${node.projectId}/services/${node.serviceId}`)}
						>
							View Service Details <ArrowRight size={13} />
						</Button>
					{/if}
					{#if node.projectId}
						<Button
							variant="secondary"
							size="sm"
							onclick={() => goto(`/projects/${node.projectId}`)}
						>
							Open Project Page
						</Button>
					{/if}
					{#if node.type === 'container'}
						<Button variant="ghost" size="sm" onclick={() => goto('/runtime/containers')}>
							Runtime Dashboard
						</Button>
					{/if}
					{#if node.type === 'volume'}
						<Button variant="ghost" size="sm" onclick={() => goto('/runtime/volumes')}>
							View All Volumes
						</Button>
					{/if}
					{#if node.type === 'network'}
						<Button variant="ghost" size="sm" onclick={() => goto('/runtime/networks')}>
							View All Networks
						</Button>
					{/if}
					{#if node.type === 'domain'}
						<Button variant="ghost" size="sm" onclick={() => goto('/networking/domains')}>
							Networking Table
						</Button>
					{/if}
				</div>

				<!-- Region Inspection (Project or Pod) -->
			{:else}
				{@const region = selected.item}
				<div class="flex flex-col gap-4 text-xs">
					<div class="flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Region Territory</span>
						<span class="text-xs text-[var(--text-secondary)]">
							{region.type === 'project'
								? 'Logical project isolation territory grouping services and storage.'
								: 'Podman runtime pod grouping co-located containers sharing localhost.'}
						</span>
					</div>

					{#if region.type === 'project' && region.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							{#if region.raw.description}
								<div class="flex flex-col gap-0.5">
									<span class="text-[11px] text-[var(--text-tertiary)]">Description</span>
									<span class="text-xs text-[var(--text-secondary)]">{region.raw.description}</span>
								</div>
							{/if}
							<div class="mt-1 grid grid-cols-2 gap-2">
								<div
									class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
								>
									<div class="text-[10px] text-[var(--text-tertiary)]">CPU Usage</div>
									<div class="mt-0.5 font-mono text-base text-[var(--text-primary)]">
										{region.raw.cpu}%
									</div>
								</div>
								<div
									class="rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2"
								>
									<div class="text-[10px] text-[var(--text-tertiary)]">Memory</div>
									<div class="mt-0.5 font-mono text-base text-[var(--text-primary)]">
										{region.raw.memory} MB
									</div>
								</div>
							</div>
						</div>
					{/if}

					{#if region.type === 'pod' && region.raw}
						<div class="flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-2">
							<div class="flex flex-col gap-1">
								<span class="text-[11px] text-[var(--text-tertiary)]">Member Containers</span>
								<div class="flex flex-col gap-1">
									{#each region.raw.containers as cName}
										<div
											class="flex items-center gap-1.5 rounded border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-secondary)]"
										>
											<Cube size={11} class="text-[var(--text-tertiary)]" />
											{cName}
										</div>
									{/each}
								</div>
							</div>
							{#if region.raw.network}
								<div class="mt-1 flex flex-col gap-0.5">
									<span class="text-[11px] text-[var(--text-tertiary)]">Attached Network</span>
									<span class="font-mono text-[11.5px] text-[var(--text-secondary)]"
										>{region.raw.network}</span
									>
								</div>
							{/if}
						</div>
					{/if}
				</div>

				<!-- Region Actions -->
				<div class="mt-auto flex flex-col gap-2 border-t border-[var(--border-subtle)] pt-4">
					{#if region.type === 'project' && region.projectId}
						{#if onFocusProject}
							<Button variant="primary" size="sm" onclick={() => onFocusProject(region.projectId!)}>
								Focus Project Topology <ArrowRight size={13} />
							</Button>
						{/if}
						<Button
							variant="secondary"
							size="sm"
							onclick={() => goto(`/projects/${region.projectId}`)}
						>
							Open Project Page
						</Button>
					{/if}
					{#if region.type === 'pod'}
						<Button variant="secondary" size="sm" onclick={() => goto('/runtime/pods')}>
							View in Pods Table
						</Button>
					{/if}
				</div>
			{/if}
		</div>
	</aside>
{/if}
