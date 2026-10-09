<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input, FormField } from '$lib/components/primitives';
	import { Modal } from '$lib/components/ui';
	import { dataStore, projects } from '$lib/data';
	import type { Service, ServiceType } from '$lib/types';
	import WorkloadTypeSelector, { type WorkloadChoice } from './WorkloadTypeSelector.svelte';
	import { ArrowRight, FileText } from 'phosphor-svelte';

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

	// Sub-options
	let stackSubtype = $state<'compose' | 'kubernetes' | 'pod'>('compose');
	let dbEngine = $state<'postgres' | 'redis' | 'mysql' | 'mongodb'>('postgres');

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
				port: parseInt(servicePort.trim(), 10) || 3000,
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
			let defaultUnit = `[Unit]\nDescription=${sName} Service\nAfter=network-online.target\n\n[Container]\nImage=docker.io/library/nginx:alpine\nPublishPort=8080:80\nAutoUpdate=registry\nRestart=always\n\n[Service]\nRestart=always\n\n[Install]\nWantedBy=default.target`;

			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'quadlet',
				status: 'stopped',
				source: `${sSlug}.container`,
				image: 'docker.io/library/nginx:alpine',
				port: 8080,
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
			const dbMap: Record<string, { image: string; port: number }> = {
				postgres: { image: 'docker.io/library/postgres:17-alpine', port: 5432 },
				redis: { image: 'docker.io/library/redis:7-alpine', port: 6379 },
				mysql: { image: 'docker.io/library/mysql:8.4', port: 3306 },
				mongodb: { image: 'docker.io/library/mongo:7.0', port: 27017 }
			};
			const selectedDb = dbMap[dbEngine] ?? dbMap.postgres;

			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'image',
				status: 'stopped',
				source: '',
				sourceType: 'image',
				image: selectedDb.image,
				port: selectedDb.port,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || `${dbEngine.toUpperCase()} database preset`,
				envVars: [
					{
						key: `${dbEngine.toUpperCase()}_PASSWORD`,
						value:
							typeof crypto !== 'undefined' && crypto.randomUUID
								? crypto.randomUUID().replace(/-/g, '').slice(0, 20)
								: Math.random().toString(36).slice(2, 12),
						secret: true
					}
				],
				deployments: [],
				createdAt: new Date().toISOString()
			};
		} else {
			// Stack / Compose
			const realType: ServiceType =
				stackSubtype === 'kubernetes' ? 'kubernetes' : stackSubtype === 'pod' ? 'pod' : 'compose';
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: realType,
				status: 'stopped',
				source:
					stackSubtype === 'compose'
						? 'compose.yaml'
						: stackSubtype === 'kubernetes'
							? 'manifest.yaml'
							: '',
				port: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || `Multi-container ${stackSubtype} stack`,
				composeYaml: `version: "3.8"\nservices:\n  ${sSlug || 'app'}:\n    image: nginx:alpine\n    ports:\n      - "8080:80"\n    restart: always`,
				k8sYaml: `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${sSlug}\nspec:\n  containers:\n    - name: main\n      image: nginx:alpine\n      ports:\n        - containerPort: 80`,
				workloads: [{ name: sSlug || 'app', image: 'nginx:alpine', status: 'stopped' }],
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
					Select the service category and specify its name. Runtime, domains, and git source details
					are configured inside Service Detail.
				</p>
			</div>

			{#if !projectId && projects.length > 1}
				<div class="flex max-w-[280px] flex-col gap-1.5">
					<label
						for="inline-select-project"
						class="text-xs font-medium text-[var(--text-secondary)]">Target Project</label
					>
					<div
						class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
					>
						<select
							id="inline-select-project"
							value={selectedProjectId}
							onchange={(e) => (projectOverride = (e.target as HTMLSelectElement).value)}
							class="w-full cursor-pointer border-0 bg-transparent text-xs font-[var(--font-sans)] text-[var(--text-primary)] outline-none"
						>
							{#each projects as p}
								<option value={p.id}>{p.name}</option>
							{/each}
						</select>
					</div>
				</div>
			{/if}

			<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

			<!-- Details card -->
			<div
				class="flex flex-col gap-4 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] p-5"
			>
				{#if selectedType === 'quadlet'}
					<div
						class="flex items-center gap-2.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 text-xs text-[var(--text-secondary)]"
					>
						<FileText size={16} class="shrink-0 text-[var(--accent)]" />
						<span
							>Generates a declarative systemd <code class="font-mono text-[var(--text-primary)]"
								>.container</code
							>
							service unit in
							<code class="font-mono text-[var(--text-primary)]">~/.config/containers/systemd/</code
							>.</span
						>
					</div>
				{:else if selectedType === 'database'}
					<div class="flex flex-col gap-1.5">
						<span class="text-xs font-medium text-[var(--text-secondary)]">Database Engine</span>
						<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
							{#each [{ id: 'postgres', label: 'PostgreSQL' }, { id: 'redis', label: 'Redis' }, { id: 'mysql', label: 'MySQL' }, { id: 'mongodb', label: 'MongoDB' }] as db}
								<button
									type="button"
									onclick={() => (dbEngine = db.id as any)}
									class="cursor-pointer rounded-[var(--radius-sm)] border p-2.5 text-center text-xs transition-colors {dbEngine ===
									db.id
										? 'border-[var(--accent)] bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
										: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
								>
									{db.label}
								</button>
							{/each}
						</div>
					</div>
				{:else if selectedType === 'compose'}
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
							<p class="m-0 mt-1 text-[11px] text-[var(--text-tertiary)]">
								💡 Containers in a Pod share localhost IP and networking (e.g. Web + Redis sidecar).
								You can add additional containers to this Pod inside the Service Detail Studio.
							</p>
						{/if}
					</div>
				{/if}

				<div class="flex flex-col gap-1.5">
					<label for="inline-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
						Service Name <span class="text-[var(--status-red)]">*</span>
					</label>
					<Input
						id="inline-svc-name"
						bind:value={serviceName}
						placeholder="e.g. metube, api-gateway, or cache"
						class="text-xs"
					/>
					{#if selectedType === 'quadlet' && serviceName}
						<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">
							Target unit: ~/.config/containers/systemd/{slugify(serviceName)}.container
						</span>
					{/if}
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
				{/if}

				<div
					class="flex items-center justify-end gap-2 border-t border-[var(--border-subtle)] pt-2"
				>
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button variant="primary" size="sm" disabled={!isFormValid} onclick={handleCreate}>
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
			subtitle="Deploy an application, native systemd Quadlet, multi-container compose stack, or database."
			size="xl"
		>
			<!-- Vertical Workload Type Selector -->
			<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

			{#if selectedType === 'quadlet'}
				<div
					class="flex items-center gap-2.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3.5 text-xs text-[var(--text-secondary)]"
				>
					<FileText size={16} class="shrink-0 text-[var(--accent)]" />
					<span
						>Generates a declarative systemd <code class="font-mono text-[var(--text-primary)]"
							>.container</code
						>
						service unit in
						<code class="font-mono text-[var(--text-primary)]">~/.config/containers/systemd/</code
						>.</span
					>
				</div>
			{:else if selectedType === 'database'}
				<div
					class="flex flex-col gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50 p-3.5"
				>
					<span class="text-xs font-medium text-[var(--text-secondary)]">Database Engine</span>
					<div class="grid grid-cols-2 gap-2 sm:grid-cols-4">
						{#each [{ id: 'postgres', label: 'PostgreSQL' }, { id: 'redis', label: 'Redis' }, { id: 'mysql', label: 'MySQL' }, { id: 'mongodb', label: 'MongoDB' }] as db (db.id)}
							<button
								type="button"
								onclick={() => (dbEngine = db.id as any)}
								class="cursor-pointer rounded-md border p-2 text-center text-xs transition-colors {dbEngine ===
								db.id
									? 'border-[var(--accent)] bg-[var(--bg-surface)] font-medium text-[var(--text-primary)]'
									: 'border-[var(--border)] bg-[var(--bg-panel)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
							>
								{db.label}
							</button>
						{/each}
					</div>
				</div>
			{:else if selectedType === 'compose'}
				<div
					class="flex flex-col gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50 p-3.5"
				>
					<span class="text-xs font-medium text-[var(--text-secondary)]"
						>Workload Architecture & Manifest</span
					>
					<div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
						{#each [{ id: 'compose', label: 'Compose Stack', desc: 'Standard compose.yaml multi-service' }, { id: 'kubernetes', label: 'Kubernetes YAML', desc: 'Native podman play kube manifest' }, { id: 'pod', label: 'Podman Pod', desc: 'Multi-container shared localhost network' }] as s (s.id)}
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
						<p class="m-0 mt-1 text-[11px] text-[var(--text-tertiary)]">
							💡 Containers in a Pod share localhost IP and networking (e.g. Web + Redis sidecar).
							You can add additional containers to this Pod inside the Service Detail Studio.
						</p>
					{/if}
				</div>
			{/if}

			<!-- Service Name FormField -->
			<FormField
				label="Service Name"
				required
				error={createError}
				description={selectedType === 'quadlet' && serviceName
					? `Target unit: ~/.config/containers/systemd/${slugify(serviceName)}.container`
					: undefined}
			>
				<Input
					id="modal-svc-name"
					bind:value={serviceName}
					placeholder="e.g. metube, web-app, or cache"
					class="text-xs"
				/>
			</FormField>

			{#if selectedType === 'application'}
				<FormField
					label="Container Port (Optional)"
					description="Internal listening port inside container (default: 3000, e.g. 80 for Whoami/Nginx)."
				>
					<Input
						id="modal-svc-port"
						bind:value={servicePort}
						placeholder="3000 (e.g. 80, 8080)"
						class="text-xs font-[var(--font-mono)]"
					/>
				</FormField>
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
