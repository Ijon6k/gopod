<script lang="ts">
	import { DropdownMenu } from 'kumo-svelte';
	import { DotsThreeVertical, CircleNotch } from 'phosphor-svelte';

	export interface RowAction {
		label: string;
		disabled?: boolean;
		/** Destructive actions render in the danger tone, bunched after one separator. */
		danger?: boolean;
		onselect: () => void;
	}

	interface Props {
		actions: RowAction[];
		/** While true the trigger is disabled so one row cannot be double-acted. */
		busy?: boolean;
		itemName: string;
	}

	let { actions, busy = false, itemName }: Props = $props();

	let rendered = $derived.by(() => {
		let separatorPlaced = false;
		return actions.map((action) => {
			const showSeparator = Boolean(action.danger) && !separatorPlaced;
			if (action.danger) separatorPlaced = true;
			return { action, showSeparator };
		});
	});
</script>

<DropdownMenu>
	<DropdownMenu.Trigger
		aria-label={`${itemName} row actions`}
		title={`${itemName} row actions`}
		disabled={busy}
		class="flex h-7 w-7 cursor-pointer items-center justify-center rounded p-1.5 text-[var(--text-secondary)] outline-none hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] disabled:cursor-not-allowed disabled:opacity-60"
	>
		<DotsThreeVertical size={14} class={busy ? 'hidden' : ''} />
		{#if busy}
			<CircleNotch size={14} class="animate-spin" />
		{/if}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content class="min-w-40" align="end">
		{#each rendered as entry (entry.action.label)}
			{#if entry.showSeparator}
				<DropdownMenu.Separator />
			{/if}
			<DropdownMenu.Item
				variant={entry.action.danger ? 'danger' : 'default'}
				disabled={entry.action.disabled}
				onSelect={() => entry.action.onselect()}
			>
				<span class="text-xs">{entry.action.label}</span>
			</DropdownMenu.Item>
		{/each}
	</DropdownMenu.Content>
</DropdownMenu>
