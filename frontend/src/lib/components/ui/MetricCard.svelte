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

<div class={cn('flex flex-col gap-1', className)}>
	<span class="text-xs font-medium text-kumo-subtle">
		{label}
	</span>
	<strong class="text-2xl leading-tight font-semibold whitespace-nowrap text-kumo-default">
		{value}
	</strong>
	{#if detail}
		<small class="text-xs text-kumo-subtle">{detail}</small>
	{/if}
</div>
