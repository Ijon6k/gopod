<script lang="ts">
	import {
		CheckCircle,
		WarningCircle,
		Warning,
		Package,
		HardDrives,
		CaretDown,
		CaretUp
	} from 'phosphor-svelte';
	import { Badge, Button } from 'kumo-svelte';
	import type { ManifestParseResult } from '$lib/parsers/manifest-engine';

	interface Props {
		parseResult: ManifestParseResult;
		expanded?: boolean;
		onToggleExpand?: () => void;
	}

	let { parseResult, expanded = $bindable(false), onToggleExpand }: Props = $props();

	function handleToggle() {
		expanded = !expanded;
		onToggleExpand?.();
	}
</script>

<div
	class="flex flex-col overflow-hidden rounded-md border border-[var(--border)] bg-[var(--bg-card)] shadow-2xs"
>
	<!-- Compact Summary Bar (Always Visible, Placed Directly Above Editor) -->
	<div
		class="flex flex-wrap items-center justify-between gap-2.5 px-3.5 py-2.5 text-xs text-[var(--text-secondary)]"
	>
		<!-- Left: Status Indicator -->
		<div class="flex min-w-0 items-center gap-2">
			{#if parseResult.valid}
				<span
					class="inline-flex shrink-0 items-center gap-1 rounded px-2 py-0.5 text-[11.5px] font-medium text-emerald-400 bg-emerald-500/10 ring-1 ring-emerald-500/20"
				>
					<CheckCircle size={13} weight="fill" />
					<span>Valid {parseResult.type === 'kubernetes' ? 'Pod manifest' : 'Compose stack'}</span>
				</span>
			{:else}
				<span
					class="inline-flex shrink-0 items-center gap-1 rounded px-2 py-0.5 text-[11.5px] font-medium text-rose-400 bg-rose-500/10 ring-1 ring-rose-500/20"
				>
					<WarningCircle size={13} weight="fill" />
					<span>Syntax error</span>
				</span>
			{/if}

			<!-- Workload & Volume Chips -->
			{#if parseResult.valid && parseResult.workloads.length > 0}
				<div class="hidden items-center gap-1.5 sm:flex">
					<span
						class="inline-flex items-center gap-1 rounded bg-[var(--bg-table-row-alt)] px-2 py-0.5 font-mono text-[11px] text-[var(--text-secondary)]"
					>
						<Package size={12} class="text-[var(--accent)]" />
						<span>{parseResult.workloads.length} {parseResult.workloads.length === 1 ? 'container' : 'containers'}</span>
					</span>

					{#if parseResult.volumes.length > 0}
						<span
							class="inline-flex items-center gap-1 rounded bg-[var(--bg-table-row-alt)] px-2 py-0.5 font-mono text-[11px] text-[var(--text-secondary)]"
						>
							<HardDrives size={12} />
							<span>{parseResult.volumes.length} {parseResult.volumes.length === 1 ? 'volume' : 'volumes'}</span>
						</span>
					{/if}
				</div>
			{/if}

			<!-- Inline Error / Warning snippet -->
			{#if !parseResult.valid && parseResult.errors.length > 0}
				<span class="truncate font-mono text-[11px] text-rose-400">
					{parseResult.errors[0]}
				</span>
			{:else if parseResult.warnings.length > 0}
				<span
					class="inline-flex max-w-[340px] items-center gap-1 truncate rounded px-1.5 py-0.5 text-[11px] font-medium text-amber-400 bg-amber-500/10 ring-1 ring-amber-500/20"
					title={parseResult.warnings.join(' • ')}
				>
					<Warning size={12} weight="fill" />
					<span class="truncate">{parseResult.warnings[0]}</span>
				</span>
			{/if}
		</div>

		<!-- Right: Toggle Inspector CTA -->
		{#if parseResult.valid && parseResult.workloads.length > 0}
			<Button
				variant="ghost"
				size="sm"
				onclick={handleToggle}
				class="h-7 gap-1 px-2 text-[11.5px] font-medium text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
			>
				<span>{expanded ? 'Hide preview' : 'Inspect workloads'}</span>
				<span
					class="rounded bg-[var(--bg-table-row-alt)] px-1.5 py-0.2 font-mono text-[10px] text-[var(--accent)]"
				>
					{parseResult.workloads.length}
				</span>
				{#if expanded}
					<CaretUp size={12} />
				{:else}
					<CaretDown size={12} />
				{/if}
			</Button>
		{/if}
	</div>

	<!-- Collapsible Workload Inspection Drawer -->
	{#if expanded && parseResult.valid && parseResult.workloads.length > 0}
		<div class="border-t border-[var(--border)] bg-[var(--bg-table-row-alt)] p-3">
			<div class="grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-3">
				{#each parseResult.workloads as w}
					<div
						class="flex flex-col gap-1.5 rounded border border-[var(--border)] bg-[var(--bg-card)] p-2.5 shadow-2xs"
					>
						<!-- Workload Name & Role -->
						<div class="flex items-center justify-between gap-1.5">
							<span class="truncate font-mono text-[12.5px] font-semibold text-[var(--text-primary)]">
								{w.name}
							</span>
							<Badge
								variant={w.role === 'main' ? 'primary' : w.role === 'sidecar' ? 'neutral' : 'neutral'}
								class="text-[10px] uppercase font-semibold tracking-wider"
							>
								{w.role}
							</Badge>
						</div>

						<!-- Image -->
						<div class="truncate text-[11px] text-[var(--text-tertiary)]" title={w.image}>
							<span class="text-[var(--text-muted)]">Image:</span>
							<span class="font-mono text-[var(--text-secondary)]">{w.image || '(none)'}</span>
						</div>

						<!-- Ports -->
						{#if w.ports.length > 0}
							<div class="flex flex-wrap items-center gap-1 pt-0.5">
								{#each w.ports as p}
									<span
										class="rounded bg-[var(--bg-table-row)] px-1.5 py-0.5 font-mono text-[10.5px] text-[var(--accent)] border border-[var(--border-subtle)]"
									>
										{p.raw}
									</span>
								{/each}
							</div>
						{/if}

						<!-- Volume Mounts -->
						{#if w.volumeMounts && w.volumeMounts.length > 0}
							<div class="truncate text-[10.5px] text-[var(--text-tertiary)]" title={w.volumeMounts.join(', ')}>
								<span class="text-[var(--text-muted)]">Mounts:</span>
								<span class="font-mono text-[var(--text-secondary)]">{w.volumeMounts[0]}</span>
							</div>
						{/if}
					</div>
				{/each}
			</div>

			<!-- Warnings list if any -->
			{#if parseResult.warnings.length > 0}
				<div class="mt-2.5 flex flex-col gap-1 rounded bg-amber-500/10 p-2 text-[11.5px] text-amber-300 ring-1 ring-amber-500/20">
					{#each parseResult.warnings as warn}
						<div class="flex items-start gap-1.5">
							<Warning size={13} weight="fill" class="shrink-0 mt-0.5 text-amber-400" />
							<span>{warn}</span>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{/if}
</div>
