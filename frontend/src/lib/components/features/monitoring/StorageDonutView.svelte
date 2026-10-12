<script lang="ts">
	import * as echarts from 'echarts';
	import { Chart, Button } from '$lib/components/ui';
	import { uiStore } from '$lib/stores/ui.svelte';
	import { CircleNotch, Trash } from 'phosphor-svelte';

	interface Props {
		storageCategories: any[];
		totalPodmanDisplay: string;
		totalReclaimableDisplay: string;
		totalPodmanNum: number;
		pruningTarget: string | null;
		onpruneCategory: (type: 'images' | 'containers' | 'volumes') => void;
	}

	let {
		storageCategories,
		totalPodmanDisplay,
		totalReclaimableDisplay,
		totalPodmanNum,
		pruningTarget,
		onpruneCategory
	}: Props = $props();

	let isDark = $derived(uiStore.theme === 'dark');

	let donutOptions = $derived({
		tooltip: {
			trigger: 'item' as const,
			backgroundColor: isDark ? '#18181b' : '#ffffff',
			borderColor: isDark ? '#27272a' : '#e4e4e7',
			textStyle: { color: isDark ? '#f4f4f5' : '#18181b', fontSize: 12 },
			formatter: (params: any) => {
				const cat = storageCategories.find(
					(c) => c.name.toLowerCase() === params.name.toLowerCase()
				);
				const reclaimText =
					cat?.reclaimableDisplay && cat.reclaimableDisplay !== '0 B'
						? `<div style="color:#10b981;margin-top:2px;">Reclaimable: <b>${cat.reclaimableDisplay}</b></div>`
						: '';
				return `<div style="font-weight:600;margin-bottom:4px;">${params.name}</div>
						<div>Allocated: <b>${cat?.sizeDisplay || params.value + ' GB'}</b> (${params.percent}%)</div>
						${reclaimText}`;
			}
		},
		series: [
			{
				name: 'Podman storage',
				type: 'pie' as const,
				radius: ['64%', '88%'],
				center: ['50%', '50%'],
				avoidLabelOverlap: false,
				itemStyle: {
					borderRadius: 4,
					borderColor: isDark ? '#13171f' : '#ffffff',
					borderWidth: 2
				},
				label: { show: false },
				emphasis: {
					scale: true,
					scaleSize: 4
				},
				data: storageCategories.map((c) => ({
					value: c.sizeGB || 0,
					name: c.name,
					itemStyle: { color: c.hexColor }
				}))
			}
		]
	});
</script>

<div class="flex flex-col items-center justify-around gap-6 md:flex-row">
	<!-- Native ECharts Donut with Center Summary -->
	<div class="relative flex h-[210px] w-[210px] shrink-0 items-center justify-center">
		<Chart
			{echarts}
			options={donutOptions}
			isDarkMode={isDark}
			height={210}
			class="h-[210px] w-[210px]"
		/>

		<!-- Donut Center Label -->
		<div
			class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center select-none"
		>
			<span class="font-mono text-xl font-semibold text-[var(--text-primary)]">
				{totalPodmanDisplay}
			</span>
			<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase">
				Allocated
			</span>
			<span class="mt-0.5 font-mono text-[10px] font-medium text-[var(--status-green)]">
				{totalReclaimableDisplay} freeable
			</span>
		</div>
	</div>

	<!-- Synchronized Subsystem Cards with Direct 1-Click Prune Actions -->
	<div class="grid w-full flex-1 grid-cols-1 gap-3 sm:grid-cols-3">
		{#each storageCategories as cat}
			{@const pct = Math.round((cat.sizeGB / (totalPodmanNum || 1)) * 100)}
			<div
				class="flex flex-col justify-between gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3.5"
			>
				<!-- Card Header: Color Dot, Subsystem Name, Percentage -->
				<div class="flex items-center justify-between">
					<div class="flex items-center gap-2">
						<span class="h-2.5 w-2.5 rounded-full {cat.colorClass}"></span>
						<span class="text-xs font-semibold text-[var(--text-primary)]">{cat.name}</span>
					</div>
					<span class="font-mono text-xs font-semibold text-[var(--text-secondary)]">
						{pct}%
					</span>
				</div>

				<!-- Card Value: Allocated Size & Active Status -->
				<div class="flex items-baseline justify-between">
					<span class="font-mono text-base font-semibold text-[var(--text-primary)]">
						{cat.sizeDisplay}
					</span>
					<span class="font-mono text-[11px] text-[var(--text-tertiary)]">
						{cat.totalCount} items
					</span>
				</div>

				<!-- Reclaimable Space & Inline Direct Prune Trigger -->
				<div
					class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-2 text-xs"
				>
					<div class="flex flex-col">
						<span class="text-[10px] text-[var(--text-tertiary)]">Reclaimable</span>
						<span
							class="font-mono text-xs font-medium {cat.rawReclaimable > 0
								? 'text-[var(--status-green)]'
								: 'text-[var(--text-tertiary)]'}"
						>
							{cat.reclaimableDisplay}
						</span>
					</div>

					<Button
						variant="secondary"
						size="xs"
						disabled={pruningTarget !== null || cat.rawReclaimable === 0}
						onclick={() => onpruneCategory(cat.id as 'images' | 'containers' | 'volumes')}
						aria-label={`Prune unreferenced ${cat.viewLabel}`}
					>
						{#if pruningTarget === cat.id}
							<CircleNotch size={11} class="animate-spin" />
							<span>Pruning</span>
						{:else}
							<Trash size={11} />
							<span>Prune</span>
						{/if}
					</Button>
				</div>
			</div>
		{/each}
	</div>
</div>
