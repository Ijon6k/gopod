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
	<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase">
		{label}
	</span>
	<strong
		class="text-2xl leading-none font-medium tracking-[-0.03em] whitespace-nowrap text-[var(--text-primary)]"
	>
		{value}
	</strong>
	{#if detail}
		<small class="text-[11px] text-[var(--text-tertiary)]">{detail}</small>
	{/if}
</div>
