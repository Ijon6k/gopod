<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore, projects } from '$lib/data';
	import type { Service, ServiceType } from '$lib/types';
	import WorkloadTypeSelector, { type WorkloadChoice } from './WorkloadTypeSelector.svelte';
	import { X, ArrowRight, FileText } from 'phosphor-svelte';

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
	let selectedProjectId = $derived(projectOverride ?? (projectId || (projects[0]?.id ?? 'aerochat')));
	let selectedType = $state<WorkloadChoice>(initialType);

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
		const internalId = `${targetProj}-${sSlug}`;
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
				port: 3000,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Application deployed from Git repository',
				envVars: [{ key: 'NODE_ENV', value: 'production', secret: false }],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
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
				createdAt: new Date().toISOString().split('T')[0]
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
					{ key: `${dbEngine.toUpperCase()}_PASSWORD`, value: 'generated_secret', secret: true }
				],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else {
			// Stack / Compose
			const realType: ServiceType = stackSubtype === 'kubernetes' ? 'kubernetes' : stackSubtype === 'pod' ? 'pod' : 'compose';
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: realType,
				status: 'stopped',
				source: stackSubtype === 'compose' ? 'compose.yaml' : stackSubtype === 'kubernetes' ? 'manifest.yaml' : '',
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
				createdAt: new Date().toISOString().split('T')[0]
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
		<div class="w-full max-w-[760px] flex flex-col gap-6">
			<div>
				<h1 class="text-xl font-medium text-[var(--text-primary)] m-0 mb-1">Create Service</h1>
				<p class="text-xs text-[var(--text-tertiary)] m-0">
					Select the service category and specify its name. Runtime, domains, and git source details are configured inside Service Detail.
				</p>
			</div>

			{#if !projectId && projects.length > 1}
				<div class="flex flex-col gap-1.5 max-w-[280px]">
					<label for="inline-select-project" class="text-xs text-[var(--text-secondary)] font-medium">Target Project</label>
					<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
						<select
							id="inline-select-project"
							value={selectedProjectId}
							onchange={(e) => (projectOverride = (e.target as HTMLSelectElement).value)}
							class="w-full bg-transparent border-0 outline-none text-xs text-[var(--text-primary)] font-[var(--font-sans)] cursor-pointer"
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
			<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-4">
				{#if selectedType === 'quadlet'}
					<div class="flex items-center gap-2.5 p-3 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-xs text-[var(--text-secondary)]">
						<FileText size={16} class="text-[var(--accent)] shrink-0" />
						<span>Generates a declarative systemd <code class="font-mono text-[var(--text-primary)]">.container</code> service unit in <code class="font-mono text-[var(--text-primary)]">~/.config/containers/systemd/</code>.</span>
					</div>
				{:else if selectedType === 'database'}
					<div class="flex flex-col gap-1.5">
						<label class="text-xs text-[var(--text-secondary)] font-medium">Database Engine</label>
						<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
							{#each [
								{ id: 'postgres', label: 'PostgreSQL' },
								{ id: 'redis', label: 'Redis' },
								{ id: 'mysql', label: 'MySQL' },
								{ id: 'mongodb', label: 'MongoDB' }
							] as db}
								<button
									type="button"
									onclick={() => (dbEngine = db.id as any)}
									class="p-2.5 rounded-[var(--radius-sm)] border text-xs text-center cursor-pointer transition-colors {dbEngine === db.id
										? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium'
										: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
								>
									{db.label}
								</button>
							{/each}
						</div>
					</div>
				{:else if selectedType === 'compose'}
					<div class="flex flex-col gap-2 p-3.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50">
						<label class="text-xs text-[var(--text-secondary)] font-medium">Workload Architecture & Manifest</label>
						<div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
							{#each [
								{ id: 'compose', label: 'Compose Stack', desc: 'Standard compose.yaml multi-service' },
								{ id: 'kubernetes', label: 'Kubernetes YAML', desc: 'Native podman play kube manifest' },
								{ id: 'pod', label: 'Podman Pod', desc: 'Multi-container shared localhost network' }
							] as s}
								<button
									type="button"
									onclick={() => (stackSubtype = s.id as any)}
									class="p-2.5 rounded-[var(--radius-sm)] border text-left cursor-pointer transition-colors {stackSubtype === s.id
										? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium shadow-xs'
										: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
								>
									<div class="font-semibold text-xs text-[var(--text-primary)]">{s.label}</div>
									<div class="text-[10px] text-[var(--text-tertiary)] mt-0.5 leading-tight">{s.desc}</div>
								</button>
							{/each}
						</div>
						{#if stackSubtype === 'pod'}
							<p class="text-[11px] text-[var(--text-tertiary)] m-0 mt-1">
								💡 Containers in a Pod share localhost IP and networking (e.g. Web + Redis sidecar). You can add additional containers to this Pod inside the Service Detail Studio.
							</p>
						{/if}
					</div>
				{/if}

				<div class="flex flex-col gap-1.5">
					<label for="inline-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
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

				<div class="flex items-center justify-end gap-2 pt-2 border-t border-[var(--border-subtle)]">
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button variant="primary" size="sm" disabled={!isFormValid} onclick={handleCreate}>
						Create & Configure <ArrowRight size={13} />
					</Button>
				</div>
			</div>
		</div>
	{:else}
		<!-- Modal Dialog View (Spacious layout with generous room) -->
		<div
			class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-xs"
			role="dialog"
			aria-modal="true"
		>
			<div
				class="w-full max-w-[660px] rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] shadow-2xl flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-150"
			>
				<div class="px-6 py-4 border-b border-[var(--border)] flex items-center justify-between">
					<div class="flex flex-col gap-0.5">
						<h3 class="text-base font-semibold text-[var(--text-primary)] m-0">Create Service</h3>
						<p class="text-xs text-[var(--text-tertiary)] m-0">Deploy an application, native systemd Quadlet, multi-container compose stack, or database.</p>
					</div>
					<button
						type="button"
						onclick={handleCancel}
						class="p-1 rounded text-[var(--text-tertiary)] hover:text-[var(--text-primary)] bg-transparent border-0 cursor-pointer"
						aria-label="Close"
					>
						<X size={16} />
					</button>
				</div>

				<div class="p-6 flex flex-col gap-5 max-h-[82vh] overflow-y-auto">
					<!-- Vertical Workload Type Selector -->
					<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

					{#if selectedType === 'quadlet'}
						<div class="flex items-center gap-2.5 p-3.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-xs text-[var(--text-secondary)]">
							<FileText size={16} class="text-[var(--accent)] shrink-0" />
							<span>Generates a declarative systemd <code class="font-mono text-[var(--text-primary)]">.container</code> service unit in <code class="font-mono text-[var(--text-primary)]">~/.config/containers/systemd/</code>.</span>
						</div>
					{:else if selectedType === 'database'}
						<div class="flex flex-col gap-2 p-3.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50">
							<label class="text-xs text-[var(--text-secondary)] font-medium">Database Engine</label>
							<div class="grid grid-cols-2 sm:grid-cols-4 gap-2">
								{#each [
									{ id: 'postgres', label: 'PostgreSQL' },
									{ id: 'redis', label: 'Redis' },
									{ id: 'mysql', label: 'MySQL' },
									{ id: 'mongodb', label: 'MongoDB' }
								] as db}
									<button
										type="button"
										onclick={() => (dbEngine = db.id as any)}
										class="p-2 rounded-md border text-xs text-center cursor-pointer transition-colors {dbEngine === db.id
											? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium'
											: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
									>
										{db.label}
									</button>
								{/each}
							</div>
						</div>
					{:else if selectedType === 'compose'}
						<div class="flex flex-col gap-2 p-3.5 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)]/50">
							<label class="text-xs text-[var(--text-secondary)] font-medium">Workload Architecture & Manifest</label>
							<div class="grid grid-cols-1 sm:grid-cols-3 gap-2">
								{#each [
									{ id: 'compose', label: 'Compose Stack', desc: 'Standard compose.yaml multi-service' },
									{ id: 'kubernetes', label: 'Kubernetes YAML', desc: 'Native podman play kube manifest' },
									{ id: 'pod', label: 'Podman Pod', desc: 'Multi-container shared localhost network' }
								] as s}
									<button
										type="button"
										onclick={() => (stackSubtype = s.id as any)}
										class="p-2.5 rounded-md border text-left cursor-pointer transition-colors {stackSubtype === s.id
											? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium shadow-xs'
											: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
									>
										<div class="font-semibold text-xs text-[var(--text-primary)]">{s.label}</div>
										<div class="text-[10px] text-[var(--text-tertiary)] mt-0.5 leading-tight">{s.desc}</div>
									</button>
								{/each}
							</div>
							{#if stackSubtype === 'pod'}
								<p class="text-[11px] text-[var(--text-tertiary)] m-0 mt-1">
									💡 Containers in a Pod share localhost IP and networking (e.g. Web + Redis sidecar). You can add additional containers to this Pod inside the Service Detail Studio.
								</p>
							{/if}
						</div>
					{/if}

					<!-- Service Name Input -->
					<div class="flex flex-col gap-1.5">
						<label for="modal-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
							Service Name <span class="text-[var(--status-red)]">*</span>
						</label>
						<Input
							id="modal-svc-name"
							bind:value={serviceName}
							placeholder="e.g. metube, web-app, or cache"
							class="text-xs"
						/>
						{#if selectedType === 'quadlet' && serviceName}
							<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">
								Target unit: ~/.config/containers/systemd/{slugify(serviceName)}.container
							</span>
						{/if}
					</div>

					{#if createError}
						<div class="rounded-md p-2.5 bg-[var(--status-red-subtle)] border border-[var(--status-red)] text-xs text-[var(--status-red)]">
							{createError}
						</div>
					{/if}
				</div>

				<div class="px-6 py-4 border-t border-[var(--border)] bg-[var(--bg-panel)] flex items-center justify-end gap-2">
					<Button variant="ghost" size="sm" onclick={handleCancel} disabled={isCreating}>Cancel</Button>
					<Button variant="primary" size="sm" disabled={!isFormValid || isCreating} onclick={handleCreate}>
						{#if isCreating}
							Creating…
						{:else}
							Create & Configure <ArrowRight size={13} />
						{/if}
					</Button>
				</div>
			</div>
		</div>
	{/if}
{/if}
