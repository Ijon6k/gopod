<script lang="ts">
	import { Button, Select, SearchInput } from '$lib/components/ui';
	import { SlidersHorizontal } from 'phosphor-svelte';

	interface Props {
		searchQuery: string;
		selectedHost: string;
		selectedProjectId: string;
		selectedServiceId: string;
		selectedStatusGroup: string;
		selectedMethod: string;
		retentionDays: number;
		isRetentionModalOpen: boolean;
		uniqueHosts: string[];
		projects: { id: string; name: string }[];
		services: { id: string; name: string; projectId: string }[];
		onResetFilters: () => void;
	}

	let {
		searchQuery = $bindable(),
		selectedHost = $bindable(),
		selectedProjectId = $bindable(),
		selectedServiceId = $bindable(),
		selectedStatusGroup = $bindable(),
		selectedMethod = $bindable(),
		retentionDays = $bindable(),
		isRetentionModalOpen = $bindable(),
		uniqueHosts,
		projects,
		services,
		onResetFilters
	}: Props = $props();

	let hasActiveFilters = $derived(
		Boolean(
			searchQuery ||
			selectedHost !== 'all' ||
			selectedProjectId !== 'all' ||
			selectedServiceId !== 'all' ||
			selectedStatusGroup !== 'all' ||
			selectedMethod !== 'all'
		)
	);
</script>

<div class="flex flex-col items-stretch justify-between gap-3 sm:flex-row sm:items-center">
	<div class="flex flex-1 flex-wrap items-center gap-2.5">
		<!-- Search query -->
		<div class="w-full sm:w-56">
			<SearchInput bind:value={searchQuery} placeholder="Filter path, IP, host…" />
		</div>

		<!-- Project Filter -->
		<div class="w-36">
			<Select
				value={selectedProjectId}
				onValueChange={(val) => {
					if (typeof val === 'string') selectedProjectId = val;
				}}
				options={[
					{ label: 'All projects', value: 'all' },
					...projects.map((p) => ({ label: p.name, value: p.id }))
				]}
				size="sm"
			/>
		</div>

		<!-- Service Filter -->
		<div class="w-36">
			<Select
				value={selectedServiceId}
				onValueChange={(val) => {
					if (typeof val === 'string') selectedServiceId = val;
				}}
				options={[
					{ label: 'All services', value: 'all' },
					...services
						.filter((s) => selectedProjectId === 'all' || s.projectId === selectedProjectId)
						.map((s) => ({ label: s.name, value: s.id }))
				]}
				size="sm"
			/>
		</div>

		<!-- Host select -->
		<div class="w-36">
			<Select
				value={selectedHost}
				onValueChange={(val) => {
					if (typeof val === 'string') selectedHost = val;
				}}
				options={[
					{ label: 'All hosts', value: 'all' },
					...uniqueHosts.filter((h) => h !== 'all').map((h) => ({ label: h, value: h }))
				]}
				size="sm"
			/>
		</div>

		<!-- Status category -->
		<div class="w-32">
			<Select
				value={selectedStatusGroup}
				onValueChange={(val) => {
					if (typeof val === 'string') selectedStatusGroup = val as any;
				}}
				options={[
					{ label: 'All statuses', value: 'all' },
					{ label: '2xx Success', value: '2xx' },
					{ label: '3xx Redirect', value: '3xx' },
					{ label: '4xx Client error', value: '4xx' },
					{ label: '5xx Server error', value: '5xx' }
				]}
				size="sm"
			/>
		</div>

		<!-- Method select -->
		<div class="w-28">
			<Select
				value={selectedMethod}
				onValueChange={(val) => {
					if (typeof val === 'string') selectedMethod = val;
				}}
				options={[
					{ label: 'All methods', value: 'all' },
					{ label: 'GET', value: 'GET' },
					{ label: 'POST', value: 'POST' },
					{ label: 'PUT', value: 'PUT' },
					{ label: 'DELETE', value: 'DELETE' },
					{ label: 'OPTIONS', value: 'OPTIONS' }
				]}
				size="sm"
			/>
		</div>

		{#if hasActiveFilters}
			<button
				type="button"
				onclick={onResetFilters}
				class="cursor-pointer border-0 bg-transparent p-0 text-xs text-[var(--accent)] hover:underline"
			>
				Reset filters
			</button>
		{/if}
	</div>

	<!-- Retention Setting Drawer Trigger -->
	<div class="flex shrink-0 items-center gap-2">
		<span class="text-xs text-[var(--text-tertiary)]">
			Retention: <strong class="font-medium text-[var(--text-secondary)]"
				>{retentionDays} days</strong
			>
		</span>
		<button
			type="button"
			onclick={() => (isRetentionModalOpen = !isRetentionModalOpen)}
			class="flex cursor-pointer items-center gap-1.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-shell)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-surface)] hover:text-[var(--text-primary)]"
			aria-label="Configure access log retention"
		>
			<SlidersHorizontal size={13} />
			<span>Log policy</span>
		</button>
	</div>
</div>

<!-- Retention Setting Dropdown / Inline Drawer -->
{#if isRetentionModalOpen}
	<div
		class="flex flex-col items-center justify-between gap-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] p-4 text-xs sm:flex-row"
	>
		<div class="flex flex-col gap-1">
			<span class="font-medium text-[var(--text-primary)]"
				>Caddy access log rotation & retention</span
			>
			<span class="text-xs text-[var(--text-tertiary)]">
				Caddy writes JSON telemetry into <code
					class="rounded bg-[var(--bg-surface)] px-1 py-0.5 font-mono text-[var(--text-primary)]"
					>/var/log/caddy/access.log</code
				>. Automatically rotated via roll_size 20MiB.
			</span>
		</div>
		<div class="flex items-center gap-2">
			<div class="w-48">
				<Select
					value={retentionDays}
					onValueChange={(val) => {
						if (typeof val === 'number') retentionDays = val;
					}}
					options={[
						{ label: '3 days', value: 3 },
						{ label: '7 days (recommended)', value: 7 },
						{ label: '14 days', value: 14 },
						{ label: '30 days', value: 30 }
					]}
					size="sm"
				/>
			</div>
			<Button variant="secondary" size="sm" onclick={() => (isRetentionModalOpen = false)}>
				Done
			</Button>
		</div>
	</div>
{/if}
