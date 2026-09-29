<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { Button, Input, SectionCard } from '$lib/components/primitives';
	import { CodeEditor, StatusBadge } from '$lib/components/ui';
	import { GitBranch, PencilSimple, Check, ArrowRight, ArrowClockwise, Plus, Trash } from 'phosphor-svelte';

	interface Props {
		service: Service;
		onNavigateTab?: (tabId: string) => void;
	}

	let { service, onNavigateTab }: Props = $props();

	// Edit states for common properties
	let editingSource = $state(false);
	let editingBuild = $state(false);
	let editingImage = $state(false);

	let repoVal = $state('');
	let branchVal = $state('main');
	let dockerfilePath = $state('./Dockerfile');
	let buildContext = $state('./');
	let imageVal = $state('');
	let commandVal = $state('');

	// Deployment triggers
	let webhookEnabled = $state(true);
	let autoUpdateImage = $state(false);

	// Compose / K8s / Quadlet editor content
	let composeYaml = $state('');
	let k8sYaml = $state('');
	let quadletConfig = $state('');
	let composeSaved = $state(true);

	$effect(() => {
		repoVal = service.source || '';
		branchVal = service.branch || 'main';
		imageVal = service.image || '';
		commandVal = service.command || '';
		composeYaml =
			service.composeYaml ||
			`services:
  web:
    image: nginx:alpine
    ports:
      - "${service.port || 80}:80"
    restart: always`;
		k8sYaml =
			service.k8sYaml ||
			`apiVersion: v1
kind: Pod
metadata:
  name: ${service.name}
spec:
  containers:
    - name: main
      image: ${service.image || 'nginx:alpine'}
      ports:
        - containerPort: ${service.port || 80}`;
		quadletConfig =
			service.quadletConfig ||
			`[Unit]
Description=${service.name} Quadlet Service
After=network-online.target

[Container]
Image=${service.image || 'docker.io/library/nginx:alpine'}
PublishPort=${service.port || 8080}:80
AutoUpdate=registry

[Service]
Restart=always

[Install]
WantedBy=default.target`;
	});

	function saveSource() {
		service.source = repoVal;
		service.branch = branchVal;
		editingSource = false;
	}

	function saveImage() {
		service.image = imageVal;
		service.command = commandVal || undefined;
		editingImage = false;
	}

	function saveCompose() {
		service.composeYaml = composeYaml;
		composeSaved = true;
	}

	function addPodContainer() {
		if (!service.workloads) service.workloads = [];
		service.workloads.push({
			name: `worker-${service.workloads.length + 1}`,
			image: 'docker.io/library/alpine:latest',
			status: 'running',
			cpu: 0.1,
			memory: 32
		});
	}

	function removePodContainer(idx: number) {
		if (service.workloads && service.workloads.length > 1) {
			service.workloads.splice(idx, 1);
		}
	}
</script>

