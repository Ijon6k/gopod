<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { Info } from 'phosphor-svelte';
	import PodPortForwardCard from './pod/PodPortForwardCard.svelte';
	import PodMembersTable from './pod/PodMembersTable.svelte';
	import PodAttachMemberModal from './pod/PodAttachMemberModal.svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let isAddingContainer = $state(false);

	// Find member services belonging to this Pod
	let memberServices = $derived.by(() => {
		return dataStore.services.filter(
			(s) =>
				s.projectId === service.projectId &&
				s.id !== service.id &&
				(s.podId === service.id ||
					s.podId === service.name ||
					s.podId === `pod_${service.name}` ||
					(s.inPod && s.type !== 'pod'))
		);
	});

	// Unattached candidate services in the same project
	let candidateServices = $derived.by(() => {
		return dataStore.services.filter(
			(s) =>
				s.projectId === service.projectId &&
				s.id !== service.id &&
				s.type !== 'pod' &&
				s.podId !== service.id &&
				s.podId !== service.name &&
				!s.inPod
		);
	});

	let podName = $derived(`pod_${service.name}`);
</script>

<div class="flex w-full flex-col gap-5">
	<!-- 1. Informative banner (Localhost Network Namespace) -->
	<div
		class="flex items-center gap-3 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] px-4 py-3 text-xs text-[var(--color-kumo-default)]"
	>
		<Info size={18} weight="fill" class="shrink-0 text-[var(--accent)]" />
		<div>
			<span class="font-semibold">Localhost Network Namespace:</span>
			<span class="text-[var(--color-kumo-subtle)]">
				Containers inside this pod communicate directly with one another via
				<code class="rounded bg-black/20 px-1.5 py-0.5 font-mono text-[var(--accent)]"
					>localhost:&lt;port&gt;</code
				>. Host ports are exposed exclusively at the Pod level.
			</span>
		</div>
	</div>

	<!-- 2. Card 1: Pod Infrastructure & Exposed Ports -->
	<PodPortForwardCard {service} {podName} />

	<!-- 3. Card 2: Pod Member Containers Table -->
	<PodMembersTable {memberServices} onOpenAddModal={() => (isAddingContainer = true)} />
</div>

<!-- Modal: Add or Attach Container to Pod -->
<PodAttachMemberModal
	bind:open={isAddingContainer}
	{service}
	{podName}
	{candidateServices}
	onclose={() => (isAddingContainer = false)}
/>
