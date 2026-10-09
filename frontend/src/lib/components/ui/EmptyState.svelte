<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import type { Snippet } from 'svelte';

	interface Props {
		title?: string;
		description?: string;
		icon?: Snippet;
		class?: string;
	}

	let { title = 'No data found', description, icon, class: className = '' }: Props = $props();
</script>

<div class={cn('flex flex-col items-center gap-3 py-12 text-center', className)}>
	{#if icon}
		<div class="text-[var(--text-tertiary)]">
			{@render icon()}
		</div>
	{/if}
	<div class="flex flex-col gap-1">
		<strong class="text-md font-medium text-[var(--text-secondary)]">{title}</strong>
		{#if description}
			<p class="m-0 text-base text-[var(--text-tertiary)]">{description}</p>
		{/if}
	</div>
</div>
