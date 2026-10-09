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
		'flex flex-col rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] px-[22px] pt-5 pb-4',
		className
	)}
>
	<div class="mb-4 flex min-h-[22px] items-center justify-between gap-4">
		<span class="text-[10px] font-medium tracking-[0.1em] text-[var(--text-tertiary)] uppercase">
			Resource usage
		</span>
		<Tabs {tabs} bind:active={activeTab} size="sm" />
	</div>
	<div class="mb-1.5 flex items-baseline gap-2.5">
		<strong class="text-2xl font-medium tracking-[-0.03em] text-[var(--text-primary)]">
			{currentMeta?.current ?? '—'}
		</strong>
		<small class="text-[11px] text-[var(--text-tertiary)]">last 24 hours</small>
	</div>
	<div class="-mx-0.5 mt-auto">
		<AreaChart data={currentData} height={168} />
	</div>
</section>
