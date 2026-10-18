<script lang="ts">
	import { Input, Select, CopyButton } from '$lib/components/ui';
	import { Switch } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { SlidersHorizontal, CaretDown } from 'phosphor-svelte';

	interface Props {
		showAdvanced: boolean;
		runtimeTarget: 'standalone' | 'pod' | 'quadlet';
		podName: string;
		cpuLimit: string;
		memLimit: string;
		autoDeploy: boolean;
		watchPaths: string;
		webhookUrl: string;
	}

	let {
		showAdvanced = $bindable(),
		runtimeTarget = $bindable(),
		podName = $bindable(),
		cpuLimit = $bindable(),
		memLimit = $bindable(),
		autoDeploy = $bindable(),
		watchPaths = $bindable(),
		webhookUrl
	}: Props = $props();
</script>

<div class="flex flex-col border-t border-kumo-hairline pt-3">
	<button
		type="button"
		onclick={() => (showAdvanced = !showAdvanced)}
		class="flex w-full cursor-pointer items-center justify-between rounded-md py-2 text-xs font-medium text-kumo-subtle hover:text-kumo-default"
	>
		<div class="flex items-center gap-2">
			<SlidersHorizontal size={14} />
			<span>Advanced options</span>
			<span class="text-[11px] font-normal text-kumo-subtle">
				(Resource limits, auto-deploy triggers, runtime target)
			</span>
		</div>
		<CaretDown
			size={14}
			class="transition-transform duration-150 {showAdvanced ? 'rotate-180' : ''}"
		/>
	</button>

	{#if showAdvanced}
		<div class="flex flex-col gap-5 pt-3 pb-1">
			<!-- 1. Runtime Execution Target -->
			<div class="bg-kumo-surface flex flex-col gap-2 rounded-lg border border-kumo-line p-3.5">
				<span class="text-xs font-medium text-kumo-default">Runtime execution mode</span>
				<div class="flex flex-wrap items-center gap-2">
					<button
						type="button"
						onclick={() => (runtimeTarget = 'standalone')}
						class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
						'standalone'
							? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
							: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
					>
						Standalone container
					</button>
					<button
						type="button"
						onclick={() => (runtimeTarget = 'pod')}
						class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
						'pod'
							? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
							: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
					>
						Podman pod
					</button>
					<button
						type="button"
						onclick={() => (runtimeTarget = 'quadlet')}
						class="cursor-pointer rounded-sm border px-3 py-1.5 text-xs transition-colors {runtimeTarget ===
						'quadlet'
							? 'border-kumo-line bg-kumo-base font-medium text-kumo-default shadow-xs'
							: 'border-transparent text-kumo-subtle hover:text-kumo-default'}"
					>
						Systemd Quadlet
					</button>
				</div>

				{#if runtimeTarget === 'pod'}
					<div class="flex items-center gap-3 pt-2">
						<span class="text-xs text-kumo-subtle">Target pod:</span>
						<Select
							bind:value={podName}
							options={dataStore.pods.map((p) => ({ value: p.name, label: p.name }))}
							class="h-8 w-52 text-xs"
						/>
					</div>
				{/if}
			</div>

			<!-- 2. Resource Limits (cgroups v2) -->
			<div class="bg-kumo-surface flex flex-col gap-2 rounded-lg border border-kumo-line p-3.5">
				<span class="text-xs font-medium text-kumo-default">Resource limits (cgroups v2)</span>
				<div class="flex flex-wrap items-center gap-4">
					<div class="flex w-40 flex-col gap-1">
						<label for="cpu-limit" class="text-[11px] text-kumo-subtle">CPU limit (cores)</label>
						<Input
							id="cpu-limit"
							bind:value={cpuLimit}
							placeholder="e.g. 1.0 (unlimited)"
							class="h-8 font-mono text-xs"
						/>
					</div>

					<div class="flex w-40 flex-col gap-1">
						<label for="mem-limit" class="text-[11px] text-kumo-subtle">Memory limit</label>
						<Input
							id="mem-limit"
							bind:value={memLimit}
							placeholder="e.g. 512MB, 1GB"
							class="h-8 font-mono text-xs"
						/>
					</div>
				</div>
			</div>

			<!-- 3. Deployment Triggers & Webhooks -->
			<div class="bg-kumo-surface flex flex-col gap-3 rounded-lg border border-kumo-line p-3.5">
				<span class="text-xs font-medium text-kumo-default">Deployment triggers</span>

				<div class="flex max-w-md items-center justify-between gap-4">
					<div class="flex flex-col gap-0.5">
						<span class="text-xs font-medium text-kumo-default">Auto-deploy on git push</span>
						<span class="text-[11px] text-kumo-subtle"
							>Roll out updates when new commits arrive.</span
						>
					</div>
					<Switch bind:checked={autoDeploy} size="sm" />
				</div>

				<div class="flex max-w-md flex-col gap-1 pt-1">
					<label for="watch-paths" class="text-[11px] text-kumo-subtle">
						Watch paths (optional)
					</label>
					<Input
						id="watch-paths"
						bind:value={watchPaths}
						placeholder="src/**, packages/app/**"
						class="h-8 font-mono text-xs"
					/>
				</div>

				<div class="flex max-w-md flex-col gap-1 pt-1">
					<span class="text-[11px] text-kumo-subtle">Webhook endpoint</span>
					<div class="flex items-center gap-2">
						<Input
							value={webhookUrl}
							readonly
							placeholder="Loading webhook token…"
							class="h-8 flex-1 bg-kumo-base font-mono text-xs text-kumo-subtle"
						/>
						{#if webhookUrl}
							<CopyButton text={webhookUrl} />
						{/if}
					</div>
				</div>
			</div>
		</div>
	{/if}
</div>
