<script lang="ts">
	import type { Service } from '$lib/types';
	import { api } from '$lib/api';
	import { dataStore } from '$lib/data';
	import { untrack } from 'svelte';
	import { Button, Banner, LayerCard, useKumoToastManager } from 'kumo-svelte';
	import {
		Table,
		FileCode,
		SpinnerGap,
		Check,
		ShieldCheck,
		Lightning,
		ArrowsClockwise,
		WarningCircle
	} from 'phosphor-svelte';
	import QuadletEditorCard from '../QuadletEditorCard.svelte';
	import QuadletUnitCard from '../QuadletUnitCard.svelte';
	import PromotionWorkflowModal from '../advanced/PromotionWorkflowModal.svelte';
	import { splitQuadletText } from '$lib/parsers/quadlet-units';
	import { joinQuadletFiles, inferQuadletSourceType } from '../quadlet-promotion';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	// Supervision truth: the service is quadlet-managed when its type is
	// quadlet, or when it is a pod with the generated bundle recorded on it.
	let isQuadletManaged = $derived(
		service.type === 'quadlet' ||
			(service.type === 'pod' && (service.quadletConfig || '').trim() !== '')
	);

	let mode = $state<'structured' | 'raw'>('structured');

	// ── Structured editing state ────────────────────────────────────────────
	let loadedFor = $state('');
	let isLoadingPreview = $state(false);
	let previewError = $state('');
	let files = $state<Record<string, string>>({});
	let isValidating = $state(false);
	let validateResult = $state<{ valid: boolean; output: string; error?: string } | null>(null);
	let isSaving = $state(false);

	let units = $derived(splitQuadletText(toBundle(files)));

	function toBundle(map: Record<string, string>): string {
		const entries = Object.entries(map);
		if (entries.length <= 1) return entries[0]?.[1] ?? '';
		return entries.map(([fname, content]) => `# File: ${fname}\n${content.trim()}`).join('\n---\n');
	}

	async function loadPreview() {
		isLoadingPreview = true;
		previewError = '';
		validateResult = null;
		try {
			const res = await api.services.quadletPreview(service.id);
			files = res.files || {};
			if (Object.keys(files).length === 0) {
				previewError = res.error || 'The generator produced no unit files for this service.';
			}
		} catch (err: any) {
			previewError = err?.message || 'Failed to generate unit files';
			toast.add({
				title: 'Quadlet preview failed',
				description: err?.message || 'Could not load generated unit files',
				variant: 'error'
			});
		} finally {
			isLoadingPreview = false;
		}
	}

	// Preview once per service so per-unit cards always mirror what is on disk.
	$effect(() => {
		if (!isQuadletManaged) return;
		if (loadedFor === service.id) return;
		loadedFor = service.id;
		untrack(() => {
			loadPreview();
		});
	});

	function handleApplyUnit(name: string, nextContent: string) {
		files = { ...files, [name]: nextContent };
		toast.add({
			title: 'Changes applied locally',
			description: 'Use “Save unit files” to persist, or Validate to dry-run the current text.',
			variant: 'info'
		});
	}

	async function handleValidate() {
		isValidating = true;
		validateResult = null;
		try {
			const res = await api.services.validateQuadlet(service.id, { files: { ...files } });
			validateResult = { valid: res.valid, output: res.output, error: res.error };
			toast.add({
				title: res.valid ? 'Dry-run passed' : 'Dry-run failed',
				description:
					res.error ||
					(res.valid ? 'Every unit passed quadlet -dryrun' : 'See the generator output'),
				variant: res.valid ? 'success' : 'error'
			});
		} catch (err: any) {
			toast.add({
				title: 'Validation request failed',
				description: err?.message || 'Could not reach the backend validator',
				variant: 'error'
			});
		} finally {
			isValidating = false;
		}
	}

	async function handleSave() {
		isSaving = true;
		try {
			service.quadletConfig = toBundle(files);
			await dataStore.updateService(service);
			toast.add({
				title: 'Quadlet unit files saved',
				description: 'Persisted configuration reloaded through systemd.',
				variant: 'success'
			});
			// Reload so the cards reflect the backend-healed truth.
			loadedFor = '';
		} catch (err: any) {
			toast.add({
				title: 'Save failed',
				description: err?.message || 'Could not save Quadlet configuration',
				variant: 'error'
			});
		} finally {
			isSaving = false;
		}
	}

	// ── Promotion flow (single source: the backend preview endpoint) ────────
	let isPromotionReviewOpen = $state(false);
	let isPromoting = $state(false);
	let isValidatingDryRun = $state(false);
	let validationMessage = $state<{ text: string; success: boolean } | null>(null);
	let activePromoTab = $state('');
	let promoFiles = $state<Record<string, string>>({});
	let promoCode = $state('');
	let dryrunOutput = $state('');

	async function startPromotionFlow() {
		isPromoting = false;
		isValidatingDryRun = false;
		isLoadingPreview = true;
		try {
			const res = await api.services.quadletPreview(service.id);
			promoFiles = res.files || {};
			const firstKey = Object.keys(promoFiles)[0] || '';
			activePromoTab = firstKey;
			promoCode = promoFiles[firstKey] || '';
			validationMessage = res.valid
				? {
						text: 'Generated unit files passed the real quadlet dry-run. Review and activate.',
						success: true
					}
				: {
						text: `Generated units failed quadlet -dryrun: ${res.error || 'invalid definition'}`,
						success: false
					};
			dryrunOutput = res.dryrun || '';
			isPromotionReviewOpen = true;
		} catch (err: any) {
			toast.add({
				title: 'Quadlet preview failed',
				description: err?.message || 'Could not generate unit files for promotion',
				variant: 'error'
			});
		} finally {
			isLoadingPreview = false;
		}
	}

	function handleSwitchPromoTab(tabKey: string) {
		promoFiles[activePromoTab] = promoCode;
		activePromoTab = tabKey;
		promoCode = promoFiles[tabKey] || '';
	}

	async function handlePromoDryRun() {
		isValidatingDryRun = true;
		validationMessage = null;
		dryrunOutput = '';
		promoFiles[activePromoTab] = promoCode;
		try {
			const res = await api.services.validateQuadlet(service.id, { files: { ...promoFiles } });
			validationMessage = {
				text: res.valid
					? 'Pre-flight dry-run passed! The Podman Quadlet generator validated every unit without errors.'
					: `Pre-flight dry-run failed: ${res.error || 'Invalid Quadlet unit definition'}`,
				success: res.valid
			};
			dryrunOutput = res.output;
		} catch (err: any) {
			validationMessage = {
				text: `Dry-run request error: ${err?.message || 'Failed to connect to backend validator'}`,
				success: false
			};
		} finally {
			isValidatingDryRun = false;
		}
	}

	async function handleConfirmPromotion() {
		isPromoting = true;
		const originalType = service.type;
		const originalTarget = service.runtimeTarget;

		service.sourceType = inferQuadletSourceType(service, originalType);
		service.quadletConfig = joinQuadletFiles(promoFiles, activePromoTab, promoCode);
		service.type = 'quadlet';
		service.runtimeTarget = 'quadlet';

		try {
			await dataStore.updateService(service);
			await dataStore.deployService(service.id, 'promote-quadlet');
			toast.add({
				title: 'Promoted to Systemd Quadlet',
				description: `Workload ${service.name} is now supervised by the systemd user session.`,
				variant: 'success'
			});
			isPromotionReviewOpen = false;
			loadedFor = '';
		} catch (err: any) {
			service.type = originalType;
			service.runtimeTarget = originalTarget;
			await dataStore.updateService(service).catch(() => {});
			validationMessage = {
				text: `Promotion activation failed: ${err?.message || 'systemd daemon-reload error'}. Previous state restored.`,
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
</script>

<div class="flex w-full flex-col gap-4">
	{#if !isQuadletManaged}
		<!-- Not yet supervised: the promotion workspace -->
		<LayerCard class="w-full shadow-xs">
			<div class="flex flex-col justify-between gap-3 p-5 sm:flex-row sm:items-center">
				<div class="flex flex-col gap-1">
					<div class="flex items-center gap-2">
						<Lightning size={16} weight="fill" class="text-[var(--accent)]" />
						<span class="text-sm font-semibold text-[var(--text-primary)]">
							Systemd Quadlet Supervision
						</span>
					</div>
					<span class="max-w-2xl text-xs text-[var(--text-tertiary)]">
						Generate the declarative unit files from this service's current configuration through
						the backend Podman Quadlet generator — every pod child receives its own
						<code class="font-[var(--font-mono)] text-[var(--accent)]">.container</code> unit. Review
						them, dry-run the bundle, then activate to hand supervision to the host systemd user session.
					</span>
				</div>
				<Button
					variant="primary"
					size="sm"
					disabled={isLoadingPreview}
					onclick={startPromotionFlow}
					class="shrink-0 gap-1.5"
				>
					{#if isLoadingPreview}
						<SpinnerGap size={14} class="animate-spin" /> Generating…
					{:else}
						<Lightning size={14} weight="bold" />
						<span>Generate unit files (preview)</span>
					{/if}
				</Button>
			</div>
		</LayerCard>
	{:else}
		<!-- Mode switch bar -->
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div
				class="flex items-center gap-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-1"
			>
				<button
					type="button"
					onclick={() => (mode = 'structured')}
					class="flex cursor-pointer items-center gap-1.5 rounded px-3 py-1.5 text-xs font-medium transition-colors {mode ===
					'structured'
						? 'bg-[var(--accent)] text-white shadow-xs'
						: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					<Table size={14} /> Structured (recommended)
				</button>
				<button
					type="button"
					onclick={() => (mode = 'raw')}
					class="flex cursor-pointer items-center gap-1.5 rounded px-3 py-1.5 text-xs font-medium transition-colors {mode ===
					'raw'
						? 'bg-[var(--accent)] text-white shadow-xs'
						: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
				>
					<FileCode size={14} /> Raw INI
				</button>
			</div>
			{#if mode === 'structured'}
				<div class="flex items-center gap-2">
					<Button
						variant="secondary"
						size="sm"
						disabled={isLoadingPreview}
						onclick={loadPreview}
						class="gap-1.5"
					>
						<ArrowsClockwise size={13} /> Reload
					</Button>
					<Button
						variant="secondary"
						size="sm"
						disabled={isValidating || isLoadingPreview}
						onclick={handleValidate}
						class="gap-1.5"
					>
						<ShieldCheck size={14} /> Validate
					</Button>
					<Button
						variant="primary"
						size="sm"
						disabled={isSaving || isLoadingPreview}
						onclick={handleSave}
						class="gap-1.5"
					>
						{#if isSaving}
							<SpinnerGap size={13} class="animate-spin" /> Saving…
						{:else}
							<Check size={13} weight="bold" /> Save unit files
						{/if}
					</Button>
				</div>
			{/if}
		</div>

		{#if mode === 'structured'}
			{#if isLoadingPreview}
				<div
					class="flex items-center justify-center gap-2 py-16 text-xs text-[var(--text-tertiary)]"
				>
					<SpinnerGap size={18} class="animate-spin text-[var(--accent)]" /> Generating unit files…
				</div>
			{:else if previewError}
				<Banner variant="alert" class="shadow-xs">
					{#snippet icon()}
						<WarningCircle size={18} weight="fill" class="shrink-0 text-kumo-warning" />
					{/snippet}
					<span class="text-xs font-semibold text-[var(--text-primary)]">Unit preview failed</span>
					<span class="text-xs text-[var(--text-secondary)]">{previewError}</span>
				</Banner>
			{:else if Object.keys(files).length === 0}
				<LayerCard class="w-full shadow-xs">
					<div class="p-8 text-center text-xs text-[var(--text-tertiary)]">
						No unit files yet. Use “Reload” to generate them from the service configuration.
					</div>
				</LayerCard>
			{:else}
				{#each units as unit (unit.name)}
					<QuadletUnitCard {unit} onApply={handleApplyUnit} />
				{/each}

				{#if validateResult}
					<div
						class="flex flex-col gap-2 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-4"
					>
						<span
							class="flex items-center gap-2 text-xs font-semibold {validateResult.valid
								? 'text-[var(--status-green)]'
								: 'text-[var(--status-red)]'}"
						>
							{#if validateResult.valid}
								<ShieldCheck size={14} /> quadlet -dryrun passed — verbatim generator output
							{:else}
								<WarningCircle size={14} /> quadlet -dryrun failed{validateResult.error
									? `: ${validateResult.error}`
									: ''}
							{/if}
						</span>
						<pre
							class="m-0 max-h-72 overflow-auto rounded-md border border-[var(--border-subtle)] bg-[var(--bg-table-header)] p-3 font-mono text-[11px] leading-relaxed whitespace-pre-wrap text-[var(--text-secondary)] select-text">{validateResult.output ||
								'(no output — see error above)'}</pre>
					</div>
				{/if}
			{/if}
		{:else}
			<QuadletEditorCard {service} />
		{/if}
	{/if}

	<!-- Promotion review modal (single promotion flow for every workload type) -->
	<PromotionWorkflowModal
		bind:isOpen={isPromotionReviewOpen}
		{service}
		bind:podFiles={promoFiles}
		bind:activePromoTab
		bind:promoCode
		bind:validationMessage
		bind:dryrunOutput
		{isPromoting}
		{isValidatingDryRun}
		onSwitchTab={handleSwitchPromoTab}
		onValidateDryRun={handlePromoDryRun}
		onConfirm={handleConfirmPromotion}
		onCancel={() => (isPromotionReviewOpen = false)}
	/>
</div>
