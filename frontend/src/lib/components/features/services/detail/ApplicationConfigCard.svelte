<script lang="ts">
	import type { Service } from '$lib/types';
	import { LayerCard, Button, useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { untrack } from 'svelte';
	import { dataStore } from '$lib/data';
	import { Cube, Check, ArrowClockwise } from 'phosphor-svelte';
	import SourceBuildConfig from './config/SourceBuildConfig.svelte';
	import ServicePortConfig from './config/ServicePortConfig.svelte';
	import ServiceAdvancedConfig from './config/ServiceAdvancedConfig.svelte';
	import GitWebhookSection from './config/GitWebhookSection.svelte';

	interface Props {
		service: Service;
		onOpenCredentialModal?: (type: 'ssh' | 'registry' | 'git') => void;
	}

	let { service, onOpenCredentialModal }: Props = $props();

	const toast = useKumoToastManager();

	// 1. Source & Build State
	let sourceType = $state<'git' | 'image' | 'drop'>('git');
	let repoUrl = $state('');
	let branch = $state('main');
	let buildPath = $state('/');
	let sshKeyId = $state('');
	let gitCredentialId = $state('');
	let buildType = $state<'dockerfile' | 'nixpacks' | 'static'>('dockerfile');
	let dockerfilePath = $state('./Dockerfile');
	let buildArgsText = $state('');
	let webhookProvider = $state<'' | 'github' | 'gitlab' | 'gitea'>('');
	let rawImageName = $state('');
	let selectedRegistry = $state('docker.io');
	let dropFileName = $state<string | null>(null);

	// 2. Port & Networking State (Non-forced, optional)
	let containerPort = $state('');
	let hostPort = $state('');
	let hostIp = $state('0.0.0.0');

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

	// Synchronize on service prop change
	let initializedFor = $state('');

	// Mirror the server copy once per service. Re-syncing on every provider
	// identity change would wipe in-progress edits between status polls.
	$effect(() => {
		if (initializedFor === service.id) return;
		initializedFor = service.id;
		untrack(() => {
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
			gitCredentialId = service.gitCredentialId || '';
			buildType =
				service.buildType === 'nixpacks' || service.buildType === 'static'
					? 'dockerfile'
					: service.buildType || 'dockerfile';
			dockerfilePath = service.dockerfilePath || './Dockerfile';
			buildArgsText = (service.buildArgs ?? []).join('\n');
			webhookProvider =
				service.webhookProvider === 'gitlab' ||
				service.webhookProvider === 'gitea' ||
				service.webhookProvider === 'github'
					? service.webhookProvider
					: '';

			rawImageName = service.image || '';
			if (service.image) {
				if (service.image.includes('ghcr.io/')) selectedRegistry = 'ghcr.io';
				else if (service.image.includes('quay.io/')) selectedRegistry = 'quay.io';
				else selectedRegistry = 'docker.io';
			}

			containerPort = service.port ? String(service.port) : '';
			hostPort = service.hostPort ? String(service.hostPort) : '';
			hostIp = service.hostIp === '127.0.0.1' ? '127.0.0.1' : '0.0.0.0';

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
	});

	function parseBuildArgs(raw: string): string[] {
		return raw
			.split(/\r?\n/)
			.map((line) => line.trim())
			.filter(Boolean);
	}

	// This card owns application/image/drop sources. A leftover Compose,
	// Kubernetes, or Quadlet manifest would win over the selected source in
	// the deploy dispatcher, so clear those fields when the card is saved.
	function clearDeclarativeWorkloadConfig() {
		// Stale manifests would take precedence over the selected source.
		service.composeYaml = '';
		service.k8sYaml = '';
		// Quadlet-managed workloads are edited in the Quadlet tab; this card is
		// also rendered on their General tab and must not erase their unit.
		if (service.type === 'quadlet' || service.runtimeTarget === 'quadlet') return;
		service.quadletConfig = '';
	}

	async function handleSave() {
		isSaving = true;
		try {
			// 1. Source & Image validation
			if (sourceType === 'git') {
				if (!repoUrl.trim()) {
					toast.add(
						errorToastOptions(
							'Repository URL is required',
							'Please specify a Git clone URL for this application.'
						)
					);
					return;
				}
				const buildArgs = parseBuildArgs(buildArgsText);
				const invalidArg = buildArgs.find((arg) => {
					const separator = arg.indexOf('=');
					if (separator <= 0) return true;
					return /\s/.test(arg.slice(0, separator).trim());
				});
				if (invalidArg) {
					toast.add(
						errorToastOptions(
							'Invalid build argument',
							`"${invalidArg}" must use the KEY=VALUE form.`
						)
					);
					return;
				}
				service.sourceType = 'git';
				service.source = repoUrl.trim();
				service.branch = branch.trim() || 'main';
				service.buildPath = buildPath.trim() || '/';
				service.buildType = 'dockerfile';
				service.dockerfilePath = dockerfilePath.trim();
				service.sshKeyId = sshKeyId || undefined;
				service.gitCredentialId = gitCredentialId || undefined;
				service.buildArgs = buildArgs;
				service.webhookProvider = webhookProvider;
				service.image = undefined;
			} else if (sourceType === 'image') {
				if (!rawImageName.trim()) {
					toast.add(
						errorToastOptions(
							'Container image is required',
							'Please specify an image name (e.g. redis:alpine, nginx:latest).'
						)
					);
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
					toast.add(
						errorToastOptions(
							'Invalid container port',
							'Container internal port must be a number between 1 and 65535.'
						)
					);
					return;
				}
				service.port = cp;
			} else {
				service.port = undefined;
			}

			if (hostPort.trim()) {
				const hp = parseInt(hostPort.trim(), 10);
				if (isNaN(hp) || hp < 1 || hp > 65535) {
					toast.add(
						errorToastOptions(
							'Invalid host port',
							'Published host port must be a number between 1 and 65535.'
						)
					);
					return;
				}
				service.hostPort = hp;
				service.hostIp = hostIp === '127.0.0.1' ? '127.0.0.1' : '0.0.0.0';
			} else {
				service.hostPort = undefined;
				service.hostIp = undefined;
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
			clearDeclarativeWorkloadConfig();

			await dataStore.updateService(service);
			saveStatus = 'saved';
			toast.add({
				title: 'Settings saved',
				description: `Application configuration for ${service.name} has been updated.`,
				variant: 'success'
			});
			setTimeout(() => (saveStatus = 'idle'), 2000);
		} catch (err: any) {
			toast.add(
				errorToastOptions('Save failed', err?.message || 'Could not save application configuration')
			);
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
			bind:sshKeyId
			bind:gitCredentialId
			bind:buildArgsText
			bind:rawImageName
			bind:selectedRegistry
			bind:dropFileName
			bind:containerPort
			{onOpenCredentialModal}
		/>

		{#if sourceType === 'git'}
			<GitWebhookSection {service} bind:provider={webhookProvider} />
		{/if}

		<!-- SECTION 2: Networking & Ports -->
		<ServicePortConfig {service} bind:containerPort bind:hostPort bind:hostIp />

		<!-- SECTION 3: Advanced Options -->
		<ServiceAdvancedConfig
			bind:showAdvanced
			bind:runtimeTarget
			bind:podName
			bind:cpuLimit
			bind:memLimit
			bind:autoDeploy
			bind:watchPaths
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
