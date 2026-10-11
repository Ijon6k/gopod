<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { Input } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import {
		Stack,
		Eye,
		Check,
		FileCode,
		GitBranch,
		GithubLogo,
		GitlabLogo
	} from 'phosphor-svelte';

	import { getDefaultComposeYaml } from '../manifests/defaults';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let providerMode = $state<'raw' | 'git'>('raw');
	let gitProvider = $state<'github' | 'gitlab' | 'custom'>('github');
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
			providerMode = service.source && service.source.trim() !== '' ? 'git' : 'raw';
		}
	});

	function toggleInPod(checked?: boolean) {
		inPod = checked !== undefined ? checked : !inPod;
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		dataStore.updateService(service);
		toast.add({
			title: inPod ? 'Podman Pod Mode Enabled' : 'Standalone Mode Enabled',
			description: inPod
				? 'Services will share localhost network namespace in an isolated Pod.'
				: 'Services will run as independent containers with bridge DNS.',
			variant: 'info'
		});
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
		if (providerMode === 'git') {
			service.source = gitRepoUrl.trim();
			service.branch = gitBranch.trim();
		} else {
			service.source = '';
		}

		try {
			await dataStore.updateService(service);
			saveStatus = 'saved';
			toast.add({
				title: 'Compose Manifest Saved',
				description: `Configuration and Pod strategy for ${service.name} have been updated.`,
				variant: 'success'
			});
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err: any) {
			console.error('Failed to save compose config:', err);
			saveStatus = 'idle';
			toast.add({
				title: 'Save Failed',
				description: err?.message || 'Failed to save compose configuration',
				variant: 'error'
			});
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) -->
	<LayerCard.Secondary class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3">
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring ring-kumo-line"
			>
				<FileCode size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<h3 class="m-0 text-xs font-semibold text-kumo-default">Compose Specification</h3>
				<span class="text-[11px] text-kumo-subtle">
					Multi-container orchestration definition for Podman & Docker Compose
				</span>
			</div>
		</div>

		<!-- Right Controls: Source Mode Selector + Preview Button -->
		<div class="flex items-center gap-2.5">
			<!-- Compact Source Segmented Pill -->
			<div
				class="inline-flex items-center rounded-md bg-kumo-base p-0.5 ring ring-kumo-hairline"
				role="tablist"
			>
				<button
					type="button"
					onclick={() => (providerMode = 'raw')}
					class="cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors {providerMode ===
					'raw'
						? 'bg-kumo-elevated text-kumo-default shadow-xs ring ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					Inline YAML
				</button>
				<button
					type="button"
					onclick={() => (providerMode = 'git')}
					class="cursor-pointer rounded px-2.5 py-1 text-xs font-medium transition-colors {providerMode ===
					'git'
						? 'bg-kumo-elevated text-kumo-default shadow-xs ring ring-kumo-line'
						: 'text-kumo-subtle hover:text-kumo-default'}"
				>
					Git Repo
				</button>
			</div>

			<!-- Preview Button -->
			<Button
				variant="secondary"
				size="sm"
				onclick={() => (previewOpen = !previewOpen)}
				class="gap-1.5"
			>
				<Eye size={13} />
				<span>{previewOpen ? 'Hide' : 'Preview Stack'}</span>
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Main Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-4">
		<!-- Git Source Inputs (Shown only if Git mode selected) -->
		{#if providerMode === 'git'}
			<div
				class="flex flex-col gap-3 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line"
			>
				<!-- Quick Provider selector -->
				<div class="flex items-center gap-2">
					<button
						type="button"
						onclick={() => (gitProvider = 'github')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2 py-1 text-xs transition-colors {gitProvider ===
						'github'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GithubLogo size={13} /> GitHub
					</button>
					<button
						type="button"
						onclick={() => (gitProvider = 'gitlab')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2 py-1 text-xs transition-colors {gitProvider ===
						'gitlab'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GitlabLogo size={13} /> GitLab
					</button>
					<button
						type="button"
						onclick={() => (gitProvider = 'custom')}
						class="flex cursor-pointer items-center gap-1.5 rounded px-2 py-1 text-xs transition-colors {gitProvider ===
						'custom'
							? 'bg-kumo-elevated text-kumo-default ring ring-kumo-line font-medium'
							: 'text-kumo-subtle hover:text-kumo-default'}"
					>
						<GitBranch size={13} /> Custom Git / SSH
					</button>
				</div>

				<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1 sm:col-span-2">
						<label for="compose-repo" class="text-xs font-medium text-kumo-subtle">
							Repository URL
						</label>
						<Input
							id="compose-repo"
							bind:value={gitRepoUrl}
							placeholder="https://github.com/user/stack-repo.git"
							class="font-mono text-xs"
						/>
					</div>

					<div class="flex flex-col gap-1">
						<label for="compose-branch" class="text-xs font-medium text-kumo-subtle">
							Branch
						</label>
						<Input
							id="compose-branch"
							bind:value={gitBranch}
							placeholder="main"
							class="font-mono text-xs"
						/>
					</div>
				</div>

				<div class="flex flex-col gap-1">
					<label for="compose-path" class="text-xs font-medium text-kumo-subtle">
						Compose File Path in Repo
					</label>
					<Input
						id="compose-path"
						bind:value={gitComposePath}
						placeholder="docker-compose.yml"
						class="font-mono text-xs"
					/>
				</div>
			</div>
		{/if}

		<!-- Podman Pod Encapsulation Banner -->
		<div
			class="flex flex-col justify-between gap-2.5 rounded-md bg-kumo-base p-3 ring ring-kumo-line sm:flex-row sm:items-center"
		>
			<div class="flex items-start gap-2.5">
				<Switch
					size="sm"
					id="compose-in-pod"
					bind:checked={inPod}
					onCheckedChange={toggleInPod}
					aria-label="Encapsulate stack inside Podman Pod"
					class="mt-0.5"
				/>
				<label for="compose-in-pod" class="flex cursor-pointer flex-col select-none">
					<span class="text-xs font-semibold text-kumo-default">
						Encapsulate stack inside Podman Pod
					</span>
					<span class="text-[11px] text-kumo-subtle leading-normal">
						Creates an isolated Pod with shared localhost. Uncheck to run services as independent containers with bridge network DNS.
					</span>
				</label>
			</div>

			<span
				class="shrink-0 rounded border px-2 py-0.5 font-mono text-[10px] {inPod
					? 'border-[var(--accent)]/40 bg-[var(--accent)]/10 text-[var(--accent-text)]'
					: 'border-kumo-line bg-kumo-recessed text-kumo-subtle'}"
			>
				{inPod ? '--in-pod=true' : '--in-pod=false'}
			</span>
		</div>

		<!-- Monospace Code Editor -->
		<div class="overflow-hidden rounded-md ring ring-kumo-line">
			<CodeEditor bind:value={composeYaml} language="yaml" height="auto" />
		</div>

		<!-- Workload Preview Drawer -->
		{#if previewOpen}
			<div
				class="flex flex-col gap-2.5 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line"
			>
				<span class="flex items-center gap-1.5 text-xs font-semibold text-kumo-default">
					<Stack size={14} class="text-[var(--accent)]" /> Detected Workloads in Stack ({parsedServices.length})
				</span>
				<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 md:grid-cols-3">
					{#each parsedServices as svc}
						<div
							class="flex flex-col gap-1 rounded bg-kumo-elevated p-2.5 ring ring-kumo-hairline"
						>
							<span class="font-mono text-xs font-bold text-kumo-default truncate">
								{svc.name}
							</span>
							{#if svc.image}
								<span class="truncate font-mono text-[11px] text-kumo-subtle">
									{svc.image}
								</span>
							{/if}
							{#if svc.ports}
								<span class="font-mono text-[10px] text-[var(--accent-text)]">
									{svc.ports}
								</span>
							{/if}
						</div>
					{/each}
				</div>
			</div>
		{/if}

		<!-- Footer with Save CTA -->
		<div
			class="flex items-center justify-between border-t border-kumo-hairline pt-3"
		>
			<span class="text-xs text-kumo-subtle">
				Configuration is synchronized with the GoPod runtime engine.
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="flex items-center gap-1 text-xs text-kumo-success">
						<Check size={14} weight="bold" /> Saved
					</span>
				{/if}
				<Button
					variant="primary"
					size="sm"
					onclick={handleSave}
					disabled={saveStatus === 'saving'}
				>
					{saveStatus === 'saving' ? 'Saving…' : 'Save Compose'}
				</Button>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
