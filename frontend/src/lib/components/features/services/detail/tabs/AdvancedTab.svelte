<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { onMount } from 'svelte';
	import { useKumoToastManager } from 'kumo-svelte';
	import { Check, FloppyDisk, SpinnerGap } from 'phosphor-svelte';
	import { generatePromotionFiles, validateQuadletSyntax } from '../advanced/promotion-generator';

	import RuntimeSecurityCard from '../advanced/RuntimeSecurityCard.svelte';
	import LifecycleConfigCard from '../advanced/LifecycleConfigCard.svelte';
	import StorageVolumesCard from '../advanced/StorageVolumesCard.svelte';
	import ResourceTuningCard from '../advanced/ResourceTuningCard.svelte';
	import GeneratedQuadletPreview from '../advanced/GeneratedQuadletPreview.svelte';
	import PromotionWorkflowModal from '../advanced/PromotionWorkflowModal.svelte';

	interface Props {
		service: Service;
	}

	let { service = $bindable() }: Props = $props();

	// Linger status state
	let lingerInfo = $state<{ linger: boolean; user: string } | null>(null);

	onMount(async () => {
		try {
			const res = await api.system.getLinger();
			lingerInfo = res;
		} catch (err) {
			console.warn('Failed to query system linger status:', err);
		}
	});

	// Section collapse states
	let openSections = $state<Record<string, boolean>>({
		runtime: true,
		lifecycle: true,
		security: true,
		storage: true,
		resources: true,
		generated: true
	});

	// Runtime state
	let runtimeMode = $state<'rootless' | 'rootful'>('rootless');
	let userNamespace = $state('');
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

	// Resources state
	let cpuLimit = $state('2.0');
	let memoryLimit = $state('512MB');
	let pidsLimit = $state('2048');
	let swapLimit = $state('0');

	$effect(() => {
		runtimeMode = service.advanced?.runtime.mode ?? 'rootless';
		userNamespace =
			(service.advanced?.runtime.userNamespace === 'default'
				? ''
				: service.advanced?.runtime.userNamespace) ?? '';
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
	let isValidatingDryRun = $state(false);
	let dryrunOutput = $state('');

	function startPromotionFlow() {
		const res = generatePromotionFiles(service, restartPolicy, volumes, dataStore.services);
		podFiles = res.podFiles;
		activePromoTab = res.activePromoTab;
		promoCode = res.promoCode;
		validationMessage = null;
		dryrunOutput = '';
		isPromotionReviewOpen = true;
	}

	function handleSwitchPromoTab(tabKey: string) {
		if (service.type === 'pod' || Object.keys(podFiles).length > 0) {
			podFiles[activePromoTab] = promoCode;
			activePromoTab = tabKey;
			promoCode = podFiles[tabKey] || '';
		}
	}

	async function handleDryRunValidation() {
		isValidatingDryRun = true;
		validationMessage = null;
		dryrunOutput = '';

		if (service.type === 'pod' || Object.keys(podFiles).length > 0) {
			podFiles[activePromoTab] = promoCode;
		}

		try {
			const payload =
				service.type === 'pod' || Object.keys(podFiles).length > 0
					? { files: podFiles }
					: { content: promoCode };
			const res = await api.services.validateQuadlet(service.id, payload);
			if (res.valid) {
				validationMessage = {
					text: 'Pre-flight dry-run passed! Podman Quadlet generator (/usr/libexec/podman/quadlet) validated unit without errors.',
					success: true
				};
				dryrunOutput = res.output;
			} else {
				validationMessage = {
					text: `Pre-flight dry-run failed: ${res.error || 'Invalid Quadlet unit definition'}`,
					success: false
				};
				dryrunOutput = res.output;
			}
		} catch (err: any) {
			validationMessage = {
				text: `Dry-run request error: ${err?.message || 'Failed to connect to backend validator'}`,
				success: false
			};
		} finally {
			isValidatingDryRun = false;
		}
	}

	let isDemoting = $state(false);

	async function handleDemoteQuadlet() {
		if (
			!confirm(
				`Demote '${service.name}' from systemd Quadlet supervision? This will stop the systemd user service and run the workload directly via Podman.`
			)
		) {
			return;
		}
		isDemoting = true;
		try {
			const updated = await dataStore.demoteQuadlet(service.id);
			if (updated) {
				service = updated;
			}
			toast.add({
				title: 'Demoted to Standalone',
				description: `Workload ${service.name} is now managed directly by Podman.`,
				variant: 'success'
			});
		} catch (err: any) {
			toast.add({
				title: 'Demote Failed',
				description: err?.message || 'Failed to demote quadlet workload',
				variant: 'error'
			});
		} finally {
			isDemoting = false;
		}
	}

	function handleValidateSyntax() {
		validationMessage = validateQuadletSyntax(promoCode);
	}

	async function handleConfirmPromotion() {
		isPromoting = true;
		const originalType = service.type;
		const originalTarget = service.runtimeTarget;

		if (!service.sourceType || service.sourceType === 'git') {
			if (
				originalType === 'compose' ||
				(service.composeYaml && service.composeYaml.trim() !== '')
			) {
				service.sourceType = 'compose';
			} else if (
				originalType === 'kubernetes' ||
				(service.k8sYaml && service.k8sYaml.trim() !== '')
			) {
				service.sourceType = 'kubernetes';
			} else if (originalType === 'pod') {
				service.sourceType = 'pod';
			} else if (originalType === 'image') {
				service.sourceType = 'image';
			} else {
				service.sourceType = originalType;
			}
		}

		if (service.type === 'pod' || Object.keys(podFiles).length > 0) {
			podFiles[activePromoTab] = promoCode;
			const units = Object.entries(podFiles).map(([fname, content]) => {
				return `# File: ${fname}\n${content.trim()}`;
			});
			service.quadletConfig = units.join('\n---\n');
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
					userNamespace: userNamespace.trim() === 'default' ? '' : userNamespace.trim(),
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
					<Check size={13} class="text-white" /> Saved
				{:else}
					<FloppyDisk size={13} /> Save Configuration
				{/if}
			</Button>
		</div>
	</div>

	<!-- 1. Runtime & 3. Security Configuration Cards -->
	<RuntimeSecurityCard
		bind:openRuntime={openSections.runtime}
		bind:openSecurity={openSections.security}
		bind:runtimeMode
		bind:userNamespace
		bind:devices
		bind:isPrivileged
		bind:selinuxLabel
		bind:apparmorProfile
		bind:capAdd
		bind:capDrop
	/>

	<!-- 2. Lifecycle & Systemd Supervision Card -->
	<LifecycleConfigCard
		bind:openLifecycle={openSections.lifecycle}
		{service}
		{lingerInfo}
		bind:restartPolicy
		{isDemoting}
		onDemoteQuadlet={handleDemoteQuadlet}
		onStartPromotion={startPromotionFlow}
	/>

	<!-- Promotion Review Modal (if active) -->
	<PromotionWorkflowModal
		bind:isOpen={isPromotionReviewOpen}
		{service}
		bind:podFiles
		bind:activePromoTab
		bind:promoCode
		bind:validationMessage
		bind:dryrunOutput
		{isPromoting}
		{isValidatingDryRun}
		onSwitchTab={handleSwitchPromoTab}
		onValidateDryRun={handleDryRunValidation}
		onValidateSyntax={handleValidateSyntax}
		onConfirm={handleConfirmPromotion}
		onCancel={() => (isPromotionReviewOpen = false)}
	/>

	<!-- 4. Storage & Volume Mounts Card -->
	<StorageVolumesCard
		bind:openStorage={openSections.storage}
		serviceName={service.name}
		bind:volumes
	/>

	<!-- 5. Resource Limits Card -->
	<ResourceTuningCard
		bind:openResources={openSections.resources}
		bind:cpuLimit
		bind:memoryLimit
		bind:pidsLimit
		bind:swapLimit
	/>

	<!-- 6. Generated Configuration Card -->
	<GeneratedQuadletPreview
		bind:openGenerated={openSections.generated}
		{service}
		{restartPolicy}
		{userNamespace}
		{volumes}
		{cpuLimit}
		{memoryLimit}
		{pidsLimit}
		{isPrivileged}
		{noNewPrivileges}
	/>

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
