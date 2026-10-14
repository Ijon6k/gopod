<script lang="ts">
	import type { Service } from '$lib/types';
	import { LayerCard, Button, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { Cube, Check, ArrowClockwise } from 'phosphor-svelte';
	import SourceBuildConfig from './config/SourceBuildConfig.svelte';
	import ServicePortConfig from './config/ServicePortConfig.svelte';
	import ServiceAdvancedConfig from './config/ServiceAdvancedConfig.svelte';

	interface Props {
		service: Service;
		onOpenCredentialModal?: (type: 'ssh' | 'registry') => void;
	}

	let { service, onOpenCredentialModal }: Props = $props();

	const toast = useKumoToastManager();

	// 1. Source & Build State
	let sourceType = $state<'git' | 'image' | 'drop'>('git');
	let repoUrl = $state('');
	let branch = $state('main');
	let buildPath = $state('/');
	let sshKeyId = $state('');
	let buildType = $state<'dockerfile' | 'nixpacks' | 'static'>('dockerfile');
	let dockerfilePath = $state('./Dockerfile');
	let publishDir = $state('dist');
	let rawImageName = $state('');
	let selectedRegistry = $state('docker.io');
	let dropFileName = $state<string | null>(null);

	// 2. Port & Networking State (Non-forced, optional)
	let containerPort = $state('');
	let hostPort = $state('');

	// 3. Advanced / Secondary State (Progressive disclosure)
	let showAdvanced = $state(false);
	let runtimeTarget = $state<'standalone' | 'pod' | 'quadlet'>('standalone');
	let podName = $state('');
	let cpuLimit = $state('');
	let memLimit = $state('');
	let autoDeploy = $state(true);
	let watchPaths = $state('');

	let isSaving = $state(false);
	let saveStatus = $state<'idle' | 'saved'>('idle');

	let resolvedImage = $derived.by(() => {
		const clean = rawImageName.trim();
		if (!clean) return '';
		if (clean.includes('/') && (clean.startsWith('http') || clean.split('/')[0].includes('.'))) {
			return clean;
		}
		if (selectedRegistry === 'docker.io') {
			return clean.includes('/') ? `docker.io/${clean}` : `docker.io/library/${clean}`;
		}
		if (selectedRegistry === 'ghcr.io') {
			return `ghcr.io/${clean}`;
		}
		if (selectedRegistry === 'quay.io') {
			return `quay.io/${clean}`;
		}
		const customReg = dataStore.registries.find((r) => r.id === selectedRegistry);
		if (customReg) {
			const cleanUrl = customReg.url.replace(/^https?:\/\//, '').replace(/\/$/, '');
			return `${cleanUrl}/${clean}`;
		}
		return clean;
	});

	let webhookUrl = $derived(api.services.getWebhookUrl(service.webhookToken || service.id));

	// Synchronize on service prop change
	$effect(() => {
		sourceType =
			service.sourceType === 'image' || service.sourceType === 'drop'
				? service.sourceType
				: service.image
					? 'image'
					: 'git';
		repoUrl = service.source || '';
		branch = service.branch || 'main';
		buildPath = service.buildPath || '/';
		sshKeyId = service.sshKeyId || (dataStore.sshKeys[0]?.id ?? '');
		buildType = service.buildType || 'dockerfile';
		dockerfilePath = service.dockerfilePath || './Dockerfile';
		publishDir = service.publishDir || 'dist';

		rawImageName = service.image || '';
		if (service.image) {
			if (service.image.includes('ghcr.io/')) selectedRegistry = 'ghcr.io';
			else if (service.image.includes('quay.io/')) selectedRegistry = 'quay.io';
			else selectedRegistry = 'docker.io';
		}

		containerPort = service.port ? String(service.port) : '';
		hostPort = service.hostPort ? String(service.hostPort) : '';

		runtimeTarget =
			service.runtimeTarget === 'quadlet'
				? 'quadlet'
				: service.inPod || service.runtimeTarget === 'pod'
					? 'pod'
					: 'standalone';
		podName = service.podId || (dataStore.pods[0]?.name ?? '');
		cpuLimit = service.cpuLimit != null ? String(service.cpuLimit) : '';
		memLimit = service.memoryLimit != null ? String(service.memoryLimit) : '';
		autoDeploy = service.autoDeploy ?? true;
		watchPaths = service.watchPaths || '';
	});

	async function handleSave() {
		isSaving = true;
		try {
			// 1. Source & Image validation
			if (sourceType === 'git') {
				if (!repoUrl.trim()) {
					toast.add({
						title: 'Repository URL is required',
						description: 'Please specify a Git clone URL for this application.',
						variant: 'error'
					});
					return;
				}
				service.sourceType = 'git';
				service.source = repoUrl.trim();
				service.branch = branch.trim() || 'main';
				service.buildPath = buildPath.trim() || '/';
				service.buildType = buildType;
				service.dockerfilePath = dockerfilePath.trim();
				service.publishDir = publishDir.trim();
				service.sshKeyId = sshKeyId || undefined;
				service.image = undefined;
			} else if (sourceType === 'image') {
				if (!rawImageName.trim()) {
					toast.add({
						title: 'Container image is required',
						description: 'Please specify an image name (e.g. redis:alpine, nginx:latest).',
						variant: 'error'
					});
					return;
				}
				service.sourceType = 'image';
				service.image = resolvedImage;
				service.source = undefined;
			} else {
				service.sourceType = 'drop';
			}

			// 2. Ports (Optional, non-forced)
			if (containerPort.trim()) {
				const cp = parseInt(containerPort.trim(), 10);
				if (isNaN(cp) || cp < 1 || cp > 65535) {
					toast.add({
						title: 'Invalid container port',
						description: 'Container internal port must be a number between 1 and 65535.',
						variant: 'error'
					});
					return;
				}
				service.port = cp;
			} else {
				service.port = undefined;
			}

			if (hostPort.trim()) {
				const hp = parseInt(hostPort.trim(), 10);
				if (isNaN(hp) || hp < 1 || hp > 65535) {
					toast.add({
						title: 'Invalid host port',
						description: 'Published host port must be a number between 1 and 65535.',
						variant: 'error'
					});
					return;
				}
				service.hostPort = hp;
			} else {
				service.hostPort = undefined;
			}

			// 3. Runtime & Resources
			service.runtimeTarget = runtimeTarget;
			service.inPod = runtimeTarget === 'pod';
			service.podId = runtimeTarget === 'pod' ? podName : undefined;
			service.cpuLimit = cpuLimit.trim() ? parseFloat(cpuLimit.trim()) : undefined;
			service.memoryLimit = memLimit.trim() ? parseFloat(memLimit.trim()) : undefined;

			// 4. Triggers
			service.autoDeploy = autoDeploy;
			service.watchPaths = watchPaths.trim();

			await dataStore.updateService(service);
			saveStatus = 'saved';
			toast.add({
				title: 'Settings saved',
				description: `Application configuration for ${service.name} has been updated.`,
				variant: 'success'
			});
			setTimeout(() => (saveStatus = 'idle'), 2000);
		} catch (err: any) {
			toast.add({
				title: 'Save failed',
				description: err?.message || 'Could not save application configuration',
				variant: 'error'
			});
		} finally {
			isSaving = false;
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Unified Header with Title, Context, and Single Save Action -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-5 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring-1 ring-kumo-line"
			>
				<Cube size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<span class="text-xs font-semibold text-kumo-default">Application configuration</span>
				<span class="text-[11px] text-kumo-subtle"
					>Source, networking, and deployment settings.</span
				>
			</div>
		</div>

		<Button
			variant="primary"
			size="sm"
			onclick={handleSave}
			disabled={isSaving}
			class="gap-1.5 px-3.5 shadow-xs"
		>
			{#if isSaving}
				<ArrowClockwise size={13} class="animate-spin" />
				<span>Saving…</span>
			{:else if saveStatus === 'saved'}
				<Check size={13} weight="bold" />
				<span>Saved</span>
			{:else}
				<span>Save changes</span>
			{/if}
		</Button>
	</LayerCard.Secondary>

	<!-- Main Form Body -->
	<div class="flex flex-col gap-6 p-5">
		<!-- SECTION 1: Source & Build -->
		<SourceBuildConfig
			{service}
			bind:sourceType
			bind:repoUrl
			bind:branch
			bind:buildType
			bind:dockerfilePath
			bind:publishDir
			bind:sshKeyId
			bind:rawImageName
			bind:selectedRegistry
			bind:dropFileName
			bind:containerPort
			{onOpenCredentialModal}
		/>

		<!-- SECTION 2: Networking & Ports -->
		<ServicePortConfig {service} bind:containerPort bind:hostPort />

		<!-- SECTION 3: Advanced Options -->
		<ServiceAdvancedConfig
			bind:showAdvanced
			bind:runtimeTarget
			bind:podName
			bind:cpuLimit
			bind:memLimit
			bind:autoDeploy
			bind:watchPaths
			{webhookUrl}
		/>

		<!-- Single Unified Bottom Save Button -->
		<div class="flex items-center justify-end border-t border-kumo-hairline pt-4">
			<Button
				variant="primary"
				size="sm"
				onclick={handleSave}
				disabled={isSaving}
				class="gap-1.5 px-4 shadow-xs"
			>
				{#if isSaving}
					<ArrowClockwise size={13} class="animate-spin" />
					<span>Saving…</span>
				{:else if saveStatus === 'saved'}
					<Check size={13} weight="bold" />
					<span>Saved</span>
				{:else}
					<span>Save changes</span>
				{/if}
			</Button>
		</div>
	</div>
</LayerCard>
