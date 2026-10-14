<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input, useKumoToastManager } from 'kumo-svelte';
	import { Package } from 'phosphor-svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';

	interface Props {
		open: boolean;
		service: Service;
		podName: string;
		candidateServices: Service[];
		onclose: () => void;
	}

	let { open = $bindable(false), service, podName, candidateServices, onclose }: Props = $props();

	const toast = useKumoToastManager();

	let attachMode = $state<'existing' | 'new'>('existing');
	let selectedServiceId = $state('');
	let newContainerName = $state('');
	let newContainerImage = $state('');
	let newContainerPort = $state('80');
	let isSubmitting = $state(false);

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
			onclose();
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
		const cleanName = newContainerName
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9_-]/g, '-');

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
			let deployOk = true;
			try {
				await dataStore.deployService(created.id, 'manual');
			} catch (depErr: any) {
				deployOk = false;
				console.error('[GOPOD] Pod member initial deployment notice:', depErr);
				toast.add({
					title: 'Container Created with Deployment Warning',
					description: `${cleanName} created in ${podName}, but initial deploy failed: ${depErr?.message || 'Check logs'}`,
					variant: 'warning'
				});
			}
			if (deployOk) {
				toast.add({
					title: 'Container Created in Pod',
					description: `${cleanName} deployed into ${podName}`,
					variant: 'success'
				});
			}
			newContainerName = '';
			newContainerImage = '';
			newContainerPort = '80';
			onclose();
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
</script>

{#if open}
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
					onclick={onclose}
					class="cursor-pointer border-0 bg-transparent text-xs text-[var(--text-tertiary)] hover:text-[var(--text-primary)]"
				>
					✕
				</button>
			</div>

			<!-- Mode Selector -->
			<div
				class="grid grid-cols-2 gap-2 rounded-[var(--radius-sm)] bg-[var(--bg-panel)] p-1 text-xs"
			>
				<button
					type="button"
					onclick={() => (attachMode = 'existing')}
					class="cursor-pointer rounded py-1.5 font-medium transition-colors {attachMode ===
					'existing'
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
						<div
							class="rounded border border-[var(--border)] bg-[var(--bg-panel)] p-4 text-center text-xs text-[var(--text-tertiary)]"
						>
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
					<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>
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
						<span class="text-xs font-medium text-[var(--text-primary)]"
							>Internal Listening Port</span
						>
						<Input
							bind:value={newContainerPort}
							placeholder="e.g. 6379"
							class="font-mono text-xs"
							aria-label="Internal Port"
						/>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Will be reachable at localhost:{newContainerPort || 'PORT'} inside this pod
						</span>
					</div>
				</div>

				<div class="flex items-center justify-end gap-2 border-t border-[var(--border)] pt-3">
					<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>
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
