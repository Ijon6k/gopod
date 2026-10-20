<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, CodeEditor } from '$lib/components/ui';
	import { FileCode, ShieldCheck, SpinnerGap, Check, WarningCircle } from 'phosphor-svelte';

	interface Props {
		isOpen: boolean;
		service: Service;
		podFiles: Record<string, string>;
		activePromoTab: string;
		promoCode: string;
		validationMessage: { text: string; success: boolean } | null;
		dryrunOutput: string;
		isPromoting: boolean;
		isValidatingDryRun: boolean;
		onSwitchTab: (tabKey: string) => void;
		onValidateDryRun: () => void;
		onConfirm: () => void;
		onCancel: () => void;
	}

	let {
		isOpen = $bindable(),
		service,
		podFiles = $bindable(),
		activePromoTab = $bindable(),
		promoCode = $bindable(),
		validationMessage = $bindable(),
		dryrunOutput = $bindable(),
		isPromoting,
		isValidatingDryRun,
		onSwitchTab,
		onValidateDryRun,
		onConfirm,
		onCancel
	}: Props = $props();
</script>

{#if isOpen}
	<div class="mt-3 flex flex-col gap-3 border-t border-[var(--color-kumo-hairline)] pt-4">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<FileCode size={15} class="text-[var(--accent)]" />
				<span class="text-xs font-medium text-[var(--color-kumo-default)]">
					Review Generated Quadlet Unit Definition
				</span>
			</div>

			<!-- Multi-file tabs for Pod workload -->
			{#if service.type === 'pod' && Object.keys(podFiles).length > 0}
				<div class="flex items-center gap-1 rounded bg-[var(--color-kumo-base)] p-0.5">
					{#each Object.keys(podFiles) as fKey}
						<button
							type="button"
							onclick={() => onSwitchTab(fKey)}
							class="cursor-pointer rounded px-2.5 py-1 font-mono text-[11px] transition-colors {activePromoTab ===
							fKey
								? 'bg-[var(--color-kumo-elevated)] font-semibold text-[var(--color-kumo-default)] shadow-xs'
								: 'bg-transparent text-[var(--color-kumo-subtle)] hover:text-[var(--color-kumo-default)]'}"
						>
							{fKey.endsWith('.pod') ? '📦 ' : '🌐 '}{fKey}
						</button>
					{/each}
				</div>
			{/if}
		</div>

		<!-- Editor for generated unit -->
		<div class="overflow-hidden rounded border border-[var(--color-kumo-line)]">
			<CodeEditor bind:value={promoCode} language="quadlet" height="auto" />
		</div>

		{#if validationMessage}
			<div
				class="flex items-center gap-2 rounded px-3 py-2 text-xs {validationMessage.success
					? 'bg-kumo-success-tint text-kumo-success'
					: 'bg-[var(--status-red-muted)] text-[var(--status-red)]'}"
			>
				{#if validationMessage.success}
					<Check size={14} weight="bold" />
				{:else}
					<WarningCircle size={14} weight="bold" />
				{/if}
				<span>{validationMessage.text}</span>
			</div>
		{/if}

		{#if dryrunOutput}
			<div
				class="min-h-24 overflow-x-auto rounded border border-[var(--border)] bg-[var(--bg-table-header)] p-3.5 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-[var(--text-secondary)] select-text"
			>
				{dryrunOutput}
			</div>
		{/if}

		<div class="flex flex-wrap items-center justify-between gap-2 pt-1">
			<div class="flex items-center">
				<Button
					variant="secondary"
					size="sm"
					disabled={isValidatingDryRun}
					onclick={onValidateDryRun}
					class="gap-1.5"
				>
					{#if isValidatingDryRun}
						<SpinnerGap size={14} class="animate-spin" />
						<span>Validating…</span>
					{:else}
						<ShieldCheck size={14} />
						<span>Pre-flight Dry-Run</span>
					{/if}
				</Button>
			</div>

			<div class="flex items-center gap-2">
				<Button variant="secondary" size="sm" onclick={onCancel}>Cancel</Button>
				<Button
					variant="primary"
					size="sm"
					onclick={onConfirm}
					disabled={isPromoting}
					class="gap-1.5"
				>
					{#if isPromoting}
						<SpinnerGap size={14} class="animate-spin" />
						<span>Activating Unit…</span>
					{:else}
						<Check size={14} weight="bold" />
						<span>Confirm & Activate Systemd Unit</span>
					{/if}
				</Button>
			</div>
		</div>
	</div>
{/if}
