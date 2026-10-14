<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Badge } from 'kumo-svelte';
	import { StatusBadge, CopyButton } from '$lib/components/ui';
	import { Cpu, Database, Cube, FileText, ArrowRight, Terminal } from 'phosphor-svelte';
	import type { Service, Domain } from '$lib/types';

	interface Props {
		service: Service;
		containers: any[];
		domains: Domain[];
		raw?: any;
	}

	let { service, containers, domains, raw }: Props = $props();
</script>

<!-- Real Telemetry Cards -->
<div class="grid grid-cols-2 gap-2">
	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Cpu size={13} />
			<span>CPU Usage</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{(service?.cpu || raw?.cpu || 0).toFixed(1)}%
		</div>
	</div>

	<div
		class="flex flex-col justify-between rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-2.5 shadow-2xs"
	>
		<div class="flex items-center gap-1.5 text-[11px] text-[var(--text-tertiary)]">
			<Database size={13} />
			<span>Memory</span>
		</div>
		<div class="mt-1 font-mono text-base font-semibold text-[var(--text-primary)]">
			{service?.memory || raw?.memory || 0} MB
		</div>
	</div>
</div>

<!-- Member Containers List -->
{#if containers.length > 0}
	<div
		class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
			<span class="flex items-center gap-1.5">
				<Cube size={14} class="text-[var(--status-green)]" />
				<span>Containers ({containers.length})</span>
			</span>
		</div>

		<div class="mt-1 flex flex-col gap-1.5">
			{#each containers as c}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-2 text-xs"
				>
					<div class="flex min-w-0 items-center gap-2">
						<StatusBadge status={c.status} size="sm" />
						<div class="min-w-0">
							<span class="block truncate font-medium text-[var(--text-primary)]">{c.name}</span>
							<span class="block truncate font-mono text-[10px] text-[var(--text-tertiary)]">
								{c.image}
							</span>
						</div>
					</div>
					<div class="shrink-0 text-right font-mono text-[10px] text-[var(--text-secondary)]">
						<div>{(c.cpu || 0).toFixed(1)}%</div>
						<div>{c.memory || 0}MB</div>
					</div>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Ports & Ingress Bindings -->
{#if service?.port || service?.hostPort || domains.length > 0}
	<div
		class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<span class="text-xs font-semibold text-[var(--text-primary)]">Networking & Ingress</span>
		<div class="flex flex-col gap-1.5">
			{#if service?.hostPort}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-xs"
				>
					<span class="text-[var(--text-tertiary)]">Host Port</span>
					<span class="font-medium text-[var(--text-primary)]"
						>0.0.0.0:{service.hostPort} → :{service.port || service.hostPort}</span
					>
				</div>
			{:else if service?.port}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-xs"
				>
					<span class="text-[var(--text-tertiary)]">Container Port</span>
					<span class="font-medium text-[var(--text-primary)]">:{service.port}</span>
				</div>
			{/if}

			{#each domains as d}
				<div
					class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs"
				>
					<a
						href={`https://${d.hostname}`}
						target="_blank"
						rel="noreferrer"
						class="truncate font-mono text-xs text-[var(--accent)] hover:underline"
					>
						{d.hostname}
					</a>
					<Badge variant={d.tls ? 'success' : 'secondary'} size="sm">
						{d.tls ? 'TLS' : 'HTTP'}
					</Badge>
				</div>
			{/each}
		</div>
	</div>
{/if}

<!-- Quadlet Systemd Unit Definition -->
{#if service?.quadletConfig}
	<div
		class="flex flex-col gap-1.5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-3 shadow-2xs"
	>
		<div class="flex items-center justify-between text-xs font-semibold text-[var(--text-primary)]">
			<span class="flex items-center gap-1.5">
				<FileText size={14} class="text-[var(--accent)]" />
				<span>Systemd Quadlet Unit</span>
			</span>
			<CopyButton text={service.quadletConfig} size={12} label="Copy Unit" variant="inline" />
		</div>
		<pre
			class="mt-1 max-h-40 overflow-auto rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2 font-mono text-[10.5px] leading-relaxed whitespace-pre text-[var(--text-secondary)]">{service.quadletConfig}</pre>
	</div>
{/if}

<!-- Action Buttons -->
{#if service.projectId && service.id}
	<div class="mt-2 flex flex-col gap-2">
		<Button
			variant="primary"
			size="sm"
			class="w-full justify-center"
			onclick={() => goto(`/projects/${service.projectId}/services/${service.id}`)}
		>
			<span>Open Service Details</span>
			<ArrowRight size={13} />
		</Button>
		<Button
			variant="secondary"
			size="sm"
			class="w-full justify-center"
			onclick={() => goto(`/projects/${service.projectId}/services/${service.id}?tab=logs`)}
		>
			<Terminal size={13} />
			<span>Service Logs & Terminal</span>
		</Button>
	</div>
{/if}
