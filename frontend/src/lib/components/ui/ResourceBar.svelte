<script lang="ts">
	import { cn } from '$lib/utils/cn';

	interface Props {
		label: string;
		value: number;
		max: number;
		unit?: string;
		class?: string;
	}

	let { label, value, max, unit = '', class: className = '' }: Props = $props();

	let percentage = $derived(max > 0 ? Math.min((value / max) * 100, 100) : 0);
</script>

<div class={cn('flex flex-col gap-2', className)}>
	<div class="flex items-center justify-between">
		<span class="text-[11px] text-[var(--text-tertiary)]">{label}</span>
		<span class="text-[11px] text-[var(--text-secondary)] tabular-nums">
			{value.toFixed(1)}{unit} / {max}{unit}
		</span>
	</div>
	<div class="h-1.5 rounded-full bg-[var(--bg-hover)] overflow-hidden">
		<div
			class="h-full rounded-full bg-[var(--accent)] transition-[width] duration-300"
			style="width: {percentage}%"
		></div>
	</div>
</div>
