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

<!-- Level 3: View Level Sub-Header (Kumo Sentence Case, font-semibold, normal tracking) -->
<header
	class={cn('flex flex-col justify-between gap-3 sm:flex-row sm:items-center sm:gap-4', className)}
>
	<div class="flex flex-col gap-1">
		<div class="flex items-center gap-3">
			<h1 class="m-0 text-xl font-semibold text-[var(--text-primary)]">
				{title}
			</h1>
			{#if meta}
				{@render meta()}
			{/if}
		</div>
		{#if subtitle}
			<p class="m-0 text-xs text-[var(--text-tertiary)]">{subtitle}</p>
		{/if}
	</div>
	{#if actions}
		<div class="flex shrink-0 items-center gap-2">
			{@render actions()}
		</div>
	{/if}
</header>
