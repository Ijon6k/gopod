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
		'rounded-[3px] border border-[var(--border)] bg-[var(--bg-panel)] px-1 py-px text-[10px] font-[var(--font-sans)] text-[var(--text-tertiary)]',
		className
	)}
>
	{@render children()}
</kbd>
