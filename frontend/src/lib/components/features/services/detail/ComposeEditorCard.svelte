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

	$effect(() => {
		if (service.id !== lastLoadedServiceId) {
			lastLoadedServiceId = service.id;
			gitRepoUrl = service.source || '';
			gitBranch = service.branch || 'main';
			gitComposePath = 'docker-compose.yml';
			composeYaml = service.composeYaml || service.k8sYaml || getDefaultComposeYaml(service);
		}
	});

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
				: [{ name: service.name, image: service.image || 'nginx:alpine', ports: `${service.port || 8080}:80` }];
		} catch {
			return [{ name: service.name, image: service.image || 'nginx:alpine', ports: `${service.port || 8080}:80` }];
		}
	});

	async function handleSave() {
		saveStatus = 'saving';
		if (service.type === 'kubernetes') {
			service.k8sYaml = composeYaml;
		} else {
			service.composeYaml = composeYaml;
		}
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
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col shadow-xs">
		<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex flex-wrap items-center justify-between gap-3">
			<div class="flex flex-col gap-0.5">
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Provider</h3>
				<p class="text-xs text-[var(--text-tertiary)] m-0">Select the source of your code</p>
			</div>

			<!-- Right: Preview Compose Button -->
			<button
				type="button"
				onclick={() => (previewOpen = !previewOpen)}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md border border-[var(--border)] bg-[var(--bg-surface)] hover:bg-[var(--bg-hover)] text-xs font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)] cursor-pointer transition-colors"
			>
				<Eye size={14} /> {previewOpen ? 'Hide Preview' : 'Preview Compose'}
			</button>
		</div>

		<!-- Provider Sub-Tabs (Dokploy Screenshot: GitHub, GitLab, Bitbucket, Gitea, Git, Raw) -->
		<div class="px-5 py-3 border-b border-[var(--border-subtle)] flex items-center gap-2 bg-[var(--bg-surface)]/30 overflow-x-auto">
			<button
				type="button"
				onclick={() => (providerTab = 'github')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {providerTab ===
				'github'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<GithubLogo size={14} /> GitHub
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'gitlab')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {providerTab ===
				'gitlab'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<GitlabLogo size={14} /> GitLab
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'git')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {providerTab ===
				'git'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<GitBranch size={14} /> Git / SSH
			</button>

			<button
				type="button"
				onclick={() => (providerTab = 'raw')}
				class="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-colors cursor-pointer border-0 shrink-0 {providerTab ===
				'raw'
					? 'bg-[var(--accent)] text-white shadow-xs'
					: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'}"
			>
				<FileCode size={14} /> Raw
			</button>
		</div>

		<!-- Provider Content if Git based -->
		{#if providerTab !== 'raw'}
			<div class="p-5 flex flex-col gap-4">
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
					<div class="sm:col-span-2 flex flex-col gap-1.5">
						<label for="compose-repo" class="text-xs font-medium text-[var(--text-secondary)]">Repository URL</label>
						<Input
							id="compose-repo"
							bind:value={gitRepoUrl}
							placeholder="https://github.com/user/my-stack.git"
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
					<div class="flex flex-col gap-1.5">
						<label for="compose-branch" class="text-xs font-medium text-[var(--text-secondary)]">Branch</label>
						<Input
							id="compose-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="font-[var(--font-mono)] text-xs"
						/>
					</div>
				</div>
				<div class="flex flex-col gap-1.5">
					<label for="compose-path" class="text-xs font-medium text-[var(--text-secondary)]">Compose File Path in Repo</label>
					<Input
						id="compose-path"
						bind:value={gitComposePath}
						placeholder="docker-compose.yml"
						class="font-[var(--font-mono)] text-xs"
					/>
				</div>
			</div>
		{/if}
	</div>

	<!-- 2. Compose File Editor Card -->
	<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col shadow-xs">
		<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between gap-3">
			<div class="flex flex-col gap-0.5">
				<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Compose File</h3>
				<p class="text-xs text-[var(--text-tertiary)] m-0">
					Configure your Docker / Podman Compose file for this service.
				</p>
			</div>

			<span class="text-xs font-[var(--font-mono)] text-[var(--text-tertiary)]">
				YAML
			</span>
		</div>

		<div class="p-5 flex flex-col gap-4">
			<!-- Monospace Code Editor -->
			<div class="rounded-md border border-[var(--border)] overflow-hidden shadow-inner">
				<CodeEditor bind:value={composeYaml} language="yaml" height="auto" />
			</div>

			<!-- Compose Preview Drawer -->
			{#if previewOpen}
				<div class="p-4 rounded-md border border-[var(--border-subtle)] bg-[var(--bg-surface)]/90 flex flex-col gap-3">
					<span class="text-xs font-semibold text-[var(--text-primary)] flex items-center gap-1.5">
						<Stack size={14} class="text-[var(--accent)]" /> Detected Workloads in Stack ({parsedServices.length})
					</span>
					<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
						{#each parsedServices as svc}
							<div class="p-3 rounded-md bg-[var(--bg-panel)] border border-[var(--border-subtle)] flex flex-col gap-1">
								<span class="text-xs font-bold text-[var(--text-primary)] font-[var(--font-mono)]">{svc.name}</span>
								{#if svc.image}
									<span class="text-[11px] text-[var(--text-secondary)] font-[var(--font-mono)] truncate">{svc.image}</span>
								{/if}
								{#if svc.ports}
									<span class="text-[10px] text-[var(--accent)] font-[var(--font-mono)]">{svc.ports}</span>
								{/if}
							</div>
						{/each}
					</div>
				</div>
			{/if}
		</div>

		<!-- Single Explicit Save Button -->
		<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-between gap-3">
			<span class="text-xs text-[var(--text-tertiary)]">
				Saved configuration is stored in the GoPod workspace engine.
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-1">
						<Check size={14} weight="bold" /> Saved
					</span>
				{/if}
				<button
					type="button"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
					class="px-4 py-2 rounded-md bg-[var(--accent)] hover:opacity-90 text-white text-xs font-semibold cursor-pointer border-0 transition-opacity disabled:opacity-50"
				>
					{saveStatus === 'saving' ? 'Saving...' : 'Save Compose'}
				</button>
			</div>
		</div>
	</div>
</div>
