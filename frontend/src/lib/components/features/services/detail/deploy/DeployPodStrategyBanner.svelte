<script lang="ts">
	import type { Service } from '$lib/types';
	import { Switch, Badge } from 'kumo-svelte';
	import { Package } from 'phosphor-svelte';

	interface Props {
		service: Service;
		inPod: boolean;
		onToggleInPod: () => void;
	}

	let { service, inPod, onToggleInPod }: Props = $props();
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
				(All member containers share localhost namespace · pod_{service.name})
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
					? `(Shared localhost network namespace · pod_${service.id})`
					: '(Independent containers attached to gopod-net bridge)'}
			</span>
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
