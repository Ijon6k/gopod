<script lang="ts">
	import { cn } from '$lib/utils/cn';

	export type TabsVariant = 'segmented' | 'underline';
	export type TabsSize = 'base' | 'sm';

	export interface Tab {
		id?: string;
		value?: string;
		label: string;
	}

	interface Props {
		tabs?: Tab[];
		items?: Tab[];
		active?: string;
		value?: string;
		onchange?: (tabId: string) => void;
		variant?: TabsVariant;
		size?: 'sm' | 'md' | TabsSize;
		class?: string;
	}

	let {
		tabs,
		items,
		active = $bindable(''),
		value = $bindable(''),
		onchange,
		variant = 'underline',
		size = 'md',
		class: className = ''
	}: Props = $props();

	let currentActive = $derived(active || value || '');

	let normalizedTabs = $derived.by(() => {
		const raw = tabs ?? items ?? [];
		return raw.map((t) => ({
			id: t.id || t.value || '',
			label: t.label
		}));
	});

	function handleValueChange(val: string) {
		active = val;
		value = val;
		onchange?.(val);
	}
</script>

{#if variant === 'segmented'}
	<!-- Kumo Segmented Control (Native, resilient, zero MutationObserver tick loop) -->
	<div
		class={cn(
			'inline-flex items-center gap-1 rounded-lg bg-kumo-recessed p-0.5 ring ring-kumo-hairline/70',
			className
		)}
		role="tablist"
	>
		{#each normalizedTabs as tab (tab.id)}
			<button
				type="button"
				role="tab"
				aria-selected={currentActive === tab.id}
				onclick={() => handleValueChange(tab.id)}
				class={cn(
					'relative z-1 flex cursor-pointer items-center rounded-md font-medium whitespace-nowrap transition-all focus:outline-none focus-visible:ring-2 focus-visible:ring-kumo-brand',
					size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-sm',
					currentActive === tab.id
						? 'bg-kumo-base text-kumo-default shadow-xs ring ring-kumo-line font-medium'
						: 'text-kumo-subtle hover:text-kumo-default'
				)}
			>
				{tab.label}
			</button>
		{/each}
	</div>
{:else}
	<!-- Kumo Underline Tabs (Native Svelte 5, zero MutationObserver tick loop) -->
	<div class={cn('w-full border-b border-kumo-hairline', className)}>
		<div class="flex items-center gap-6 overflow-x-auto no-scrollbar" role="tablist">
			{#each normalizedTabs as tab (tab.id)}
				<button
					type="button"
					role="tab"
					aria-selected={currentActive === tab.id}
					onclick={() => handleValueChange(tab.id)}
					class={cn(
						'relative flex cursor-pointer items-center py-2.5 font-medium whitespace-nowrap transition-colors focus:outline-none focus-visible:text-kumo-default',
						size === 'sm' ? 'text-xs' : 'text-sm',
						currentActive === tab.id
							? 'text-kumo-default font-semibold after:absolute after:bottom-0 after:inset-x-0 after:h-0.5 after:bg-kumo-brand after:rounded-full'
							: 'text-kumo-subtle hover:text-kumo-default'
					)}
				>
					{tab.label}
				</button>
			{/each}
		</div>
	</div>
{/if}



