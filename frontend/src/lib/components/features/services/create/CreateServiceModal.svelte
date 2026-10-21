<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input, Field, Select, Banner } from '$lib/components/ui';
	import { Modal } from '$lib/components/ui';
	import { dataStore, projects } from '$lib/data';
	import type { Service, ServiceType } from '$lib/types';
	import WorkloadTypeSelector, { type WorkloadChoice } from './WorkloadTypeSelector.svelte';
	import { ArrowRight } from 'phosphor-svelte';
	import { serializePodK8sYaml } from '$lib/parsers/pod-ports';

	interface Props {
		projectId?: string;
		open?: boolean;
		inline?: boolean;
		initialType?: WorkloadChoice;
		onclose?: () => void;
	}

	let {
		projectId = '',
		open = $bindable(true),
		inline = false,
		initialType = 'application',
		onclose
	}: Props = $props();

	let projectOverride = $state<string | null>(null);
	let selectedProjectId = $derived(
		projectOverride ?? (projectId || (projects[0]?.id ?? 'aerochat'))
	);
	let selectedType = $state<WorkloadChoice>('application');

	$effect(() => {
		if (initialType) {
			selectedType = initialType;
		}
	});

	// Sub-options for Stack / Multi-Container
	let stackSubtype = $state<'compose' | 'pod'>('compose');

	let serviceName = $state('');
	let serviceDescription = $state('');
	let servicePort = $state('');
	let isCreating = $state(false);
	let createError = $state<string | null>(null);

	let isFormValid = $derived(serviceName.trim().length > 0);

	function slugify(text: string) {
		return text
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9_-]/g, '-')
			.replace(/-+/g, '-');
	}

	async function handleCreate() {
		if (!isFormValid || isCreating) return;
		isCreating = true;
		createError = null;

		const targetProj = selectedProjectId;
		const sName = serviceName.trim();
		const sSlug = slugify(sName);
		const randomSuffix =
			typeof crypto !== 'undefined' && crypto.getRandomValues
				? Array.from(crypto.getRandomValues(new Uint8Array(3)))
						.map((b) => b.toString(16).padStart(2, '0'))
						.join('')
				: Math.random().toString(36).substring(2, 8);
		const internalId = `${targetProj}-${sSlug}-${randomSuffix}`;
		const desc = serviceDescription.trim();

		let newService: Service;

		if (selectedType === 'application') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'application',
				status: 'stopped',
				source: '',
				sourceType: 'git',
				branch: 'main',
				buildType: 'dockerfile',
				runtimeTarget: 'standalone',
				autoDeploy: true,
				port: servicePort.trim() ? parseInt(servicePort.trim(), 10) || 0 : 0,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Application deployed from Git repository',
				envVars: [{ key: 'NODE_ENV', value: 'production', secret: false }],
				deployments: [],
				createdAt: new Date().toISOString()
			};
		} else if (selectedType === 'quadlet') {
			const defaultUnit = `[Unit]
Description=${sName} Quadlet Service
After=network-online.target

[Container]
Image=docker.io/library/nginx:alpine
PublishPort=8080:80
AutoUpdate=registry

[Service]
Restart=always

[Install]
WantedBy=default.target`;

			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'quadlet',
				sourceType: 'quadlet',
				status: 'stopped',
				source: `${sSlug}.container`,
				image: 'docker.io/library/nginx:alpine',
				port: 8080,
				hostPort: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Declarative systemd Quadlet service unit',
				quadletConfig: defaultUnit,
				runtimeTarget: 'quadlet',
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString()
			};
		} else if (selectedType === 'database') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'database',
				status: 'stopped',
				source: '',
				sourceType: 'image',
				image: '',
				port: 0,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Dedicated database instance',
				envVars: [],
				advanced: {
					runtime: { userNamespace: '', devices: [] },
					lifecycle: {
						restartPolicy: 'always'
					},
					security: {
						privileged: false,
						selinuxLabel: '',
						capAdd: [],
						capDrop: [],
						noNewPrivileges: true
					},
					storage: {
						volumes: [{ source: `${sSlug}-data`, target: '/data', options: 'Z' }]
					},
					resources: { pidsLimit: 0, swapLimit: '0' }
				},
				deployments: [],
				createdAt: new Date().toISOString()
			};
		} else {
			// Stack / Multi-Container (Compose or Podman Pod)
			// NOTE: Pod starts with ZERO published ports. The operator adds host
			// bindings explicitly in Visual/Manifest view; a hardcoded 8080:80
			// default would look like a phantom binding and can collide.
			const realType: ServiceType = stackSubtype === 'pod' ? 'pod' : 'compose';
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: realType,
				sourceType: realType,
				status: 'stopped',
				runtimeTarget: realType === 'pod' ? 'pod' : 'standalone',
				source: stackSubtype === 'compose' ? 'compose.yaml' : 'manifest.yaml',
				port: 0,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				composeYaml:
					realType === 'compose'
						? `version: "3.8"\nservices:\n  ${sSlug || 'app'}:\n    image: nginx:alpine\n    ports:\n      - "8080:80"\n    restart: always`
						: `version: "3.8"\nservices:\n  ${sSlug || 'pod'}:\n    restart: always\n`,
				k8sYaml: realType === 'pod' ? serializePodK8sYaml(sSlug || 'pod', []) : '',
				workloads:
					realType === 'pod'
						? []
						: [{ name: sSlug || 'app', image: 'nginx:alpine', status: 'stopped' }],
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString()
			};
		}

		try {
			await dataStore.addService(newService);
			open = false;
			onclose?.();
			await goto(`/projects/${targetProj}/services/${newService.id}`);
		} catch (err: any) {
			createError = err?.message || 'Failed to create service';
		} finally {
			isCreating = false;
		}
	}

	function handleCancel() {
		open = false;
		onclose?.();
		if (inline) {
			goto(projectId ? `/projects/${projectId}` : '/projects');
		}
	}
