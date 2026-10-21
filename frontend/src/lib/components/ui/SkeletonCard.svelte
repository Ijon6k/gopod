<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { SkeletonLine, LayerCard } from 'kumo-svelte';
	import SkeletonText from './SkeletonText.svelte';

	interface Props {
		lines?: number;
		/** Omit the title line when a panel only shows body content. */
		title?: boolean;
		class?: string;
	}

	let {
		lines = 3,
		title = true,
		class: className = ''
	}: Props = $props();
</script>

<!-- Ring (not border+shadow) per workspace shadow-borders rule; natural height -->
<LayerCard class={cn('w-full p-4', className)}>
	{#if title}
		<div class="mb-3 w-40 min-w-0">
			<SkeletonLine minWidth={100} maxWidth={100} class="h-3.5" />
		</div>
	{/if}
	<SkeletonText lines={lines} />
</LayerCard>
