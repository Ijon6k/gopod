<script lang="ts">
	import { InlineCopyText } from 'kumo-svelte';
	import { shortenMountPath } from '$lib/components/features/runtime/runtime-helpers';

	interface Props {
		/** Full mount path — the value that ends up on the clipboard. */
		value?: string | null;
		class?: string;
	}

	let { value, class: className = '' }: Props = $props();

	let missing = $derived(!value || value === '—');
</script>

<!-- Mount paths are long; the cell shows a short tail form and delegates the
     full path to title + click-to-copy, so nothing functional is lost. -->
{#if missing}
	<span class="text-[var(--text-tertiary)]">—</span>
{:else}
	<span class={className} title={value ?? ''}>
		<InlineCopyText
			value={value ?? ''}
			variant="mono-secondary"
			labels={{ copyAction: 'Copy mount path' }}
			class="align-middle font-mono text-xs"
		>
			{shortenMountPath(value)}
		</InlineCopyText>
	</span>
{/if}
