<script lang="ts">
	import type { Service } from '$lib/types';
	import { Button, Badge } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { onMount, untrack } from 'svelte';
	import { useKumoToastManager } from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Check, FloppyDisk, SpinnerGap, WarningCircle } from 'phosphor-svelte';

	import RuntimeSecurityCard from '../advanced/RuntimeSecurityCard.svelte';
	import LifecycleConfigCard from '../advanced/LifecycleConfigCard.svelte';
	import StorageVolumesCard from '../advanced/StorageVolumesCard.svelte';
	import ResourceTuningCard from '../advanced/ResourceTuningCard.svelte';
	import EffectiveConfigPanel from '../advanced/EffectiveConfigPanel.svelte';
	import {
		parseCpuLimit,
		parseMemoryMb,
		parsePidsLimit,
		normalizeSwapLimit,
		parseCapList,
		validateVolumeRow,
		validateUserNamespace,
		validateDeviceList
	} from '$lib/parsers/advanced-validation';

	interface Props {
		service: Service;
		onNavigateTab?: (tabId: string) => void;
	}

	let { service = $bindable(), onNavigateTab }: Props = $props();

	const toast = useKumoToastManager();

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
		effective: true
	});

	// ── Workload-type awareness ─────────────────────────────────────────────
	// Which types actually consume Advanced? Runtime deploys read it for
	// image/application/database containers; quadlet generation bakes it into
	// unit files; compose/kubernetes deploy purely from their manifest and a
	// pod parent delegates per-child settings to each member service.
	const APPLYING_TYPES = ['image', 'application', 'database', 'quadlet'];
	let advancedApplies = $derived(APPLYING_TYPES.includes(service.type));
	let applicabilityReason = $derived.by(() => {
		if (service.type === 'pod') {
			return 'Pod members apply their own Advanced settings when they deploy — edit each member service. The full pod bundle (one unit per child) is generated in the Quadlet tab.';
		}
		if (service.type === 'compose') {
			return 'Compose workloads deploy from the Compose manifest, not from these fields — edit the manifest in the General tab.';
		}
		if (service.type === 'kubernetes') {
			return 'Kubernetes workloads deploy from the YAML manifest, not from these fields — edit the manifest in the General tab.';
		}
		return '';
	});
	let quadletGenerationNote = $derived(
		service.type === 'quadlet'
			? 'These settings are baked into unit files when they are generated. A promoted workload is edited directly in the Quadlet tab.'
			: ''
	);

	// ── Editable state (display strings for numeric fields) ─────────────────
	let initializedFor = $state('');
	let userNamespace = $state('');
	let devices = $state('');
	let isPrivileged = $state(false);
	let selinuxLabel = $state('');
	let apparmorProfile = $state('');
	let capAdd = $state('');
	let capDrop = $state('');
	let noNewPrivileges = $state(true);
	let volumes = $state<{ source: string; target: string; options: string }[]>([]);
	let restartPolicy = $state('always');
	let cpuText = $state('');
	let memText = $state('');
	let pidsText = $state('');
	let swapText = $state('');

	// Mirror the server copy once per service. Re-syncing on every poll would
	// wipe in-progress edits between status polls.
	$effect(() => {
		if (initializedFor === service.id) return;
		initializedFor = service.id;
		untrack(() => {
			userNamespace =
				(service.advanced?.runtime.userNamespace === 'default'
					? ''
					: service.advanced?.runtime.userNamespace) ?? '';
			devices = service.advanced?.runtime.devices.join(', ') ?? '';
			isPrivileged = service.advanced?.security.privileged ?? false;
			selinuxLabel = service.advanced?.security.selinuxLabel ?? '';
			apparmorProfile = service.advanced?.security.apparmorProfile ?? '';
			capAdd = service.advanced?.security.capAdd.join(', ') ?? '';
			capDrop = service.advanced?.security.capDrop.join(', ') ?? '';
			noNewPrivileges = service.advanced?.security.noNewPrivileges ?? true;
			volumes = (service.advanced?.storage.volumes ?? []).map((v) => ({ ...v }));
			restartPolicy = service.restartPolicy || 'always';
			cpuText = service.cpuLimit ? String(service.cpuLimit) : '';
			memText = service.memoryLimit ? String(service.memoryLimit) : '';
			pidsText = service.advanced?.resources.pidsLimit
				? String(service.advanced.resources.pidsLimit)
				: '';
			swapText = service.advanced?.resources.swapLimit ?? '';
			snap = snapshot(swapText);
		});
	});

	// ── Inline validation (derived, so errors never lag behind the fields) ──
	let cpuError = $derived.by(() => {
		if (Number.isNaN(parseCpuLimit(cpuText)))
			return 'Use decimal cores, e.g. 1.5 (empty = unlimited)';
		return '';
	});
	let memError = $derived.by(() => {
		if (Number.isNaN(parseMemoryMb(memText)))
			return 'Use MB/GB, e.g. 512MB or 1G (empty = unlimited)';
		return '';
	});
	let pidsError = $derived.by(() => {
		if (Number.isNaN(parsePidsLimit(pidsText)))
			return 'Whole number of processes (empty = unlimited)';
		return '';
	});
	let swapError = $derived(normalizeSwapLimit(swapText).error);
	let capAddItems = $derived(parseCapList(capAdd));
	let capAddError = $derived(
		capAddItems === null ? 'Capability names use letters, digits and underscores' : ''
	);
	let capDropItems = $derived(parseCapList(capDrop));
	let capDropError = $derived(
		capDropItems === null ? 'Capability names use letters, digits and underscores' : ''
	);
	let userNsError = $derived(validateUserNamespace(userNamespace));
	let devicesError = $derived(validateDeviceList(devices));
	let volumeErrors = $derived(volumes.map((v) => validateVolumeRow(v)));

	let privilegedCapDropWarning = $derived.by(() => {
		if (!isPrivileged) return '';
		if ((capDropItems ?? []).some((c) => c.toUpperCase() === 'ALL')) {
			return 'Privileged mode grants all capabilities, so dropping ALL is contradictory — podman keeps the privileged grant and ignores the drop.';
		}
		return '';
	});
	let privilegedUserNsWarning = $derived.by(() => {
		if (!isPrivileged) return '';
		const ns = userNamespace.trim();
		if (ns !== '' && ns.toLowerCase() !== 'default' && ns.toLowerCase() !== 'none') {
			return 'Privileged mode and a custom user namespace cannot combine — podman rejects the run before it starts.';
		}
		return '';
	});

	let hasValidationErrors = $derived(
		cpuError !== '' ||
			memError !== '' ||
			pidsError !== '' ||
			swapError !== '' ||
			capAddError !== '' ||
			capDropError !== '' ||
			userNsError !== '' ||
			devicesError !== '' ||
			volumeErrors.some((e) => e !== '')
	);

	// ── Per-panel dirty tracking ────────────────────────────────────────────
	interface AdvancedSnapshot {
		runtimeSecurity: string;
		resources: string;
		storage: string;
		lifecycle: string;
	}
	let snap = $state<AdvancedSnapshot>({
		runtimeSecurity: '',
		resources: '',
		storage: '',
		lifecycle: ''
	});

	function snapshot(swap: string): AdvancedSnapshot {
		return {
			runtimeSecurity: JSON.stringify([
				userNamespace,
				devices,
				isPrivileged,
				selinuxLabel,
				apparmorProfile,
				capAdd,
				capDrop,
				noNewPrivileges
			]),
			resources: JSON.stringify([cpuText, memText, pidsText, swap]),
			storage: JSON.stringify(volumes),
			lifecycle: restartPolicy
		};
	}

	function currentSnapshot(): AdvancedSnapshot {
		return snapshot(normalizeSwapLimit(swapText).value);
	}

	let dirtyRuntimeSecurity = $derived.by(() => {
		const s = snapshot(normalizeSwapLimit(swapText).value);
		return s.runtimeSecurity !== snap.runtimeSecurity;
	});
	let dirtyResources = $derived.by(() => {
		const s = snapshot(normalizeSwapLimit(swapText).value);
		return s.resources !== snap.resources;
	});
	let dirtyStorage = $derived.by(() => {
		const s = snapshot(normalizeSwapLimit(swapText).value);
		return s.storage !== snap.storage;
	});
	let dirtyLifecycle = $derived.by(() => {
		const s = snapshot(normalizeSwapLimit(swapText).value);
		return s.lifecycle !== snap.lifecycle;
	});
	let dirtyPanels = $derived(
		[
			dirtyRuntimeSecurity ? 'Runtime & Security' : '',
			dirtyLifecycle ? 'Lifecycle' : '',
			dirtyStorage ? 'Storage' : '',
			dirtyResources ? 'Resources' : ''
		].filter(Boolean)
	);

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
			toast.add(
				errorToastOptions('Demote Failed', err?.message || 'Failed to demote quadlet workload')
			);
		} finally {
			isDemoting = false;
		}
	}

	let isSaving = $state(false);
	let saveStatus = $state<'idle' | 'saved' | 'error'>('idle');

	async function handleSave() {
		if (hasValidationErrors) {
			toast.add(
				errorToastOptions(
					'Fix the highlighted fields first',
					'Invalid values are never sent to the engine.'
				)
			);
			return;
		}
		const swap = normalizeSwapLimit(swapText);
		if (swap.value !== '' && swap.value !== '-1' && !parseMemoryMb(memText)) {
			toast.add(
				errorToastOptions(
					'Memory limit required',
					'podman bounds swap against the memory limit — set a memory limit as well.'
				)
			);
			return;
		}
		// Make sure the change is not lost silently: running services only get
		// these settings on the next deploy.
		const isRunning = service.status === 'running' || service.status === 'deploying';
		if (isRunning) {
			const proceed = confirm(
				'Save advanced settings? They take effect on the next deploy, not on the running container.'
			);
			if (!proceed) return;
		}

		isSaving = true;
		try {
			const parsedCpu = parseCpuLimit(cpuText);
			const parsedMem = parseMemoryMb(memText);
			const parsedPids = parsePidsLimit(pidsText);

			service.cpuLimit = parsedCpu;
			service.memoryLimit = parsedMem;
			service.restartPolicy = restartPolicy;
			service.advanced = {
				runtime: {
					userNamespace: userNamespace.trim() === 'default' ? '' : userNamespace.trim(),
					devices: devices
						.split(',')
						.map((d) => d.trim())
						.filter(Boolean)
				},
				lifecycle: {
					restartPolicy
				},
				security: {
					privileged: isPrivileged,
					selinuxLabel: selinuxLabel.trim(),
					apparmorProfile: apparmorProfile.trim(),
					capAdd: capAddItems ?? [],
					capDrop: capDropItems ?? [],
					noNewPrivileges
				},
				storage: {
					volumes: volumes
						.filter((v) => v.source.trim() && v.target.trim())
						.map((v) => ({
							source: v.source.trim(),
							target: v.target.trim(),
							options: v.options.trim()
						}))
				},
				resources: {
					pidsLimit: parsedPids,
					swapLimit: swap.value
				}
			};

			await dataStore.updateService(service);
			snap = snapshot(normalizeSwapLimit(swapText).value);
			saveStatus = 'saved';
			setTimeout(() => (saveStatus = 'idle'), 2500);
			toast.add({
				title: 'Advanced settings saved',
				description: isRunning
					? 'Stored — the next deploy picks them up.'
					: 'Stored — the next deploy picks them up.',
				variant: 'success'
			});
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
	<!-- Top Save Bar -->
	<div
		class="flex flex-col justify-between gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4 sm:flex-row sm:items-center"
	>
		<div class="flex flex-col gap-0.5">
			<span class="text-sm font-semibold text-[var(--text-primary)]"
				>Advanced Service Configuration</span
			>
			<span class="text-xs text-[var(--text-tertiary)]">
				Every field here maps to a real podman flag or unit directive; saved settings apply on the
				next deploy.
			</span>
		</div>
		<div class="flex flex-wrap items-center gap-2">
			{#each dirtyPanels as panel (panel)}
				<Badge variant="warning" class="shrink-0 px-2 py-0.5 text-[11px]">{panel} changed</Badge>
			{/each}
			<Button
				variant="primary"
				size="sm"
				disabled={isSaving}
				onclick={handleSave}
			>
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

	{#if !advancedApplies}
		<!-- Honest workload-type state: these fields genuinely do not apply -->
		<div
			class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4"
		>
			<WarningCircle size={18} weight="fill" class="mt-0.5 shrink-0 text-kumo-warning" />
			<div class="flex flex-col gap-1">
				<span class="text-sm font-semibold text-[var(--text-primary)]">
					Advanced settings do not apply to this workload type
				</span>
				<span class="text-xs leading-relaxed text-[var(--text-secondary)]">
					{applicabilityReason}
				</span>
			</div>
		</div>
	{:else}
		{#if quadletGenerationNote}
			<div
				class="flex items-start gap-3 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4"
			>
				<WarningCircle size={18} weight="fill" class="mt-0.5 shrink-0 text-kumo-warning" />
				<div class="flex flex-col gap-1">
					<span class="text-sm font-semibold text-[var(--text-primary)]"> Quadlet workload </span>
					<span class="text-xs leading-relaxed text-[var(--text-secondary)]">
						{quadletGenerationNote}
					</span>
				</div>
			</div>
		{/if}

		<!-- 1. Runtime & Security Panels -->
		<RuntimeSecurityCard
			bind:openRuntime={openSections.runtime}
			bind:openSecurity={openSections.security}
			bind:userNamespace
			{userNsError}
			bind:devices
			{devicesError}
			bind:isPrivileged
			bind:selinuxLabel
			bind:apparmorProfile
			bind:capAdd
			{capAddError}
			bind:capDrop
			{capDropError}
			bind:noNewPrivileges
			{privilegedCapDropWarning}
			{privilegedUserNsWarning}
		/>

		<!-- 2. Lifecycle & Systemd Supervision Panel -->
		<LifecycleConfigCard
			bind:openLifecycle={openSections.lifecycle}
			{service}
			{lingerInfo}
			bind:restartPolicy
			{isDemoting}
			onDemoteQuadlet={handleDemoteQuadlet}
			onOpenQuadlet={() => onNavigateTab?.('quadlet')}
		/>

		<!-- 3. Storage & Volume Mounts Panel -->
		<StorageVolumesCard
			bind:openStorage={openSections.storage}
			serviceName={service.name}
			bind:volumes
			{volumeErrors}
		/>

		<!-- 4. Resource Limits Panel -->
		<ResourceTuningCard
			bind:openResources={openSections.resources}
			bind:cpuText
			{cpuError}
			bind:memText
			{memError}
			bind:pidsText
			{pidsError}
			bind:swapText
			{swapError}
		/>

		<!-- 5. Effective Configuration (backend generator output) -->
		<EffectiveConfigPanel bind:openEffective={openSections.effective} {service} />
	{/if}
</div>
