<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input, Select, useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Modal, Tabs } from '$lib/components/ui';
	import { Package } from 'phosphor-svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';

	interface Props {
		open: boolean;
		service: Service;
		podName: string;
		candidateServices: Service[];
		onclose: () => void;
		onmemberattached?: () => void;
	}

	let {
		open = $bindable(false),
		service,
		podName,
		candidateServices,
		onclose,
		onmemberattached
	}: Props = $props();

	const toast = useKumoToastManager();

	let attachMode = $state<'existing' | 'new'>('existing');
	let selectedServiceId = $state('');
	let newContainerName = $state('');
	let newContainerImage = $state('');
	let newContainerPort = $state('80');
	let isSubmitting = $state(false);

	let candidateOptions = $derived(
		candidateServices.map((c) => ({
			value: c.id,
			label: `${c.name} (${c.type} - ${c.image || c.source || 'workload'})`
		}))
	);

	async function handleAttachExisting() {
		if (!selectedServiceId) return;
		isSubmitting = true;

		const target = dataStore.services.find((s) => s.id === selectedServiceId);
		if (!target) {
			isSubmitting = false;
			return;
		}

		try {
			await dataStore.groupIntoPod(target.id, podName, false);
			toast.add({
				title: 'Service Attached to Pod',
				description: `${target.name} joined ${podName}`,
				variant: 'success'
			});
			selectedServiceId = '';
			onmemberattached?.();
			onclose();
		} catch (err: any) {
			toast.add(errorToastOptions('Attach Failed', err?.message || 'Failed to attach service'));
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
			onmemberattached?.();
			onclose();
		} catch (err: any) {
			toast.add(errorToastOptions('Failed to Create Container', err?.message || 'Creation error'));
		} finally {
			isSubmitting = false;
		}
	}
</script>

<Modal
	{open}
	{onclose}
	title="Add / Attach Container to Pod"
	subtitle={`Target pod ${podName} shares one localhost network namespace`}
	icon={Package}
	size="md"
>
	<Tabs
		variant="segmented"
		size="sm"
		active={attachMode}
		onchange={(v: string) => (attachMode = v as 'existing' | 'new')}
		tabs={[
			{ id: 'existing', label: 'Attach Existing Service' },
			{ id: 'new', label: 'Deploy New Image' }
		]}
	/>

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
					<Select
						value={selectedServiceId}
						onValueChange={(v: any) => {
							if (typeof v === 'string') selectedServiceId = v;
						}}
						options={candidateOptions}
						size="sm"
					/>
				</div>
			{/if}
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
					class="font-mono text-xs"
					aria-label="Internal Port"
				/>
				<span class="text-xs text-[var(--text-tertiary)]">
					Will be reachable at localhost:{newContainerPort || 'PORT'} inside this pod
				</span>
			</div>
		</div>
	{/if}

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={onclose}>Cancel</Button>
		{#if attachMode === 'existing'}
			<Button
				variant="primary"
				size="sm"
				onclick={handleAttachExisting}
				disabled={!selectedServiceId || isSubmitting}
			>
				Attach to Pod
			</Button>
		{:else}
			<Button
				variant="primary"
				size="sm"
				onclick={handleCreateNewContainer}
				disabled={!newContainerName || !newContainerImage || isSubmitting}
			>
				Deploy into Pod
			</Button>
		{/if}
	{/snippet}
</Modal>
