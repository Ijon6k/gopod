<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Input } from '$lib/components/primitives';
	import {
		Stack,
		Eye,
		Check,
		GitBranch,
		GithubLogo,
		GitlabLogo,
		FileCode,
		ArrowSquareOut
	} from 'phosphor-svelte';

	import { getDefaultComposeYaml } from '../manifests/defaults';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let providerTab = $state<'raw' | 'github' | 'gitlab' | 'git'>('raw');
	let gitRepoUrl = $state('');
	let gitBranch = $state('main');
	let gitComposePath = $state('docker-compose.yml');
	let composeYaml = $state('');

	let previewOpen = $state(false);
	let saveStatus = $state<'idle' | 'saving' | 'saved'>('idle');

	let lastLoadedServiceId = $state<string | null>(null);

	let inPod = $state(false);

	$effect(() => {
		if (service.id !== lastLoadedServiceId) {
			lastLoadedServiceId = service.id;
			gitRepoUrl = service.source || '';
			gitBranch = service.branch || 'main';
			gitComposePath = 'docker-compose.yml';
			composeYaml = service.composeYaml || service.k8sYaml || getDefaultComposeYaml(service);
			inPod = service.inPod || service.runtimeTarget === 'pod';
		}
	});

	function toggleInPod() {
		inPod = !inPod;
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		dataStore.updateService(service);
	}

	// Simple parser for previewing services defined in Compose
	let parsedServices = $derived.by(() => {
		try {
			const lines = composeYaml.split('\n');
			const servicesFound: { name: string; image?: string; ports?: string }[] = [];
			let inServices = false;
			let currentSvc: { name: string; image?: string; ports?: string } | null = null;

			for (const line of lines) {
				const trimmed = line.trim();
				if (trimmed.startsWith('services:')) {
					inServices = true;
					continue;
				}
				if (inServices) {
					// 2 spaces indentation = service name
					const matchSvc = line.match(/^ {2}([a-zA-Z0-9_-]+):/);
					if (matchSvc) {
						if (currentSvc) servicesFound.push(currentSvc);
						currentSvc = { name: matchSvc[1] };
					} else if (currentSvc) {
						const imgMatch = line.match(/image:\s*([^\s]+)/);
						if (imgMatch) currentSvc.image = imgMatch[1];
						const portMatch = line.match(/- ["']?([0-9:]+)["']?/);
						if (portMatch && !currentSvc.ports) currentSvc.ports = portMatch[1];
					}
				}
			}
			if (currentSvc) servicesFound.push(currentSvc);
			return servicesFound.length > 0
				? servicesFound
				: [
						{
							name: service.name,
							image: service.image || 'nginx:alpine',
							ports: `${service.port || 8080}:80`
						}
					];
		} catch {
			return [
				{
					name: service.name,
					image: service.image || 'nginx:alpine',
					ports: `${service.port || 8080}:80`
				}
			];
		}
	});

	async function handleSave() {
		saveStatus = 'saving';
		if (service.type === 'kubernetes') {
			service.k8sYaml = composeYaml;
		} else {
			service.composeYaml = composeYaml;
		}
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		if (providerTab !== 'raw') {
			service.source = gitRepoUrl;
			service.branch = gitBranch;
		}

		try {
			await dataStore.updateService(service);
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err) {
			console.error('Failed to save compose config:', err);
			saveStatus = 'idle';
		}
	}
</script>

<div class="flex flex-col gap-6">
	<!-- 1. Provider Card (Dokploy exact replica) -->
	<div
		class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs"
	>
		<div
			class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] px-5 py-4"
		>
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Provider</h3>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">Select the source of your code</p>
			</div>

			<!-- Right: Preview Compose Button -->
			<button
				type="button"
				onclick={() => (previewOpen = !previewOpen)}
				class="flex cursor-pointer items-center gap-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-1.5 text-xs font-medium text-[var(--text-secondary)] transition-colors hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
			>
				<Eye size={14} />
				{previewOpen ? 'Hide Preview' : 'Preview Compose'}
			</button>
		</div>

		<!-- Provider Sub-Tabs (Dokploy Screenshot: GitHub, GitLab, Bitbucket, Gitea, Git, Raw) -->
		<div
			class="flex items-center gap-2 overflow-x-auto border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]/30 px-5 py-3"
		>
			<button
				type="button"
				onclick={() => (providerTab = 'github')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'github'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<GithubLogo size={14} /> GitHub
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'gitlab')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'gitlab'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<GitlabLogo size={14} /> GitLab
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'git')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'git'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<GitBranch size={14} /> Git / SSH
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'raw')}
				class="flex shrink-0 cursor-pointer items-center gap-1.5 rounded-md border-0 px-3 py-1.5 text-xs font-medium transition-colors {providerTab ===
				'raw'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'}"
			>
				<FileCode size={14} /> Raw
			</button>
		</div>

		<!-- Provider Content if Git based -->
		{#if providerTab !== 'raw'}
			<div class="flex flex-col gap-4 p-5">
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1.5 sm:col-span-2">
						<label for="compose-repo" class="text-xs font-medium text-[var(--text-secondary)]"
							>Repository URL</label
						>
						<Input
							id="compose-repo"
							bind:value={gitRepoUrl}
							placeholder="https://github.com/user/my-stack.git"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
					<div class="flex flex-col gap-1.5">
						<label for="compose-branch" class="text-xs font-medium text-[var(--text-secondary)]"
							>Branch</label
						>
						<Input
							id="compose-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
				</div>
				<div class="flex flex-col gap-1.5">
					<label for="compose-path" class="text-xs font-medium text-[var(--text-secondary)]"
						>Compose File Path in Repo</label
					>
					<Input
						id="compose-path"
						bind:value={gitComposePath}
						placeholder="docker-compose.yml"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>
		{/if}
	</div>

	<!-- 2. Compose File Editor Card -->
	<div
		class="flex flex-col overflow-hidden rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-xs"
	>
		<div
			class="flex items-center justify-between gap-3 border-b border-[var(--border-subtle)] px-5 py-4"
		>
			<div class="flex flex-col gap-0.5">
				<h3 class="m-0 text-sm font-semibold text-[var(--text-primary)]">Compose File</h3>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Configure your Docker / Podman Compose file for this service.
				</p>
			</div>

			<span class="text-xs font-[var(--font-mono)] text-[var(--text-tertiary)]"> YAML </span>
		</div>

		<div class="flex flex-col gap-4 p-5">
			<!-- Podman Pod Encapsulation Checkbox (Refactoring UI / UX Heuristics) -->
			<div
				class="flex flex-col justify-between gap-3 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] p-3.5 sm:flex-row sm:items-center"
			>
				<div class="flex items-start gap-3">
					<input
						type="checkbox"
						id="compose-in-pod"
						checked={inPod}
						onchange={toggleInPod}
						class="mt-0.5 h-4 w-4 cursor-pointer rounded border-[var(--border)] accent-[var(--accent)]"
					/>
					<label for="compose-in-pod" class="flex cursor-pointer flex-col select-none">
						<span class="text-xs font-semibold text-[var(--text-primary)]">
							Encapsulate stack inside Podman Pod
						</span>
						<span class="text-[11px] leading-relaxed text-[var(--text-tertiary)]">
							When enabled, podman-compose creates an isolated Pod with shared localhost. Uncheck to
							run services as independent containers with bridge DNS.
						</span>
					</label>
				</div>
				<span
					class="shrink-0 rounded border border-[var(--border)] bg-[var(--bg-panel)] px-2.5 py-1 text-[11px] font-[var(--font-mono)] {inPod
						? 'border-[var(--accent)]/40 text-[var(--accent)]'
						: 'text-[var(--text-secondary)]'}"
				>
					{inPod ? 'Flag: --in-pod true' : 'Flag: --in-pod false'}
				</span>
			</div>

			<!-- Monospace Code Editor -->
			<div class="overflow-hidden rounded-md border border-[var(--border)] shadow-inner">
				<CodeEditor bind:value={composeYaml} language="yaml" height="auto" />
			</div>

			<!-- Compose Preview Drawer -->
			{#if previewOpen}
				<div
					class="flex flex-col gap-3 rounded-md border border-[var(--border-subtle)] bg-[var(--bg-surface)]/90 p-4"
				>
					<span class="flex items-center gap-1.5 text-xs font-semibold text-[var(--text-primary)]">
						<Stack size={14} class="text-[var(--accent)]" /> Detected Workloads in Stack ({parsedServices.length})
					</span>
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3">
						{#each parsedServices as svc}
							<div
								class="flex flex-col gap-1 rounded-md border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-3"
							>
								<span class="text-xs font-[var(--font-mono)] font-bold text-[var(--text-primary)]"
									>{svc.name}</span
								>
								{#if svc.image}
									<span
										class="truncate text-[11px] font-[var(--font-mono)] text-[var(--text-secondary)]"
										>{svc.image}</span
									>
								{/if}
								{#if svc.ports}
									<span class="text-[10px] font-[var(--font-mono)] text-[var(--accent)]"
										>{svc.ports}</span
									>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<!-- Single Explicit Save Button -->
		<div
			class="flex items-center justify-between gap-3 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
		>
			<span class="text-xs text-[var(--text-tertiary)]">
				Saved configuration is stored in the GoPod workspace engine.
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="mr-1 flex items-center gap-1 text-xs text-[var(--status-green)]">
						<Check size={14} weight="bold" /> Saved
					</span>
				{/if}
				<button
					type="button"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
					class="cursor-pointer rounded-md border-0 bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-white transition-opacity hover:opacity-90 disabled:opacity-50"
				>
					{saveStatus === 'saving' ? 'Saving...' : 'Save Compose'}
				</button>
			</div>
		</div>
	</div>
</div>
