<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { TerminalView } from '$lib/components/ui';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let workloads = $derived<Workload[]>(
		service.workloads ?? [{ name: service.name, image: service.image ?? '—', status: service.status }]
	);
	let containerOverride = $state<string | null>(null);
	let selectedContainer = $derived(containerOverride ?? (workloads[0]?.name ?? service.name));
</script>

<div class="w-full flex flex-col gap-4">
	<TerminalView
		title={service.name}
		{workloads}
		selectedWorkload={selectedContainer}
		height="480px"
	/>
</div>

