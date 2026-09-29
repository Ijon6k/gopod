<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input, SectionCard } from '$lib/components/primitives';
	import { CodeEditor } from '$lib/components/ui';
	import { DownloadSimple, Copy, Check, Plus, Trash } from 'phosphor-svelte';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	// Section collapse states
	let openSections = $state<Record<string, boolean>>({
		runtime: true,
		lifecycle: true,
		security: true,
		storage: true,
		resources: true,
		generated: true
	});

	function toggleSection(sec: string) {
		openSections[sec] = !openSections[sec];
	}

	// Runtime state
	let runtimeMode = $state<'rootless' | 'rootful'>('rootless');
	let userNamespace = $state('keep-id');
	let devices = $state('');

	// Lifecycle state
	let quadletEnabled = $state(true);
	let systemdUnit = $state('');
	let restartPolicy = $state('always');

	// Security state
	let isPrivileged = $state(false);
	let selinuxLabel = $state('container_file_t');
	let capAdd = $state('NET_BIND_SERVICE');
	let capDrop = $state('ALL');
	let noNewPrivileges = $state(true);

	// Storage state
	let volumes = $state<{ source: string; target: string; options: string }[]>([]);

	function addVolume() {
		volumes.push({
			source: `${service.name}_vol_${volumes.length + 1}`,
			target: '/data',
			options: 'Z'
		});
	}

	function removeVolume(idx: number) {
		volumes.splice(idx, 1);
	}

	// Resources state
	let cpuLimit = $state('2.0');
	let memoryLimit = $state('512MB');
	let pidsLimit = $state('2048');
	let swapLimit = $state('0');

	$effect(() => {
		runtimeMode = service.advanced?.runtime.mode ?? 'rootless';
		userNamespace = service.advanced?.runtime.userNamespace ?? 'keep-id';
		devices = service.advanced?.runtime.devices.join(', ') ?? '';
		quadletEnabled = service.advanced?.lifecycle.quadletEnabled ?? true;
		systemdUnit = service.advanced?.lifecycle.systemdUnitName ?? `${service.name}.service`;
		restartPolicy = service.restartPolicy || 'always';
		isPrivileged = service.advanced?.security.privileged ?? false;
		selinuxLabel = service.advanced?.security.selinuxLabel ?? 'container_file_t';
		capAdd = service.advanced?.security.capAdd.join(', ') ?? 'NET_BIND_SERVICE';
		capDrop = service.advanced?.security.capDrop.join(', ') ?? 'ALL';
		noNewPrivileges = service.advanced?.security.noNewPrivileges ?? true;
		volumes = service.advanced?.storage.volumes ?? [
			{ source: `${service.name}_data`, target: '/app/data', options: 'Z' }
		];
		cpuLimit = service.advanced?.resources.cpuLimit ?? '2.0';
		memoryLimit = service.advanced?.resources.memoryLimit ?? '512MB';
		pidsLimit = String(service.advanced?.resources.pidsLimit ?? 2048);
		swapLimit = service.advanced?.resources.swapLimit ?? '0';
	});

	// Generated configuration tab
	let genTab = $state<'quadlet' | 'kubernetes' | 'podman'>('quadlet');

	let generatedQuadlet = $derived(`[Unit]
Description=GoPod ${service.name} Service
After=network-online.target

[Container]
Image=${service.image || service.source || 'localhost/' + service.name + ':latest'}
ContainerName=${service.projectId}-${service.name}
PublishPort=${service.port || 3000}:${service.port || 3000}
AutoUpdate=registry
UserNS=${userNamespace}
Restart=${restartPolicy}
${volumes.map((v) => `Volume=${v.source}:${v.target}:${v.options}`).join('\n')}

[Service]
Restart=${restartPolicy}
TimeoutStartSec=300

[Install]
WantedBy=default.target`);

	let generatedKube = $derived(`apiVersion: v1
kind: Pod
metadata:
  name: ${service.projectId}-${service.name}
  labels:
    app.kubernetes.io/name: ${service.name}
    gopod.io/project: ${service.projectId}
spec:
  restartPolicy: Always
  containers:
    - name: ${service.name}
      image: ${service.image || 'localhost/' + service.name + ':latest'}
      ports:
        - containerPort: ${service.port || 3000}
      resources:
        limits:
          cpu: "${cpuLimit}"
          memory: "${memoryLimit}"
      securityContext:
        privileged: ${isPrivileged}
        allowPrivilegeEscalation: ${!noNewPrivileges}`);

	let generatedPodmanRun = $derived(`podman run -d \\
  --name ${service.projectId}-${service.name} \\
  --userns ${userNamespace} \\
  --restart ${restartPolicy} \\
  -p ${service.port || 3000}:${service.port || 3000} \\
  --cpus ${cpuLimit} \\
  --memory ${memoryLimit} \\
  --pids-limit ${pidsLimit} \\
  ${volumes.map((v) => `-v ${v.source}:${v.target}:${v.options} \\`).join('\n  ')}
  ${service.image || 'localhost/' + service.name + ':latest'}`);

	let currentGenCode = $derived(
		genTab === 'quadlet'
			? generatedQuadlet
			: genTab === 'kubernetes'
				? generatedKube
				: generatedPodmanRun
	);

	function downloadConfig() {
		const ext = genTab === 'quadlet' ? 'container' : genTab === 'kubernetes' ? 'yaml' : 'sh';
		const blob = new Blob([currentGenCode], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${service.name}.${ext}`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<div class="w-full flex flex-col gap-5">
	<!-- ══════════════════════════════════════════════════════════════
	     1. RUNTIME CONFIGURATION
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Runtime Execution"
		description="Rootless execution isolation, user namespace mappings, and host devices."
		collapsible={true}
		bind:open={openSections.runtime}
	>
		<!-- Rootless / Rootful (Rootless is default; neither labeled "Recommended") -->
		<div class="flex flex-col gap-1.5 pt-2">
			<span class="text-xs text-[var(--text-secondary)] font-medium">Podman Mode</span>
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
				<label
					class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border cursor-pointer transition-colors {runtimeMode === 'rootless'
						? 'bg-[var(--bg-surface)] border-[var(--accent)]'
						: 'bg-[var(--bg-panel)] border-[var(--border)]'}"
				>
					<input
						type="radio"
						bind:group={runtimeMode}
						value="rootless"
						class="mt-0.5 accent-[var(--accent)]"
					/>
					<div class="flex flex-col gap-0.5">
						<span class="text-xs font-medium text-[var(--text-primary)]">Rootless (Default)</span>
						<span class="text-[11px] text-[var(--text-tertiary)] leading-relaxed">
							Runs entirely in unprivileged user space without root privileges. Standard for PaaS workloads.
						</span>
					</div>
				</label>

				<label
					class="flex items-start gap-3 p-3 rounded-[var(--radius-sm)] border cursor-pointer transition-colors {runtimeMode === 'rootful'
						? 'bg-[var(--bg-surface)] border-[var(--accent)]'
						: 'bg-[var(--bg-panel)] border-[var(--border)]'}"
				>
					<input
						type="radio"
						bind:group={runtimeMode}
						value="rootful"
						class="mt-0.5 accent-[var(--accent)]"
					/>
					<div class="flex flex-col gap-0.5">
						<span class="text-xs font-medium text-[var(--text-primary)]">Rootful</span>
						<span class="text-[11px] text-[var(--text-tertiary)] leading-relaxed">
							Runs via system daemon with root permissions. Required for low privileged system ports &lt;1024 or direct raw devices.
						</span>
					</div>
				</label>
			</div>
		</div>

		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
			<div class="flex flex-col gap-1.5">
				<label for="rt-userns" class="text-xs text-[var(--text-secondary)] font-medium">User Namespace</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="rt-userns" bind:value={userNamespace} placeholder="keep-id" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="rt-devices" class="text-xs text-[var(--text-secondary)] font-medium">Host Devices</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="rt-devices" bind:value={devices} placeholder="/dev/fuse, /dev/net/tun" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>
		</div>
	</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     2. LIFECYCLE & SYSTEMD
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Lifecycle & Systemd"
		description="Quadlet unit registration, systemd supervisor integration, and restart policies."
		collapsible={true}
		bind:open={openSections.lifecycle}
	>
		<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
			<div class="flex items-start justify-between p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]">
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-medium text-[var(--text-primary)]">Systemd Quadlet Supervisor</span>
					<span class="text-[11px] text-[var(--text-tertiary)]">
						Automatically generates a systemd user service unit for process supervision.
					</span>
				</div>
				<input type="checkbox" bind:checked={quadletEnabled} class="mt-1 accent-[var(--accent)]" />
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="lc-restart" class="text-xs text-[var(--text-secondary)] font-medium">Restart Policy</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<select
						id="lc-restart"
						bind:value={restartPolicy}
						class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
					>
						<option value="always">always</option>
						<option value="on-failure">on-failure</option>
						<option value="unless-stopped">unless-stopped</option>
						<option value="no">no</option>
					</select>
				</div>
			</div>
		</div>
	</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     3. SECURITY & CAPABILITIES
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Security & Capabilities"
		description="SELinux confinement, Linux kernel capabilities, and privilege boundaries."
		collapsible={true}
		bind:open={openSections.security}
	>
		<div class="flex items-start justify-between p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] mt-2">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">Privileged Container Mode</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Disables SELinux isolation and grants access to all host kernel devices. Keep disabled for standard PaaS apps.
				</span>
			</div>
			<input type="checkbox" bind:checked={isPrivileged} class="mt-1 accent-[var(--status-red)]" />
		</div>

		<div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
			<div class="flex flex-col gap-1.5">
				<label for="sec-selinux" class="text-xs text-[var(--text-secondary)] font-medium">SELinux Label</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="sec-selinux" bind:value={selinuxLabel} placeholder="container_file_t" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="sec-cap-add" class="text-xs text-[var(--text-secondary)] font-medium">Add Capabilities</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="sec-cap-add" bind:value={capAdd} placeholder="NET_BIND_SERVICE" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="sec-cap-drop" class="text-xs text-[var(--text-secondary)] font-medium">Drop Capabilities</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="sec-cap-drop" bind:value={capDrop} placeholder="ALL" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>
		</div>
	</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     4. STORAGE & MOUNTS
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Storage & Volumes"
		description="Persistent named volumes, host bind mounts, and SELinux volume options."
		collapsible={true}
		bind:open={openSections.storage}
	>
		<div class="flex items-center justify-between pt-1">
			<span class="text-xs text-[var(--text-secondary)] font-medium">Configured Volume Mounts</span>
			<Button variant="secondary" size="sm" onclick={addVolume}>
				<Plus size={13} /> Add Mount
			</Button>
		</div>

		<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] overflow-hidden">
			{#each volumes as vol, idx}
				<div class="p-3 grid grid-cols-1 sm:grid-cols-3 gap-3 items-center">
					<div class="flex flex-col gap-1">
						<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Source / Volume</span>
						<Input bind:value={vol.source} placeholder="volume_name" class="font-[var(--font-mono)] text-xs" />
					</div>
					<div class="flex flex-col gap-1">
						<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Container Mount</span>
						<Input bind:value={vol.target} placeholder="/data" class="font-[var(--font-mono)] text-xs" />
					</div>
					<div class="flex items-center gap-2">
						<div class="flex flex-col gap-1 flex-1">
							<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Flags</span>
							<Input bind:value={vol.options} placeholder="Z" class="font-[var(--font-mono)] text-xs" />
						</div>
						<button
							type="button"
							onclick={() => removeVolume(idx)}
							class="mt-4 text-[var(--text-tertiary)] hover:text-[var(--status-red)] p-1.5 bg-transparent border-0 cursor-pointer"
						>
							<Trash size={14} />
						</button>
					</div>
				</div>
			{/each}
		</div>
	</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     5. RESOURCE LIMITS
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Resource Limits"
		description="Cgroups v2 limits for CPU cores, memory limits, and maximum process IDs."
		collapsible={true}
		bind:open={openSections.resources}
	>
		<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-2">
			<div class="flex flex-col gap-1.5">
				<label for="res-cpu" class="text-xs text-[var(--text-secondary)] font-medium">CPU Quota</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="res-cpu" bind:value={cpuLimit} placeholder="2.0" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-mem" class="text-xs text-[var(--text-secondary)] font-medium">Memory Limit</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="res-mem" bind:value={memoryLimit} placeholder="512MB" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-pids" class="text-xs text-[var(--text-secondary)] font-medium">PIDs Limit</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="res-pids" bind:value={pidsLimit} placeholder="2048" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-swap" class="text-xs text-[var(--text-secondary)] font-medium">Swap Limit</label>
				<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
					<Input id="res-swap" bind:value={swapLimit} placeholder="0" class="font-[var(--font-mono)] text-xs" />
				</div>
			</div>
		</div>
	</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     6. GENERATED CONFIGURATION (Quadlet / Kube / Podman)
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard
		title="Generated Configuration"
		description="Export generated Quadlet unit, Kubernetes YAML, or standalone Podman run command."
		collapsible={true}
		bind:open={openSections.generated}
	>
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pt-2">
			<!-- Format selector tabs -->
			<div class="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]">
				<button
					type="button"
					onclick={() => (genTab = 'quadlet')}
					class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer {genTab === 'quadlet'
						? 'bg-[var(--bg-panel)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Quadlet (.container)
				</button>
				<button
					type="button"
					onclick={() => (genTab = 'kubernetes')}
					class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer {genTab === 'kubernetes'
						? 'bg-[var(--bg-panel)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Kubernetes YAML
				</button>
				<button
					type="button"
					onclick={() => (genTab = 'podman')}
					class="px-2.5 py-1 rounded text-xs transition-colors border-0 cursor-pointer {genTab === 'podman'
						? 'bg-[var(--bg-panel)] text-[var(--text-primary)] font-medium'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Podman Run CLI
				</button>
			</div>

			<Button variant="secondary" size="sm" onclick={downloadConfig}>
				<DownloadSimple size={13} /> Download
			</Button>
		</div>

		<CodeEditor
			value={currentGenCode}
			readOnly={true}
			language={genTab === 'quadlet' ? 'quadlet' : genTab === 'kubernetes' ? 'yaml' : 'text'}
			height="auto"
		/>
	</SectionCard>
</div>
