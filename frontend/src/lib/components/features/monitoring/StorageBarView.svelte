<script lang="ts">
	import * as echarts from 'echarts';
	import { Chart, Button } from '$lib/components/ui';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { formatBytes } from '$lib/utils/format';
	import { CircleNotch, Trash } from 'phosphor-svelte';

	interface Props {
		storageCategories: any[];
		totalPodmanNum: number;
		pruningTarget: string | null;
		onpruneCategory: (type: 'images' | 'containers' | 'volumes') => void;
	}

	let { storageCategories, totalPodmanNum, pruningTarget, onpruneCategory }: Props = $props();

	let isDark = $derived(uiStore.theme === 'dark');

	let barChartOptions = $derived.by(() => {
		const cats = [...storageCategories].reverse();
		const categoryNames = cats.map((c) => c.name);

		const inUseGB = cats.map((c) => Math.max(0, c.sizeGB - c.reclaimableGB));
		const reclaimableGB = cats.map((c) => c.reclaimableGB);

		const textColor = isDark ? '#a1a1aa' : '#52525b';
		const titleColor = isDark ? '#f4f4f5' : '#18181b';
		const splitLineColor = isDark ? '#27272a' : '#f0f1f4';
		const axisLineColor = isDark ? '#3f3f46' : '#d4d4d8';
		const tooltipBg = isDark ? '#18181b' : '#ffffff';
		const tooltipBorder = isDark ? '#27272a' : '#e4e4e7';
		const tooltipText = isDark ? '#f4f4f5' : '#18181b';

		return {
			tooltip: {
				trigger: 'axis' as const,
				axisPointer: { type: 'shadow' as const },
				backgroundColor: tooltipBg,
				borderColor: tooltipBorder,
				borderWidth: 1,
				textStyle: { color: tooltipText, fontSize: 12 },
				formatter: (params: any) => {
					if (!Array.isArray(params) || params.length === 0) return '';
					const catName = params[0].name;
					const cat = storageCategories.find((c) => c.name === catName);
					return `<div style="font-weight:600;margin-bottom:4px;color:${tooltipText};">${catName}</div>
							<div>Total: <b>${cat?.sizeDisplay || '0 B'}</b> (${cat?.totalCount || 0} items)</div>
							<div>In use: <b>${formatBytes(Math.max(0, (cat?.rawSize || 0) - (cat?.rawReclaimable || 0)))}</b></div>
							<div style="color:#10b981;">Reclaimable: <b>${cat?.reclaimableDisplay || '0 B'}</b></div>`;
				}
			},
			legend: {
				data: ['In use', 'Reclaimable'],
				textStyle: { color: textColor, fontSize: 12 },
				top: 0,
				right: 12
			},
			grid: {
				left: '3%',
				right: '8%',
				bottom: '6%',
				top: '14%',
				containLabel: true
			},
			xAxis: {
				type: 'value' as const,
				axisLabel: {
					color: textColor,
					formatter: '{value} GB'
				},
				splitLine: {
					lineStyle: { color: splitLineColor, type: 'dashed' as const }
				}
			},
			yAxis: {
				type: 'category' as const,
				data: categoryNames,
				axisLabel: {
					color: titleColor,
					fontWeight: 500
				},
				axisLine: { lineStyle: { color: axisLineColor } }
			},
			series: [
				{
					name: 'In use',
					type: 'bar' as const,
					stack: 'total',
					barWidth: 26,
					itemStyle: {
						borderRadius: [0, 0, 0, 0],
						color: (params: any) => {
							const matched = cats[params.dataIndex];
							return matched ? matched.hexColor : '#6366f1';
						}
					},
					data: inUseGB
				},
				{
					name: 'Reclaimable',
					type: 'bar' as const,
					stack: 'total',
					barWidth: 26,
					itemStyle: {
						borderRadius: [0, 4, 4, 0],
						color: '#22c55e'
					},
					data: reclaimableGB
				}
			]
		};
	});
</script>

<div class="flex flex-col gap-5">
	<!-- Generous ECharts Horizontal Comparative Bar Chart -->
	<div class="w-full">
		<Chart {echarts} options={barChartOptions} isDarkMode={isDark} height={240} class="w-full" />
	</div>

	<!-- Subsystem Metric Cards with Inline Prune Triggers -->
	<div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
		{#each storageCategories as cat}
			{@const pct = Math.round((cat.sizeGB / (totalPodmanNum || 1)) * 100)}
			<div
				class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3 text-xs"
			>
				<div class="flex items-center gap-2.5">
					<span class="h-2.5 w-2.5 rounded-full {cat.colorClass}"></span>
					<div class="flex flex-col">
						<span class="font-medium text-[var(--text-primary)]">{cat.name}</span>
						<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
							{cat.sizeDisplay} ({pct}%)
						</span>
					</div>
				</div>

				<div class="flex items-center gap-2">
					<span
						class="font-mono text-[11px] {cat.rawReclaimable > 0
							? 'text-[var(--status-green)]'
							: 'text-[var(--text-tertiary)]'}"
					>
						~{cat.reclaimableDisplay}
					</span>

					<Button
						variant="secondary"
						size="xs"
						disabled={pruningTarget !== null || cat.rawReclaimable === 0}
						onclick={() => onpruneCategory(cat.id as 'images' | 'containers' | 'volumes')}
						aria-label={`Prune ${cat.viewLabel}`}
					>
						{#if pruningTarget === cat.id}
							<CircleNotch size={11} class="animate-spin" />
						{:else}
							<Trash size={11} />
						{/if}
					</Button>
				</div>
			</div>
		{/each}
	</div>
</div>
