<script lang="ts">
	import { goto } from '$app/navigation';
	import { StatusBadge } from '$lib/components/ui';
	import type { Service, Project } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { Trash, HardDrives } from 'phosphor-svelte';

	interface Props {
		service: Service;
		project: Project;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, project }: Props = $props();

	let deleteConfirmOpen = $state(false);

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

	function handleDeleteService() {
		dataStore.deleteService(service.id);
		goto(`/projects/${project.id}`);
	}
</script>

<!-- Clean, spacious Service Header (No redundant duplicate breadcrumbs) -->
<div class="w-full flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-3 border-b border-[var(--border)]">
	<div class="flex flex-col gap-1">
		<div class="flex items-center gap-3">
			<h1 class="text-xl font-bold text-[var(--text-primary)] tracking-tight m-0">
				{service.name}
			</h1>
			<StatusBadge status={service.status} size="sm" />
		</div>
		<div class="flex items-center gap-2 text-xs text-[var(--text-tertiary)] font-[var(--font-mono)]">
			<span>{service.id}</span>
			<span>·</span>
			<span>{workloadSubtitle}</span>
		</div>
	</div>

	<!-- Right Badges & Controls -->
	<div class="flex items-center gap-2.5">
		<!-- Host Engine Badge -->
		<div class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-[var(--border)] bg-[var(--bg-surface)] text-xs text-[var(--text-secondary)] font-medium">
			<HardDrives size={14} class="text-[var(--accent)]" />
			<span>Podman Local Engine</span>
		</div>

		<!-- Delete Service Button -->
		<button
			type="button"
			onclick={() => (deleteConfirmOpen = true)}
			class="p-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-red-500/10 hover:border-red-500/30 text-[var(--text-tertiary)] hover:text-red-400 transition-colors cursor-pointer"
			title="Delete service"
			aria-label="Delete service"
		>
			<Trash size={15} />
		</button>
	</div>
</div>

<!-- Delete Confirmation Modal (100% English) -->
{#if deleteConfirmOpen}
	<div
		class="fixed inset-0 z-50 bg-black/70 backdrop-blur-xs flex items-center justify-center p-4"
		role="dialog"
		aria-modal="true"
	>
		<div class="w-full max-w-md rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-6 flex flex-col gap-4 shadow-xl">
			<div class="flex items-center gap-3 text-red-400">
				<Trash size={22} />
				<h3 class="text-base font-semibold text-[var(--text-primary)] m-0">Delete Service "{service.name}"?</h3>
			</div>

			<p class="text-xs text-[var(--text-secondary)] leading-relaxed m-0">
				This will stop the running container workload, purge its system configuration, and remove it from project
				<strong>{project.name}</strong>. Persistent volumes will remain unattached.
			</p>

			<div class="flex items-center justify-end gap-2.5 pt-2">
				<button
					type="button"
					onclick={() => (deleteConfirmOpen = false)}
					class="px-3.5 py-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleDeleteService}
					class="px-4 py-2 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold cursor-pointer border-0"
				>
					Confirm Delete
				</button>
			</div>
		</div>
	</div>
{/if}
