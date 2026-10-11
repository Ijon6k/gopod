<script lang="ts">
	import { InputGroup, InputGroupAddon, InputGroupInput } from 'kumo-svelte';
	import { MagnifyingGlass, X } from 'phosphor-svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		class?: string;
		oninput?: (e: Event) => void;
	}

	let {
		value = $bindable(''),
		placeholder = 'Search…',
		class: className = '',
		oninput
	}: Props = $props();
</script>

<InputGroup class={className} size="base">
	<InputGroupAddon align="start">
		<MagnifyingGlass size={15} />
	</InputGroupAddon>
	<InputGroupInput
		type="search"
		bind:value
		{placeholder}
		{oninput}
		aria-label="Search"
	/>
	{#if value}
		<InputGroupAddon align="end">
			<button
				type="button"
				onclick={() => (value = '')}
				class="cursor-pointer border-0 bg-transparent p-0.5 text-kumo-subtle hover:text-kumo-default transition-colors"
				aria-label="Clear search"
			>
				<X size={14} />
			</button>
		</InputGroupAddon>
	{/if}
</InputGroup>
