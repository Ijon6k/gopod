<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input, Select } from '$lib/components/ui';
	import { LayerCard, Button, useKumoToastManager } from 'kumo-svelte';
	import { dataStore } from '$lib/data';
	import { Cube, Stack, FileText, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let runtimeTarget = $state<'standalone' | 'pod' | 'quadlet'>('standalone');
	let targetPort = $state('3000');
	let hostPort = $state('');
	let podName = $state('');
	let cpuLimit = $state('');
	let memLimit = $state('');
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		runtimeTarget =
			service.runtimeTarget ||
			(service.type === 'pod' ? 'pod' : service.type === 'quadlet' ? 'quadlet' : 'standalone');
		targetPort = String(service.port || 3000);
		hostPort = service.hostPort && service.hostPort > 0 ? String(service.hostPort) : '';
		podName = service.podId || (dataStore.pods[0]?.name ?? '');

		const existingCpu =
			service.cpuLimit || parseFloat(service.advanced?.resources?.cpuLimit || '0') || 0;
		cpuLimit = existingCpu > 0 ? String(existingCpu) : '';

		const existingMem = service.memoryLimit || 0;
		if (existingMem > 0) {
			memLimit = `${existingMem}MB`;
		} else if (
			service.advanced?.resources?.memoryLimit &&
			service.advanced.resources.memoryLimit !== '0'
		) {
			memLimit = service.advanced.resources.memoryLimit;
		} else {
			memLimit = '';
		}
	});

	async function handleSave() {
		const wasQuadlet = service.runtimeTarget === 'quadlet' || service.type === 'quadlet';
		service.runtimeTarget = runtimeTarget;
		service.inPod = runtimeTarget === 'pod';
		service.port = parseInt(targetPort, 10) || 3000;
		service.hostPort = parseInt(hostPort, 10) || 0;
		if (runtimeTarget === 'pod') {
			service.podId = podName;
		}

		// Parse CPU limit (0 = unlimited)
		const parsedCpu = parseFloat(cpuLimit.trim());
		const finalCpu = !isNaN(parsedCpu) && parsedCpu > 0 ? parsedCpu : 0;
		service.cpuLimit = finalCpu;

		// Parse Memory limit in MB (0 = unlimited)
		let parsedMem = 0;
		const trimmedMem = memLimit.trim().toLowerCase();
		if (trimmedMem.endsWith('gb') || trimmedMem.endsWith('g')) {
			parsedMem = Math.round((parseFloat(trimmedMem) || 0) * 1024);
		} else if (trimmedMem.endsWith('mb') || trimmedMem.endsWith('m')) {
			parsedMem = Math.round(parseFloat(trimmedMem) || 0);
		} else if (trimmedMem) {
			parsedMem = Math.round(parseFloat(trimmedMem) || 0);
		}
		service.memoryLimit = parsedMem;

		if (!service.advanced) {
			service.advanced = {
				runtime: { mode: 'rootless', userNamespace: 'keep-id', devices: [] },
				lifecycle: {
					quadletEnabled: runtimeTarget === 'quadlet',
					systemdUnitName: `${service.name}.container`,
					restartPolicy: 'always'
				},
				security: {
					privileged: false,
					selinuxLabel: 'container_file_t',
					capAdd: [],
					capDrop: [],
					noNewPrivileges: true
				},
				storage: { volumes: [] },
				resources: {
					cpuLimit: finalCpu > 0 ? String(finalCpu) : '0',
					memoryLimit: parsedMem > 0 ? `${parsedMem}MB` : '0',
					pidsLimit: 2048,
					swapLimit: '0'
				}
			};
		} else {
			service.advanced.lifecycle.quadletEnabled = runtimeTarget === 'quadlet';
			service.advanced.resources.cpuLimit = finalCpu > 0 ? String(finalCpu) : '0';
			service.advanced.resources.memoryLimit = parsedMem > 0 ? `${parsedMem}MB` : '0';
		}

		if (wasQuadlet && runtimeTarget !== 'quadlet') {
			try {
				await dataStore.demoteQuadlet(service.id);
			} catch (err) {
				console.warn('Demote quadlet during runtime target change:', err);
			}
		}

		if (runtimeTarget === 'pod' && podName) {
			try {
				await dataStore.groupIntoPod(
					service.id,
					podName,
					true,
					service.hostPort ? [`${service.hostPort}:${service.port || 80}`] : undefined
				);
			} catch (err) {
				console.warn('Group into pod during runtime save:', err);
			}
		}

		await dataStore.updateService(service);
		saveStatus = 'saved';
		toast.add({
			title: 'Runtime Config Saved',
			description: `Workload set to ${runtimeTarget} environment.`,
			variant: 'success'
		});
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<LayerCard class="w-full shadow-xs">
	<!-- Elevated Header (LayerCard.Secondary) with Title & Save Action -->
	<LayerCard.Secondary
		class="flex flex-wrap items-center justify-between gap-3 border-b border-kumo-hairline px-4 py-3"
	>
		<div class="flex items-center gap-2.5">
			<div
				class="flex size-7 shrink-0 items-center justify-center rounded-md bg-kumo-base text-[var(--accent)] ring ring-kumo-line"
			>
				<Cube size={15} weight="duotone" />
			</div>
			<div class="flex flex-col">
				<div class="flex items-center gap-2">
					<span class="text-xs font-semibold text-kumo-default">Runtime & Resources</span>
					<span
						class="rounded-sm bg-kumo-base px-1.5 py-0.5 font-mono text-[10px] text-kumo-subtle ring-1 ring-kumo-hairline"
					>
						{cpuLimit.trim() || memLimit.trim() ? 'Limits Active' : 'Unlimited (Dynamic)'}
					</span>
				</div>
				<span class="text-[11px] text-kumo-subtle">
					Podman container execution mode and cgroups v2 resource limits
				</span>
			</div>
		</div>

		<!-- Action in Header -->
		<div class="flex items-center gap-2">
			{#if saveStatus === 'saved'}
				<span class="flex items-center gap-1 font-mono text-xs text-kumo-success">
					<Check size={13} weight="bold" /> Saved
				</span>
			{/if}
			<Button
				variant="primary"
				size="sm"
				onclick={handleSave}
				class="gap-1.5 font-medium shadow-xs"
			>
				Save Runtime
			</Button>
		</div>
	</LayerCard.Secondary>

	<!-- Content Body (LayerCard.Primary) -->
	<LayerCard.Primary class="flex flex-col gap-4 p-4">
		<!-- Compact Segmented Selector (Under 36px) -->
		<div
			class="inline-flex rounded-md bg-kumo-elevated p-1 ring-1 ring-kumo-hairline"
			role="tablist"
			aria-label="Runtime Target"
		>
			<button
				type="button"
				role="tab"
				aria-selected={runtimeTarget === 'standalone'}
				onclick={() => (runtimeTarget = 'standalone')}
				class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {runtimeTarget ===
				'standalone'
					? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
					: 'text-kumo-subtle hover:text-kumo-default'}"
			>
				<Cube size={14} class={runtimeTarget === 'standalone' ? 'text-[var(--accent)]' : ''} />
				<span>Standalone Container</span>
			</button>

			<button
				type="button"
				role="tab"
				aria-selected={runtimeTarget === 'pod'}
				onclick={() => (runtimeTarget = 'pod')}
				class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {runtimeTarget ===
				'pod'
					? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
					: 'text-kumo-subtle hover:text-kumo-default'}"
			>
				<Stack size={14} class={runtimeTarget === 'pod' ? 'text-[var(--accent)]' : ''} />
				<span>Podman Pod (Shared Net)</span>
			</button>

			<button
				type="button"
				role="tab"
				aria-selected={runtimeTarget === 'quadlet'}
				onclick={() => (runtimeTarget = 'quadlet')}
				class="flex cursor-pointer items-center gap-1.5 rounded-sm px-3 py-1 text-xs font-medium transition-all {runtimeTarget ===
				'quadlet'
					? 'bg-kumo-base text-kumo-default shadow-xs ring-1 ring-kumo-line'
					: 'text-kumo-subtle hover:text-kumo-default'}"
			>
				<FileText size={14} class={runtimeTarget === 'quadlet' ? 'text-[var(--accent)]' : ''} />
				<span>Systemd Quadlet</span>
			</button>
		</div>

		<!-- Conditional Mode Specific Info -->
		{#if runtimeTarget === 'pod'}
			<div class="flex flex-col gap-1 rounded-md bg-kumo-elevated p-3 ring-1 ring-kumo-hairline">
				<Select
					id="rt-pod-name"
					label="Target Pod"
					value={podName}
					onValueChange={(val) => {
						if (typeof val === 'string') podName = val;
					}}
					options={[
						...dataStore.pods.map((p) => ({
							label: `${p.name} (${p.projectName})`,
							value: p.name
						})),
						{ label: `+ Create New Pod (${service.name}-pod)`, value: service.name + '-pod' }
					]}
					size="sm"
				/>
			</div>
		{:else if runtimeTarget === 'quadlet'}
			<div
				class="rounded-md bg-kumo-elevated px-3 py-2 text-xs text-kumo-subtle ring-1 ring-kumo-hairline"
			>
				Systemd manages this service directly via <code class="font-mono text-[var(--accent)]"
					>~/.config/containers/systemd/{service.name}.container</code
				> with automatic restart on host boot.
			</div>
		{/if}

		<!-- Resource Limits (cgroups v2) in compact 2-column layout -->
		<div class="flex flex-col gap-2 pt-1">
			<span class="text-xs font-semibold text-kumo-default">Resource Limits (Optional)</span>

			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<div class="flex flex-col gap-1">
					<label
						for="rt-cpu-limit"
						class="flex items-center justify-between text-xs font-medium text-kumo-default"
					>
						<span>CPU Limit (Cores)</span>
						<span class="text-[10px] text-kumo-subtle font-mono">e.g. 1.0, 2.0</span>
					</label>
					<Input
						id="rt-cpu-limit"
						bind:value={cpuLimit}
						placeholder="Unlimited"
						class="text-xs font-mono"
					/>
					<p class="m-0 text-[11px] text-kumo-subtle">
						Max CPU cores available to container. Leave empty for dynamic host scheduling.
					</p>
				</div>

				<div class="flex flex-col gap-1">
					<label
						for="rt-mem-limit"
						class="flex items-center justify-between text-xs font-medium text-kumo-default"
					>
						<span>Memory Limit</span>
						<span class="text-[10px] text-kumo-subtle font-mono">e.g. 512MB, 1GB</span>
					</label>
					<Input
						id="rt-mem-limit"
						bind:value={memLimit}
						placeholder="Unlimited"
						class="text-xs font-mono"
					/>
					<p class="m-0 text-[11px] text-kumo-subtle">
						Max RAM before throttling or OOM termination. Leave empty for unlimited.
					</p>
				</div>
			</div>
		</div>
	</LayerCard.Primary>
</LayerCard>
