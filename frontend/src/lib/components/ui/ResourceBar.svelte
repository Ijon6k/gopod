<script lang="ts">
	import { Meter } from 'kumo-svelte';

	interface Props {
		label: string;
		value: number;
		max: number;
		unit?: string;
		class?: string;
	}

	let { label, value, max, unit = '', class: className = '' }: Props = $props();

	let customValue = $derived(`${value.toFixed(1)}${unit} / ${max}${unit}`);
</script>

<Meter
	{label}
	{value}
	min={0}
	{max}
	{customValue}
	class={className}
/>
