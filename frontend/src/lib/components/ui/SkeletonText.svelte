<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { SkeletonLine } from 'kumo-svelte';

	interface Props {
		/** Number of placeholder lines. The last line renders shorter, like prose. */
		lines?: number;
		class?: string;
		/** Tailwind height utility on every line (default 0.75rem). */
		lineClass?: string;
	}

	let { lines = 3, class: className = '', lineClass = 'h-3' }: Props = $props();
</script>

<!-- Kumo card-loading-state pattern: staggered line widths only on the closing line -->
<div class={cn('flex flex-col gap-2', className)}>
	{#each Array(lines) as _, i (i)}
		<!-- SkeletonLine (kumo) randomizes width per line for a natural look -->
		<div class="min-w-0" style:width={i === lines - 1 ? '72%' : `${85 - ((i * 97) % 13)}%`}>
			<SkeletonLine minWidth={100} maxWidth={100} class={cn('w-full', lineClass)} />
		</div>
	{/each}
</div>