<div class="w-full flex flex-col gap-6">
	<!-- ══════════════════════════════════════════════════════════════
	     1. APPLICATION WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{#if service.type === 'application'}
		<!-- Source Section -->
		<SectionCard title="Source Repository">
			{#snippet headerActions()}
				{#if !editingSource}
					<Button variant="secondary" size="sm" onclick={() => (editingSource = true)}>
						<PencilSimple size={13} /> Change
					</Button>
				{:else}
					<Button variant="primary" size="sm" onclick={saveSource}>
						<Check size={13} /> Done
					</Button>
				{/if}
			{/snippet}

			{#if !editingSource}
				<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-2 pt-1 text-xs">
					<div class="flex flex-col gap-0.5">
						<span class="text-[var(--text-tertiary)]">Repository</span>
						<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{service.source || '—'}</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-[var(--text-tertiary)]">Branch</span>
						<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{service.branch || 'main'}</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-[var(--text-tertiary)]">Commit</span>
						<span class="font-[var(--font-mono)] text-[var(--text-secondary)]">main · 4b89c02</span>
					</div>
				</div>
			{:else}
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1">
					<div class="sm:col-span-2 flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Git URL</span>
						<Input bind:value={repoVal} placeholder="github.com/org/repo" class="font-[var(--font-mono)] text-xs" />
					</div>
					<div class="flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Branch</span>
						<Input bind:value={branchVal} placeholder="main" class="font-[var(--font-mono)] text-xs" />
					</div>
				</div>
			{/if}
		</SectionCard>

		<!-- Build Section -->
		<SectionCard title="Build Configuration">
			{#snippet headerActions()}
				{#if !editingBuild}
					<Button variant="secondary" size="sm" onclick={() => (editingBuild = true)}>
						<PencilSimple size={13} /> Edit
					</Button>
				{:else}
					<Button variant="primary" size="sm" onclick={() => (editingBuild = false)}>
						<Check size={13} /> Done
					</Button>
				{/if}
			{/snippet}

			<div class="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1 text-xs">
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Build Method</span>
					<span class="text-[var(--text-primary)] font-medium">Dockerfile</span>
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Dockerfile Path</span>
					{#if !editingBuild}
						<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{dockerfilePath}</span>
					{:else}
						<Input bind:value={dockerfilePath} class="font-[var(--font-mono)] text-xs py-1" />
					{/if}
				</div>
				<div class="flex flex-col gap-0.5">
					<span class="text-[var(--text-tertiary)]">Build Context</span>
					{#if !editingBuild}
						<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{buildContext}</span>
					{:else}
						<Input bind:value={buildContext} class="font-[var(--font-mono)] text-xs py-1" />
					{/if}
				</div>
			</div>
		</SectionCard>

		<!-- Deployment Triggers Section -->
		<SectionCard title="Deployment Triggers">
			<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1">
				<div class="flex items-start justify-between p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]">
					<div class="flex flex-col gap-0.5">
						<span class="text-xs font-medium text-[var(--text-primary)]">Git Push Webhook</span>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Automatically redeploy when new commits are pushed to {service.branch || 'main'}.
						</span>
					</div>
					<input
						type="checkbox"
						bind:checked={webhookEnabled}
						class="mt-1 accent-[var(--accent)] cursor-pointer"
					/>
				</div>

				<div class="flex items-start justify-between p-3 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)]">
					<div class="flex flex-col gap-0.5">
						<span class="text-xs font-medium text-[var(--text-primary)]">Podman Image Auto-Update</span>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Periodic systemd timer pulls newer image tags when available.
						</span>
					</div>
					<input
						type="checkbox"
						bind:checked={autoUpdateImage}
						class="mt-1 accent-[var(--accent)] cursor-pointer"
					/>
				</div>
			</div>
		</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     2. CONTAINER IMAGE WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{:else if service.type === 'image'}
		<SectionCard title="Image & Execution">
			{#snippet headerActions()}
				{#if !editingImage}
					<Button variant="secondary" size="sm" onclick={() => (editingImage = true)}>
						<PencilSimple size={13} /> Edit
					</Button>
				{:else}
					<Button variant="primary" size="sm" onclick={saveImage}>
						<Check size={13} /> Done
					</Button>
				{/if}
			{/snippet}

			{#if !editingImage}
				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-1 text-xs">
					<div class="flex flex-col gap-0.5">
						<span class="text-[var(--text-tertiary)]">Image Reference</span>
						<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{service.image || '—'}</span>
					</div>
					<div class="flex flex-col gap-0.5">
						<span class="text-[var(--text-tertiary)]">Command / Entrypoint</span>
						<span class="font-[var(--font-mono)] text-[var(--text-secondary)]">{service.command || 'Default entrypoint'}</span>
					</div>
				</div>
			{:else}
				<div class="flex flex-col gap-3 pt-1">
					<div class="flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Image reference</span>
						<Input bind:value={imageVal} placeholder="registry/repo:tag" class="font-[var(--font-mono)] text-xs" />
					</div>
					<div class="flex flex-col gap-1">
						<span class="text-[11px] text-[var(--text-tertiary)]">Command override</span>
						<Input bind:value={commandVal} placeholder="e.g. server --port 8080" class="font-[var(--font-mono)] text-xs" />
					</div>
				</div>
			{/if}
		</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     3. COMPOSE WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{:else if service.type === 'compose'}
		<SectionCard
			title="Compose Specification (compose.yaml)"
			description="Source of truth for this multi-container workload"
		>
			{#snippet headerActions()}
				<Button
					variant="primary"
					size="sm"
					disabled={composeSaved}
					onclick={saveCompose}
				>
					Save YAML
				</Button>
			{/snippet}

			<CodeEditor
				bind:value={composeYaml}
				language="yaml"
				height="320px"
				onchange={() => (composeSaved = false)}
			/>

			<!-- Parsed Workload Summary -->
			<div class="mt-2 pt-3 border-t border-[var(--border-subtle)] flex flex-col gap-2">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Declared Services</span>
				<div class="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
					{#each service.workloads ?? [{ name: 'web', image: 'nginx:alpine', status: 'running' }] as w}
						<div class="p-2.5 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] flex items-center justify-between">
							<div class="flex flex-col gap-0.5 min-w-0">
								<span class="text-xs font-medium text-[var(--text-primary)] truncate">{w.name}</span>
								<span class="font-[var(--font-mono)] text-[10.5px] text-[var(--text-tertiary)] truncate">{w.image}</span>
							</div>
							<StatusBadge status={w.status} size="sm" />
						</div>
					{/each}
				</div>
			</div>
		</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     4. PODMAN POD WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{:else if service.type === 'pod'}
		<SectionCard
			title="Pod Members & Identity"
			description="Containers co-located in Podman Pod {service.projectId}-{service.name} sharing localhost and cgroups."
		>
			{#snippet headerActions()}
				<Button variant="secondary" size="sm" onclick={addPodContainer}>
					<Plus size={13} /> Add Container
				</Button>
			{/snippet}

			<div class="flex flex-col divide-y divide-[var(--border-subtle)] border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] overflow-hidden">
				{#each service.workloads ?? [] as w, idx}
					<div class="flex items-center justify-between p-3 gap-3">
						<div class="flex-1 grid grid-cols-1 sm:grid-cols-3 gap-3 items-center">
							<div class="flex flex-col gap-0.5">
								<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Container Name</span>
								<span class="text-xs font-medium text-[var(--text-primary)] font-[var(--font-mono)]">{w.name}</span>
							</div>
							<div class="flex flex-col gap-0.5">
								<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Image</span>
								<span class="text-xs text-[var(--text-secondary)] font-[var(--font-mono)] truncate">{w.image}</span>
							</div>
							<div class="flex flex-col gap-0.5">
								<span class="text-[10px] text-[var(--text-tertiary)] uppercase font-medium">Status</span>
								<StatusBadge status={w.status} size="sm" />
							</div>
						</div>

						{#if (service.workloads?.length ?? 0) > 1}
							<button
								type="button"
								onclick={() => removePodContainer(idx)}
								class="text-[var(--text-tertiary)] hover:text-[var(--status-red)] p-1 bg-transparent border-0 cursor-pointer"
								title="Remove container"
							>
								<Trash size={14} />
							</button>
						{/if}
					</div>
				{/each}
			</div>
		</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     5. KUBERNETES MANIFEST WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{:else if service.type === 'kubernetes'}
		<SectionCard
			title="Kubernetes Manifest (kube play)"
			description="Native Podman manifest execution"
		>
			{#snippet headerActions()}
				<Button variant="primary" size="sm" onclick={() => (service.k8sYaml = k8sYaml)}>
					Save Manifest
				</Button>
			{/snippet}

			<CodeEditor bind:value={k8sYaml} language="yaml" height="320px" />
		</SectionCard>

	<!-- ══════════════════════════════════════════════════════════════
	     6. QUADLET WORKLOAD
	     ══════════════════════════════════════════════════════════════ -->
	{:else if service.type === 'quadlet'}
		<SectionCard
			title="Quadlet Unit Definition"
			description="Declarative systemd generator unit"
		>
			{#snippet headerActions()}
				<Button variant="primary" size="sm" onclick={() => (service.quadletConfig = quadletConfig)}>
					Save Quadlet
				</Button>
			{/snippet}

			<CodeEditor bind:value={quadletConfig} language="quadlet" height="320px" />
		</SectionCard>
	{/if}

	<!-- ══════════════════════════════════════════════════════════════
	     RUNTIME SUMMARY (All types — linking to Advanced)
	     ══════════════════════════════════════════════════════════════ -->
	<SectionCard title="Podman Runtime Summary">
		{#snippet headerActions()}
			<Button variant="ghost" size="sm" onclick={() => onNavigateTab?.('advanced')}>
				Advanced configuration <ArrowRight size={13} />
			</Button>
		{/snippet}

		<div class="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-1 text-xs">
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Execution Mode</span>
				<span class="text-[var(--text-primary)] font-medium">Rootless (Standard)</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Restart Policy</span>
				<span class="font-[var(--font-mono)] text-[var(--text-primary)]">{service.restartPolicy || 'always'}</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Lifecycle Integration</span>
				<span class="text-[var(--text-primary)]">systemd / quadlet</span>
			</div>
			<div class="flex flex-col gap-0.5">
				<span class="text-[var(--text-tertiary)]">Internal Port</span>
				<span class="font-[var(--font-mono)] text-[var(--text-primary)]">:{service.port || 3000}</span>
			</div>
		</div>
	</SectionCard>
</div>
