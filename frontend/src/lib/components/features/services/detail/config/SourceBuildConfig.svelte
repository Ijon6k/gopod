<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, Select } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { GitBranch, Package, UploadSimple, Check, SpinnerGap } from 'phosphor-svelte';

	interface Props {
		service: Service;
		sourceType: 'git' | 'image' | 'drop';
		repoUrl: string;
		branch: string;
		buildType: 'dockerfile' | 'nixpacks' | 'static';
		dockerfilePath: string;
		sshKeyId: string;
		gitCredentialId: string;
		buildArgsText: string;
		rawImageName: string;
		selectedRegistry: string;
		dropFileName: string | null;
		containerPort: string;
		onOpenCredentialModal?: (type: 'ssh' | 'registry' | 'git') => void;
	}

	let {
		service,
		sourceType = $bindable(),
		repoUrl = $bindable(),
		branch = $bindable(),
		buildType = $bindable(),
		dockerfilePath = $bindable(),
		sshKeyId = $bindable(),
		gitCredentialId = $bindable(),
		buildArgsText = $bindable(),
		rawImageName = $bindable(),
		selectedRegistry = $bindable(),
		dropFileName = $bindable(),
		containerPort = $bindable(),
		onOpenCredentialModal
	}: Props = $props();

	const toast = useKumoToastManager();

	let fileInputRef = $state<HTMLInputElement | null>(null);
	let isUploading = $state(false);
	let extractedCount = $state<number | null>(null);

	async function uploadZipFile(file: File) {
		if (!file.name.toLowerCase().endsWith('.zip')) {
			toast.add(
				errorToastOptions(
					'Invalid file format',
					'Please upload a .zip archive containing your static files or dist folder.'
				)
			);
			return;
		}

		isUploading = true;
		dropFileName = file.name;

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
			// A stale manifest would take precedence over the drop source in the
			// deploy dispatcher. Never erase a Quadlet-managed unit from here.
			service.composeYaml = '';
			service.k8sYaml = '';
			if (service.type !== 'quadlet' && service.runtimeTarget !== 'quadlet') {
				service.quadletConfig = '';
			}
			await dataStore.updateService(service);
		} catch (err: any) {
			toast.add(
				errorToastOptions('Upload Failed', err?.message || 'Failed to process static archive')
			);
		} finally {
			isUploading = false;
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
			class="bg-kumo-surface inline-flex rounded-md border border-kumo-line p-0.5 text-xs"
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
		<div class="bg-kumo-surface flex flex-col gap-3 rounded-lg border border-kumo-line p-4">
			<div class="flex flex-wrap items-end gap-3">
				<div class="flex max-w-lg min-w-[280px] flex-1 flex-col gap-1.5">
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

			<div class="flex flex-wrap items-end gap-4 pt-1">
				<div class="flex flex-col gap-1.5">
					<span class="text-[11px] font-medium text-kumo-subtle">Build method</span>
					<div class="flex items-center gap-1.5">
						<span
							class="rounded-sm border border-kumo-line bg-kumo-base px-2.5 py-1 text-xs font-medium text-kumo-default shadow-xs"
						>
							Dockerfile
						</span>
						<span class="text-[10px] text-kumo-subtle">
							Buildpack autodetect is not available yet.
						</span>
					</div>
				</div>

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

				<div class="flex flex-col gap-1.5">
					<div class="flex items-center justify-between gap-2">
						<span class="text-[11px] font-medium text-kumo-subtle">HTTPS access token</span>
						{#if onOpenCredentialModal}
							<button
								type="button"
								onclick={() => onOpenCredentialModal('git')}
								class="cursor-pointer text-[10px] text-[var(--accent)] hover:underline"
							>
								+ Add token
							</button>
						{/if}
					</div>
					<Select
						bind:value={gitCredentialId}
						options={[
							{ value: '', label: 'None (SSH or public)' },
							...dataStore.gitTokens.map((t) => ({
								value: t.id,
								label: `${t.name} (${t.provider})`
							}))
						]}
						class="h-8 w-48 text-xs"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="build-args" class="text-[11px] font-medium text-kumo-subtle">
					Build arguments (optional)
				</label>
				<textarea
					id="build-args"
					bind:value={buildArgsText}
					rows="2"
					placeholder={'NODE_ENV=production\nAPI_URL=https://api.example.test'}
					class="w-full resize-y rounded-sm border border-kumo-line bg-kumo-base px-2.5 py-1.5 font-mono text-xs text-kumo-default placeholder:text-kumo-subtle focus:border-[var(--accent)] focus:outline-none"
				></textarea>
				<span class="text-[10px] text-kumo-subtle">
					One KEY=VALUE per line. Build args are stored in plain text; keep secrets out of them.
				</span>
			</div>
		</div>
	{:else if sourceType === 'image'}
		<div class="bg-kumo-surface flex flex-col gap-3 rounded-lg border border-kumo-line p-4">
			<div class="flex flex-wrap items-end gap-3">
				<div class="flex max-w-md min-w-[280px] flex-1 flex-col gap-1.5">
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
		<div
			ondragover={(e) => e.preventDefault()}
			ondrop={handleDrop}
			onclick={() => !isUploading && fileInputRef?.click()}
			role="button"
			tabindex="0"
			onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && fileInputRef?.click()}
			aria-label="Upload static build artifact"
			class="bg-kumo-surface flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-kumo-line p-6 text-center transition-all hover:border-[var(--accent)] hover:bg-kumo-elevated/40"
		>
			{#if isUploading}
				<SpinnerGap size={24} class="animate-spin text-[var(--accent)]" />
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-medium text-kumo-default">Extracting {dropFileName}...</span>
					<span class="text-[11px] text-kumo-subtle"
						>Unpacking archive and verifying security...</span
					>
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
						{extractedCount !== null
							? `${extractedCount} files extracted`
							: 'Static files served via Nginx'} (Click or drop another .zip to replace)
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
