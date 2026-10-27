<script lang="ts">
	import type { Service } from '$lib/types';
	import { goto } from '$app/navigation';
	import {
		LayerCard,
		Button,
		Badge,
		Table,
		TableHeader,
		TableRow,
		TableCell,
		TableHead,
		TableBody,
		useKumoToastManager
	} from 'kumo-svelte';
	import { errorToastOptions } from '$lib/utils/toast';
	import { Package, ArrowsClockwise, Stop, Play, Terminal, Plus, FileCode } from 'phosphor-svelte';
	import { parseDocument } from 'yaml';
	import { dataStore } from '$lib/data';
	import { parseComposeServices } from '$lib/parsers/compose';
	import { parseKubernetesPodManifest } from '$lib/parsers/manifest-engine';

	interface Props {
		service?: Service;
		podName?: string;
		memberServices: Service[];
		onOpenAddModal: () => void;
		onmemberchange?: () => void;
	}

	let { service, podName, memberServices, onOpenAddModal, onmemberchange }: Props = $props();

	// Manifest workloads extracted from Kubernetes YAML (e.g. sidecars / main containers)
	let manifestWorkloads = $derived.by(() => {
		if (!service?.k8sYaml || !service.k8sYaml.trim()) return [];
		const res = parseKubernetesPodManifest(service.k8sYaml);
		if (!res.valid || res.workloads.length === 0) return [];
		const memberNames = new Set(memberServices.map((m) => m.name.toLowerCase()));
		return res.workloads.filter((w) => !memberNames.has(w.name.toLowerCase()));
	});

	// Find live container for a given workload name
	function findLiveContainer(workloadName: string) {
		const targetPod = podName || service?.podId || service?.id;
		return dataStore.containers.find(
			(c) =>
				(c.pod === targetPod || (targetPod && c.name.includes(targetPod))) &&
				c.name.toLowerCase().includes(workloadName.toLowerCase())
		);
	}

	const toast = useKumoToastManager();

	async function handleRestartMember(member: Service) {
		try {
			await dataStore.deployService(member.id, 'manual');
			toast.add({
				title: 'Restarting Container',
				description: `Triggered restart for ${member.name}`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add(errorToastOptions('Restart Failed', err?.message || 'Error restarting container'));
		}
	}

	async function handleStopMember(member: Service) {
		try {
			const updated = await dataStore.stopService(member.id);
			member.status = (updated as Service)?.status || 'stopped';
			toast.add({
				title: 'Container Stopped',
				description: `${member.name} has been stopped`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add(errorToastOptions('Stop Failed', err?.message || 'Error stopping container'));
		}
	}

	async function handleRemoveMember(member: Service) {
		try {
			// Server-side ungroup removes the live container from the pod's
			// localhost namespace and redeploys it standalone. A DB-only flag
			// flip would leave the ghost container serving stale traffic.
			await dataStore.ungroupFromPod(member.id);
			toast.add({
				title: 'Removed from Pod',
				description: `${member.name} detached to standalone mode`,
				variant: 'success'
			});
			onmemberchange?.();
		} catch (err: any) {
			toast.add(errorToastOptions('Removal Failed', err?.message || 'Error detaching container'));
		}
	}

	async function handleRestartLiveContainer(container: any, name: string) {
		try {
			await dataStore.restartContainer(container.id);
			toast.add({
				title: 'Container Restarted',
				description: `Restarted container '${name}'`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add(errorToastOptions('Restart Failed', err?.message || 'Error restarting container'));
		}
	}

	async function handleStopLiveContainer(container: any, name: string) {
		try {
			await dataStore.stopContainer(container.id);
			container.status = 'stopped';
			toast.add({
				title: 'Container Stopped',
				description: `Stopped container '${name}'`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add(errorToastOptions('Stop Failed', err?.message || 'Error stopping container'));
		}
	}

	async function handleStartLiveContainer(container: any, name: string) {
		try {
			await dataStore.startContainer(container.id);
			container.status = 'running';
			toast.add({
				title: 'Container Started',
				description: `Started container '${name}'`,
				variant: 'success'
			});
		} catch (err: any) {
			toast.add(errorToastOptions('Start Failed', err?.message || 'Error starting container'));
		}
	}

	async function handleRemoveManifestWorkload(workloadName: string) {
		if (!service?.k8sYaml) return;
		try {
			const doc = parseDocument(service.k8sYaml);
			const containers = doc.getIn(['spec', 'containers']) as any;
			if (containers && containers.items) {
				const idx = containers.items.findIndex(
					(c: any) => c.get('name')?.toLowerCase() === workloadName.toLowerCase()
				);
				if (idx !== -1) {
					containers.items.splice(idx, 1);
					service.k8sYaml = doc.toString();
					await dataStore.updateService(service);
					toast.add({
						title: 'Removed from Manifest',
						description: `Removed container '${workloadName}' from pod manifest. Re-deploy pod to apply.`,
						variant: 'info'
					});
					onmemberchange?.();
				}
			}
		} catch (err: any) {
			toast.add(
				errorToastOptions('Detach Failed', err?.message || 'Error removing container from manifest')
			);
		}
	}
</script>

<LayerCard class="overflow-hidden p-0 shadow-xs">
	<LayerCard.Secondary
		class="flex items-center justify-between border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<Package size={16} class="text-[var(--accent)]" />
			<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
				Pod Member Containers
			</h3>
			<Badge variant="neutral" class="font-mono text-xs">
				{memberServices.length + manifestWorkloads.length}
				{memberServices.length + manifestWorkloads.length === 1 ? 'container' : 'containers'}
			</Badge>
		</div>

		<Button variant="primary" size="sm" onclick={onOpenAddModal} class="gap-1.5">
			<Plus size={14} weight="bold" />
			<span>Add / Attach Container</span>
		</Button>
	</LayerCard.Secondary>

	<!-- Member Containers Table -->
	<div class="w-full overflow-x-auto md:overflow-x-visible">
		<Table class="w-full min-w-[660px] text-left text-xs">
			<TableHeader
				class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
			>
				<TableRow>
					<TableHead
						class="px-4 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
					>
						Container Name
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
					>
						Image
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
					>
						Status
					</TableHead>
					<TableHead
						class="px-3 py-2.5 text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
					>
						Internal Port
					</TableHead>
					<TableHead
						class="px-4 py-2.5 text-right text-[12px] font-semibold text-[var(--text-tertiary)] normal-case"
					>
						Actions
					</TableHead>
				</TableRow>
			</TableHeader>
			<TableBody class="divide-y divide-[var(--border-subtle)]">
				{#if memberServices.length === 0 && manifestWorkloads.length === 0}
					<TableRow>
						<TableCell colspan={5} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
							<div class="flex flex-col items-center justify-center gap-2">
								<Package size={32} class="opacity-40" />
								<span class="font-medium">No containers attached to this pod yet.</span>
								<span class="text-[11px]">
									Click "Add / Attach Container" above to attach an existing workload or declare
									containers in the Kubernetes Manifest tab.
								</span>
							</div>
						</TableCell>
					</TableRow>
				{:else}
					{#each memberServices as member (member.id)}
						<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
							<!-- Container Name -->
							<TableCell class="px-4 py-3 text-[13px] font-medium text-[var(--text-primary)]">
								<div class="flex items-center gap-2">
									<button
										type="button"
										onclick={() => goto(`/projects/${member.projectId}/services/${member.id}`)}
										class="cursor-pointer border-0 bg-transparent font-semibold text-[var(--text-primary)] hover:text-[var(--accent)] hover:underline"
									>
										{member.name}
									</button>
									<span class="font-mono text-[10px] text-[var(--text-tertiary)]">
										({member.id})
									</span>
								</div>
							</TableCell>

							<!-- Image -->
							<TableCell class="px-3 py-3 font-mono text-xs text-[var(--text-secondary)]">
								{#if member.composeYaml}
									{@const csList = parseComposeServices(member.composeYaml)}
									{#if csList.length > 0}
										<div class="flex flex-col gap-0.5">
											{#each csList as cs}
												<span>{cs.name}: {cs.image || 'compose'}</span>
											{/each}
										</div>
									{:else}
										{member.image || 'compose.yaml'}
									{/if}
								{:else}
									{member.image || member.source || 'image:latest'}
								{/if}
							</TableCell>

							<!-- Status -->
							<TableCell class="px-3 py-3">
								<Badge
									variant={member.status === 'running' ? 'success' : 'neutral'}
									class="text-[11px] capitalize"
								>
									{member.status}
								</Badge>
							</TableCell>

							<!-- Internal Port -->
							<TableCell class="px-3 py-3 font-mono text-xs text-[var(--text-secondary)]">
								{#if member.composeYaml}
									{@const csList = parseComposeServices(member.composeYaml)}
									{#if csList.length > 0}
										<div class="flex flex-col gap-0.5">
											{#each csList as cs}
												<span
													>{cs.name}: localhost:{cs.ports.length > 0
														? cs.ports.join(', localhost:')
														: member.port || 80}</span
												>
											{/each}
										</div>
									{:else}
										localhost:{member.port || 80}
									{/if}
								{:else}
									localhost:{member.port || 80}
								{/if}
							</TableCell>

							<!-- Actions -->
							<TableCell class="px-4 py-3 text-right">
								<div class="flex items-center justify-end gap-1.5">
									<!-- Restart -->
									<Button
										variant="secondary"
										size="sm"
										onclick={() => handleRestartMember(member)}
										aria-label={`Restart ${member.name}`}
									>
										<ArrowsClockwise size={13} />
									</Button>

									<!-- Stop -->
									{#if member.status === 'running'}
										<Button
											variant="secondary-destructive"
											size="sm"
											onclick={() => handleStopMember(member)}
											aria-label={`Stop ${member.name}`}
										>
											<Stop size={13} weight="fill" />
										</Button>
									{/if}

									<!-- Detach from Pod -->
									<Button
										variant="secondary"
										size="sm"
										onclick={() => handleRemoveMember(member)}
										aria-label={`Detach ${member.name} from Pod`}
									>
										<span class="text-xs">Detach</span>
									</Button>

									<!-- Open Terminal -->
									<Button
										variant="secondary"
										size="sm"
										onclick={() =>
											goto(`/projects/${member.projectId}/services/${member.id}?tab=terminal`)}
										aria-label={`Terminal for ${member.name}`}
									>
										<Terminal size={13} />
									</Button>
								</div>
							</TableCell>
						</TableRow>
					{/each}

					{#each manifestWorkloads as mw (mw.name)}
						{@const live = findLiveContainer(mw.name)}
						<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
							<!-- Container Name -->
							<TableCell class="px-4 py-3 text-[13px] font-medium text-[var(--text-primary)]">
								<div class="flex items-center gap-2">
									<span class="font-semibold text-[var(--text-primary)]">
										{mw.name}
									</span>
									<span
										class="rounded border border-[var(--border-subtle)] bg-[var(--bg-table-row-alt)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--accent)] uppercase"
									>
										{mw.role}
									</span>
								</div>
							</TableCell>

							<!-- Image -->
							<TableCell class="px-3 py-3 font-mono text-xs text-[var(--text-secondary)]">
								{mw.image || '(none)'}
							</TableCell>

							<!-- Status -->
							<TableCell class="px-3 py-3">
								{#if live}
									<Badge
										variant={live.status === 'running' ? 'success' : 'neutral'}
										class="text-[11px] capitalize"
									>
										{live.status}
									</Badge>
								{:else}
									<span
										class="inline-flex items-center rounded border border-[var(--border-subtle)] bg-[var(--bg-table-row-alt)] px-2 py-0.5 text-[11px] font-medium text-[var(--text-tertiary)]"
									>
										Previewed
									</span>
								{/if}
							</TableCell>

							<!-- Internal Port -->
							<TableCell class="px-3 py-3 font-mono text-xs text-[var(--text-secondary)]">
								{#if mw.ports.length > 0}
									localhost:{mw.ports.map((p) => p.containerPort).join(', localhost:')}
								{:else}
									<span class="text-[var(--text-muted)]">-</span>
								{/if}
							</TableCell>

							<!-- Actions -->
							<TableCell class="px-4 py-3 text-right">
								{#if live}
									<div class="flex items-center justify-end gap-1.5">
										<!-- Restart -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleRestartLiveContainer(live, mw.name)}
											aria-label={`Restart ${mw.name}`}
										>
											<ArrowsClockwise size={13} />
										</Button>

										<!-- Stop / Start -->
										{#if live.status === 'running'}
											<Button
												variant="secondary-destructive"
												size="sm"
												onclick={() => handleStopLiveContainer(live, mw.name)}
												aria-label={`Stop ${mw.name}`}
											>
												<Stop size={13} weight="fill" />
											</Button>
										{:else}
											<Button
												variant="secondary"
												size="sm"
												onclick={() => handleStartLiveContainer(live, mw.name)}
												aria-label={`Start ${mw.name}`}
											>
												<Play size={13} weight="fill" />
											</Button>
										{/if}

										<!-- Detach from Pod Manifest -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleRemoveManifestWorkload(mw.name)}
											aria-label={`Detach ${mw.name} from Pod`}
										>
											<span class="text-xs">Detach</span>
										</Button>

										<!-- Open Terminal -->
										{#if service}
											<Button
												variant="secondary"
												size="sm"
												onclick={() =>
													goto(
														`/projects/${service.projectId}/services/${service.id}?tab=terminal`
													)}
												aria-label={`Terminal for ${mw.name}`}
											>
												<Terminal size={13} />
											</Button>
										{/if}
									</div>
								{:else}
									<div class="flex items-center justify-end gap-1.5">
										<span class="mr-1 text-[11px] text-[var(--text-tertiary)] italic">
											Previewed in manifest
										</span>
										<Button
											variant="secondary"
											size="sm"
											onclick={() => handleRemoveManifestWorkload(mw.name)}
											aria-label={`Remove ${mw.name} from Manifest`}
										>
											<span class="text-xs">Remove</span>
										</Button>
									</div>
								{/if}
							</TableCell>
						</TableRow>
					{/each}
				{/if}
			</TableBody>
		</Table>
	</div>
</LayerCard>
