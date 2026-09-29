<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		class?: string;
		children: import('svelte').Snippet;
	}

	let { class: className = '', children }: Props = $props();
</script>

<kbd
	class={cn(
		'px-1 py-px border border-[var(--border)] rounded-[3px] bg-[var(--bg-panel)] text-[10px] text-[var(--text-tertiary)] font-[var(--font-sans)]',
		className
	)}
>
	{@render children()}
</kbd>
