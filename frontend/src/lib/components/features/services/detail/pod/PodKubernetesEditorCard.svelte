<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { CodeEditor, CopyButton } from '$lib/components/ui';
	import { LayerCard, Badge, Button, useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { FileCode, Check, FloppyDisk, Info } from 'phosphor-svelte';
	import {
		parseDetailedPorts,
		parseK8sPodPorts,
		serializePodComposeYaml,
		serializePodK8sYaml,
		portBindingId,
		normalizePortHostIp
	} from '$lib/parsers/pod-ports';
	import { parseKubernetesPodManifest } from '$lib/parsers/manifest-engine';
	import ArchitecturePreviewBar from '../manifests/ArchitecturePreviewBar.svelte';

	interface Props {
		service: Service;
		podName: string;
		memberServices?: Service[];
		onmanifestchange?: () => void;
	}

	let { service, podName, memberServices = [], onmanifestchange }: Props = $props();

	const toast = useKumoToastManager();

	let k8sYaml = $state('');
	let isSaving = $state(false);
	let saveStatus = $state<'idle' | 'saved'>('idle');
	let initializedFor = $state('');
	let previewExpanded = $state(false);

	// The editor value must stay live for bindings and save; only the parse work
	// is debounced so typing does not run the full AST on every keystroke.
	let parseTimer: ReturnType<typeof setTimeout> | undefined;
	let debouncedYaml = $state('');

	$effect(() => {
		const value = k8sYaml;
		if (value === debouncedYaml) return;
		clearTimeout(parseTimer);
		parseTimer = setTimeout(() => (debouncedYaml = value), 200);
		return () => clearTimeout(parseTimer);
	});

	let parseResult = $derived.by(() => parseKubernetesPodManifest(debouncedYaml));

	// Initialize YAML once per service version; never clobber in-progress edits
	$effect(() => {
		const key = `${service.id}:${(service.k8sYaml || '').length}:${(service.composeYaml || '').length}`;
		if (initializedFor === key) return;
		initializedFor = key;
		if (service.k8sYaml && service.k8sYaml.trim()) {
			k8sYaml = service.k8sYaml;
		} else {
			const existingPorts = service.composeYaml
				? parseDetailedPorts(service.composeYaml)
				: service.port && service.port > 0
					? [
							{
								id: 'port-init',
								hostPort: service.hostPort || service.port,
								podPort: service.port,
								protocol: 'tcp' as const,
								hostIp: normalizePortHostIp(service.hostIp)
							}
						]
					: [];
			const membersDef = memberServices.map((m) => ({
				name: m.name,
				image: m.image,
				port: m.port
			}));
			k8sYaml = serializePodK8sYaml(service.name, existingPorts, membersDef, service.k8sYaml);
		}
	});

	async function handleSaveManifest() {
		// Validate the live editor contents, not the debounced parse snapshot,
		// so a fast save right after typing cannot bypass syntax errors.
		const current = parseKubernetesPodManifest(k8sYaml);
		if (!current.valid) {
			toast.add(
				errorToastOptions(
					'Invalid Manifest Syntax',
					current.errors[0] || 'Please resolve syntax errors before saving manifest.'
				)
			);
			return;
		}

		isSaving = true;
		try {
			service.k8sYaml = k8sYaml;

			// Bi-directional sync: parse ports from the edited Kubernetes manifest.
			// Always keep composeYaml in sync (even when no ports remain) so Visual view never goes stale.
			// K8s manifests carry no bind address, so preserve each binding's existing
			// hostIp (matched by host/pod port + protocol); new bindings default to 0.0.0.0.
			const previousPorts = service.composeYaml ? parseDetailedPorts(service.composeYaml) : [];
			const previousIpByBinding = new Map(
				previousPorts.map((p) => [
					`${p.hostPort}:${p.podPort}:${p.protocol}`,
					normalizePortHostIp(p.hostIp)
				])
			);
			const extractedPorts = parseK8sPodPorts(k8sYaml).map((p) => {
				const hostIp =
					previousIpByBinding.get(`${p.hostPort}:${p.podPort}:${p.protocol}`) ?? '0.0.0.0';
				return { ...p, hostIp, id: portBindingId(p.hostPort, p.podPort, p.protocol, hostIp) };
			});
			if (extractedPorts.length > 0) {
				service.hostPort = extractedPorts[0].hostPort;
				service.port = extractedPorts[0].podPort;
			} else {
				service.hostPort = 0;
				service.port = 0;
			}
			service.composeYaml = serializePodComposeYaml(
				service.name,
				extractedPorts,
				service.composeYaml
			);

			await dataStore.updateService(service);

			saveStatus = 'saved';
			setTimeout(() => {
				saveStatus = 'idle';
			}, 3000);

			toast.add({
				title: 'Kubernetes Manifest Saved',
				description:
					'Configuration updated. Recreate the pod to apply changes to running containers.',
				variant: 'success'
			});

			onmanifestchange?.();
		} catch (err: any) {
			toast.add(
				errorToastOptions(
					'Failed to Save Manifest',
					err?.message || 'Error updating Kubernetes YAML manifest'
				)
			);
		} finally {
			isSaving = false;
		}
	}
</script>

<LayerCard class="overflow-hidden p-0 shadow-xs">
	<!-- Card Header with Badges & Copy Button -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<FileCode size={16} class="text-[var(--accent)]" />
			<div>
				<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
					Kubernetes Manifest (apiVersion: v1 / kind: Pod)
				</h3>
			</div>
		</div>

		<div class="flex items-center gap-2">
			<Badge variant="neutral" class="font-mono text-xs">spec: kubernetes-pod</Badge>
			<Badge variant="neutral" class="font-mono text-xs">engine: podman-play-kube</Badge>
			<CopyButton
				text={k8sYaml}
				label="Copy YAML"
				variant="icon"
				size={14}
				class="text-[var(--color-kumo-subtle)] hover:text-[var(--color-kumo-default)]"
			/>
		</div>
	</LayerCard.Secondary>

	<!-- Explanatory note -->
	<div
		class="border-b border-[var(--color-kumo-hairline)] bg-[var(--bg-panel)] px-5 py-2.5 text-xs text-[var(--color-kumo-subtle)]"
	>
		<div class="flex items-center gap-2">
			<Info size={14} class="shrink-0 text-[var(--accent)]" />
			<span>
				This declarative manifest is executed by Podman via <code
					class="font-mono text-[var(--accent)]">podman play kube</code
				>. Port bindings and container configurations saved here automatically synchronize with the
				Visual view. Bind addresses (0.0.0.0 vs 127.0.0.1) are managed in the Visual view.
			</span>
		</div>
	</div>

	<!-- CodeMirror Monospace YAML Editor -->
	<div class="flex flex-col gap-3 p-4">
		<!-- Compact Architecture Preview Bar directly above Editor -->
		<ArchitecturePreviewBar {parseResult} bind:expanded={previewExpanded} />

		<div class="overflow-hidden rounded-md border border-[var(--border)] shadow-inner">
			<CodeEditor bind:value={k8sYaml} language="yaml" height="auto" />
		</div>
	</div>

	<!-- Card Footer with Save Action -->
	<div
		class="flex items-center justify-between gap-3 border-t border-[var(--border)] bg-[var(--bg-panel)] px-5 py-3"
	>
		<span class="text-xs text-[var(--color-kumo-subtle)]">
			Pod target: <code class="font-mono text-[var(--color-kumo-default)]">{podName}</code>
		</span>

		<div class="flex items-center gap-2">
			{#if saveStatus === 'saved'}
				<span class="mr-1 flex items-center gap-1 text-xs text-[var(--status-green)]">
					<Check size={14} weight="bold" /> Saved
				</span>
			{/if}
			<Button variant="primary" onclick={handleSaveManifest} disabled={isSaving} class="gap-1.5">
				<FloppyDisk size={14} />
				<span>{isSaving ? 'Saving...' : 'Save Manifest'}</span>
			</Button>
		</div>
	</div>
</LayerCard>
