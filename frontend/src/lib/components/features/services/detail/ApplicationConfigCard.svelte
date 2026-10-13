<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, Select, CopyButton } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import {
		Cube,
		GitBranch,
		Package,
		UploadSimple,
		Check,
		Info,
		Plus,
		SlidersHorizontal,
		CaretDown,
		ArrowClockwise,
		Stack,
		FileText,
		SpinnerGap
	} from 'phosphor-svelte';

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
		// Source
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

		const img = service.image || '';
		if (service.registryId && dataStore.registries.some((r) => r.id === service.registryId)) {
			selectedRegistry = service.registryId;
			const regObj = dataStore.registries.find((r) => r.id === service.registryId);
			const cleanUrl = regObj ? regObj.url.replace(/^https?:\/\//, '').replace(/\/$/, '') : '';
			rawImageName = cleanUrl && img.startsWith(cleanUrl + '/') ? img.slice(cleanUrl.length + 1) : img;
		} else if (img.startsWith('ghcr.io/')) {
			selectedRegistry = 'ghcr.io';
			rawImageName = img.replace(/^ghcr\.io\//, '');
		} else if (img.startsWith('quay.io/')) {
			selectedRegistry = 'quay.io';
			rawImageName = img.replace(/^quay\.io\//, '');
		} else if (img.startsWith('docker.io/')) {
			selectedRegistry = 'docker.io';
			rawImageName = img.replace(/^docker\.io\/(library\/)?/, '');
		} else {
			selectedRegistry = 'docker.io';
			rawImageName = img;
		}

		// Ports: Leave empty if unset, no forced 3000
		containerPort = service.port && service.port > 0 ? String(service.port) : '';
		hostPort = service.hostPort && service.hostPort > 0 ? String(service.hostPort) : '';

		// Advanced
		runtimeTarget =
			service.runtimeTarget ||
			(service.type === 'pod' ? 'pod' : service.type === 'quadlet' ? 'quadlet' : 'standalone');
		podName = service.podId || (dataStore.pods[0]?.name ?? '');

		const existingCpu = service.cpuLimit || parseFloat(service.advanced?.resources?.cpuLimit || '0') || 0;
		cpuLimit = existingCpu > 0 ? String(existingCpu) : '';

		const existingMem = service.memoryLimit || 0;
		if (existingMem > 0) {
			memLimit = `${existingMem}MB`;
		} else if (service.advanced?.resources?.memoryLimit && service.advanced.resources.memoryLimit !== '0') {
			memLimit = service.advanced.resources.memoryLimit;
		} else {
			memLimit = '';
		}

		autoDeploy = service.autoDeploy ?? true;
		watchPaths = service.watchPaths || '';
	});

	async function handleSave() {
		try {
			isSaving = true;
			const wasQuadlet = service.runtimeTarget === 'quadlet' || service.type === 'quadlet';

			// 1. Source
			service.sourceType = sourceType;
			if (sourceType === 'git') {
				service.source = repoUrl.trim();
				service.branch = branch.trim() || 'main';
				service.buildPath = buildPath.trim() || '/';
				service.sshKeyId = sshKeyId;
				service.buildType = buildType;
				service.dockerfilePath = dockerfilePath.trim() || './Dockerfile';
				service.publishDir = publishDir.trim() || 'dist';
			} else if (sourceType === 'image') {
				service.image = resolvedImage;
				const isCustom = dataStore.registries.some((r) => r.id === selectedRegistry);
				service.registryId = isCustom ? selectedRegistry : '';
			}

			// 2. Ports: optional, no forced fallback
			service.port = containerPort.trim() ? parseInt(containerPort.trim(), 10) || 0 : 0;
			service.hostPort = hostPort.trim() ? parseInt(hostPort.trim(), 10) || 0 : 0;

			// 3. Runtime & Resources
			service.runtimeTarget = runtimeTarget;
			service.inPod = runtimeTarget === 'pod';
			if (runtimeTarget === 'pod') {
				service.podId = podName;
			}

			const parsedCpu = parseFloat(cpuLimit.trim());
			const finalCpu = !isNaN(parsedCpu) && parsedCpu > 0 ? parsedCpu : 0;
			service.cpuLimit = finalCpu;

			let parsedMem = 0;
			const trimmedMem = memLimit.trim().toLowerCase();
			if (trimmedMem.endsWith('gb') || trimmedMem.endsWith('g')) {
				parsedMem = Math.round((parseFloat(trimmedMem) || 0) * 1024);
			} else if (trimmedMem.endsWith('mb') || trimmedMem.endsWith('m')) {
				parsedMem = Math.round(parseFloat(trimmedMem) || 0);
			} else if (trimmedMem) {
				parsedMem = Math.round(parseFloat(trimmedMem) || 0);
			}
			service.memoryLimit = parsedMem;

			if (!service.advanced) {
				service.advanced = {
					runtime: { mode: 'rootless', userNamespace: '', devices: [] },
					lifecycle: {
						quadletEnabled: runtimeTarget === 'quadlet',
						systemdUnitName: `${service.name}.container`,
						restartPolicy: 'always'
					},
					security: {
						privileged: false,
						selinuxLabel: 'container_file_t',
						capAdd: [],
						capDrop: [],
						noNewPrivileges: true
					},
					storage: { volumes: [] },
					resources: {
						cpuLimit: finalCpu > 0 ? String(finalCpu) : '0',
						memoryLimit: parsedMem > 0 ? `${parsedMem}MB` : '0',
						pidsLimit: 2048,
						swapLimit: '0'
					}
				};
			} else {
				service.advanced.lifecycle.quadletEnabled = runtimeTarget === 'quadlet';
				service.advanced.resources.cpuLimit = finalCpu > 0 ? String(finalCpu) : '0';
				service.advanced.resources.memoryLimit = parsedMem > 0 ? `${parsedMem}MB` : '0';
			}

			if (wasQuadlet && runtimeTarget !== 'quadlet') {
				try {
					await dataStore.demoteQuadlet(service.id);
				} catch (err) {
					console.warn('Demote quadlet during runtime target change:', err);
				}
			}

			if (runtimeTarget === 'pod' && podName) {
				try {
					await dataStore.groupIntoPod(
						service.id,
						podName,
						true,
						service.hostPort ? [`${service.hostPort}:${service.port || 80}`] : undefined
					);
				} catch (err) {
					console.warn('Group into pod during runtime save:', err);
				}
			}

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

	let fileInputRef = $state<HTMLInputElement | null>(null);
	let isUploading = $state(false);
	let uploadStatus = $state<string | null>(null);
	let extractedCount = $state<number | null>(null);

	async function uploadZipFile(file: File) {
		if (!file.name.toLowerCase().endsWith('.zip')) {
			toast.add({
				title: 'Invalid file format',
				description: 'Please upload a .zip archive containing your static files or dist folder.',
				variant: 'error'
			});
			return;
		}

		isUploading = true;
		dropFileName = file.name;
		uploadStatus = 'Uploading and extracting archive...';

		try {
			const formData = new FormData();
			formData.append('file', file);
			const res = await fetch(`/api/services/${service.id}/drop-upload`, {
				method: 'POST',
				body: formData
			});
			if (!res.ok) {
				const errData = await res.json().catch(() => ({ error: 'Upload failed' }));
				throw new Error(errData.error || 'Failed to upload static zip archive');
			}
			const data = await res.json();
			extractedCount = data.filesCount;
			sourceType = 'drop';
			service.sourceType = 'drop';
			service.source = data.directory;
			if (!containerPort) containerPort = '80';
			toast.add({
				title: 'Static Archive Extracted',
				description: `Successfully extracted ${data.filesCount} static files to disk.`,
				variant: 'success'
			});
			await dataStore.updateService(service);
		} catch (err: any) {
			toast.add({
				title: 'Upload Failed',
				description: err?.message || 'Failed to process static archive',
				variant: 'error'
			});
		} finally {
			isUploading = false;
			uploadStatus = null;
		}
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer?.files && e.dataTransfer.files[0]) {
			uploadZipFile(e.dataTransfer.files[0]);
		}
	}

	function handleFileInputChange(e: Event) {
		const input = e.target as HTMLInputElement;
		if (input.files && input.files[0]) {
			uploadZipFile(input.files[0]);
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
				<span class="text-[11px] text-kumo-subtle">Source, networking, and deployment settings.</span>
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
		<div class="flex flex-col gap-4">
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div class="flex flex-col gap-0.5">
					<h3 class="m-0 text-xs font-semibold text-kumo-default">Source & build</h3>
					<p class="m-0 text-[11px] text-kumo-subtle">
						Select the build origin for this container image.
					</p>
				</div>

				<!-- Source Type Segmented Pills -->
				<div
					class="inline-flex rounded-md border border-kumo-line bg-kumo-surface p-0.5 text-xs"
					role="tablist"
					aria-label="Source Type"
				>
					<button
						type="button"
						onclick={() => (sourceType = 'git')}
						class="flex cursor-pointer items-center gap-1.5 rounded-sm px-2.5 py-1 text-xs transition-colors {sourceType ===
						'git'
							? 'bg-kumo-base font-medium text-kumo-default shadow-xs ring-1 ring-kumo-line'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GitBranch size={13} />
						<span>Git repository</span>
					</button>
					<button
						type="button"
						onclick={() => (sourceType = 'image')}
						class="flex cursor-pointer items-center gap-1.5 rounded-sm px-2.5 py-1 text-xs transition-colors {sourceType ===
						'image'
							? 'bg-kumo-base font-medium text-kumo-default shadow-xs ring-1 ring-kumo-line'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<Package size={13} />
						<span>Container image</span>
					</button>
					<button
						type="button"
						onclick={() => (sourceType = 'drop')}
						class="flex cursor-pointer items-center gap-1.5 rounded-sm px-2.5 py-1 text-xs transition-colors {sourceType ===
						'drop'
							? 'bg-kumo-base font-medium text-kumo-default shadow-xs ring-1 ring-kumo-line'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<UploadSimple size={13} />
						<span>Static drop</span>
					</button>
				</div>
			</div>

			{#if sourceType === 'git'}
				<div class="flex flex-col gap-3 rounded-lg border border-kumo-line bg-kumo-surface p-4">
					<!-- Repo URL & Branch (Constrained Widths) -->
					<div class="flex flex-wrap items-end gap-3">
						<div class="flex flex-1 min-w-[280px] max-w-lg flex-col gap-1.5">
							<label for="repo-url" class="text-xs font-medium text-kumo-default">
								Repository URL <span class="text-kumo-danger">*</span>
							</label>
							<Input
								id="repo-url"
								bind:value={repoUrl}
								placeholder="https://github.com/org/repo or git@github.com:org/repo"
								class="h-8.5 font-mono text-xs"
							/>
						</div>

						<div class="flex w-36 flex-col gap-1.5">
							<label for="repo-branch" class="text-xs font-medium text-kumo-default">Branch</label>
							<Input
								id="repo-branch"
								bind:value={branch}
								placeholder="main"
								class="h-8.5 font-mono text-xs"
							/>
						</div>
					</div>

					<!-- Build Strategy & Context -->
					<div class="flex flex-wrap items-center gap-4 pt-1">
						<div class="flex flex-col gap-1.5">
							<span class="text-[11px] font-medium text-kumo-subtle">Build method</span>
							<div class="flex items-center gap-1.5">
								<button
									type="button"
									onclick={() => (buildType = 'dockerfile')}
									class="cursor-pointer rounded-sm border px-2.5 py-1 text-xs transition-colors {buildType ===
									'dockerfile'
										? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
										: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
								>
									Dockerfile
								</button>
								<button
									type="button"
									onclick={() => (buildType = 'nixpacks')}
									class="cursor-pointer rounded-sm border px-2.5 py-1 text-xs transition-colors {buildType ===
									'nixpacks'
										? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
										: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
								>
									Nixpacks
								</button>
								<button
									type="button"
									onclick={() => (buildType = 'static')}
									class="cursor-pointer rounded-sm border px-2.5 py-1 text-xs transition-colors {buildType ===
									'static'
										? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
										: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
								>
									Static SPA
								</button>
							</div>
						</div>

						{#if buildType === 'dockerfile'}
							<div class="flex w-44 flex-col gap-1.5">
								<label for="dockerfile-path" class="text-[11px] font-medium text-kumo-subtle">
									Dockerfile path
								</label>
								<Input
									id="dockerfile-path"
									bind:value={dockerfilePath}
									placeholder="./Dockerfile"
									class="h-8 font-mono text-xs"
								/>
							</div>
						{:else if buildType === 'static'}
							<div class="flex w-36 flex-col gap-1.5">
								<label for="publish-dir" class="text-[11px] font-medium text-kumo-subtle">
									Publish directory
								</label>
								<Input
									id="publish-dir"
									bind:value={publishDir}
									placeholder="dist"
									class="h-8 font-mono text-xs"
								/>
							</div>
						{/if}

						<!-- Deploy Key Picker -->
						<div class="flex flex-col gap-1.5">
							<div class="flex items-center justify-between gap-2">
								<span class="text-[11px] font-medium text-kumo-subtle">SSH deploy key</span>
								{#if onOpenCredentialModal}
									<button
										type="button"
										onclick={() => onOpenCredentialModal('ssh')}
										class="cursor-pointer text-[10px] text-[var(--accent)] hover:underline"
									>
										+ Add key
									</button>
								{/if}
							</div>
							<Select
								bind:value={sshKeyId}
								options={[
									{ value: '', label: 'None (Public repo)' },
									...dataStore.sshKeys.map((k) => ({ value: k.id, label: `${k.name} (${k.type})` }))
								]}
								class="h-8 w-48 text-xs"
							/>
						</div>
					</div>
				</div>
			{:else if sourceType === 'image'}
				<div class="flex flex-col gap-3 rounded-lg border border-kumo-line bg-kumo-surface p-4">
					<div class="flex flex-wrap items-end gap-3">
						<div class="flex flex-1 min-w-[280px] max-w-md flex-col gap-1.5">
							<label for="image-name" class="text-xs font-medium text-kumo-default">
								Container image
							</label>
							<Input
								id="image-name"
								bind:value={rawImageName}
								placeholder="e.g. docker.io/library/alpine:latest"
								class="h-8.5 font-mono text-xs"
							/>
						</div>

						<div class="flex w-48 flex-col gap-1.5">
							<div class="flex items-center justify-between gap-2">
								<span class="text-xs font-medium text-kumo-default">Registry</span>
								{#if onOpenCredentialModal}
									<button
										type="button"
										onclick={() => onOpenCredentialModal('registry')}
										class="cursor-pointer text-[10px] text-[var(--accent)] hover:underline"
									>
										+ Add registry
									</button>
								{/if}
							</div>
							<Select
								bind:value={selectedRegistry}
								options={[
									{ value: 'docker.io', label: 'Docker Hub' },
									{ value: 'ghcr.io', label: 'GitHub (ghcr.io)' },
									{ value: 'quay.io', label: 'Red Hat Quay' },
									...dataStore.registries.map((r) => ({ value: r.id, label: r.name }))
								]}
								class="h-8.5 text-xs"
							/>
						</div>
					</div>
				</div>
			{:else}
				<input
					type="file"
					accept=".zip"
					bind:this={fileInputRef}
					onchange={handleFileInputChange}
					class="hidden"
				/>
				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					ondragover={(e) => e.preventDefault()}
					ondrop={handleDrop}
					onclick={() => !isUploading && fileInputRef?.click()}
					role="button"
					tabindex="0"
					onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && fileInputRef?.click()}
					aria-label="Upload static build artifact"
					class="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-kumo-line bg-kumo-surface p-6 text-center transition-all hover:border-[var(--accent)] hover:bg-kumo-elevated/40"
				>
					{#if isUploading}
						<SpinnerGap size={24} class="animate-spin text-[var(--accent)]" />
						<div class="flex flex-col gap-0.5">
							<span class="text-xs font-medium text-kumo-default">Extracting {dropFileName}...</span>
							<span class="text-[11px] text-kumo-subtle">Unpacking archive and verifying security...</span>
						</div>
					{:else if extractedCount !== null || service.sourceType === 'drop'}
						<div
							class="flex size-8 items-center justify-center rounded-full bg-kumo-success/20 text-kumo-success ring-1 ring-kumo-success/30"
						>
							<Check size={16} weight="bold" />
						</div>
						<div class="flex flex-col gap-0.5">
							<span class="text-xs font-semibold text-kumo-default">
								{dropFileName ? `Uploaded: ${dropFileName}` : 'Static Site Archive Ready'}
							</span>
							<span class="text-[11px] text-kumo-subtle">
								{extractedCount !== null ? `${extractedCount} files extracted` : 'Static files served via Nginx'} (Click or drop another .zip to replace)
							</span>
						</div>
					{:else}
						<UploadSimple size={24} class="text-kumo-subtle" />
						<div class="flex flex-col gap-0.5">
							<span class="text-xs font-medium text-kumo-default">
								{dropFileName || 'Drop .zip archive here or click to browse'}
							</span>
							<span class="text-[11px] text-kumo-subtle">
								HTML, SPA (React, Svelte, Vue), or static asset bundle. Nginx will serve it instantly.
							</span>
						</div>
					{/if}
				</div>
			{/if}
		</div>

		<!-- SECTION 2: Networking & Ports (Streamlined & Non-Forced) -->
		<div class="flex flex-col gap-3 border-t border-kumo-hairline pt-5">
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-xs font-semibold text-kumo-default">Networking & ports</h3>
				<p class="m-0 text-[11px] text-kumo-subtle">
					Configure container listening port and optional host bindings.
				</p>
			</div>

			<!-- Compact 2-Column Port Inputs -->
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 max-w-xl">
				<!-- Internal Port -->
				<div class="flex flex-col gap-1.5">
					<label for="internal-port" class="text-xs font-medium text-kumo-default">
						Internal container port <span class="font-normal text-kumo-subtle">(optional)</span>
					</label>
					<Input
						id="internal-port"
						bind:value={containerPort}
						placeholder="e.g. 80, 8080 (optional)"
						class="h-8.5 w-full font-mono text-xs"
					/>
					<span class="text-[11px] text-kumo-subtle leading-normal">
						Port your application listens on. Leave empty if this is a background worker or non-HTTP service.
					</span>
				</div>

				<!-- Published Host Port -->
				<div class="flex flex-col gap-1.5">
					<label for="host-port" class="text-xs font-medium text-kumo-default">
						Published host port <span class="font-normal text-kumo-subtle">(optional)</span>
					</label>
					<Input
						id="host-port"
						bind:value={hostPort}
						placeholder="e.g. 8080 (optional)"
						class="h-8.5 w-full font-mono text-xs"
					/>
					<span class="text-[11px] text-kumo-subtle leading-normal">
						Direct host exposure (0.0.0.0:port). Leave empty to isolate container on <code>gopod-net</code>.
					</span>
				</div>
			</div>

			<!-- Compact Inline Tip Banner (Minimal 1-line Banner) -->
			<div
				class="flex items-center gap-2 rounded-md border border-kumo-hairline bg-kumo-base px-3 py-2 text-xs text-kumo-subtle"
			>
				<Info size={14} class="shrink-0 text-[var(--accent)]" />
				<span>
					Containers on <strong>gopod-net</strong> are reachable by service name (e.g.
					<code class="font-mono text-kumo-default">http://{service.name}:{containerPort || '80'}</code>)
					without publishing host ports.
				</span>
			</div>
		</div>

		<!-- SECTION 3: Advanced Options (Collapsible Progressive Disclosure) -->
		<div class="flex flex-col border-t border-kumo-hairline pt-3">
			<button
				type="button"
				onclick={() => (showAdvanced = !showAdvanced)}
				class="flex w-full cursor-pointer items-center justify-between rounded-md py-2 text-xs font-medium text-kumo-subtle hover:text-kumo-default"
			>
				<div class="flex items-center gap-2">
					<SlidersHorizontal size={14} />
					<span>Advanced options</span>
					<span class="text-[11px] text-kumo-subtle font-normal">
						(Resource limits, auto-deploy triggers, runtime target)
					</span>
				</div>
				<CaretDown
					size={14}
					class="transition-transform duration-150 {showAdvanced ? 'rotate-180' : ''}"
				/>
			</button>

			{#if showAdvanced}
				<div class="flex flex-col gap-5 pt-3 pb-1">
					<!-- 1. Runtime Execution Target -->
					<div class="flex flex-col gap-2 rounded-lg border border-kumo-line bg-kumo-surface p-3.5">
						<span class="text-xs font-medium text-kumo-default">Runtime execution mode</span>
						<div class="flex flex-wrap items-center gap-2">
							<button
								type="button"
								onclick={() => (runtimeTarget = 'standalone')}
								class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
								'standalone'
									? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
									: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
							>
								Standalone container
							</button>
							<button
								type="button"
								onclick={() => (runtimeTarget = 'pod')}
								class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
								'pod'
									? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
									: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
							>
								Podman pod
							</button>
							<button
								type="button"
								onclick={() => (runtimeTarget = 'quadlet')}
								class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
								'quadlet'
									? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
									: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
							>
								Systemd Quadlet
							</button>
						</div>

						{#if runtimeTarget === 'pod'}
							<div class="flex items-center gap-3 pt-2">
								<span class="text-xs text-kumo-subtle">Target pod:</span>
								<Select
									bind:value={podName}
									options={dataStore.pods.map((p) => ({ value: p.name, label: p.name }))}
									class="h-8 w-52 text-xs"
								/>
							</div>
						{/if}
					</div>

					<!-- 2. Resource Limits (cgroups v2) -->
					<div class="flex flex-col gap-2 rounded-lg border border-kumo-line bg-kumo-surface p-3.5">
						<span class="text-xs font-medium text-kumo-default">Resource limits (cgroups v2)</span>
						<div class="flex flex-wrap items-center gap-4">
							<div class="flex w-40 flex-col gap-1">
								<label for="cpu-limit" class="text-[11px] text-kumo-subtle">CPU limit (cores)</label>
								<Input
									id="cpu-limit"
									bind:value={cpuLimit}
									placeholder="e.g. 1.0 (unlimited)"
									class="h-8 font-mono text-xs"
								/>
							</div>

							<div class="flex w-40 flex-col gap-1">
								<label for="mem-limit" class="text-[11px] text-kumo-subtle">Memory limit</label>
								<Input
									id="mem-limit"
									bind:value={memLimit}
									placeholder="e.g. 512MB, 1GB"
									class="h-8 font-mono text-xs"
								/>
							</div>
						</div>
					</div>

					<!-- 3. Deployment Triggers & Webhooks -->
					<div class="flex flex-col gap-3 rounded-lg border border-kumo-line bg-kumo-surface p-3.5">
						<span class="text-xs font-medium text-kumo-default">Deployment triggers</span>

						<div class="flex items-center justify-between gap-4 max-w-md">
							<div class="flex flex-col gap-0.5">
								<span class="text-xs font-medium text-kumo-default">Auto-deploy on git push</span>
								<span class="text-[11px] text-kumo-subtle">Roll out updates when new commits arrive.</span>
							</div>
							<Switch bind:checked={autoDeploy} size="sm" />
						</div>

						<div class="flex max-w-md flex-col gap-1 pt-1">
							<label for="watch-paths" class="text-[11px] text-kumo-subtle">
								Watch paths (optional)
							</label>
							<Input
								id="watch-paths"
								bind:value={watchPaths}
								placeholder="src/**, packages/app/**"
								class="h-8 font-mono text-xs"
							/>
						</div>

						<div class="flex max-w-md flex-col gap-1 pt-1">
							<span class="text-[11px] text-kumo-subtle">Webhook endpoint</span>
							<div class="flex items-center gap-2">
								<Input
									value={webhookUrl}
									readonly
									class="h-8 flex-1 font-mono text-xs bg-kumo-base text-kumo-subtle"
								/>
								<CopyButton text={webhookUrl} />
							</div>
						</div>
					</div>
				</div>
			{/if}
		</div>

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
