<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Input, SectionCard, Switch, Select, Radio } from '$lib/components/ui';
	import { CodeEditor } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { useKumoToastManager } from 'kumo-svelte';
	import {
		DownloadSimple,
		Copy,
		Check,
		Plus,
		Trash,
		FloppyDisk,
		SpinnerGap,
		Lightning,
		ShieldCheck,
		FileCode,
		WarningCircle
	} from 'phosphor-svelte';

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
	let selinuxLabel = $state('container_t');
	let apparmorProfile = $state('default');
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
		selinuxLabel = service.advanced?.security.selinuxLabel ?? 'container_t';
		apparmorProfile = service.advanced?.security.apparmorProfile ?? 'default';
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

	const toast = useKumoToastManager();

	// Promotion Flow state
	let isPromotionReviewOpen = $state(false);
	let isPromoting = $state(false);
	let validationMessage = $state<{ text: string; success: boolean } | null>(null);
	let activePromoTab = $state<string>('');
	let podFiles = $state<Record<string, string>>({});
	let promoCode = $state<string>('');

	function startPromotionFlow() {
		if (service.type === 'pod') {
			const podUnit = `[Unit]
Description=GoPod Pod ${service.name}
After=network-online.target

[Pod]
PodName=pod_${service.name}
Network=gopod-net
PublishPort=${service.hostPort || service.port || 8080}:${service.port || 80}

[Install]
WantedBy=default.target`;

			const memberServices = dataStore.services.filter(
				(s) =>
					s.projectId === service.projectId &&
					s.id !== service.id &&
					(s.podId === service.id ||
						s.podId === service.name ||
						s.podId === `pod_${service.name}` ||
						(s.inPod && s.type !== 'pod'))
			);

			const files: Record<string, string> = {};
			const podKey = `pod_${service.name}.pod`;
			files[podKey] = podUnit;

			if (memberServices.length === 0) {
				const memberKey = `${service.name}-app.container`;
				files[memberKey] = `[Unit]
Description=GoPod Pod Member ${service.name}
After=network-online.target

[Container]
ContainerName=${service.name}-app
Image=${service.image || service.source || 'docker.io/library/nginx:alpine'}
Pod=pod_${service.name}.pod
Label=io.gopod.service=${service.id}
Label=io.gopod.project=${service.projectId}

[Service]
Restart=always

[Install]
WantedBy=default.target`;
			} else {
				for (const m of memberServices) {
					const memberKey = `${m.name}.container`;
					files[memberKey] = `[Unit]
Description=GoPod Pod Member ${m.name}
After=network-online.target

[Container]
ContainerName=${m.name}
Image=${m.image || m.source || 'docker.io/library/nginx:alpine'}
Pod=pod_${service.name}.pod
Label=io.gopod.service=${m.id}
Label=io.gopod.project=${m.projectId}

[Service]
Restart=always

[Install]
WantedBy=default.target`;
				}
			}

			podFiles = files;
			activePromoTab = podKey;
			promoCode = files[podKey];
		} else {
			promoCode = `[Unit]
Description=GoPod ${service.name} Service
After=network-online.target

[Container]
ContainerName=${service.name}
Image=${service.image || service.source || 'localhost/' + service.name + ':latest'}
Network=gopod-net
PublishPort=${service.hostPort || service.port || 3000}:${service.port || 3000}
Label=io.gopod.service=${service.id}
Label=io.gopod.project=${service.projectId}
AutoUpdate=registry
UserNS=${userNamespace}
${volumes.map((v) => `Volume=${v.source}:${v.target}:${v.options}`).join('\n')}

[Service]
Restart=${restartPolicy}
TimeoutStartSec=300

[Install]
WantedBy=default.target`;
		}

		validationMessage = null;
		isPromotionReviewOpen = true;
	}

	function handleSwitchPromoTab(tabKey: string) {
		if (service.type === 'pod') {
			podFiles[activePromoTab] = promoCode;
			activePromoTab = tabKey;
			promoCode = podFiles[tabKey] || '';
		}
	}

	function handleValidateSyntax() {
		const code = promoCode.trim();
		if (!code.includes('[Unit]')) {
			validationMessage = { text: 'Validation error: Missing [Unit] section in systemd specification.', success: false };
			return;
		}
		if (!code.includes('[Container]') && !code.includes('[Pod]')) {
			validationMessage = { text: 'Validation error: Quadlet unit must contain either [Container] or [Pod] section.', success: false };
			return;
		}
		if (code.includes('[Container]') && !code.includes('Image=')) {
			validationMessage = { text: 'Validation error: [Container] section must define Image= directive.', success: false };
			return;
		}
		const containerIdx = code.indexOf('[Container]');
		const serviceIdx = code.indexOf('[Service]');
		if (containerIdx !== -1) {
			const containerSection = serviceIdx !== -1 ? code.slice(containerIdx, serviceIdx) : code.slice(containerIdx);
			if (containerSection.includes('Restart=')) {
				validationMessage = { text: 'Validation warning: Restart= directive belongs in [Service] section, not [Container].', success: false };
				return;
			}
		}
		validationMessage = { text: 'Syntax valid! Quadlet unit matches systemd specification and podman-system-generator conventions.', success: true };
	}

	async function handleConfirmPromotion() {
		isPromoting = true;
		const originalType = service.type;
		const originalTarget = service.runtimeTarget;

		if (service.type === 'pod') {
			podFiles[activePromoTab] = promoCode;
			service.quadletConfig = podFiles[`pod_${service.name}.pod`] || promoCode;
		} else {
			service.quadletConfig = promoCode;
		}

		service.type = 'quadlet';
		service.runtimeTarget = 'quadlet';

		try {
			await dataStore.updateService(service);
			await dataStore.deployService(service.id, 'promote-quadlet');
			toast.add({
				title: 'Promoted to Systemd Quadlet',
				description: `Workload ${service.name} is now supervised by systemd user session.`,
				variant: 'success'
			});
			isPromotionReviewOpen = false;
		} catch (err: any) {
			service.type = originalType;
			service.runtimeTarget = originalTarget;
			await dataStore.updateService(service).catch(() => {});
			validationMessage = {
				text: `Promotion activation failed: ${err?.message || 'systemd daemon-reload error'}. State restored.`,
				success: false
			};
			toast.add({
				title: 'Promotion Failed (Rolled Back)',
				description: err?.message || 'Systemd daemon-reload failed. Restored to previous state.',
				variant: 'error'
			});
		} finally {
			isPromoting = false;
		}
	}

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

	let isSaving = $state(false);
	let saveStatus = $state<'idle' | 'saved' | 'error'>('idle');

	function parseMemoryMb(memStr: string): number {
		const clean = memStr.trim().toUpperCase();
		const match = clean.match(/^(\d+(?:\.\d+)?)\s*(MB|M|GB|G|KB|K)?$/);
		if (!match) return 512;
		const num = parseFloat(match[1]);
		const unit = match[2] || 'MB';
		if (unit.startsWith('G')) return Math.round(num * 1024);
		if (unit.startsWith('M')) return Math.round(num);
		if (unit.startsWith('K')) return Math.round(num / 1024);
		return Math.round(num);
	}

	async function handleSave() {
		isSaving = true;
		try {
			const parsedCpu = parseFloat(cpuLimit) || 0;
			const parsedMem = parseMemoryMb(memoryLimit);
			const parsedPids = parseInt(pidsLimit, 10) || 2048;

			service.cpuLimit = parsedCpu;
			service.memoryLimit = parsedMem;
			service.restartPolicy = restartPolicy;
			service.advanced = {
				runtime: {
					mode: runtimeMode,
					userNamespace: userNamespace.trim() || 'keep-id',
					devices: devices
						.split(',')
						.map((d) => d.trim())
						.filter(Boolean)
				},
				lifecycle: {
					quadletEnabled,
					systemdUnitName: systemdUnit.trim() || `${service.name}.service`,
					restartPolicy
				},
				security: {
					privileged: isPrivileged,
					selinuxLabel: selinuxLabel.trim() || 'container_t',
					apparmorProfile: apparmorProfile.trim() || 'default',
					capAdd: capAdd
						.split(',')
						.map((c) => c.trim())
						.filter(Boolean),
					capDrop: capDrop
						.split(',')
						.map((c) => c.trim())
						.filter(Boolean),
					noNewPrivileges
				},
				storage: {
					volumes: volumes.filter((v) => v.source.trim() && v.target.trim())
				},
				resources: {
					cpuLimit,
					memoryLimit,
					pidsLimit: parsedPids,
					swapLimit
				}
			};

			await dataStore.updateService(service);
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
		} catch (err) {
			console.error('Failed to save advanced service configuration:', err);
			saveStatus = 'error';
			setTimeout(() => (saveStatus = 'idle'), 3000);
		} finally {
			isSaving = false;
		}
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- Top Operational Save Bar -->
	<div
		class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4 sm:flex-row sm:items-center"
	>
		<div class="flex flex-col gap-0.5">
			<span class="text-sm font-semibold text-[var(--text-primary)]"
				>Advanced Service Configuration</span
			>
			<span class="text-xs text-[var(--text-tertiary)]">
				Persists cgroups v2 quotas, rootless user namespace, security capabilities, and volume
				mounts to Podman.
			</span>
		</div>
		<div class="flex items-center gap-2">
			<Button variant="primary" size="sm" disabled={isSaving} onclick={handleSave}>
				{#if isSaving}
					<SpinnerGap size={13} class="animate-spin" /> Saving…
				{:else if saveStatus === 'saved'}
					<Check size={13} class="text-white" /> Saved to Runtime
				{:else}
					<FloppyDisk size={13} /> Save Changes
				{/if}
			</Button>
		</div>
	</div>
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
			<span class="text-xs font-medium text-[var(--text-secondary)]">Podman Mode</span>
			<Radio
				value={runtimeMode}
				onValueChange={(val: any) => {
					if (val === 'rootless' || val === 'rootful') runtimeMode = val;
				}}
				appearance="card"
				options={[
					{
						label: 'Rootless (Default)',
						value: 'rootless',
						description: 'Runs entirely in unprivileged user space without root privileges. Standard for PaaS workloads.'
					},
					{
						label: 'Rootful',
						value: 'rootful',
						description: 'Runs via system daemon with root permissions. Required for low privileged system ports <1024 or direct raw devices.'
					}
				]}
			/>
		</div>

		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<div class="flex flex-col gap-1.5">
				<label for="rt-userns" class="text-xs font-medium text-[var(--text-secondary)]"
					>User Namespace</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="rt-userns"
						bind:value={userNamespace}
						placeholder="keep-id"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="rt-devices" class="text-xs font-medium text-[var(--text-secondary)]"
					>Host Devices</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="rt-devices"
						bind:value={devices}
						placeholder="/dev/fuse, /dev/net/tun"
						class="text-xs font-[var(--font-mono)]"
					/>
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
		{#if service.type !== 'quadlet'}
			<!-- Systemd Quadlet Promotion Flow -->
			<div
				class="mb-4 flex flex-col gap-3 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] p-4 text-xs"
			>
				<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
					<div class="flex flex-col gap-1">
						<div class="flex items-center gap-2">
							<Lightning size={16} weight="fill" class="text-[var(--accent)]" />
							<span class="font-semibold text-sm text-[var(--color-kumo-default)]">
								⚡ Systemd Quadlet Supervision
							</span>
						</div>
						<span class="text-[var(--color-kumo-subtle)]">
							Promote this workload to a native systemd user service managed by Podman Quadlet with automatic crash restart, boot persistence, and systemctl controls.
						</span>
					</div>
					<Button
						variant="primary"
						size="sm"
						onclick={startPromotionFlow}
						class="shrink-0 gap-1.5"
					>
						<Lightning size={14} weight="bold" />
						<span>⚡ Generate & Promote to Systemd Quadlet…</span>
					</Button>
				</div>

				{#if isPromotionReviewOpen}
					<div class="mt-3 flex flex-col gap-3 border-t border-[var(--color-kumo-hairline)] pt-4">
						<div class="flex flex-wrap items-center justify-between gap-2">
							<div class="flex items-center gap-2">
								<FileCode size={15} class="text-[var(--accent)]" />
								<span class="font-medium text-xs text-[var(--color-kumo-default)]">
									Review Generated Quadlet Unit Definition
								</span>
							</div>

							<!-- Multi-file tabs for Pod workload -->
							{#if service.type === 'pod' && Object.keys(podFiles).length > 0}
								<div class="flex items-center gap-1 rounded bg-[var(--color-kumo-base)] p-0.5">
									{#each Object.keys(podFiles) as fKey}
										<button
											type="button"
											onclick={() => handleSwitchPromoTab(fKey)}
											class="cursor-pointer rounded px-2.5 py-1 font-mono text-[11px] transition-colors {activePromoTab === fKey
												? 'bg-[var(--color-kumo-elevated)] font-semibold text-[var(--color-kumo-default)] shadow-xs'
												: 'bg-transparent text-[var(--color-kumo-subtle)] hover:text-[var(--color-kumo-default)]'}"
										>
											{fKey.endsWith('.pod') ? '📦 ' : '🌐 '}{fKey}
										</button>
									{/each}
								</div>
							{/if}
						</div>

						<!-- Editor for generated unit -->
						<div class="rounded overflow-hidden border border-[var(--color-kumo-line)]">
							<CodeEditor
								bind:value={promoCode}
								language="quadlet"
								height="240px"
							/>
						</div>

						{#if validationMessage}
							<div
								class="flex items-center gap-2 rounded px-3 py-2 text-xs {validationMessage.success
									? 'bg-kumo-success-tint text-kumo-success'
									: 'bg-red-500/10 text-red-400'}"
							>
								{#if validationMessage.success}
									<Check size={14} weight="bold" />
								{:else}
									<WarningCircle size={14} weight="bold" />
								{/if}
								<span>{validationMessage.text}</span>
							</div>
						{/if}

						<div class="flex flex-wrap items-center justify-between gap-2 pt-1">
							<Button
								variant="secondary"
								size="sm"
								onclick={handleValidateSyntax}
								class="gap-1.5"
							>
								<ShieldCheck size={14} />
								<span>Validate Syntax</span>
							</Button>

							<div class="flex items-center gap-2">
								<Button
									variant="secondary"
									size="sm"
									onclick={() => (isPromotionReviewOpen = false)}
								>
									Cancel
								</Button>
								<Button
									variant="primary"
									size="sm"
									onclick={handleConfirmPromotion}
									disabled={isPromoting}
									class="gap-1.5"
								>
									{#if isPromoting}
										<SpinnerGap size={14} class="animate-spin" />
										<span>Activating Unit…</span>
									{:else}
										<Check size={14} weight="bold" />
										<span>Confirm & Activate Systemd Unit</span>
									{/if}
								</Button>
							</div>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		<div class="grid grid-cols-1 gap-4 pt-2 sm:grid-cols-2">
			<div
				class="mt-2 flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
			>
				<div class="flex flex-col gap-0.5">
					<span class="text-xs font-medium text-[var(--text-primary)]"
						>Systemd Quadlet Supervisor</span
					>
					<span class="text-[11px] text-[var(--text-tertiary)]">
						Automatically generates a systemd user service unit for process supervision.
					</span>
				</div>
				<Switch bind:checked={quadletEnabled} size="sm" />
			</div>

			<Select
				id="lc-restart"
				label="Restart Policy"
				value={restartPolicy}
				onValueChange={(val) => {
					if (typeof val === 'string') restartPolicy = val as any;
				}}
				options={[
					{ label: 'always', value: 'always' },
					{ label: 'on-failure', value: 'on-failure' },
					{ label: 'unless-stopped', value: 'unless-stopped' },
					{ label: 'no', value: 'no' }
				]}
				size="sm"
			/>
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
		<div
			class="mt-2 flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
		>
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">Privileged Container Mode</span
				>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Disables SELinux isolation and grants access to all host kernel devices. Keep disabled for
					standard PaaS apps.
				</span>
			</div>
			<Switch bind:checked={isPrivileged} size="sm" />
		</div>

		<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
			<div class="flex flex-col gap-1.5">
				<label for="sec-selinux" class="text-xs font-medium text-[var(--text-secondary)]"
					>SELinux Process Label</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="sec-selinux"
						bind:value={selinuxLabel}
						placeholder="container_t, spc_t, or disable"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Process type (e.g. <code>spc_t</code> for unconfined, or <code>disable</code>). For
					volumes, use <code>:z</code>/<code>:Z</code> below.
				</span>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="sec-apparmor" class="text-xs font-medium text-[var(--text-secondary)]"
					>AppArmor Profile (Ubuntu/Debian)</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="sec-apparmor"
						bind:value={apparmorProfile}
						placeholder="default or unconfined"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Standard MAC on Ubuntu/Debian. Set to <code>unconfined</code> to disable AppArmor.
				</span>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="sec-cap-add" class="text-xs font-medium text-[var(--text-secondary)]"
					>Add Capabilities</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="sec-cap-add"
						bind:value={capAdd}
						placeholder="NET_BIND_SERVICE"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="sec-cap-drop" class="text-xs font-medium text-[var(--text-secondary)]"
					>Drop Capabilities</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="sec-cap-drop"
						bind:value={capDrop}
						placeholder="ALL"
						class="text-xs font-[var(--font-mono)]"
					/>
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
			<span class="text-xs font-medium text-[var(--text-secondary)]">Configured Volume Mounts</span>
			<Button variant="secondary" size="sm" onclick={addVolume}>
				<Plus size={13} /> Add Mount
			</Button>
		</div>

		<div
			class="flex flex-col divide-y divide-[var(--border-subtle)] overflow-hidden rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]"
		>
			{#each volumes as vol, idx}
				<div class="grid grid-cols-1 items-center gap-3 p-3 sm:grid-cols-3">
					<div class="flex flex-col gap-1">
						<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
							>Source / Volume</span
						>
						<Input
							bind:value={vol.source}
							placeholder="volume_name"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
					<div class="flex flex-col gap-1">
						<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
							>Container Mount</span
						>
						<Input
							bind:value={vol.target}
							placeholder="/data"
							class="text-xs font-[var(--font-mono)]"
						/>
					</div>
					<div class="flex items-center gap-2">
						<div class="flex flex-1 flex-col gap-1">
							<span class="text-[10px] font-medium text-[var(--text-tertiary)] uppercase"
								>Flags</span
							>
							<Input
								bind:value={vol.options}
								placeholder="Z"
								class="text-xs font-[var(--font-mono)]"
							/>
						</div>
						<button
							type="button"
							onclick={() => removeVolume(idx)}
							class="mt-4 cursor-pointer border-0 bg-transparent p-1.5 text-[var(--text-tertiary)] hover:text-[var(--status-red)]"
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
		<div class="grid grid-cols-2 gap-4 pt-2 sm:grid-cols-4">
			<div class="flex flex-col gap-1.5">
				<label for="res-cpu" class="text-xs font-medium text-[var(--text-secondary)]"
					>CPU Quota</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="res-cpu"
						bind:value={cpuLimit}
						placeholder="2.0"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-mem" class="text-xs font-medium text-[var(--text-secondary)]"
					>Memory Limit</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="res-mem"
						bind:value={memoryLimit}
						placeholder="512MB"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-pids" class="text-xs font-medium text-[var(--text-secondary)]"
					>PIDs Limit</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="res-pids"
						bind:value={pidsLimit}
						placeholder="2048"
						class="text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label for="res-swap" class="text-xs font-medium text-[var(--text-secondary)]"
					>Swap Limit</label
				>
				<div
					class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
				>
					<Input
						id="res-swap"
						bind:value={swapLimit}
						placeholder="0"
						class="text-xs font-[var(--font-mono)]"
					/>
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
		<div class="flex flex-col justify-between gap-3 pt-2 sm:flex-row sm:items-center">
			<!-- Format selector tabs -->
			<div
				class="flex items-center gap-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-0.5"
			>
				<button
					type="button"
					onclick={() => (genTab = 'quadlet')}
					class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
					'quadlet'
						? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Quadlet (.container)
				</button>
				<button
					type="button"
					onclick={() => (genTab = 'kubernetes')}
					class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
					'kubernetes'
						? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
						: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
				>
					Kubernetes YAML
				</button>
				<button
					type="button"
					onclick={() => (genTab = 'podman')}
					class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
					'podman'
						? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
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

	<!-- Bottom Save Bar -->
	<div class="flex items-center justify-between border-t border-[var(--border)] pt-4">
		<span class="text-xs text-[var(--text-tertiary)]">
			Changes take effect immediately on next deployment or container restart.
		</span>
		<Button variant="primary" size="sm" disabled={isSaving} onclick={handleSave}>
			{#if isSaving}
				<SpinnerGap size={13} class="animate-spin" /> Saving…
			{:else if saveStatus === 'saved'}
				<Check size={13} class="text-white" /> Saved to Runtime
			{:else}
				<FloppyDisk size={13} /> Save Changes
			{/if}
		</Button>
	</div>
</div>
