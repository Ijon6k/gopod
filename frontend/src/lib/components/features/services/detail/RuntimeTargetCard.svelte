<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore } from '$lib/data';
	import { Cube, Stack, FileText, Check } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let runtimeTarget = $state<'standalone' | 'pod' | 'quadlet'>('standalone');
	let targetPort = $state('3000');
	let hostPort = $state('');
	let podName = $state('');
	let cpuLimit = $state('');
	let memLimit = $state('');
	let saveStatus = $state<'idle' | 'saved'>('idle');

	$effect(() => {
		runtimeTarget = service.runtimeTarget || (service.type === 'pod' ? 'pod' : service.type === 'quadlet' ? 'quadlet' : 'standalone');
		targetPort = String(service.port || 3000);
		hostPort = service.hostPort && service.hostPort > 0 ? String(service.hostPort) : '';
		podName = service.podId || (dataStore.pods[0]?.name ?? '');
		
		const existingCpu = service.cpuLimit || parseFloat(service.advanced?.resources?.cpuLimit || '0') || 0;
		cpuLimit = existingCpu > 0 ? String(existingCpu) : '';

		const existingMem = service.memoryLimit || 0;
		if (existingMem > 0) {
			memLimit = `${existingMem}MB`;
		} else if (service.advanced?.resources?.memoryLimit && service.advanced.resources.memoryLimit !== '0') {
			memLimit = service.advanced.resources.memoryLimit;
		} else {
			memLimit = '';
		}
	});

	function handleSave() {
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
				lifecycle: { quadletEnabled: runtimeTarget === 'quadlet', systemdUnitName: `${service.name}.container`, restartPolicy: 'always' },
				security: { privileged: false, selinuxLabel: 'container_file_t', capAdd: [], capDrop: [], noNewPrivileges: true },
				storage: { volumes: [] },
				resources: { cpuLimit: finalCpu > 0 ? String(finalCpu) : '0', memoryLimit: parsedMem > 0 ? `${parsedMem}MB` : '0', pidsLimit: 2048, swapLimit: '0' }
			};
		} else {
			service.advanced.lifecycle.quadletEnabled = runtimeTarget === 'quadlet';
			service.advanced.resources.cpuLimit = finalCpu > 0 ? String(finalCpu) : '0';
			service.advanced.resources.memoryLimit = parsedMem > 0 ? `${parsedMem}MB` : '0';
		}

		dataStore.updateService(service);
		saveStatus = 'saved';
		setTimeout(() => (saveStatus = 'idle'), 2000);
	}
</script>

<div class="rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] overflow-hidden flex flex-col">
	<!-- Card Header -->
	<div class="px-5 py-4 border-b border-[var(--border-subtle)] flex items-center justify-between">
		<div class="flex flex-col gap-0.5">
			<h3 class="text-sm font-semibold text-[var(--text-primary)] m-0">Podman Runtime & Lifecycle</h3>
			<p class="text-xs text-[var(--text-tertiary)] m-0">Configure how this workload executes inside the Podman engine.</p>
		</div>
	</div>

	<!-- Content -->
	<div class="p-5 flex flex-col gap-5">
		<!-- Runtime Target Mode -->
		<div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
			<button
				type="button"
				onclick={() => (runtimeTarget = 'standalone')}
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {runtimeTarget === 'standalone'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<Cube size={18} class={runtimeTarget === 'standalone' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Standalone Container</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Independent rootless OCI</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (runtimeTarget = 'pod')}
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {runtimeTarget === 'pod'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<Stack size={18} class={runtimeTarget === 'pod' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Inside Podman Pod</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Shared localhost network</span>
				</div>
			</button>

			<button
				type="button"
				onclick={() => (runtimeTarget = 'quadlet')}
				class="flex items-center gap-3 p-3 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-all {runtimeTarget === 'quadlet'
					? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)]'
					: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
			>
				<FileText size={18} class={runtimeTarget === 'quadlet' ? 'text-[var(--accent)]' : ''} />
				<div class="flex flex-col">
					<span class="text-xs font-medium">Systemd Quadlet</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Auto-start on OS boot</span>
				</div>
			</button>
		</div>

		{#if runtimeTarget === 'pod'}
			<div class="flex flex-col gap-1.5">
				<label for="rt-pod-name" class="text-xs font-medium text-[var(--text-secondary)]">Target Pod</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<select
						id="rt-pod-name"
						bind:value={podName}
						class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
					>
						{#each dataStore.pods as p (p.id || p.name)}
							<option value={p.name}>{p.name} ({p.projectName})</option>
						{/each}
						<option value={service.name + '-pod'}>+ Create New Pod ({service.name}-pod)</option>
					</select>
				</div>
			</div>
		{:else if runtimeTarget === 'quadlet'}
			<div class="p-3 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]/60 text-xs text-[var(--text-secondary)] leading-relaxed">
				Generates <code>~/.config/containers/systemd/{service.name}.container</code>. Systemd manages process supervision, log rotation, and restart automatically.
			</div>
		{/if}

		<!-- Resource Limits -->
		<div class="flex flex-col gap-3 pt-3 border-t border-[var(--border-subtle)]">
			<div class="flex items-center justify-between">
				<div class="flex flex-col">
					<span class="text-xs font-semibold text-[var(--text-primary)]">Resource Limits (Optional)</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">Leave empty for unlimited (recommended). Podman & Linux will dynamically schedule host resources.</span>
				</div>
				<div class="px-2 py-0.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-[10px] text-[var(--text-tertiary)] font-mono">
					{cpuLimit.trim() || memLimit.trim() ? 'Limits Active' : 'Unlimited (Default)'}
				</div>
			</div>

			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
				<div class="flex flex-col gap-1.5">
					<label for="rt-cpu-limit" class="text-xs font-medium text-[var(--text-secondary)] flex items-center justify-between">
						<span>CPU Limit (Cores)</span>
						<span class="text-[10px] text-[var(--text-tertiary)] font-normal">Default: Unlimited</span>
					</label>
					<Input
						id="rt-cpu-limit"
						bind:value={cpuLimit}
						placeholder="Unlimited (e.g. 1.0)"
						class="text-xs font-[var(--font-mono)]"
					/>
					<p class="text-[11px] text-[var(--text-tertiary)] m-0">
						Max CPU cores available to container (e.g. 0.5, 1.0, 2.0).
					</p>
				</div>

				<div class="flex flex-col gap-1.5">
					<label for="rt-mem-limit" class="text-xs font-medium text-[var(--text-secondary)] flex items-center justify-between">
						<span>Memory Limit</span>
						<span class="text-[10px] text-[var(--text-tertiary)] font-normal">Default: Unlimited</span>
					</label>
					<Input
						id="rt-mem-limit"
						bind:value={memLimit}
						placeholder="Unlimited (e.g. 512MB, 1GB)"
						class="text-xs font-[var(--font-mono)]"
					/>
					<p class="text-[11px] text-[var(--text-tertiary)] m-0">
						Max RAM before container is throttled or killed by OOM (e.g. 512MB, 2GB).
					</p>
				</div>
			</div>
		</div>
	</div>

	<!-- Explicit Card Footer -->
	<div class="px-5 py-3 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
		{#if saveStatus === 'saved'}
			<span class="text-xs text-[var(--status-green)] flex items-center gap-1 mr-2">
				<Check size={14} /> Saved
			</span>
		{/if}
		<Button variant="primary" size="sm" onclick={handleSave}>
			Save Runtime
		</Button>
	</div>
</div>
