<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import {
		LayerCard,
		Button,
		Input,
		Select,
		Badge,
		Table,
		TableHeader,
		TableRow,
		TableCell,
		TableHead,
		TableBody,
		useKumoToastManager
	} from 'kumo-svelte';
	import {
		Package,
		ArrowsClockwise,
		Stop,
		Trash,
		Terminal,
		Plus,
		Check,
		Info,
		Copy,
		Globe,
		PlugsConnected
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let isAddingContainer = $state(false);
	let attachMode = $state<'existing' | 'new'>('existing');

	// Form for existing service attach
	let selectedServiceId = $state('');

	// Form for new container deploy
	let newContainerName = $state('');
	let newContainerImage = $state('');
	let newContainerPort = $state('80');
	let isSubmitting = $state(false);

	// Exposed ports editing
	let isEditingPorts = $state(false);
	let exposedHostPort = $state('');
	let exposedTargetPort = $state('');
	let isSavingPorts = $state(false);

	$effect(() => {
		exposedHostPort = String(service.hostPort || service.port || 8080);
		exposedTargetPort = String(service.port || 80);
	});

	// Find member services belonging to this Pod
	let memberServices = $derived.by(() => {
		return dataStore.services.filter(
			(s) =>
				s.projectId === service.projectId &&
				s.id !== service.id &&
				(s.podId === service.id ||
					s.podId === service.name ||
					s.podId === `pod_${service.name}` ||
					(s.inPod && s.type !== 'pod'))
		);
	});

	// Unattached candidate services in the same project
	let candidateServices = $derived.by(() => {
		return dataStore.services.filter(
			(s) =>
				s.projectId === service.projectId &&
				s.id !== service.id &&
				s.type !== 'pod' &&
				s.podId !== service.id &&
				s.podId !== service.name &&
				!s.inPod
		);
	});

	let podName = $derived(`pod_${service.name}`);

	async function handleSavePorts() {
		isSavingPorts = true;
		const hPort = parseInt(exposedHostPort.trim(), 10) || 0;
		const tPort = parseInt(exposedTargetPort.trim(), 10) || 80;

		service.hostPort = hPort;
		service.port = tPort;

		try {
			await dataStore.updateService(service);
			toast.add({
				title: 'Pod Ports Saved',
				description: `Exposed port updated to ${hPort}:${tPort}`,
				variant: 'success'
			});
			isEditingPorts = false;
		} catch (err: any) {
			toast.add({
				title: 'Failed to Save Ports',
				description: err?.message || 'Error updating ports',
				variant: 'error'
			});
		} finally {
			isSavingPorts = false;
		}
	}

	async function handleAttachExisting() {
		if (!selectedServiceId) return;
		isSubmitting = true;

		const target = dataStore.services.find((s) => s.id === selectedServiceId);
		if (!target) return;

		target.podId = podName;
		target.inPod = true;
		target.runtimeTarget = 'pod';

		try {
			await dataStore.groupIntoPod(target.id, podName, false);
			await dataStore.updateService(target);
			toast.add({
				title: 'Service Attached to Pod',
				description: `${target.name} joined ${podName}`,
				variant: 'success'
			});
			selectedServiceId = '';
			isAddingContainer = false;
		} catch (err: any) {
			toast.add({
				title: 'Attach Failed',
				description: err?.message || 'Failed to attach service',
				variant: 'error'
			});
		} finally {
			isSubmitting = false;
		}
	}

	async function handleCreateNewContainer() {
		if (!newContainerName.trim() || !newContainerImage.trim()) return;
		isSubmitting = true;

		const portNum = parseInt(newContainerPort.trim(), 10) || 80;
		const cleanName = newContainerName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-');

		const newSvc: Partial<Service> = {
			name: cleanName,
			projectId: service.projectId,
			type: 'image',
			status: 'deploying',
			image: newContainerImage.trim(),
			port: portNum,
			inPod: true,
			podId: podName,
			runtimeTarget: 'pod',
			envVars: []
		};

		try {
			const created = await api.services.create(newSvc);
			dataStore.services.push(created);
			toast.add({
				title: 'Container Created in Pod',
				description: `${cleanName} deployed into ${podName}`,
				variant: 'success'
			});
			newContainerName = '';
			newContainerImage = '';
			newContainerPort = '80';
			isAddingContainer = false;
		} catch (err: any) {
			toast.add({
				title: 'Failed to Create Container',
				description: err?.message || 'Creation error',
				variant: 'error'
			});
		} finally {
			isSubmitting = false;
		}
	}

	async function handleRestartMember(member: Service) {
		try {
			await dataStore.deployService(member.id, 'manual');
			toast.add({
				title: 'Restarting Container',
				description: `Triggered restart for ${member.name}`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add({
				title: 'Restart Failed',
				description: err?.message || 'Error restarting container',
				variant: 'error'
			});
		}
	}

	async function handleStopMember(member: Service) {
		try {
			await dataStore.stopService(member.id);
			member.status = 'stopped';
			toast.add({
				title: 'Container Stopped',
				description: `${member.name} has been stopped`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add({
				title: 'Stop Failed',
				description: err?.message || 'Error stopping container',
				variant: 'error'
			});
		}
	}

	async function handleRemoveMember(member: Service) {
		member.inPod = false;
		member.podId = '';
		member.runtimeTarget = 'standalone';

		try {
			await dataStore.updateService(member);
			toast.add({
				title: 'Removed from Pod',
				description: `${member.name} detached to standalone mode`,
				variant: 'success'
			});
		} catch (err: any) {
			toast.add({
				title: 'Removal Failed',
				description: err?.message || 'Error detaching container',
				variant: 'error'
			});
		}
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- 1. Informative banner (Localhost Network Namespace) -->
	<div
		class="flex items-center gap-3 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] px-4 py-3 text-xs text-[var(--color-kumo-default)]"
	>
		<Info size={18} weight="fill" class="shrink-0 text-[var(--accent)]" />
		<div>
			<span class="font-semibold">Localhost Network Namespace:</span>
			<span class="text-[var(--color-kumo-subtle)]">
				Containers inside this pod communicate directly with one another via
				<code class="rounded bg-black/20 px-1.5 py-0.5 font-mono text-[var(--accent)]">localhost:&lt;port&gt;</code>.
				Host ports are exposed exclusively at the Pod level.
			</span>
		</div>
	</div>

	<!-- 2. Card 1: Pod Infrastructure & Exposed Ports -->
	<LayerCard class="overflow-hidden p-0 shadow-xs">
		<LayerCard.Secondary
			class="flex items-center justify-between border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
		>
			<div class="flex items-center gap-2.5">
				<Package size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
					Pod Infrastructure & Exposed Ports
				</h3>
			</div>
			<Badge variant="neutral" class="font-mono text-xs">
				network: gopod-net
			</Badge>
		</LayerCard.Secondary>

		<div class="grid grid-cols-1 gap-6 p-5 sm:grid-cols-2 md:grid-cols-4">
			<!-- Pod Name -->
			<div class="flex flex-col gap-1">
				<span class="text-xs text-[var(--color-kumo-subtle)]">Pod Name</span>
				<div class="flex items-center gap-2">
					<span class="font-mono text-sm font-semibold text-[var(--color-kumo-default)]">
						{podName}
					</span>
				</div>
				<span class="text-[11px] text-[var(--color-kumo-subtle)]">Podman namespace</span>
			</div>

			<!-- Network -->
			<div class="flex flex-col gap-1">
				<span class="text-xs text-[var(--color-kumo-subtle)]">Container Network</span>
				<div class="flex items-center gap-1.5">
					<PlugsConnected size={14} class="text-[var(--accent)]" />
					<span class="font-mono text-sm font-medium text-[var(--color-kumo-default)]">
						gopod-net
					</span>
				</div>
				<span class="text-[11px] text-[var(--color-kumo-subtle)]">Shared Netavark bridge</span>
			</div>

			<!-- Exposed Host Port -->
			<div class="flex flex-col gap-1 sm:col-span-2">
				<div class="flex items-center justify-between">
					<span class="text-xs text-[var(--color-kumo-subtle)]">Exposed Host Port Binding</span>
					{#if !isEditingPorts}
						<button
							type="button"
							onclick={() => (isEditingPorts = true)}
							class="cursor-pointer border-0 bg-transparent text-[11px] text-[var(--accent)] hover:underline"
						>
							Edit Port
						</button>
					{/if}
				</div>

				{#if isEditingPorts}
					<div class="flex items-center gap-2">
						<div class="w-28">
							<Input
								bind:value={exposedHostPort}
								placeholder="Host Port"
								class="h-8 font-mono text-xs"
								aria-label="Host Port"
							/>
						</div>
						<span class="text-xs font-bold text-[var(--color-kumo-subtle)]">:</span>
						<div class="w-28">
							<Input
								bind:value={exposedTargetPort}
								placeholder="Pod Port"
								class="h-8 font-mono text-xs"
								aria-label="Pod Port"
							/>
						</div>
						<Button
							variant="primary"
							size="sm"
							onclick={handleSavePorts}
							disabled={isSavingPorts}
						>
							Save
						</Button>
						<Button
							variant="secondary"
							size="sm"
							onclick={() => (isEditingPorts = false)}
						>
							Cancel
						</Button>
					</div>
				{:else}
					<div class="flex items-center gap-2">
						<Globe size={14} class="text-[var(--color-kumo-subtle)]" />
						<span class="font-mono text-sm font-bold text-[var(--color-kumo-default)]">
							{service.hostPort || service.port || 8080}:{service.port || 80}
						</span>
						<span class="text-xs text-[var(--color-kumo-subtle)]">(Host ➔ Pod)</span>
					</div>
					<span class="text-[11px] text-[var(--color-kumo-subtle)]">
						Traffic routed to internal member port
					</span>
				{/if}
			</div>
		</div>
	</LayerCard>

	<!-- 3. Card 2: Pod Member Containers (The Core List) -->
	<LayerCard class="overflow-hidden p-0 shadow-xs">
		<LayerCard.Secondary
			class="flex items-center justify-between border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
		>
			<div class="flex items-center gap-2.5">
				<Package size={16} class="text-[var(--accent)]" />
				<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
					Pod Member Containers
				</h3>
				<Badge variant="neutral" class="font-mono text-xs">
					{memberServices.length} {memberServices.length === 1 ? 'container' : 'containers'}
				</Badge>
			</div>

			<Button
				variant="primary"
				size="sm"
				onclick={() => (isAddingContainer = true)}
				class="gap-1.5"
			>
				<Plus size={14} weight="bold" />
				<span>Add / Attach Container</span>
			</Button>
		</LayerCard.Secondary>

		<!-- Member Containers Table -->
		<div class="w-full overflow-x-auto md:overflow-x-visible">
			<Table class="w-full text-left text-xs">
				<TableHeader class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]">
					<TableRow>
						<TableHead class="px-4 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case">
							Container Name
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case">
							Image
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case">
							Status
						</TableHead>
						<TableHead class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case">
							Internal Port
						</TableHead>
						<TableHead class="px-4 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)] normal-case">
							Actions
						</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border-subtle)]">
					{#if memberServices.length === 0}
						<TableRow>
							<TableCell colspan={5} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
								<div class="flex flex-col items-center justify-center gap-2">
									<Package size={32} class="opacity-40" />
									<span class="font-medium">No containers attached to this pod yet.</span>
									<span class="text-[11px]">
										Click "Add / Attach Container" above to attach an existing workload or deploy a new image.
									</span>
								</div>
							</TableCell>
						</TableRow>
					{:else}
						{#each memberServices as member (member.id)}
							<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
								<!-- Container Name -->
								<TableCell class="px-4 py-3 font-medium text-[var(--text-primary)]">
									<div class="flex items-center gap-2">
										<button
											type="button"
											onclick={() => goto(`/projects/${member.projectId}/services/${member.id}`)}
											class="cursor-pointer border-0 bg-transparent font-semibold text-[var(--text-primary)] hover:text-[var(--accent)] hover:underline"
										>
											{member.name}
										</button>
										<span class="font-mono text-[10px] text-[var(--text-tertiary)]">
											({member.id})
										</span>
									</div>
								</TableCell>

								<!-- Image -->
								<TableCell class="px-3 py-3 font-mono text-[11px] text-[var(--text-secondary)]">
									{member.image || member.source || 'image:latest'}
								</TableCell>

								<!-- Status -->
								<TableCell class="px-3 py-3">
									<Badge
										variant={member.status === 'running' ? 'success' : 'neutral'}
										class="capitalize text-[11px]"
									>
										{member.status}
									</Badge>
								</TableCell>

								<!-- Internal Port -->
								<TableCell class="px-3 py-3 font-mono text-[11px] text-[var(--text-secondary)]">
									localhost:{member.port || 80}
								</TableCell>

								<!-- Actions -->
								<TableCell class="px-4 py-3 text-right">
									<div class="flex items-center justify-end gap-1.5">
										<!-- Restart -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleRestartMember(member)}
											aria-label={`Restart ${member.name}`}
										>
											<ArrowsClockwise size={13} />
										</Button>

										<!-- Stop -->
										{#if member.status === 'running'}
											<Button
												variant="secondary-destructive"
												size="sm"
												onclick={() => handleStopMember(member)}
												aria-label={`Stop ${member.name}`}
											>
												<Stop size={13} weight="fill" />
											</Button>
										{/if}

										<!-- Detach from Pod -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleRemoveMember(member)}
											aria-label={`Detach ${member.name} from Pod`}
										>
											<span class="text-xs">Detach</span>
										</Button>

										<!-- Open Terminal -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => goto(`/projects/${member.projectId}/services/${member.id}?tab=terminal`)}
											aria-label={`Terminal for ${member.name}`}
										>
											<Terminal size={13} />
										</Button>
									</div>
								</TableCell>
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>
	</LayerCard>
</div>

<!-- Modal: Add or Attach Container to Pod -->
{#if isAddingContainer}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
		<div
			class="flex w-full max-w-lg flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] p-5 shadow-xl"
		>
			<div class="flex items-center justify-between border-b border-[var(--border)] pb-3">
				<div class="flex items-center gap-2">
					<Package size={18} class="text-[var(--accent)]" />
					<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">
						Add / Attach Container to Pod
					</h3>
				</div>
				<button
					type="button"
					onclick={() => (isAddingContainer = false)}
					class="cursor-pointer border-0 bg-transparent text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
				>
					✕
				</button>
			</div>

			<!-- Mode Selector -->
			<div class="grid grid-cols-2 gap-2 rounded-[var(--radius-sm)] bg-[var(--bg-panel)] p-1 text-xs">
				<button
					type="button"
					onclick={() => (attachMode = 'existing')}
					class="cursor-pointer rounded py-1.5 font-medium transition-colors {attachMode === 'existing'
						? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
						: 'bg-transparent text-[var(--text-tertiary)]'}"
				>
					Attach Existing Service
				</button>
				<button
					type="button"
					onclick={() => (attachMode = 'new')}
					class="cursor-pointer rounded py-1.5 font-medium transition-colors {attachMode === 'new'
						? 'bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-xs'
						: 'bg-transparent text-[var(--text-tertiary)]'}"
				>
					Deploy New Image
				</button>
			</div>

			{#if attachMode === 'existing'}
				<div class="flex flex-col gap-3 py-2">
					<span class="text-xs text-[var(--text-secondary)]">
						Select an existing standalone service in this project to join this Podman Pod:
					</span>
					{#if candidateServices.length === 0}
						<div class="rounded border border-[var(--border)] bg-[var(--bg-panel)] p-4 text-center text-xs text-[var(--text-tertiary)]">
							No standalone candidate services found in this project.
						</div>
					{:else}
						<div class="flex flex-col gap-1.5">
							<span class="text-xs font-medium text-[var(--text-primary)]">Select Service</span>
							<select
								bind:value={selectedServiceId}
								class="h-9 w-full rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 text-xs text-[var(--text-primary)]"
							>
								<option value="">-- Choose a service --</option>
								{#each candidateServices as cand}
									<option value={cand.id}>
										{cand.name} ({cand.type} · {cand.image || cand.source})
									</option>
								{/each}
							</select>
						</div>
					{/if}
				</div>

				<div class="flex items-center justify-end gap-2 border-t border-[var(--border)] pt-3">
					<Button variant="secondary" size="sm" onclick={() => (isAddingContainer = false)}>
						Cancel
					</Button>
					<Button
						variant="primary"
						size="sm"
						onclick={handleAttachExisting}
						disabled={!selectedServiceId || isSubmitting}
					>
						Attach to Pod
					</Button>
				</div>
			{:else}
				<div class="flex flex-col gap-3 py-2">
					<div class="flex flex-col gap-1">
						<span class="text-xs font-medium text-[var(--text-primary)]">Container Name</span>
						<Input
							bind:value={newContainerName}
							placeholder="e.g. redis-cache"
							class="text-xs"
							aria-label="Container Name"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<span class="text-xs font-medium text-[var(--text-primary)]">Container Image</span>
						<Input
							bind:value={newContainerImage}
							placeholder="e.g. docker.io/library/redis:alpine"
							class="text-xs"
							aria-label="Container Image"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<span class="text-xs font-medium text-[var(--text-primary)]">Internal Listening Port</span>
						<Input
							bind:value={newContainerPort}
							placeholder="e.g. 6379"
							class="text-xs font-mono"
							aria-label="Internal Port"
						/>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Will be reachable at localhost:{newContainerPort || 'PORT'} inside this pod
						</span>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 border-t border-[var(--border)] pt-3">
					<Button variant="secondary" size="sm" onclick={() => (isAddingContainer = false)}>
						Cancel
					</Button>
					<Button
						variant="primary"
						size="sm"
						onclick={handleCreateNewContainer}
						disabled={!newContainerName || !newContainerImage || isSubmitting}
					>
						Deploy into Pod
					</Button>
				</div>
			{/if}
		</div>
	</div>
{/if}
