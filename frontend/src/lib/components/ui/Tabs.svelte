<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Tab {
		id: string;
		label: string;
	}

	interface Props {
		tabs: Tab[];
		active: string;
		onchange?: (tabId: string) => void;
		size?: 'sm' | 'md';
		class?: string;
	}

	let {
		tabs,
		active = $bindable(),
		onchange,
		size = 'md',
		class: className = ''
	}: Props = $props();

	function select(tabId: string) {
		active = tabId;
		onchange?.(tabId);
	}
</script>

<div class={cn('flex gap-3.5', className)} role="tablist">
	{#each tabs as tab}
		<button
			role="tab"
			aria-selected={active === tab.id}
			onclick={() => select(tab.id)}
			class={cn(
				'cursor-pointer border-0 border-b border-transparent bg-none px-0 pb-[3px] capitalize transition-colors',
				size === 'sm' ? 'text-xs' : 'text-base',
				'font-[var(--font-sans)] font-normal',
				active === tab.id
					? 'border-b-[var(--accent)] text-[var(--text-primary)]'
					: 'text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'
			)}
		>
			{tab.label}
		</button>
	{/each}
</div>
