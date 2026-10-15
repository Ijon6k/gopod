<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import { Stack, Eye, Check } from 'phosphor-svelte';
	import { getDefaultComposeYaml } from '../manifests/defaults';
	import ComposeSourceSelector from './compose/ComposeSourceSelector.svelte';
	import { parseComposeManifest } from '$lib/parsers/manifest-engine';
	import ArchitecturePreviewBar from './manifests/ArchitecturePreviewBar.svelte';

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
			const isGit =
				service.sourceType === 'git' ||
				Boolean(
					service.source && /^(https?:\/\/|git@|ssh:\/\/|.*\.git$)/.test(service.source.trim())
				);
			gitRepoUrl = isGit ? service.source || '' : '';
			gitBranch = service.branch || 'main';
			gitComposePath = 'docker-compose.yml';
			composeYaml = service.composeYaml || service.k8sYaml || getDefaultComposeYaml(service);
			inPod = service.inPod || service.runtimeTarget === 'pod';
			providerMode = isGit ? 'git' : 'raw';
		}
	});

	function toggleInPod(checked?: boolean) {
		inPod = checked !== undefined ? checked : !inPod;
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';
		dataStore.updateService(service);
		toast.add({
			title: inPod ? 'Podman pod mode enabled' : 'Standalone bridge mode enabled',
			description: inPod
				? 'Services will share localhost network namespace in an isolated Pod.'
				: 'Services will run as independent containers with bridge DNS and custom network isolation.',
			variant: 'info'
		});
	}

	let composeParseResult = $derived.by(() => parseComposeManifest(composeYaml));

	async function handleSave() {
		if (!composeParseResult.valid) {
			toast.add({
				title: 'Invalid Compose Syntax',
				description: composeParseResult.errors[0] || 'Please resolve syntax errors before saving.',
				variant: 'error'
			});
			return;
		}

		saveStatus = 'saving';
		if (service.type === 'kubernetes') {
			service.k8sYaml = composeYaml;
		} else {
			service.composeYaml = composeYaml;
		}
		service.inPod = inPod;
		service.runtimeTarget = inPod ? 'pod' : 'standalone';

		if (providerMode === 'git') {
			service.sourceType = 'git';
			service.source = gitRepoUrl.trim();
			service.branch = gitBranch.trim() || 'main';
		} else {
			service.sourceType = 'compose';
		}

		try {
			await dataStore.updateService(service);
			saveStatus = 'saved';
			toast.add({
				title: 'Compose specification saved',
				description: `${service.name} stack manifest updated.`,
				variant: 'success'
			});
			setTimeout(() => {
				saveStatus = 'idle';
			}, 2000);
		} catch (err: any) {
			saveStatus = 'idle';
			toast.add({
				title: 'Failed to save specification',
				description: err?.message || 'Error occurred while saving manifest',
				variant: 'error'
			});
		}
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Unified Header -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3"
	>
		<!-- Left Context Title -->
		<div class="flex items-center gap-2.5">
			<Stack size={16} class="text-[var(--accent)]" />
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<h3 class="m-0 text-sm font-semibold text-kumo-default">Docker Compose & Podman Stack</h3>
					<span
						class="rounded bg-[var(--accent-muted)] px-1.5 py-0.5 font-mono text-[10.5px] text-[var(--accent-text)]"
					>
						gopod-net
					</span>
				</div>
				<span class="text-xs text-kumo-subtle">
					Declarative multi-container workloads with Podman Netavark networking.
				</span>
			</div>
		</div>

		<!-- Right Controls: Source Mode Selector + Preview Button -->
		<div class="flex items-center gap-2.5">
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
					Git repository
				</button>
			</div>

			<Button
				variant="secondary"
				size="sm"
				onclick={() => (previewOpen = !previewOpen)}
				class="gap-1.5"
			>
				<Eye size={14} />
				<span>{previewOpen ? 'Hide inspection' : 'Inspect stack'}</span>
				<span
					class="py-0.2 ml-1 inline-flex items-center rounded bg-kumo-recessed px-1.5 font-mono text-[11px] text-kumo-subtle"
				>
					{composeParseResult.workloads.length}
				</span>
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Main Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-4">
		{#if providerMode === 'git'}
			<ComposeSourceSelector bind:gitProvider bind:gitRepoUrl bind:gitBranch bind:gitComposePath />
		{/if}

		<!-- Podman Pod Encapsulation Strategy Banner -->
		<div
			class="flex flex-col justify-between gap-3 rounded-md bg-kumo-base p-3.5 ring ring-kumo-line sm:flex-row sm:items-center"
		>
			<div class="flex items-start gap-3">
				<Switch
					size="sm"
					id="compose-in-pod"
					bind:checked={inPod}
					onCheckedChange={toggleInPod}
					aria-label="Encapsulate stack inside Podman pod"
					class="mt-0.5"
				/>
				<label for="compose-in-pod" class="flex cursor-pointer flex-col select-none">
					<span class="text-sm font-semibold text-kumo-default">
						Encapsulate stack inside Podman pod
					</span>
					<span class="text-[13px] leading-normal text-kumo-subtle">
						Creates an isolated Pod with shared localhost. Uncheck to run services as independent
						containers with bridge DNS and custom network isolation.
					</span>
				</label>
			</div>

			<span
				class="shrink-0 rounded border px-2.5 py-1 font-mono text-xs {inPod
					? 'border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent-text)]'
					: 'border-kumo-line bg-kumo-recessed text-kumo-subtle'}"
			>
				{inPod ? '--in-pod=true' : '--in-pod=false'}
			</span>
		</div>

		<!-- Compact Architecture Preview Bar directly above Editor -->
		<ArchitecturePreviewBar parseResult={composeParseResult} bind:expanded={previewOpen} />

		<!-- Monospace Code Editor -->
		<div class="overflow-hidden rounded-md ring ring-kumo-line">
			<CodeEditor bind:value={composeYaml} language="yaml" height="auto" />
		</div>

		<!-- Footer with Save CTA -->
		<div class="flex items-center justify-between border-t border-kumo-hairline pt-3">
			<span class="text-xs text-kumo-subtle">
				Configuration is synchronized with the GoPod runtime engine.
			</span>

			<div class="flex items-center gap-2">
				{#if saveStatus === 'saved'}
					<span class="flex items-center gap-1 text-xs text-kumo-success">
						<Check size={14} weight="bold" /> Saved
					</span>
				{/if}
				<Button variant="primary" size="sm" onclick={handleSave} disabled={saveStatus === 'saving'}>
					{saveStatus === 'saving' ? 'Saving…' : 'Save compose specification'}
				</Button>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
