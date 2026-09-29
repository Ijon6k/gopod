<script lang="ts">
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import { Button } from '$lib/components/primitives';
	import type { Service, Project } from '$lib/types';
	import { ArrowClockwise, Terminal, Play, Stop, DotsThreeVertical } from 'phosphor-svelte';

	interface Props {
		service: Service;
		project: Project;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, project, onTerminalClick, onRedeploy }: Props = $props();

	let actionMenuOpen = $state(false);

	let workloadSubtitle = $derived.by(() => {
		if (service.type === 'application') {
			return `Application · Port ${service.port || 3000}`;
		}
		if (service.type === 'compose') {
			const count = service.workloads?.length ?? 1;
			return `Compose · ${count} workload${count > 1 ? 's' : ''}`;
		}
		if (service.type === 'pod') {
			const count = service.workloads?.length ?? 1;
			return `Pod · ${count} container${count > 1 ? 's' : ''}`;
		}
		if (service.type === 'kubernetes') {
			return `Kubernetes YAML · Port ${service.port || 80}`;
		}
		if (service.type === 'quadlet') {
			return `Quadlet (systemd) · Port ${service.port || 8080}`;
		}
		return `Container · ${service.image || 'image'} :${service.port || 8080}`;
	});

	function handleAction(action: string) {
		actionMenuOpen = false;
		if (action === 'redeploy') {
			onRedeploy?.();
		} else if (action === 'start') {
			service.status = 'running';
		} else if (action === 'stop') {
			service.status = 'stopped';
		} else if (action === 'restart') {
			service.status = 'deploying';
			setTimeout(() => {
				service.status = 'running';
			}, 1200);
		}
	}
</script>

<div class="w-full flex flex-col gap-3 pb-2 border-b border-[var(--border)]">
	<!-- Main Header Bar -->
	<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
		<div class="flex flex-col gap-1">
			<div class="flex items-center gap-3">
				<h1 class="text-xl font-semibold text-[var(--text-primary)] tracking-tight m-0">
					{service.name}
				</h1>
				<StatusBadge status={service.status} size="sm" />
			</div>
			<span class="text-xs text-[var(--text-tertiary)] font-[var(--font-mono)]">
				{workloadSubtitle}
			</span>
		</div>

		<!-- Operational Actions -->
		<div class="relative flex items-center gap-2">
			<Button variant="secondary" size="sm" onclick={onTerminalClick}>
				<Terminal size={14} /> Terminal
			</Button>

			<Button variant="secondary" size="sm" onclick={onRedeploy}>
				<ArrowClockwise size={14} /> Redeploy
			</Button>

			<!-- Action Dropdown Menu -->
			<div class="relative">
				<Button
					variant="secondary"
					size="sm"
					onclick={() => (actionMenuOpen = !actionMenuOpen)}
					ariaLabel="Service actions"
				>
					<DotsThreeVertical size={16} />
				</Button>

				{#if actionMenuOpen}
					<div
						class="absolute right-0 top-full mt-1.5 z-30 min-w-[150px] p-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] flex flex-col gap-0.5 text-xs"
						role="menu"
						tabindex="-1"
						onmouseleave={() => (actionMenuOpen = false)}
					>
						{#if service.status === 'stopped'}
							<button
								type="button"
								onclick={() => handleAction('start')}
								class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded text-left text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-panel)] bg-transparent border-0 cursor-pointer"
							>
								<Play size={13} class="text-[var(--status-green)]" /> Start service
							</button>
						{:else}
							<button
								type="button"
								onclick={() => handleAction('restart')}
								class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded text-left text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-panel)] bg-transparent border-0 cursor-pointer"
							>
								<ArrowClockwise size={13} /> Restart
							</button>
							<button
								type="button"
								onclick={() => handleAction('stop')}
								class="flex items-center gap-2 w-full px-2.5 py-1.5 rounded text-left text-[var(--text-secondary)] hover:text-[var(--status-red)] hover:bg-[var(--bg-panel)] bg-transparent border-0 cursor-pointer"
							>
								<Stop size={13} /> Stop
							</button>
						{/if}
					</div>
				{/if}
			</div>
		</div>
	</div>
</div>
