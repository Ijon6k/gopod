<script lang="ts">
	import { Button, Modal, CodeEditor } from '$lib/components/ui';
	import type { QuadletUnit } from '$lib/types';

	interface Props {
		unit: QuadletUnit | null;
		onclose: () => void;
	}

	let { unit, onclose }: Props = $props();
</script>

<Modal
	open={!!unit}
	{onclose}
	title={unit?.name ?? 'Quadlet Unit'}
	subtitle={`Systemd unit source at ${unit?.path ?? ''}`}
	size="lg"
>
	{#if unit}
		<div class="flex flex-col gap-3">
			<CodeEditor value={unit.content} language="quadlet" readOnly={true} height="260px" />
			<div class="flex justify-end">
				<Button variant="secondary" onclick={onclose}>Close</Button>
			</div>
		</div>
	{/if}
</Modal>
