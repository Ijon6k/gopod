<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		orientation?: 'horizontal' | 'vertical';
		class?: string;
	}

	let { orientation = 'horizontal', class: className = '' }: Props = $props();
</script>

{#if orientation === 'horizontal'}
	<hr class={cn('h-px border-0 bg-[var(--border)]', className)} />
{:else}
	<div class={cn('w-px self-stretch bg-[var(--border)]', className)}></div>
{/if}
