<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor } from '$lib/components/ui';
	import { LayerCard, Button, Switch, useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Stack, Eye, Check, Info } from 'phosphor-svelte';
	import { getDefaultComposeYaml } from '../manifests/defaults';
	import { parseComposeManifest } from '$lib/parsers/manifest-engine';
	import ArchitecturePreviewBar from './manifests/ArchitecturePreviewBar.svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let composeYaml = $state('');

	let previewOpen = $state(false);
	let saveStatus = $state<'idle' | 'saving' | 'saved'>('idle');
	let lastLoadedServiceId = $state<string | null>(null);
	let inPod = $state(false);

	$effect(() => {
		if (service.id !== lastLoadedServiceId) {
			lastLoadedServiceId = service.id;
			composeYaml = service.composeYaml || service.k8sYaml || getDefaultComposeYaml(service);
			inPod = service.inPod || service.runtimeTarget === 'pod';
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

	// The editor value must stay live for bindings and save; only the parse work
	// is debounced so typing does not run the full AST on every keystroke.
	let parseTimer: ReturnType<typeof setTimeout> | undefined;
	let debouncedComposeYaml = $state('');

	$effect(() => {
		const value = composeYaml;
		if (value === debouncedComposeYaml) return;
		clearTimeout(parseTimer);
		parseTimer = setTimeout(() => (debouncedComposeYaml = value), 200);
		return () => clearTimeout(parseTimer);
	});

	let composeParseResult = $derived.by(() => parseComposeManifest(debouncedComposeYaml));

	async function handleSave() {
		// Validate the live editor contents, not the debounced parse snapshot,
		// so a fast save right after typing cannot bypass syntax errors.
		const current = parseComposeManifest(composeYaml);
		if (!current.valid) {
			toast.add(
				errorToastOptions(
					'Invalid Compose Syntax',
					current.errors[0] || 'Please resolve syntax errors before saving.'
				)
			);
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

		service.sourceType = 'compose';

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
			toast.add(
				errorToastOptions(
					'Failed to save specification',
					err?.message || 'Error occurred while saving manifest'
				)
			);
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

		<!-- Right Controls: Preview Button -->
		<div class="flex items-center gap-2.5">
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
		<div
			class="flex items-start gap-2 rounded-md bg-kumo-base px-3 py-2 text-[11px] leading-relaxed text-kumo-subtle ring ring-kumo-line"
		>
			<Info size={13} class="mt-0.5 shrink-0" />
			<span>
				Stack manifests are edited inline. Pulling a compose file from a Git repository is not
				implemented yet; use an Application service for Git-based builds.
			</span>
		</div>

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
