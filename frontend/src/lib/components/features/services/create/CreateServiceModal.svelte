<script lang="ts">
	import { goto } from '$app/navigation';
	import { Button, Input } from '$lib/components/primitives';
	import { dataStore, projects } from '$lib/data';
	import type { Service, ServiceType, Workload } from '$lib/types';
	import WorkloadTypeSelector, { type WorkloadChoice } from './WorkloadTypeSelector.svelte';
	import { X, Plus, ArrowRight } from 'phosphor-svelte';

	interface Props {
		projectId?: string;
		open?: boolean;
		inline?: boolean;
		onclose?: () => void;
	}

	let {
		projectId = '',
		open = $bindable(true),
		inline = false,
		onclose
	}: Props = $props();

	let projectOverride = $state<string | null>(null);
	let selectedProjectId = $derived(projectOverride ?? (projectId || (projects[0]?.id ?? 'aerochat')));
	let selectedType = $state<WorkloadChoice>('application');

	let serviceName = $state('');
	let serviceDescription = $state('');
	let dbEngine = $state<'postgres' | 'redis' | 'mysql' | 'mongodb'>('postgres');

	let isFormValid = $derived(serviceName.trim().length > 0);

	function slugify(text: string) {
		return text
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9_-]/g, '-')
			.replace(/-+/g, '-');
	}

	function handleCreate() {
		if (!isFormValid) return;

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
				branch: 'main',
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
		} else if (selectedType === 'image') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'image',
				status: 'stopped',
				source: '',
				image: '',
				port: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Container workload deployed from registry image',
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else if (selectedType === 'compose') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'compose',
				status: 'stopped',
				source: 'compose.yaml',
				port: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Multi-container stack defined by compose.yaml',
				composeYaml: `services:\n  ${sSlug || 'app'}:\n    image: nginx:alpine\n    ports:\n      - "8080:80"\n    restart: always`,
				workloads: [{ name: sSlug || 'app', image: 'nginx:alpine', status: 'stopped' }],
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else if (selectedType === 'pod') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'pod',
				status: 'stopped',
				source: '',
				port: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Podman Pod grouping co-located containers',
				workloads: [
					{ name: 'app', image: 'ghcr.io/org/app:latest', status: 'stopped' }
				],
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else if (selectedType === 'kubernetes') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'kubernetes',
				status: 'stopped',
				source: 'manifest.yaml',
				port: 80,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Kubernetes manifest executed via Podman kube play',
				k8sYaml: `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${sSlug}\nspec:\n  containers:\n    - name: main\n      image: nginx:alpine\n      ports:\n        - containerPort: 80`,
				workloads: [{ name: sName, image: 'nginx:alpine', status: 'stopped' }],
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else if (selectedType === 'quadlet') {
			newService = {
				id: internalId,
				projectId: targetProj,
				name: sName,
				type: 'quadlet',
				status: 'stopped',
				source: `${sSlug}.container`,
				port: 8080,
				cpu: 0,
				memory: 0,
				restartPolicy: 'always',
				health: 'healthy',
				replicas: 1,
				description: desc || 'Declarative systemd Quadlet service unit',
				quadletConfig: `[Unit]\nDescription=${sName} Service\nAfter=network-online.target\n\n[Container]\nImage=nginx:alpine\nPublishPort=8080:80\nAutoUpdate=registry\n\n[Service]\nRestart=always\n\n[Install]\nWantedBy=default.target`,
				workloads: [{ name: sName, image: 'nginx:alpine', status: 'stopped' }],
				envVars: [],
				deployments: [],
				createdAt: new Date().toISOString().split('T')[0]
			};
		} else {
			// Database preset
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
		}

		// Add service to reactive store
		dataStore.addService(newService);

		// Close modal if popup
		open = false;
		onclose?.();

		// Immediately navigate to Service Detail page where the user configures the rest!
		goto(`/projects/${targetProj}/services/${newService.id}`);
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
		<!-- Inline View (e.g. /projects/[projectId]/new-service) -->
		<div class="w-full max-w-[760px] flex flex-col gap-6">
			<div>
				<h1 class="text-xl font-medium text-[var(--text-primary)] m-0 mb-1">Create Service</h1>
				<p class="text-xs text-[var(--text-tertiary)] m-0">
					Choose what you are deploying and provide its name. Deeper runtime, ports, and source details are configured inside Service Detail.
				</p>
			</div>

			<!-- Project selection if not locked by route -->
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

			<!-- Workload Type Choices -->
			<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

			<!-- Compact Identity Form -->
			<div class="p-5 rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-4">
				<div class="flex items-center justify-between pb-3 border-b border-[var(--border-subtle)]">
					<span class="text-xs font-medium text-[var(--text-primary)]">Service Identity</span>
					<span class="text-[11px] font-[var(--font-mono)] text-[var(--text-tertiary)]">
						Slug: {selectedProjectId}-{slugify(serviceName || 'service')}
					</span>
				</div>

				{#if selectedType === 'database'}
					<div class="flex flex-col gap-1.5">
						<label for="inline-db-engine" class="text-xs text-[var(--text-secondary)] font-medium">Database Engine</label>
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
									class="p-2 rounded-[var(--radius-sm)] border text-xs text-center cursor-pointer transition-colors {dbEngine === db.id
										? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium'
										: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
								>
									{db.label}
								</button>
							{/each}
						</div>
					</div>
				{/if}

				<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
					<div class="flex flex-col gap-1.5">
						<label for="inline-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
							Service name <span class="text-[var(--status-red)]">*</span>
						</label>
						<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
							<Input
								id="inline-svc-name"
								bind:value={serviceName}
								placeholder={selectedType === 'database' ? `${dbEngine}-db` : 'e.g. web-api'}
							/>
						</div>
					</div>

					<div class="flex flex-col gap-1.5">
						<label for="inline-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
							Description <span class="text-[var(--text-tertiary)]">(optional)</span>
						</label>
						<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
							<Input
								id="inline-svc-desc"
								bind:value={serviceDescription}
								placeholder="e.g. Main HTTP service or background worker"
							/>
						</div>
					</div>
				</div>

				<div class="flex items-center justify-between pt-4 border-t border-[var(--border-subtle)] mt-2">
					<Button variant="ghost" onclick={handleCancel}>Cancel</Button>
					<Button variant="primary" disabled={!isFormValid} onclick={handleCreate}>
						<Plus size={14} /> Create & Configure
					</Button>
				</div>
			</div>
		</div>
	{:else}
		<!-- Modal Dialog Backdrop & Container -->
		<div
			class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-[rgba(5,6,7,0.78)] backdrop-blur-[2px]"
			role="dialog"
			aria-modal="true"
		>
			<div class="w-full max-w-[740px] max-h-[92vh] flex flex-col rounded-[var(--radius-card)] border border-[var(--border)] bg-[var(--bg-shell)] overflow-hidden">
				<!-- Header -->
				<div class="flex items-center justify-between px-6 py-4 border-b border-[var(--border)] bg-[var(--bg-panel)]">
					<div>
						<h2 class="text-base font-medium text-[var(--text-primary)] m-0">Create Service</h2>
						<p class="text-xs text-[var(--text-tertiary)] m-0 mt-0.5">
							Name your workload. Source, build, and runtime details are configured in Service Detail.
						</p>
					</div>
					<button
						type="button"
						onclick={handleCancel}
						class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)] p-1 bg-transparent border-0 cursor-pointer rounded-[var(--radius-sm)]"
						aria-label="Close dialog"
					>
						<X size={18} />
					</button>
				</div>

				<!-- Scrollable Content Body -->
				<div class="flex-1 overflow-y-auto p-6 flex flex-col gap-5">
					{#if !projectId && projects.length > 1}
						<div class="flex flex-col gap-1.5 max-w-[280px]">
							<label for="modal-select-project" class="text-xs text-[var(--text-secondary)] font-medium">Target Project</label>
							<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)]">
								<select
									id="modal-select-project"
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

					<!-- Workload choices -->
					<WorkloadTypeSelector selected={selectedType} onselect={(t) => (selectedType = t)} />

					<!-- Compact Identity form -->
					<div class="p-4 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-panel)] flex flex-col gap-3.5">
						<div class="flex items-center justify-between pb-2 border-b border-[var(--border-subtle)]">
							<span class="text-xs font-medium text-[var(--text-secondary)]">Service Identity</span>
							<span class="text-[10.5px] font-[var(--font-mono)] text-[var(--text-tertiary)]">
								Runtime Slug: {selectedProjectId}-{slugify(serviceName || 'service')}
							</span>
						</div>

						{#if selectedType === 'database'}
							<div class="flex flex-col gap-1.5">
								<span class="text-xs text-[var(--text-secondary)] font-medium">Database Engine</span>
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
											class="p-2 rounded-[var(--radius-sm)] border text-xs text-center cursor-pointer transition-colors {dbEngine === db.id
												? 'bg-[var(--bg-surface)] border-[var(--accent)] text-[var(--text-primary)] font-medium'
												: 'bg-[var(--bg-panel)] border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]'}"
										>
											{db.label}
										</button>
									{/each}
								</div>
							</div>
						{/if}

						<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
							<div class="flex flex-col gap-1.5">
								<label for="modal-svc-name" class="text-xs text-[var(--text-secondary)] font-medium">
									Service name <span class="text-[var(--status-red)]">*</span>
								</label>
								<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
									<Input
										id="modal-svc-name"
										bind:value={serviceName}
										placeholder={selectedType === 'database' ? `${dbEngine}-db` : 'e.g. web-api'}
									/>
								</div>
							</div>

							<div class="flex flex-col gap-1.5">
								<label for="modal-svc-desc" class="text-xs text-[var(--text-secondary)] font-medium">
									Description <span class="text-[var(--text-tertiary)]">(optional)</span>
								</label>
								<div class="px-3 py-2 border border-[var(--border)] rounded-[var(--radius-sm)] bg-[var(--bg-surface)] focus-within:border-[var(--accent)]">
									<Input
										id="modal-svc-desc"
										bind:value={serviceDescription}
										placeholder="e.g. Main HTTP service or background worker"
									/>
								</div>
							</div>
						</div>
					</div>
				</div>

				<!-- Footer -->
				<div class="flex items-center justify-between px-6 py-3.5 border-t border-[var(--border)] bg-[var(--bg-panel)]">
					<Button variant="ghost" size="sm" onclick={handleCancel}>Cancel</Button>
					<Button variant="primary" size="sm" disabled={!isFormValid} onclick={handleCreate}>
						<Plus size={14} /> Create & Configure
					</Button>
				</div>
			</div>
		</div>
	{/if}
{/if}
