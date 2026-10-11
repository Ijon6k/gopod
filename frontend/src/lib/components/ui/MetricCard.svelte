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
	<span class="text-[11px] font-medium tracking-[0.08em] text-kumo-subtle uppercase">
		{label}
	</span>
	<strong
		class="text-2xl leading-none font-semibold tracking-tight whitespace-nowrap text-kumo-default"
	>
		{value}
	</strong>
	{#if detail}
		<small class="text-xs text-kumo-subtle">{detail}</small>
	{/if}
</div>
