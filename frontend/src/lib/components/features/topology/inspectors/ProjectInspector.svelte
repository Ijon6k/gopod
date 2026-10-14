<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
	import { StatusBadge } from '$lib/components/ui';
	import { Cpu, Database, Stack, Globe, Plugs, HardDrive, ArrowRight, Lock } from 'phosphor-svelte';
	import type { Project, Service, Container, Domain, Volume } from '$lib/types';

	interface Props {
		proj: Project;
		services: Service[];
		containers: Container[];
		domains: Domain[];
		volumes: Volume[];
		publishedPorts: { hostPort: number; containerPort: number; ip: string; serviceName: string }[];
		onFocusProject?: (projectId: string) => void;
	}

	let { proj, services, containers, domains, volumes, publishedPorts, onFocusProject }: Props =
		$props();

	let activeContainersCount = $derived(containers.filter((c) => c.status === 'running').length);
</script>

<!-- Aggregated Telemetry Deck -->
<div class="grid grid-cols-2 gap-2">
	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Cpu size={13} />
			<span>CPU Aggregated</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{(proj?.cpu || 0).toFixed(1)}%
		</div>
	</div>

	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Database size={13} />
			<span>Memory Total</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{proj?.memory || 0} MB
		</div>
	</div>
</div>

<!-- Workloads & Container Status Summary -->
<div
	class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
>
	<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
		<span class="flex items-center gap-1.5">
			<Stack size={14} class="text-[var(--accent)]" />
			<span>Member Services ({services.length})</span>
		</span>
		<Badge variant="secondary" size="sm">
			{activeContainersCount}/{containers.length} Containers Active
		</Badge>
	</div>

	<div class="mt-1 flex flex-col gap-1.5">
		{#each services as svc}
			<button
				type="button"
				onclick={() => goto(`/projects/${proj.id}/services/${svc.id}`)}
				class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-left text-xs transition-colors hover:border-[var(--accent)] hover:bg-[var(--bg-hover)]"
			>
				<div class="flex min-w-0 items-center gap-2">
					<StatusBadge status={svc.status} size="sm" />
					<span class="truncate font-medium text-[var(--text-primary)]">{svc.name}</span>
				</div>
				<div class="flex shrink-0 items-center gap-1.5">
					{#if svc.port || svc.hostPort}
						<span class="font-mono text-[10px] text-[var(--text-secondary)]">
							:{svc.port || svc.hostPort}
						</span>
					{/if}
					<Badge variant="outline" size="sm" class="text-[10px] capitalize">
						{svc.type}
					</Badge>
				</div>
			</button>
		{/each}
	</div>
</div>

<!-- Connected Domains & Ingress -->
{#if domains.length > 0}
	<div
		class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
			<span class="flex items-center gap-1.5">
				<Globe size={14} class="text-[var(--accent)]" />
				<span>Ingress Domains ({domains.length})</span>
			</span>
		</div>

		<div class="mt-1 flex flex-col gap-1.5">
			{#each domains as dom}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs"
				>
					<a
						href={`https://${dom.hostname}`}
						target="_blank"
						rel="noreferrer"
						class="truncate font-mono text-xs text-[var(--accent)] hover:underline"
					>
						{dom.hostname}
					</a>
					<Badge variant={dom.tls ? 'success' : 'secondary'} size="sm">
						{#if dom.tls}
							<span class="inline-flex items-center gap-1"><Lock size={10} /> TLS</span>
						{:else}
							HTTP Only
						{/if}
					</Badge>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Published Host Ports -->
{#if publishedPorts.length > 0}
	<div
		class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
			<span class="flex items-center gap-1.5">
				<Plugs size={14} class="text-[var(--status-amber)]" />
				<span>Published Host Ports</span>
			</span>
		</div>

		<div class="mt-1 grid grid-cols-2 gap-1.5">
			{#each publishedPorts as pp}
				<div
					class="flex flex-col rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 text-xs"
				>
					<span class="font-mono font-medium text-[var(--text-primary)]">
						{pp.ip}:{pp.hostPort}
					</span>
					<span class="mt-0.5 truncate text-[10px] text-[var(--text-tertiary)]">
						→ {pp.serviceName} (:{pp.containerPort})
					</span>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Persistent Storage Volumes -->
{#if volumes.length > 0}
	<div
		class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
			<span class="flex items-center gap-1.5">
				<HardDrive size={14} class="text-[var(--status-amber)]" />
				<span>Storage Volumes ({volumes.length})</span>
			</span>
		</div>

		<div class="mt-1 flex flex-col gap-1.5">
			{#each volumes as vol}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs"
				>
					<span class="truncate font-mono text-xs text-[var(--text-primary)]">{vol.name}</span>
					<span class="font-mono text-[10px] text-[var(--text-tertiary)]"
						>{vol.size || 'Dynamic'}</span
					>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Action Buttons -->
<div class="mt-2 flex flex-col gap-2">
	{#if onFocusProject && proj.id}
		<Button
			variant="primary"
			size="sm"
			class="w-full justify-center"
			onclick={() => onFocusProject(proj.id)}
		>
			<span>Focus Project Topology</span>
			<ArrowRight size={13} />
		</Button>
	{/if}
	{#if proj.id}
		<Button
			variant="secondary"
			size="sm"
			class="w-full justify-center"
			onclick={() => goto(`/projects/${proj.id}`)}
		>
			<span>Open Project Dashboard</span>
		</Button>
	{/if}
</div>
