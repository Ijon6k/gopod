<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { Switch, Badge } from 'kumo-svelte';
	import { Package } from 'phosphor-svelte';

	interface Props {
		service: Service;
		inPod: boolean;
		onToggleInPod: () => void;
	}

	let { service, inPod, onToggleInPod }: Props = $props();

	function sanitizePodSlug(name: string): string {
		return name.replace(/^pod_/, '').trim().toLowerCase().replace(/[^a-z0-9_-]+/g, '-');
	}

	let effectivePodName = $derived.by(() => {
		if (service.podId && service.podId.trim()) {
			const raw = service.podId.trim();
			return raw.startsWith('pod_') ? raw : `pod_${sanitizePodSlug(raw)}`;
		}
		if (service.id && service.id.trim()) {
			return `pod_${sanitizePodSlug(service.id)}`;
		}
		const raw = service.name || 'pod-service';
		return `pod_${sanitizePodSlug(raw)}`;
	});

	let parentPodService = $derived.by(() => {
		if (!service.podId) return undefined;
		const targetSlug = sanitizePodSlug(service.podId);
		return dataStore.services.find(
			(s) =>
				s.projectId === service.projectId &&
				(s.id === service.podId ||
					s.podId === service.podId ||
					s.name === service.podId ||
					`pod_${s.name}` === service.podId ||
					`pod_${sanitizePodSlug(s.id)}` === service.podId ||
					sanitizePodSlug(s.name) === targetSlug ||
					sanitizePodSlug(s.id) === targetSlug)
		);
	});
</script>

{#if service.type === 'pod'}
	<!-- Pod Infrastructure Summary -->
	<div
		class="flex flex-wrap items-center justify-between gap-3 border-t border-kumo-hairline bg-kumo-elevated px-5 py-3 text-xs"
	>
		<div class="flex items-center gap-2.5 text-kumo-default">
			<Package size={15} class="text-[var(--accent)]" />
			<span class="font-medium">Podman Pod Infrastructure</span>
			<span class="text-[11px] text-kumo-subtle">
				(All member containers share localhost namespace · {effectivePodName})
			</span>
		</div>
		<div class="flex items-center gap-2">
			<Badge variant="neutral" class="font-mono text-[11px]">network: gopod-net</Badge>
			<Badge variant="neutral" class="font-mono text-[11px] text-[var(--accent)]">
				mode: podman-pod
			</Badge>
		</div>
	</div>
{:else if service.type !== 'compose' && service.type !== 'quadlet'}
	<div
		class="flex flex-wrap items-center justify-between gap-3 border-t border-kumo-hairline bg-kumo-elevated px-5 py-3 text-xs"
	>
		<div class="flex items-center gap-3">
			<Switch
				checked={inPod}
				onCheckedChange={onToggleInPod}
				aria-label="Toggle encapsulation inside Podman Pod"
			/>
			<span class="font-medium text-kumo-default">Run inside Podman Pod</span>
			<span class="hidden text-[11px] text-kumo-subtle sm:inline">
				{inPod
					? `(Shared localhost network namespace · ${effectivePodName})`
					: '(Independent containers attached to gopod-net bridge)'}
			</span>
			{#if inPod && parentPodService}
				<a
					href="/projects/{service.projectId}/services/{parentPodService.id}"
					class="hidden items-center gap-1 text-[11px] font-medium text-[var(--accent)] hover:underline md:inline-flex"
				>
					pod: {parentPodService.name}
				</a>
			{/if}
		</div>
		<div class="flex items-center gap-2">
			<Badge variant="neutral" class="font-mono text-[11px]">network: gopod-net</Badge>
			<Badge
				variant="neutral"
				class="font-mono text-[11px] {inPod ? 'text-[var(--accent)]' : 'text-kumo-subtle'}"
			>
				mode: {inPod ? 'podman-pod' : 'standalone'}
			</Badge>
		</div>
	</div>
{/if}