</script>

{#if open}
	{#if inline}
		<!-- Inline Page View -->
		<div class="flex w-full max-w-[760px] flex-col gap-6">
			<div>
				<h1 class="m-0 mb-1 text-xl font-medium text-[var(--text-primary)]">Create Service</h1>
				<p class="m-0 text-xs text-[var(--text-tertiary)]">
					Select a service type and choose a name.
				</p>
			</div>

			{#if createError}
				<Banner variant="error" size="sm">
					<div class="text-xs">
						<span class="font-medium">Failed to create service:</span>
						<span>{createError}</span>
					</div>
				</Banner>
			{/if}

			{#if !projectId && projects.length > 1}
				<div class="max-w-[280px]">
					<Select
						id="inline-select-project"
						label="Target Project"
						value={selectedProjectId}
						options={projects.map((p) => ({ label: p.name, value: p.id }))}
						onValueChange={(val) => {
							if (typeof val === 'string') projectOverride = val;
						}}
						size="sm"
					/>
				</div>
			{/if}

			<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

			<!-- Details card -->
			<div
				class="flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5"
			>
				{#if selectedType === 'compose'}
					<div
						class="flex flex-col gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50 p-3.5"
					>
						<span class="text-xs font-medium text-[var(--text-secondary)]"
							>Workload Architecture & Manifest</span
						>
						<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
							{#each [{ id: 'compose', label: 'Compose Stack', desc: 'Standard compose.yaml multi-service' }, { id: 'kubernetes', label: 'Kubernetes YAML', desc: 'Native podman play kube manifest' }, { id: 'pod', label: 'Podman Pod', desc: 'Multi-container shared localhost network' }] as s}
								<button
									type="button"
									onclick={() => (stackSubtype = s.id as any)}
									class="cursor-pointer rounded-[var(--radius-sm)] border p-2.5 text-left transition-colors {stackSubtype ===
									s.id
										? 'border-[var(--accent)] bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
										: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
								>
									<div class="text-xs font-semibold text-[var(--text-primary)]">{s.label}</div>
									<div class="mt-0.5 text-[10px] leading-tight text-[var(--text-tertiary)]">
										{s.desc}
									</div>
								</button>
							{/each}
						</div>
						{#if stackSubtype === 'pod'}
							<Banner variant="default" size="sm" class="mt-2">
								<div class="text-xs">
									<span class="font-semibold text-[var(--color-kumo-default)]"
										>Shared localhost namespace:</span
									>
									<span class="text-[var(--color-kumo-subtle)]">
										Containers in a Pod share the network namespace and communicate on <code
											class="rounded bg-black/10 px-1 py-0.5 font-mono text-[0.9em]"
											>localhost:&lt;port&gt;</code
										>. Additional containers can be attached in the Service Detail Studio.
									</span>
								</div>
							</Banner>
						{/if}
					</div>
				{/if}

				<!-- Universal Service Name & Description Fields -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
					<div class="flex flex-col gap-1.5">
						<label for="inline-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
							Service Name <span class="text-[var(--status-red)]">*</span>
						</label>
						<Input
							id="inline-svc-name"
							bind:value={serviceName}
							placeholder={selectedType === 'database'
								? 'e.g. production-db or cache'
								: 'e.g. system-daemon, api-gateway, or cache'}
							class="text-xs"
						/>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="inline-svc-desc" class="text-xs font-medium text-[var(--text-secondary)]">
							Description (Optional)
						</label>
						<Input
							id="inline-svc-desc"
							bind:value={serviceDescription}
							placeholder={selectedType === 'quadlet'
								? 'Declarative systemd Quadlet service unit'
								: selectedType === 'database'
									? 'Dedicated database instance'
									: 'Service description'}
							class="text-xs"
						/>
					</div>
				</div>

				{#if selectedType === 'application'}
					<div class="flex flex-col gap-1.5">
						<label for="inline-svc-port" class="text-xs font-medium text-[var(--text-secondary)]">
							Container Port (Optional)
						</label>
						<Input
							id="inline-svc-port"
							bind:value={servicePort}
							placeholder="3000 (e.g. 80, 8080)"
							class="text-xs font-[var(--font-mono)]"
						/>
						<span class="text-[11px] text-[var(--text-tertiary)]">
							Port where your app listens inside the container.
						</span>
					</div>
				{:else if selectedType === 'quadlet'}
					<Banner variant="default" size="sm">
						<div class="text-xs">
							<span class="font-semibold text-[var(--color-kumo-default)]"
								>Systemd Quadlet specification:</span
							>
							<span class="text-[var(--color-kumo-subtle)]">
								Native systemd unit files are managed in <code
									class="rounded bg-black/10 px-1 py-0.5 font-mono text-[0.9em]"
									>~/.config/containers/systemd</code
								>. Storage volumes and restart policies are configured in Service Detail.
							</span>
						</div>
					</Banner>
				{/if}

				<div
					class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2"
				>
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button
						variant="primary"
						size="sm"
						disabled={!isFormValid || isCreating}
						onclick={handleCreate}
					>
						Create & Configure <ArrowRight size={13} />
					</Button>
				</div>
			</div>
		</div>
	{:else}
		<!-- Clean, Modular Modal Dialog View -->
		<Modal
			open={true}
			onclose={handleCancel}
			title="Create Service"
			subtitle="Select a service type and choose a name."
			size="lg"
		>
			<!-- Vertical Workload Type Selector -->
			<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

			{#if selectedType === 'compose'}
				<div
					class="flex flex-col gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50 p-3.5"
				>
					<span class="text-xs font-medium text-[var(--text-secondary)]"
						>Workload Architecture & Manifest</span
					>
					<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
						{#each [{ id: 'compose', label: 'Compose Stack', desc: 'Standard compose.yaml multi-service on bridge network' }, { id: 'pod', label: 'Podman Pod', desc: 'Kubernetes-compatible Pod sharing localhost network namespace' }] as s (s.id)}
							<button
								type="button"
								onclick={() => (stackSubtype = s.id as any)}
								class="cursor-pointer rounded-md border p-2.5 text-left transition-colors {stackSubtype ===
								s.id
									? 'border-[var(--accent)] bg-[var(--bg-surface)] font-medium text-[var(--text-primary)] shadow-xs'
									: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
							>
								<div class="text-xs font-semibold text-[var(--text-primary)]">{s.label}</div>
								<div class="mt-0.5 text-[10px] leading-tight text-[var(--text-tertiary)]">
									{s.desc}
								</div>
							</button>
						{/each}
					</div>
					{#if stackSubtype === 'pod'}
						<Banner variant="default" size="sm" class="mt-2">
							<div class="text-xs">
								<span class="font-semibold text-[var(--color-kumo-default)]"
									>Shared localhost namespace & Kubernetes parity:</span
								>
								<span class="text-[var(--color-kumo-subtle)]">
									Containers in a Pod share the network namespace and communicate on <code
										class="rounded bg-black/10 px-1 py-0.5 font-mono text-[0.9em]"
										>localhost:&lt;port&gt;</code
									>. Manage this workload visually in the Studio or declaratively using Kubernetes
									Manifest (<code class="rounded bg-black/10 px-1 py-0.5 font-mono text-[0.9em]"
										>apiVersion: v1, kind: Pod</code
									>).
								</span>
							</div>
						</Banner>
					{/if}
				</div>
			{/if}

			<!-- Universal Service Name & Description Fields in Modal -->
			<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
				<Field label="Service Name" required error={createError || undefined}>
					<Input
						id="modal-svc-name"
						bind:value={serviceName}
						placeholder="e.g. system-daemon, api-gateway, or cache"
						class="text-xs"
					/>
				</Field>

				<Field label="Description (Optional)">
					<Input
						id="modal-svc-desc"
						bind:value={serviceDescription}
						placeholder={selectedType === 'quadlet'
							? 'Declarative systemd Quadlet service unit'
							: 'Service description'}
						class="text-xs"
					/>
				</Field>
			</div>

			{#if selectedType === 'application'}
				<Field
					label="Container Port (Optional)"
					description="Internal listening port inside container (e.g. 80, 8080). Leave blank if this is a background worker or non-HTTP service."
				>
					<Input
						id="modal-svc-port"
						bind:value={servicePort}
						placeholder="e.g. 80, 8080 (optional)"
						class="text-xs font-[var(--font-mono)]"
					/>
				</Field>
			{:else if selectedType === 'quadlet'}
				<Banner variant="default" size="sm">
					<div class="text-xs">
						<span class="font-semibold text-[var(--color-kumo-default)]"
							>Systemd Quadlet specification:</span
						>
						<span class="text-[var(--color-kumo-subtle)]">
							Native systemd unit files are managed in <code
								class="rounded bg-black/10 px-1 py-0.5 font-mono text-[0.9em]"
								>~/.config/containers/systemd</code
							>. Storage volumes and restart policies are configured in Service Detail.
						</span>
					</div>
				</Banner>
			{/if}

			{#snippet footer()}
				<Button variant="ghost" size="sm" onclick={handleCancel} disabled={isCreating}
					>Cancel</Button
				>
				<Button
					variant="primary"
					size="sm"
					disabled={!isFormValid || isCreating}
					onclick={handleCreate}
				>
					{#if isCreating}
						Creating…
					{:else}
						Create & Configure <ArrowRight size={13} />
					{/if}
				</Button>
			{/snippet}
		</Modal>
	{/if}
{/if}
