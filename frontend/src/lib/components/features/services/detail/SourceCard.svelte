<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, Select } from '$lib/components/ui';
	import { LayerCard, Button, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import {
		GitBranch,
		Package,
		UploadSimple,
		Plus,
		Check,
		FileCode,
		Sparkle,
		Browser
	} from 'phosphor-svelte';

	interface Props {
		service: Service;
		onOpenCredentialModal?: (type: 'ssh' | 'registry') => void;
	}

	let { service, onOpenCredentialModal }: Props = $props();

	let sourceType = $state<'git' | 'image' | 'drop'>('git');
	let repoUrl = $state('');
	let branch = $state('main');
	let buildPath = $state('/');
	let sshKeyId = $state('');

	// Build configuration (seamlessly unified with Git source)
	let buildType = $state<'dockerfile' | 'nixpacks' | 'static'>('dockerfile');
	let dockerfilePath = $state('./Dockerfile');
	let publishDir = $state('dist');

	let rawImageName = $state('');
	let selectedRegistry = $state('docker.io');

	let dropFileName = $state<string | null>(null);
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

	$effect(() => {
		sourceType = service.sourceType || (service.image ? 'image' : 'git');
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
			rawImageName =
				cleanUrl && img.startsWith(cleanUrl + '/') ? img.slice(cleanUrl.length + 1) : img;
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
	});

	const toast = useKumoToastManager();

	function handleSave() {
		service.sourceType = sourceType;
		if (sourceType === 'git') {
			service.source = repoUrl.trim();
			service.branch = branch.trim();
			service.buildPath = buildPath.trim();
			service.sshKeyId = sshKeyId;
			service.buildType = buildType;
			service.dockerfilePath = dockerfilePath.trim();
			service.publishDir = publishDir.trim();
		} else if (sourceType === 'image') {
			service.image = resolvedImage;
			const isCustom = dataStore.registries.some((r) => r.id === selectedRegistry);
			service.registryId = isCustom ? selectedRegistry : '';
		}
		dataStore.updateService(service);
		saveStatus = 'saved';
		toast.add({
			title: 'Source & Build Saved',
			description: `Configuration for ${service.name} updated successfully.`,
			variant: 'success'
		});
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer?.files && e.dataTransfer.files[0]) {
			dropFileName = e.dataTransfer.files[0].name;
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) with Title & Save Action -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3"
	>
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring ring-kumo-line"
			>
				{#if sourceType === 'image'}
					<Package size={15} weight="duotone" />
				{:else if sourceType === 'drop'}
					<UploadSimple size={15} weight="duotone" />
				{:else}
					<GitBranch size={15} weight="duotone" />
				{/if}
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-kumo-default">Source & Build</span>
					<span
						class="rounded-sm bg-kumo-base px-1.5 py-0.5 font-mono text-[10px] font-medium text-kumo-subtle ring-1 ring-kumo-hairline uppercase"
					>
						{sourceType}
					</span>
				</div>
				<span class="text-[11px] text-kumo-subtle">
					Origin repository, registry image, and container build strategy
				</span>
			</div>
		</div>

		<!-- Action in Header for immediate F-pattern scanning -->
		<div class="flex items-center gap-2">
			{#if saveStatus === 'saved'}
				<span class="flex items-center gap-1 font-mono text-xs text-kumo-success">
					<Check size={13} weight="bold" /> Saved
				</span>
			{/if}
			<Button
				variant="primary"
				size="sm"
				onclick={handleSave}
				class="gap-1.5 font-medium shadow-xs"
			>
				Save Source & Build
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Content Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-4">
		<!-- Compact Segmented Selector (Under 36px, zero screen bloat) -->
		<div class="flex items-center justify-between">
			<div
				class="inline-flex rounded-md bg-kumo-elevated p-1 ring-1 ring-kumo-hairline"
				role="tablist"
				aria-label="Source Type"
			>
				<button
					type="button"
					role="tab"
					aria-selected={sourceType === 'git'}
					onclick={() => (sourceType = 'git')}
					class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {sourceType ===
					'git'
						? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					<GitBranch size={14} class={sourceType === 'git' ? 'text-[var(--accent)]' : ''} />
					<span>Universal Git</span>
				</button>

				<button
					type="button"
					role="tab"
					aria-selected={sourceType === 'image'}
					onclick={() => (sourceType = 'image')}
					class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {sourceType ===
					'image'
						? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					<Package size={14} class={sourceType === 'image' ? 'text-[var(--accent)]' : ''} />
					<span>Container Image</span>
				</button>

				<button
					type="button"
					role="tab"
					aria-selected={sourceType === 'drop'}
					onclick={() => (sourceType = 'drop')}
					class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {sourceType ===
					'drop'
						? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					<UploadSimple size={14} class={sourceType === 'drop' ? 'text-[var(--accent)]' : ''} />
					<span>Static Drop</span>
				</button>
			</div>

			<span class="hidden font-mono text-[11px] text-kumo-subtle md:inline">
				{#if sourceType === 'git'}
					Pull from GitHub, GitLab, or self-hosted Git
				{:else if sourceType === 'image'}
					Pull from Docker Hub, GHCR, or Quay
				{:else}
					Serve static HTML/JS directly via Caddy
				{/if}
			</span>
		</div>

		<!-- 1. Universal Git Configuration -->
		{#if sourceType === 'git'}
			<div class="flex flex-col gap-3">
				<!-- Row 1: Repo URL & Branch (2-column layout) -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1 sm:col-span-2">
						<label for="src-git-url" class="text-xs font-medium text-kumo-default">
							Repository URL <span class="text-kumo-danger">*</span>
						</label>
						<Input
							id="src-git-url"
							bind:value={repoUrl}
							placeholder="git@github.com:org/repo.git or https://github.com/org/repo"
							class="text-xs font-mono"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<label for="src-git-branch" class="text-xs font-medium text-kumo-default">Branch</label>
						<Input
							id="src-git-branch"
							bind:value={branch}
							placeholder="main"
							class="text-xs font-mono"
						/>
					</div>
				</div>

				<!-- Row 2: Build Path & SSH Deploy Key -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
					<div class="flex flex-col gap-1">
						<label for="src-git-path" class="text-xs font-medium text-kumo-default">
							Build Context Path
						</label>
						<Input
							id="src-git-path"
							bind:value={buildPath}
							placeholder="/"
							class="text-xs font-mono"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<div class="flex items-center justify-between">
							<label for="src-ssh-key" class="text-xs font-medium text-kumo-default">
								SSH Deploy Key (Private Repos)
							</label>
							<button
								type="button"
								onclick={() => onOpenCredentialModal?.('ssh')}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
							>
								<Plus size={11} /> Add Key
							</button>
						</div>

						<Select
							id="src-ssh-key"
							value={sshKeyId}
							onValueChange={(val: any) => {
								if (typeof val === 'string') sshKeyId = val;
							}}
							options={[
								{ label: 'None (Public Repository)', value: '' },
								...dataStore.sshKeys.map((k) => ({
									label: `${k.name} (${k.type})`,
									value: k.id
								}))
							]}
							size="sm"
						/>
					</div>
				</div>

				<!-- Unified Build Strategy Section -->
				<div class="mt-1 flex flex-col gap-2 rounded-md bg-kumo-elevated p-3 ring-1 ring-kumo-hairline">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<span class="text-xs font-semibold text-kumo-default">Build Strategy</span>
						<!-- Build Strategy Pills -->
						<div
							class="inline-flex rounded bg-kumo-base p-0.5 ring-1 ring-kumo-line"
							role="tablist"
						>
							<button
								type="button"
								role="tab"
								aria-selected={buildType === 'dockerfile'}
								onclick={() => (buildType = 'dockerfile')}
								class="flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-[11px] font-medium transition-all {buildType ===
								'dockerfile'
									? 'bg-kumo-elevated text-kumo-default shadow-xs ring-1 ring-kumo-hairline'
									: 'text-kumo-subtle hover:text-kumo-default'}"
							>
								<FileCode size={13} class={buildType === 'dockerfile' ? 'text-[var(--accent)]' : ''} />
								<span>Dockerfile</span>
							</button>

							<button
								type="button"
								role="tab"
								aria-selected={buildType === 'nixpacks'}
								onclick={() => (buildType = 'nixpacks')}
								class="flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-[11px] font-medium transition-all {buildType ===
								'nixpacks'
									? 'bg-kumo-elevated text-kumo-default shadow-xs ring-1 ring-kumo-hairline'
									: 'text-kumo-subtle hover:text-kumo-default'}"
							>
								<Sparkle size={13} class={buildType === 'nixpacks' ? 'text-[var(--accent)]' : ''} />
								<span>Nixpacks</span>
							</button>

							<button
								type="button"
								role="tab"
								aria-selected={buildType === 'static'}
								onclick={() => (buildType = 'static')}
								class="flex cursor-pointer items-center gap-1 rounded-sm px-2 py-1 text-[11px] font-medium transition-all {buildType ===
								'static'
									? 'bg-kumo-elevated text-kumo-default shadow-xs ring-1 ring-kumo-hairline'
									: 'text-kumo-subtle hover:text-kumo-default'}"
							>
								<Browser size={13} class={buildType === 'static' ? 'text-[var(--accent)]' : ''} />
								<span>Static SPA</span>
							</button>
						</div>
					</div>

					{#if buildType === 'dockerfile'}
						<div class="flex items-center gap-2">
							<label for="bld-dockerfile-path" class="shrink-0 text-xs text-kumo-subtle">
								Path:
							</label>
							<Input
								id="bld-dockerfile-path"
								bind:value={dockerfilePath}
								placeholder="./Dockerfile"
								class="h-7 text-xs font-mono"
							/>
							<span class="hidden text-[11px] text-kumo-subtle lg:inline">
								Relative to build context root
							</span>
						</div>
					{:else if buildType === 'nixpacks'}
						<div class="text-xs text-kumo-subtle">
							Zero-config build: Nixpacks automatically detects your stack (Node, Go, Python, Rust,
							PHP) and generates an optimized container image.
						</div>
					{:else}
						<div class="flex items-center gap-2">
							<label for="bld-publish-dir" class="shrink-0 text-xs text-kumo-subtle">
								Publish Directory:
							</label>
							<Input
								id="bld-publish-dir"
								bind:value={publishDir}
								placeholder="dist or public"
								class="h-7 text-xs font-mono"
							/>
							<span class="hidden text-[11px] text-kumo-subtle lg:inline">
								Directory containing HTML/assets
							</span>
						</div>
					{/if}
				</div>
			</div>

		<!-- 2. Container Image Configuration -->
		{:else if sourceType === 'image'}
			<div class="flex flex-col gap-3">
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<!-- Registry Selection -->
					<div class="flex flex-col gap-1">
						<div class="flex items-center justify-between">
							<label for="src-reg-select" class="text-xs font-medium text-kumo-default">
								Registry
							</label>
							<button
								type="button"
								onclick={() => onOpenCredentialModal?.('registry')}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
							>
								<Plus size={11} /> Add
							</button>
						</div>

						<Select
							id="src-reg-select"
							value={selectedRegistry}
							onValueChange={(val: any) => {
								if (typeof val === 'string') selectedRegistry = val;
							}}
							options={[
								{ label: 'Docker Hub (docker.io)', value: 'docker.io' },
								{ label: 'GitHub (ghcr.io)', value: 'ghcr.io' },
								{ label: 'Red Hat Quay (quay.io)', value: 'quay.io' },
								...dataStore.registries.map((r) => ({
									label: `${r.name} (${r.url})`,
									value: r.id
								}))
							]}
							size="sm"
						/>
					</div>

					<!-- Image Name & Tag -->
					<div class="flex flex-col gap-1 sm:col-span-2">
						<label for="src-img-name" class="text-xs font-medium text-kumo-default">
							Image Name & Tag <span class="text-kumo-danger">*</span>
						</label>
						<Input
							id="src-img-name"
							bind:value={rawImageName}
							placeholder={selectedRegistry === 'docker.io'
								? 'e.g. nginx:alpine or traefik/whoami:latest'
								: 'e.g. my-app:latest'}
							class="text-xs font-mono"
						/>
					</div>
				</div>

				<!-- Live Resolved Pull Target Preview -->
				{#if resolvedImage}
					<div
						class="flex items-center justify-between rounded-md bg-kumo-elevated px-3 py-2 text-xs ring-1 ring-kumo-hairline"
					>
						<div class="flex min-w-0 items-center gap-2">
							<span class="shrink-0 text-kumo-subtle">Target:</span>
							<code class="truncate font-mono font-medium text-[var(--accent)] select-all">
								{resolvedImage}
							</code>
						</div>
						<span class="ml-2 shrink-0 font-mono text-[11px] text-kumo-success">✓ resolved</span>
					</div>
				{/if}
			</div>

		<!-- 3. Drop Static Site -->
		{:else}
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				ondragover={(e) => e.preventDefault()}
				ondrop={handleDrop}
				class="flex cursor-pointer flex-col items-center justify-center gap-1.5 rounded-lg border-2 border-dashed border-kumo-line bg-kumo-elevated/40 p-6 text-center transition-colors hover:border-[var(--accent)]"
			>
				<UploadSimple size={24} class="text-kumo-subtle" />
				{#if dropFileName}
					<span class="text-xs font-medium text-kumo-success">Uploaded: {dropFileName}</span>
				{:else}
					<span class="text-xs font-medium text-kumo-default">
						Drop static files (.zip or dist folder) here
					</span>
					<span class="text-[11px] text-kumo-subtle">
						Caddy will serve your SPA instantly with zero build step
					</span>
				{/if}
			</div>
		{/if}
	</LayerCard.Primary>
</LayerCard>
