<script lang="ts">
	import { goto } from '$app/navigation';
	import { StatusBadge, ConfirmDialog } from '$lib/components/ui';
	import type { Service, Project } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { Trash, HardDrives } from 'phosphor-svelte';

	interface Props {
		service: Service;
		project: Project;
		onTerminalClick?: () => void;
		onRedeploy?: () => void;
	}

	let { service, project, onTerminalClick, onRedeploy }: Props = $props();

	let deleteConfirmOpen = $state(false);
	let deleteVolumes = $state(false);
	let isDeleting = $state(false);

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

	async function handleDeleteService() {
		if (isDeleting) return;
		isDeleting = true;
		try {
			await dataStore.deleteService(service.id, deleteVolumes);
			deleteConfirmOpen = false;
			goto(`/projects/${project.id}`);
		} catch (err) {
			console.error('Failed to teardown service:', err);
		} finally {
			isDeleting = false;
		}
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
			onclick={() => {
				deleteVolumes = false;
				deleteConfirmOpen = true;
			}}
			class="p-2 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-red-500/10 hover:border-red-500/30 text-[var(--text-tertiary)] hover:text-red-400 transition-colors cursor-pointer"
			title="Delete service"
			aria-label="Delete service"
		>
			<Trash size={15} />
		</button>
	</div>
</div>

<!-- Clean, Modular Confirm Dialog for Service Teardown & Deletion -->
<ConfirmDialog
	open={deleteConfirmOpen}
	title={`Delete Service "${service.name}"?`}
	description={`This action cannot be undone. GOPOD will immediately stop and force-remove all running containers/pods, destroy Quadlet systemd service units, purge Caddy reverse proxy routes, and clean up workspace files for ${service.name}.`}
	matchValue={service.name}
	matchLabel="To confirm, type the exact service name below:"
	confirmText="Confirm & Delete"
	isConfirming={isDeleting}
	oncancel={() => (deleteConfirmOpen = false)}
	onconfirm={handleDeleteService}
>
	<!-- Dokploy-style Persistent Volumes Option Checkbox -->
	<label class="flex items-center gap-2.5 p-3 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] cursor-pointer select-none">
		<input
			type="checkbox"
			bind:checked={deleteVolumes}
			class="w-4 h-4 rounded border-[var(--border)] text-red-600 focus:ring-0 focus:ring-offset-0 cursor-pointer"
		/>
		<div class="flex flex-col gap-0.5">
			<span class="text-xs font-medium text-[var(--text-primary)]">Purge persistent storage & volumes</span>
			<span class="text-[11px] text-[var(--text-tertiary)]">Delete named storage volumes and local databases attached to this service</span>
		</div>
	</label>
</ConfirmDialog>
