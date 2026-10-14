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
	import { Package, ArrowsClockwise, Stop, Terminal, Plus } from 'phosphor-svelte';
	import { dataStore } from '$lib/data';

	interface Props {
		memberServices: Service[];
		onOpenAddModal: () => void;
	}

	let { memberServices, onOpenAddModal }: Props = $props();

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
			toast.add({
				title: 'Restart Failed',
				description: err?.message || 'Error restarting container',
				variant: 'error'
			});
		}
	}

	async function handleStopMember(member: Service) {
		try {
			await dataStore.stopService(member.id);
			member.status = 'stopped';
			toast.add({
				title: 'Container Stopped',
				description: `${member.name} has been stopped`,
				variant: 'info'
			});
		} catch (err: any) {
			toast.add({
				title: 'Stop Failed',
				description: err?.message || 'Error stopping container',
				variant: 'error'
			});
		}
	}

	async function handleRemoveMember(member: Service) {
		member.inPod = false;
		member.podId = '';
		member.runtimeTarget = 'standalone';

		try {
			await dataStore.updateService(member);
			toast.add({
				title: 'Removed from Pod',
				description: `${member.name} detached to standalone mode`,
				variant: 'success'
			});
		} catch (err: any) {
			toast.add({
				title: 'Removal Failed',
				description: err?.message || 'Error detaching container',
				variant: 'error'
			});
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
				{memberServices.length}
				{memberServices.length === 1 ? 'container' : 'containers'}
			</Badge>
		</div>

		<Button variant="primary" size="sm" onclick={onOpenAddModal} class="gap-1.5">
			<Plus size={14} weight="bold" />
			<span>Add / Attach Container</span>
		</Button>
	</LayerCard.Secondary>

	<!-- Member Containers Table -->
	<div class="w-full overflow-x-auto md:overflow-x-visible">
		<Table class="w-full text-left text-xs">
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
				{#if memberServices.length === 0}
					<TableRow>
						<TableCell colspan={5} class="py-12 text-center text-xs text-[var(--text-tertiary)]">
							<div class="flex flex-col items-center justify-center gap-2">
								<Package size={32} class="opacity-40" />
								<span class="font-medium">No containers attached to this pod yet.</span>
								<span class="text-[11px]">
									Click "Add / Attach Container" above to attach an existing workload or deploy a
									new image.
								</span>
							</div>
						</TableCell>
					</TableRow>
				{:else}
					{#each memberServices as member (member.id)}
						<TableRow class="transition-colors hover:bg-[var(--bg-table-row-alt)]">
							<!-- Container Name -->
							<TableCell class="px-4 py-3 font-medium text-[var(--text-primary)]">
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
							<TableCell class="px-3 py-3 font-mono text-[11px] text-[var(--text-secondary)]">
								{member.image || member.source || 'image:latest'}
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
							<TableCell class="px-3 py-3 font-mono text-[11px] text-[var(--text-secondary)]">
								localhost:{member.port || 80}
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
				{/if}
			</TableBody>
		</Table>
	</div>
</LayerCard>
