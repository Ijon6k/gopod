<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		label: string;
		value: string;
		detail?: string;
		class?: string;
	}

	let { label, value, detail, class: className = '' }: Props = $props();
</script>

<div class={cn('flex flex-col gap-1.5', className)}>
	<span class="text-[var(--text-tertiary)] text-[10px] font-medium tracking-[0.1em] uppercase">
		{label}
	</span>
	<strong
		class="text-[var(--text-primary)] text-2xl font-medium tracking-[-0.03em] leading-none whitespace-nowrap"
	>
		{value}
	</strong>
	{#if detail}
		<small class="text-[var(--text-tertiary)] text-[11px]">{detail}</small>
	{/if}
</div>
