<script lang="ts">
	import Modal from './Modal.svelte';
	import { Button, Input } from '$lib/components/ui';
	import { WarningCircle, Copy, Check } from 'phosphor-svelte';
	import type { Snippet } from 'svelte';

	interface Props {
		open: boolean;
		title: string;
		description?: string;
		variant?: 'danger' | 'warning' | 'default';
		confirmText?: string;
		cancelText?: string;
		isConfirming?: boolean;
		confirmDisabled?: boolean;
		matchValue?: string;
		matchLabel?: string;
		onconfirm: () => void | Promise<void>;
		oncancel: () => void;
		children?: Snippet;
	}

	let {
		open,
		title,
		description,
		variant = 'danger',
		confirmText = 'Confirm',
		cancelText = 'Cancel',
		isConfirming = false,
		confirmDisabled = false,
		matchValue,
		matchLabel,
		onconfirm,
		oncancel,
		children
	}: Props = $props();

	let inputVal = $state('');
	let copied = $state(false);

	$effect(() => {
		if (open) {
			inputVal = '';
		}
	});

	let isMatchValid = $derived.by(() => {
		if (!matchValue) return true;
		return inputVal.trim() === matchValue.trim();
	});

	let canConfirm = $derived(!confirmDisabled && !isConfirming && isMatchValid);

	function handleCopy() {
		if (!matchValue) return;
		navigator.clipboard.writeText(matchValue);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<Modal {open} onclose={oncancel} size="md">
	<div class="flex items-center gap-3">
		<div
			class={variant === 'danger'
				? 'text-[var(--status-red)]'
				: variant === 'warning'
					? 'text-[var(--status-amber)]'
					: 'text-kumo-brand'}
		>
			<WarningCircle size={22} weight="bold" />
		</div>
		<h3 class="m-0 text-base font-semibold text-kumo-default">
			{title}
		</h3>
	</div>

	{#if description}
		<p class="m-0 text-xs leading-relaxed text-kumo-subtle">
			{description}
		</p>
	{/if}

	<!-- Anti-fat-finger text input confirmation -->
	{#if matchValue}
		<div class="flex flex-col gap-1.5 pt-1">
			<div class="flex items-center justify-between text-xs text-kumo-subtle">
				<span>{matchLabel ?? 'To confirm, type the exact name below:'}</span>
				<button
					type="button"
					onclick={handleCopy}
					class="inline-flex cursor-pointer items-center gap-1 rounded border border-kumo-line bg-kumo-base px-2 py-0.5 font-mono text-[11px] text-kumo-subtle transition-colors hover:text-kumo-default"
					title="Copy to clipboard"
				>
					{#if copied}
						<Check size={11} class="text-[var(--status-green)]" />
						<span class="text-[var(--status-green)]">Copied</span>
					{:else}
						<Copy size={11} />
						<span>{matchValue}</span>
					{/if}
				</button>
			</div>
			<Input type="text" bind:value={inputVal} placeholder={matchValue} size="sm" class="w-full" />
		</div>
	{/if}

	{#if children}
		{@render children()}
	{/if}

	{#snippet footer()}
		<Button variant="secondary" size="sm" disabled={isConfirming} onclick={oncancel}>
			{cancelText}
		</Button>
		<Button
			variant={variant === 'danger'
				? 'destructive'
				: variant === 'warning'
					? 'secondary'
					: 'primary'}
			size="sm"
			disabled={!canConfirm}
			loading={isConfirming}
			onclick={onconfirm}
		>
			{confirmText}
		</Button>
	{/snippet}
</Modal>
