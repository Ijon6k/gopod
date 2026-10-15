<script lang="ts">
	import type { Service } from '$lib/types';
	import { dataStore } from '$lib/data';
	import { api } from '$lib/api';
	import { Banner, Button, useKumoToastManager } from 'kumo-svelte';
	import { Tabs } from '$lib/components/ui';
	import { Info, WarningCircle, ArrowClockwise } from 'phosphor-svelte';
	import PodPortForwardCard from './pod/PodPortForwardCard.svelte';
	import PodMembersTable from './pod/PodMembersTable.svelte';
	import PodAttachMemberModal from './pod/PodAttachMemberModal.svelte';
	import PodKubernetesEditorCard from './pod/PodKubernetesEditorCard.svelte';
	import { parseDetailedPorts, normalizePortKey } from '$lib/parsers/pod-ports';
	import { findPodMembers } from './pod/pod-membership';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	const toast = useKumoToastManager();

	let viewMode = $state<'visual' | 'yaml'>('visual');
	let isAddingContainer = $state(false);
	let manualChangePending = $state(false);
	let isDeploying = $state(false);

	function sanitizePodSlug(name: string): string {
		return name
			.replace(/^pod_/, '')
			.trim()
			.toLowerCase()
			.replace(/[^a-z0-9_-]+/g, '-');
	}

	// Canonical runtime name mirrors backend PodNameForService:
	// pod_<sanitized service ID> (unique per project + service).
	let podName = $derived.by(() => {
		if (service.podId && service.podId.trim()) {
			const raw = service.podId.trim();
			return raw.startsWith('pod_') ? raw : `pod_${sanitizePodSlug(raw)}`;
		}
		if (service.id && service.id.trim()) {
			return `pod_${sanitizePodSlug(service.id)}`;
		}
		const raw = service.name || 'pod-service';
		return `pod_${sanitizePodSlug(raw)}`;
	});

	// Pending flag with timestamp: a plain Deploy/Rebuild (not only the banner
	// Apply button) also applies the manifest, so the flag must clear whenever
	// a deployment finishes successfully AFTER the last edit.
	let pendingSince = $state(0);

	function pendingKey() {
		return `gopod_pod_pending_${service.id}`;
	}

	// Restore persistent pending recreation state from sessionStorage on mount / service change
	$effect(() => {
		if (typeof window !== 'undefined') {
			const raw = sessionStorage.getItem(pendingKey());
			if (raw) {
				manualChangePending = true;
				// Legacy format stored the string 'true' (unknown time -> 0).
				const t = raw === 'true' ? 0 : parseInt(raw, 10);
				pendingSince = isNaN(t) ? 0 : t;
			}
		}
	});

	function markPending() {
		manualChangePending = true;
		pendingSince = Date.now();
		if (typeof window !== 'undefined') {
			try {
				sessionStorage.setItem(pendingKey(), String(pendingSince));
			} catch {}
		}
	}

	function clearPending() {
		manualChangePending = false;
		pendingSince = 0;
		if (typeof window !== 'undefined') {
			sessionStorage.removeItem(pendingKey());
		}
	}

	// Child cards already persist composeYaml + k8sYaml. Parent only arms the banner.
	async function handleVisualChanged() {
		markPending();
	}

	// Find member services belonging to this Pod (strict, unit-tested;
	// see pod/pod-membership.ts — empty PodIDs never match).
	let memberServices = $derived.by(() => findPodMembers(service, dataStore.services, podName));

	// Candidate single-container services in the same project eligible for attachment (excluding compose stacks).
	// Orphaned flags (inPod without PodID) are included so they can be repaired.
	let candidateServices = $derived.by(() => {
		const podSlug = sanitizePodSlug(service.id || service.name);
		const expectedPod = `pod_${podSlug}`;
		const isOurs = (podId?: string) =>
			!!podId &&
			(podId === service.id ||
				podId === service.podId ||
				podId === service.name ||
				podId === expectedPod ||
				podId === podName ||
				`pod_${sanitizePodSlug(podId)}` === expectedPod ||
				`pod_${sanitizePodSlug(podId)}` === podName);
		return dataStore.services.filter(
			(s) =>
				s.projectId === service.projectId &&
				s.id !== service.id &&
				s.type !== 'pod' &&
				s.type !== 'compose' &&
				!isOurs(s.podId) &&
				(!s.inPod || !s.podId)
		);
	});

	// Detect if runtime Pod infra container port bindings drift from desired service configuration.
	// Compares normalized host:container/proto sets instead of a coarse has-port boolean.
	// Pure runtime evidence only (no manual flag): used both for display and auto-clear.
	let driftDetected = $derived.by(() => {
		if (service.status !== 'running') return false;

		const desiredPorts = service.composeYaml ? parseDetailedPorts(service.composeYaml) : [];
		const desiredSet = new Set(desiredPorts.map(normalizePortKey));
		if (desiredSet.size === 0 && (!service.hostPort || service.hostPort === 0)) {
			return false;
		}
		if (service.hostPort && service.port) {
			const fallbackIp = service.hostIp === '127.0.0.1' ? '127.0.0.1' : '0.0.0.0';
			desiredSet.add(`${fallbackIp}:${service.hostPort}:${service.port}`);
		}

		const activePod = dataStore.pods.find(
			(p) =>
				p.name === podName ||
				p.id === podName ||
				p.name === service.name ||
				p.name === `pod_${service.name}`
		);
		if (!activePod) return false;
		if (desiredSet.size === 0) return false;

		const activeContainers = dataStore.containers.filter(
			(c) =>
				c.pod === activePod.id ||
				c.pod === activePod.name ||
				c.name.includes(podName) ||
				activePod.containers.includes(c.name)
		);
		const actualTokens = new Set<string>();
		for (const c of activeContainers) {
			const raw = (c.ports || '').trim();
			if (!raw || raw === '-') continue;
			for (const token of raw.split(/[\s,]+/)) {
				const m = token.match(/(?:(\d{1,3}(?:\.\d{1,3}){3}):)?(\d+)->(\d+)\/(tcp|udp)/i);
				if (m) {
					const ip = m[1] ?? '0.0.0.0';
					const proto = m[4].toLowerCase() === 'udp' ? '/udp' : '';
					actualTokens.add(`${ip}:${m[2]}:${m[3]}${proto}`);
					continue;
				}
				const m2 = token.match(/(?:(\d{1,3}(?:\.\d{1,3}){3}):)?(\d+):(\d+)(\/(tcp|udp))?/i);
				if (m2) {
					const ip = m2[1] ?? '0.0.0.0';
					const proto = m2[5] && m2[5].toLowerCase() === 'udp' ? '/udp' : '';
					actualTokens.add(`${ip}:${m2[2]}:${m2[3]}${proto}`);
				}
			}
		}
		// No port evidence from runtime (e.g. infra-only pod whose container
		// list entry carries no Ports string): never claim drift without
		// positive evidence, otherwise the banner can never clear.
		if (actualTokens.size === 0) return false;
		if (actualTokens.size !== desiredSet.size) return true;
		for (const k of desiredSet) {
			if (!actualTokens.has(k)) return true;
		}

		return false;
	});

	let hasPendingChanges = $derived.by(() => manualChangePending || driftDetected);

	// Success signature: re-runs the auto-clear check whenever a new
	// successful deployment for this service lands in the store (any path:
	// banner Apply, Deploy, Rebuild, another tab/device).
	let successSig = $derived.by(() => {
		try {
			return dataStore.deployments
				.filter((d) => d.serviceId === service.id && d.status === 'success')
				.map((d) => d.id)
				.join(',');
		} catch {
			return '';
		}
	});

	// Auto-clear a stuck manual flag: if the manifest was applied by ANY
	// deployment path after the last edit and runtime shows no drift, the
	// banner must go. This is what makes hard-refresh safe: the flag
	// revalidates against the server instead of living forever in one tab.
	$effect(() => {
		if (!manualChangePending || driftDetected) return;
		successSig;
		let cancelled = false;
		(async () => {
			try {
				const deps = await api.services.deployments(service.id);
				if (cancelled) return;
				const applied = (deps || []).some((d) => {
					if (d.status !== 'success') return false;
					if (!d.finishedAt) return pendingSince === 0;
					const fin = Date.parse(d.finishedAt);
					if (isNaN(fin)) return pendingSince === 0;
					return fin >= pendingSince;
				});
				if (applied) {
					await dataStore.refreshRuntime();
					if (!cancelled && !driftDetected) clearPending();
				}
			} catch {
				// Keep the flag on errors: fail visible, not silent.
			}
		})();
		return () => {
			cancelled = true;
		};
	});

	async function handleApplyAndRecreate() {
		isDeploying = true;
		try {
			const dep: any = await dataStore.deployService(service.id);
			// Wait for the real terminal state instead of trusting the 202:
			// the banner must only clear when the infra actually owns the new bindings.
			let terminal: any = null;
			for (let i = 0; i < 150; i++) {
				await new Promise((r) => setTimeout(r, 2000));
				try {
					terminal = await api.services.getDeployment(dep.id);
				} catch {
					continue;
				}
				if (
					terminal &&
					(terminal.status === 'success' ||
						terminal.status === 'failed' ||
						terminal.status === 'cancelled')
				) {
					break;
				}
			}
			if (!terminal || terminal.status !== 'success') {
				throw new Error(
					terminal?.status === 'failed'
						? 'Pod recreation failed. Check the Deployments tab logs.'
						: 'Timed out waiting for pod recreation. Check the Deployments tab.'
				);
			}
			// Refresh live runtime AND the service record BEFORE clearing so drift
			// is recomputed from truth, not from pre-deploy snapshots.
			await dataStore.refreshRuntime();
			try {
				const fresh = await api.services.get(service.id);
				if (fresh) {
					const fIdx = dataStore.services.findIndex((s) => s.id === fresh.id);
					if (fIdx >= 0) dataStore.services[fIdx] = fresh;
				}
			} catch {
				// Non-fatal: runtime refresh above already updated drift inputs.
			}
			clearPending();
			toast.add({
				title: 'Pod Recreated',
				description: `Pod ${podName} was recreated with updated port bindings. Member containers restarted.`,
				variant: 'success'
			});
		} catch (err: any) {
			toast.add({
				title: 'Recreation Failed',
				description: err?.message || 'Error deploying pod',
				variant: 'error'
			});
		} finally {
			isDeploying = false;
		}
	}

	function handleDiscardPending() {
		clearPending();
		toast.add({
			title: 'Notice Dismissed',
			description: 'Pending recreation banner dismissed. Manifest changes remain saved in configuration.',
			variant: 'default'
		});
	}
