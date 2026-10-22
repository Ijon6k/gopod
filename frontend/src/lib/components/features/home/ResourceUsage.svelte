<script lang="ts">
	import { cn } from '$lib/utils/cn';
	import { Tabs } from '$lib/components/ui';
	import { LayerCard, TimeseriesChart } from 'kumo-svelte';
	import { echarts } from '$lib/utils/echarts-lite';
	import { timeseriesTimeTick, toTimeseriesTuples } from '$lib/utils/timeseries';
	import { ui } from '$lib/stores/ui.svelte';
	import type { TimeSeriesPoint } from '$lib/types';

	interface Props {
		data: Record<string, TimeSeriesPoint[]>;
		chartMeta: Record<string, { current: string }>;
		class?: string;
	}

	let { data, chartMeta, class: className = '' }: Props = $props();

	const tabs = [
		{ id: 'cpu', label: 'CPU' },
		{ id: 'memory', label: 'Memory' },
		{ id: 'storage', label: 'Storage' }
	];

	let activeTab = $state('cpu');
	let currentData = $derived(data[activeTab] ?? []);
	let tuples = $derived(toTimeseriesTuples(currentData));
	let unit = $derived(activeTab === 'cpu' ? '%' : 'GB');
	let seriesName = $derived(tabs.find((t) => t.id === activeTab)?.label ?? 'Usage');

	// Timeliness stamp computed from the actual samples, never invented:
	// when the engine stops reporting, the stamp holds its breath.
	let stamp = $derived.by(() => {
		if (tuples.length < 2) return '';
		const first = tuples[0][0];
		const last = tuples[tuples.length - 1][0];
		const mins = Math.max(1, Math.round((last - first) / 60000));
		const d = new Date(last);
		const clock = `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
		return `as of ${clock} · last ${mins}m`;
	});
</script>

<!--
	kumo's categorical blue is identical in light and dark mode — one color
	serves both themes without a runtime palette switch. While no real sample
	has arrived yet the chart renders its built-in loading state.
-->
<LayerCard class={cn('flex flex-col p-5', className)}>
	<div class="mb-4 flex min-h-[26px] items-center justify-between gap-4">
		<h2 class="m-0 text-base font-semibold text-[var(--text-primary)]">Resource usage</h2>
		<Tabs {tabs} bind:active={activeTab} size="sm" />
	</div>
	<div class="mb-2 flex items-baseline gap-2.5">
		<strong class="text-2xl font-semibold text-[var(--text-primary)] tabular-nums">
			{chartMeta[activeTab]?.current ?? '—'}
		</strong>
		<small class="text-xs text-[var(--text-tertiary)]">currently {unit}</small>
		{#if stamp}
			<small class="ml-auto text-xs whitespace-nowrap text-[var(--text-tertiary)]">{stamp}</small>
		{/if}
	</div>
	<div class="mt-auto">
		<TimeseriesChart
			echarts={echarts}
			type="line"
			data={[{ name: seriesName, data: tuples, color: '#4290F0' }]}
			height={184}
			isDarkMode={ui.theme === 'dark'}
			gradient
			loading={tuples.length === 0}
			xAxisTickCount={4}
			xAxisTickFormat={timeseriesTimeTick}
			yAxisName={unit}
			ariaDescription="Resource usage over the recent sampling window"
		/>
	</div>
</LayerCard>
