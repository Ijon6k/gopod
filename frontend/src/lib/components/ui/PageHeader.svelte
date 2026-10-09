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

<header class={cn('mb-4 flex items-start justify-between gap-3 md:mb-7 md:gap-4', className)}>
	<div class="flex flex-col gap-1.5">
		<div class="flex items-center gap-3">
			<h1
				class="m-0 text-2xl leading-tight font-semibold tracking-[-0.02em] text-[var(--text-primary)]"
			>
				{title}
			</h1>
			{#if meta}
				{@render meta()}
			{/if}
		</div>
		{#if subtitle}
			<p class="m-0 text-base text-[var(--text-tertiary)]">{subtitle}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex shrink-0 items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
