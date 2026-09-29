<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		subtitle?: string;
		meta?: Snippet;
		actions?: Snippet;
		class?: string;
	}

	let { title, subtitle, meta, actions, class: className = '' }: Props = $props();
</script>

<header class={cn('flex items-start justify-between gap-3 md:gap-4 mb-4 md:mb-7', className)}>
	<div class="flex flex-col gap-1.5">
		<div class="flex items-center gap-3">
			<h1 class="text-2xl font-semibold tracking-[-0.02em] text-[var(--text-primary)] m-0 leading-tight">
				{title}
			</h1>
			{#if meta}
				{@render meta()}
			{/if}
		</div>
		{#if subtitle}
			<p class="text-base text-[var(--text-tertiary)] m-0">{subtitle}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex items-center gap-2 shrink-0">
			{@render actions()}
		</div>
	{/if}
</header>