</script>

<div class="flex w-full flex-col gap-5">
	<!-- Top Bar: View Mode Switcher (Visual <-> Kubernetes Manifest) -->
	<div class="flex flex-wrap items-center justify-between gap-3">
		<Tabs
			variant="segmented"
			size="sm"
			active={viewMode}
			onchange={(v: string) => (viewMode = v as 'visual' | 'yaml')}
			tabs={[
				{ id: 'visual', label: 'Visual' },
				{ id: 'yaml', label: 'Kubernetes Manifest (YAML)' }
			]}
		/>

		<span class="text-xs text-[var(--color-kumo-subtle)]">
			Engine: <span class="font-mono text-[var(--accent)]">Podman rootless pod</span>
		</span>
	</div>

	<!-- 1. Reactive Warning Banner: Pod Recreation Required (Kumo UI) -->
	{#if hasPendingChanges}
		<Banner variant="alert" class="shadow-xs">
			{#snippet icon()}
				<WarningCircle size={20} weight="fill" class="shrink-0 text-kumo-warning" />
			{/snippet}
			<div class="flex flex-col gap-1">
				<strong class="text-sm font-semibold text-kumo-warning">
					Pending configuration changes - Pod recreation required
				</strong>
				<p class="m-0 text-xs leading-relaxed text-kumo-warning/90">
					Host port bindings were updated in the service manifest. In Podman rootless architecture,
					port forwarding is bound to the pod infra container and cannot be modified dynamically
					while running. Applying will recreate the pod and restart all member containers.
				</p>
			</div>
			{#snippet action()}
				<div class="flex items-center gap-2">
					<Button
						variant="primary"
						size="sm"
						onclick={handleApplyAndRecreate}
						disabled={isDeploying}
						class="gap-1.5"
					>
						{#if isDeploying}
							<ArrowClockwise size={13} class="animate-spin" />
							<span>Recreating...</span>
						{:else}
							<span>Apply & Recreate Pod</span>
						{/if}
					</Button>
					<Button variant="ghost" size="sm" onclick={handleDiscardPending} disabled={isDeploying}>
						Dismiss
					</Button>
				</div>
			{/snippet}
		</Banner>
	{/if}

	<!-- 2. Informative banner (Localhost Network Namespace via Kumo UI) -->
	<Banner variant="default" size="sm" class="shadow-xs">
		{#snippet icon()}
			<Info size={18} weight="fill" class="shrink-0 text-[var(--accent)]" />
		{/snippet}
		<div class="text-xs">
			<span class="font-semibold text-[var(--color-kumo-default)]">Localhost network namespace:</span>
			<span class="text-[var(--color-kumo-subtle)]">
				Member containers inside this pod communicate directly with one another on
				<code
					class="rounded bg-[var(--bg-table-row-alt)] px-1.5 py-0.5 font-mono text-[0.9em] text-[var(--accent)]"
					>localhost:&lt;port&gt;</code
				>. Host ports are exposed exclusively at the Pod level.
			</span>
		</div>
	</Banner>

	<!-- 3. Workload View: Visual OR Kubernetes Manifest -->
	{#if viewMode === 'visual'}
		<!-- Card 1: Pod Infrastructure & Exposed Ports -->
		<PodPortForwardCard
			{service}
			{podName}
			{memberServices}
			onportschange={handleVisualChanged}
		/>

		<!-- Card 2: Pod Member Containers Table -->
		<PodMembersTable
			{memberServices}
			onOpenAddModal={() => (isAddingContainer = true)}
			onmemberchange={handleVisualChanged}
		/>
	{:else}
		<!-- Card: Kubernetes YAML Declarative Manifest Editor -->
		<PodKubernetesEditorCard
			{service}
			{podName}
			{memberServices}
			onmanifestchange={markPending}
		/>
	{/if}
</div>

<!-- Modal: Add or Attach Container to Pod -->
<PodAttachMemberModal
	bind:open={isAddingContainer}
	{service}
	{podName}
	{candidateServices}
	onclose={() => (isAddingContainer = false)}
	onmemberattached={handleVisualChanged}
/>
