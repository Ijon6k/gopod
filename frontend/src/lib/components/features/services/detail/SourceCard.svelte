<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { GitBranch, Package, UploadSimple, Plus, Check } from 'phosphor-svelte';

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

	function handleSave() {
		service.sourceType = sourceType;
		if (sourceType === 'git') {
			service.source = repoUrl.trim();
			service.branch = branch.trim();
			service.buildPath = buildPath.trim();
			service.sshKeyId = sshKeyId;
		} else if (sourceType === 'image') {
			service.image = resolvedImage;
			const isCustom = dataStore.registries.some((r) => r.id === selectedRegistry);
			service.registryId = isCustom ? selectedRegistry : '';
		}
		dataStore.updateService(service);
		saveStatus = 'saved';
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer?.files && e.dataTransfer.files[0]) {
			dropFileName = e.dataTransfer.files[0].name;
		}
	}
</script>

<div
	class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)]"
>
	<!-- Card Header -->
	<div class="flex items-center justify-between border-b border-[var(--border-subtle)] px-5 py-4">
		<div class="flex flex-col gap-0.5">
			<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Source & Provider</h3>
			<p class="m-0 text-xs text-[var(--text-tertiary)]">
				Select where your application code or container image originates.
			</p>
		</div>
	</div>

	<!-- Source Type Selector -->
	<div class="flex flex-col gap-5 p-5">
		<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-3">
			<button
				type="button"
				onclick={() => (sourceType = 'git')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {sourceType ===
				'git'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<GitBranch size={18} class={sourceType === 'git' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Universal Git</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">GitHub, GitLab, SSH</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (sourceType = 'image')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {sourceType ===
				'image'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<Package size={18} class={sourceType === 'image' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Container Image</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Docker Hub, GHCR</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (sourceType = 'drop')}
				class="flex cursor-pointer items-center gap-3 rounded-[var(--radius-sm)] border p-3 text-left transition-all {sourceType ===
				'drop'
					? 'border-[var(--accent)] bg-[var(--bg-surface)] text-[var(--text-primary)]'
					: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<UploadSimple size={18} class={sourceType === 'drop' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Drop (Static Site)</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Upload zip / dist</span>
				</div>
			</button>
		</div>

		<!-- Form Fields based on Source Type -->
		{#if sourceType === 'git'}
			<div class="flex flex-col gap-4">
				<div class="flex flex-col gap-1.5">
					<label for="src-git-url" class="text-xs font-medium text-[var(--text-secondary)]">
						Repository URL <span class="text-[var(--status-red)]">*</span>
					</label>
					<Input
						id="src-git-url"
						bind:value={repoUrl}
						placeholder="git@github.com:username/repository.git or https://github.com/..."
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>

				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div class="flex flex-col gap-1.5">
						<label for="src-git-branch" class="text-xs font-medium text-[var(--text-secondary)]"
							>Branch</label
						>
						<Input
							id="src-git-branch"
							bind:value={branch}
							placeholder="main"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="src-git-path" class="text-xs font-medium text-[var(--text-secondary)]"
							>Build Path</label
						>
						<Input
							id="src-git-path"
							bind:value={buildPath}
							placeholder="/"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				</div>

				<!-- SSH Deploy Key selection -->
				<div class="flex flex-col gap-1.5">
					<div class="flex items-center justify-between">
						<label for="src-ssh-key" class="text-xs font-medium text-[var(--text-secondary)]">
							SSH Deploy Key (for private repos)
						</label>
						<button
							type="button"
							onclick={() => onOpenCredentialModal?.('ssh')}
							class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
						>
							<Plus size={12} /> Add New Key
						</button>
					</div>

					<div
						class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
					>
						<select
							id="src-ssh-key"
							bind:value={sshKeyId}
							class="w-full cursor-pointer border-0 bg-transparent text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none"
						>
							<option value="">None (Public Repository)</option>
							{#each dataStore.sshKeys as k (k.id)}
								<option value={k.id}>{k.name} ({k.type})</option>
							{/each}
						</select>
					</div>
				</div>
			</div>
		{:else if sourceType === 'image'}
			<div class="flex flex-col gap-4">
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<!-- 1. Registry Dropdown (Docker Hub default) -->
					<div class="flex flex-col gap-1.5">
						<div class="flex items-center justify-between">
							<label for="src-reg-select" class="text-xs font-medium text-[var(--text-secondary)]">
								Registry
							</label>
							<button
								type="button"
								onclick={() => onOpenCredentialModal?.('registry')}
								class="flex cursor-pointer items-center gap-1 border-0 bg-transparent p-0 text-[11px] text-[var(--accent)] hover:underline"
								title="Add private container registry credentials"
							>
								<Plus size={11} /> Add Custom
							</button>
						</div>

						<div
							class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-2"
						>
							<select
								id="src-reg-select"
								bind:value={selectedRegistry}
								class="w-full cursor-pointer border-0 bg-transparent text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none"
							>
								<optgroup label="Public Registries">
									<option value="docker.io">Docker Hub (docker.io) — Default</option>
									<option value="ghcr.io">GitHub (ghcr.io)</option>
									<option value="quay.io">Red Hat Quay (quay.io)</option>
								</optgroup>
								{#if dataStore.registries.length > 0}
									<optgroup label="Saved Private Registries">
										{#each dataStore.registries as r (r.id)}
											<option value={r.id}>{r.name} ({r.url})</option>
										{/each}
									</optgroup>
								{/if}
							</select>
						</div>
					</div>

					<!-- 2. Image Name & Tag Input -->
					<div class="flex flex-col gap-1.5 sm:col-span-2">
						<label for="src-img-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Image Name & Tag <span class="text-[var(--status-red)]">*</span>
						</label>
						<Input
							id="src-img-name"
							bind:value={rawImageName}
							placeholder={selectedRegistry === 'docker.io'
								? 'e.g. nginx:alpine or wardy784/erugo:latest'
								: selectedRegistry === 'ghcr.io'
									? 'e.g. owner/repo:latest'
									: 'e.g. my-app:latest'}
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				</div>

				<!-- Live Resolved Pull Target Preview (Nielsen #1: Visibility of System Status) -->
				{#if resolvedImage}
					<div
						class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-2.5 text-[11px]"
					>
						<div class="flex min-w-0 items-center gap-2">
							<span class="shrink-0 font-medium text-[var(--text-tertiary)]"
								>Resolved Podman target:</span
							>
							<code class="truncate font-mono font-semibold text-[var(--accent)] select-all"
								>{resolvedImage}</code
							>
						</div>
						<span class="ml-2 shrink-0 font-mono text-[10px] text-[var(--status-green)]"
							>✓ ready to pull</span
						>
					</div>
				{/if}
			</div>
		{:else}
			<!-- Drop static site -->
			<!-- svelte-ignore a11y_no_static_element_interactions -->
			<div
				ondragover={(e) => e.preventDefault()}
				ondrop={handleDrop}
				class="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-[var(--radius-card)] border-2 border-dashed border-[var(--border)] bg-[var(--bg-surface)]/40 p-8 text-center transition-colors hover:border-[var(--accent)]"
			>
				<UploadSimple size={28} class="text-[var(--text-tertiary)]" />
				{#if dropFileName}
					<span class="text-xs font-medium text-[var(--status-green)]"
						>Uploaded: {dropFileName}</span
					>
				{:else}
					<span class="text-xs font-medium text-[var(--text-primary)]"
						>Drop static files (.zip or dist folder) here</span
					>
					<span class="text-[11px] text-[var(--text-tertiary)]"
						>Caddy will automatically serve your SPA with zero build overhead</span
					>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Explicit Card Footer with 1 Save Button -->
	<div
		class="flex items-center justify-end gap-2 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
	>
		{#if saveStatus === 'saved'}
			<span class="mr-2 flex items-center gap-1 text-xs text-[var(--status-green)]">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>Save Source</Button>
	</div>
</div>
