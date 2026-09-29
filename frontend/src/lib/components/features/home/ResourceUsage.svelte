<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { Tabs, AreaChart } from '$lib/components/ui';
	import type { TimeSeriesPoint } from '$lib/types';

	interface Props {
		data: Record<string, TimeSeriesPoint[]>;
		chartMeta: Record<string, { current: string }>;
		class?: string;
	}

	let { data, chartMeta, class: className = '' }: Props = $props();

	let activeTab = $state('cpu');

	const tabs = [
		{ id: 'cpu', label: 'cpu' },
		{ id: 'memory', label: 'memory' },
		{ id: 'storage', label: 'storage' }
	];

	let currentData = $derived(data[activeTab] ?? []);
	let currentMeta = $derived(chartMeta[activeTab]);
</script>

<section
	class={cn(
		'bg-[var(--bg-panel)] border border-[var(--border)] rounded-[var(--radius-card)] px-[22px] pt-5 pb-4 flex flex-col',
		className
	)}
>
	<div class="flex items-center justify-between gap-4 min-h-[22px] mb-4">
		<span
			class="text-[10px] font-medium text-[var(--text-tertiary)] tracking-[0.1em] uppercase"
		>
			Resource usage
		</span>
		<Tabs {tabs} bind:active={activeTab} size="sm" />
	</div>
	<div class="flex items-baseline gap-2.5 mb-1.5">
		<strong class="text-2xl font-medium tracking-[-0.03em] text-[var(--text-primary)]">
			{currentMeta?.current ?? '—'}
		</strong>
		<small class="text-[var(--text-tertiary)] text-[11px]">last 24 hours</small>
	</div>
	<div class="mt-auto -mx-0.5">
		<AreaChart data={currentData} height={168} />
	</div>
</section>
