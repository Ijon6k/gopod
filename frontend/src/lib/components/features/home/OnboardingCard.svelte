<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, LayerCard } from 'kumo-svelte';
	import { Empty } from 'kumo-svelte';
	import { cn } from '$lib/utils/cn';

	interface Props {
		class?: string;
	}

	let { class: className = '' }: Props = $props();
</script>

<!--
	First-run take-over: replaces the deployments/projects grid only while the
	host has neither. One screen, one sentence, one action (Krug law #1).
-->
<LayerCard class={cn('p-5', className)}>
	<Empty
		size="lg"
		title="Get started with GOPOD"
		description="Create your first project, add a service to it, and GOPOD builds and runs it on this podman host."
	>
		{#snippet contents()}
			<Button variant="primary" size="sm" onclick={() => goto('/projects/new')}>
				Create project
			</Button>
		{/snippet}
	</Empty>
</LayerCard>
