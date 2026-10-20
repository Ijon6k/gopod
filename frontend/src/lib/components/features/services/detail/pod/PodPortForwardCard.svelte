<script lang="ts">
	import type { Service } from '$lib/types';
	import {
		LayerCard,
		Badge,
		Button,
		Input,
		Select,
		Table,
		TableHeader,
		TableHead,
		TableBody,
		TableRow,
		TableCell,
		useKumoToastManager
	} from 'kumo-svelte';
	import { Package, PlugsConnected, Globe, Plus } from 'phosphor-svelte';
	import { dataStore } from '$lib/data';
	import {
		parseDetailedPorts,
		serializePodComposeYaml,
		serializePodK8sYaml,
		portBindingId,
		normalizePortHostIp,
		type PortBindingItem,
		type PortHostIp
	} from '$lib/parsers/compose';

	interface Props {
		service: Service;
		podName: string;
		memberServices?: Service[];
		onportschange?: () => void;
	}

	let { service, podName, memberServices = [], onportschange }: Props = $props();

	const toast = useKumoToastManager();

	let portsList = $state<PortBindingItem[]>([]);
	let isAddingPort = $state(false);
	let newHostPort = $state('');
	let newPodPort = $state('');
	let newProtocol = $state<'tcp' | 'udp'>('tcp');
	let newHostIp = $state<PortHostIp>('0.0.0.0');
	let isSaving = $state(false);

	// Editing state for an existing row
	let editingId = $state<string | null>(null);
	let editHostPort = $state('');
	let editPodPort = $state('');
	let editProtocol = $state<'tcp' | 'udp'>('tcp');
	let editHostIp = $state<PortHostIp>('0.0.0.0');

	const protocolOptions = [
		{ value: 'tcp', label: 'TCP' },
		{ value: 'udp', label: 'UDP' }
	];

	const bindAddressOptions = [
		{ value: '0.0.0.0', label: '0.0.0.0 (all interfaces)' },
		{ value: '127.0.0.1', label: '127.0.0.1 (loopback only)' }
	];

	let initializedFor = $state('');

	// Rebuild the editor list only when the underlying port definition changes.
	// Background polls reuse the same object content, so in-progress bindings and
	// the inline edit state survive untouched between polls.
	$effect(() => {
		const key = `${service.id}:${service.composeYaml || ''}:${service.port ?? 0}:${service.hostPort ?? 0}`;
		if (initializedFor === key) return;
		initializedFor = key;
		if (service.composeYaml) {
			const parsed = parseDetailedPorts(service.composeYaml);
			if (parsed.length > 0) {
				portsList = parsed;
				return;
			}
		}
		if (service.port && service.port > 0) {
			const fallbackIp = normalizePortHostIp(service.hostIp);
			const fallbackHost = service.hostPort || service.port;
			portsList = [
				{
					id: portBindingId(fallbackHost, service.port, 'tcp', fallbackIp),
					hostPort: fallbackHost,
					podPort: service.port,
					protocol: 'tcp',
					hostIp: fallbackIp
				}
			];
		} else {
			portsList = [];
		}
	});

	async function persistPorts(updated: PortBindingItem[]) {
		isSaving = true;
		try {
			// 1. Maintain backward compatibility for single-port consumers
			if (updated.length > 0) {
				service.hostPort = updated[0].hostPort;
				service.port = updated[0].podPort;
				service.hostIp = normalizePortHostIp(updated[0].hostIp);
			} else {
				service.hostPort = 0;
				service.port = 0;
				service.hostIp = undefined;
			}

			// 2. Synchronize compose.yaml and k8s.yaml with member containers
			const membersDef = memberServices.map((m) => ({
				name: m.name,
				image: m.image,
				port: m.port
			}));

			service.composeYaml = serializePodComposeYaml(service.name, updated, service.composeYaml);
			service.k8sYaml = serializePodK8sYaml(service.name, updated, membersDef, service.k8sYaml);

			await dataStore.updateService(service);
			portsList = updated;

			toast.add({
				title: 'Port Bindings Updated',
				description: 'Configuration saved. Recreate the pod when ready to apply host port changes.',
				variant: 'success'
			});

			onportschange?.();
		} catch (err: any) {
			toast.add({
				title: 'Failed to Save Ports',
				description: err?.message || 'Error updating port configuration',
				variant: 'error'
			});
		} finally {
			isSaving = false;
		}
	}

	function handleAddPort() {
		const hPort = parseInt(newHostPort.trim(), 10);
		const pPort = parseInt(newPodPort.trim(), 10);

		if (!hPort || hPort < 1 || hPort > 65535) {
			toast.add({
				title: 'Invalid Host Port',
				description: 'Host port must be an integer between 1 and 65535.',
				variant: 'error'
			});
			return;
		}

		if (!pPort || pPort < 1 || pPort > 65535) {
			toast.add({
				title: 'Invalid Pod Port',
				description: 'Pod target port must be an integer between 1 and 65535.',
				variant: 'error'
			});
			return;
		}

		if (portsList.some((p) => p.hostPort === hPort && p.protocol === newProtocol && normalizePortHostIp(p.hostIp) === newHostIp)) {
			toast.add({
				title: 'Port Collision',
				description: `Host port ${newHostIp}:${hPort}/${newProtocol.toUpperCase()} is already bound in this pod configuration.`,
				variant: 'error'
			});
			return;
		}

		const updated: PortBindingItem[] = [
			...portsList,
			{
				id: portBindingId(hPort, pPort, newProtocol, newHostIp),
				hostPort: hPort,
				podPort: pPort,
				protocol: newProtocol,
				hostIp: newHostIp
			}
		];

		newHostPort = '';
		newPodPort = '';
		isAddingPort = false;
		persistPorts(updated);
	}

	function startEdit(item: PortBindingItem) {
		editingId = item.id;
		editHostPort = String(item.hostPort);
		editPodPort = String(item.podPort);
		editProtocol = item.protocol;
		editHostIp = normalizePortHostIp(item.hostIp);
	}

	function cancelEdit() {
		editingId = null;
	}

	function saveEdit(item: PortBindingItem) {
		const hPort = parseInt(editHostPort.trim(), 10);
		const pPort = parseInt(editPodPort.trim(), 10);

		if (!hPort || hPort < 1 || hPort > 65535 || !pPort || pPort < 1 || pPort > 65535) {
			toast.add({
				title: 'Invalid Port Number',
				description: 'Both host and pod ports must be between 1 and 65535.',
				variant: 'error'
			});
			return;
		}

		if (portsList.some((p) => p.id !== item.id && p.hostPort === hPort && p.protocol === editProtocol && normalizePortHostIp(p.hostIp) === editHostIp)) {
			toast.add({
				title: 'Port Collision',
				description: `Host port ${editHostIp}:${hPort}/${editProtocol.toUpperCase()} is already bound to another forward rule.`,
				variant: 'error'
			});
			return;
		}

		const updated = portsList.map((p) =>
			p.id === item.id
				? { ...p, id: portBindingId(hPort, pPort, editProtocol, editHostIp), hostPort: hPort, podPort: pPort, protocol: editProtocol, hostIp: editHostIp }
				: p
		);

		editingId = null;
		persistPorts(updated);
	}

	function handleDeletePort(id: string) {
		const updated = portsList.filter((p) => p.id !== id);
		persistPorts(updated);
	}
</script>

<LayerCard class="overflow-hidden p-0 shadow-xs">
	<LayerCard.Secondary
		class="flex items-center justify-between border-b border-[var(--color-kumo-hairline)] px-5 py-3.5"
	>
		<div class="flex items-center gap-2.5">
			<Package size={16} class="text-[var(--accent)]" />
			<h3 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
				Pod Infrastructure & Host Port Forwarding
			</h3>
		</div>
		<Badge variant="neutral" class="font-mono text-xs">network: gopod-net</Badge>
	</LayerCard.Secondary>

	<!-- Metadata overview -->
	<div class="grid grid-cols-1 gap-6 border-b border-[var(--color-kumo-hairline)] p-5 sm:grid-cols-3">
		<div class="flex flex-col gap-1">
			<span class="text-xs text-[var(--color-kumo-subtle)]">Pod Name</span>
			<span class="font-mono text-sm font-semibold text-[var(--color-kumo-default)]">
				{podName}
			</span>
			<span class="text-xs text-[var(--color-kumo-subtle)]">Shared Netavark namespace</span>
		</div>

		<div class="flex flex-col gap-1">
			<span class="text-xs text-[var(--color-kumo-subtle)]">Container Network</span>
			<div class="flex items-center gap-1.5">
				<PlugsConnected size={14} class="text-[var(--accent)]" />
				<span class="font-mono text-sm font-medium text-[var(--color-kumo-default)]">
					gopod-net
				</span>
			</div>
			<span class="text-xs text-[var(--color-kumo-subtle)]">Podman rootless bridge</span>
		</div>

		<div class="flex flex-col gap-1">
			<span class="text-xs text-[var(--color-kumo-subtle)]">Published Endpoints</span>
			<div class="flex items-center gap-1.5">
				<Globe size={14} class="text-[var(--color-kumo-subtle)]" />
				<span class="text-sm font-medium text-[var(--color-kumo-default)]">
					{portsList.length} {portsList.length === 1 ? 'binding' : 'bindings'} configured
				</span>
			</div>
			<span class="text-xs text-[var(--color-kumo-subtle)]">Bound to host interface</span>
		</div>
	</div>

	<!-- Ports Management Section -->
	<div class="p-5">
		<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
			<div>
				<h4 class="m-0 text-sm font-semibold text-[var(--color-kumo-default)]">
					Host port forwardings
				</h4>
				<p class="m-0 mt-0.5 text-xs text-[var(--color-kumo-subtle)]">
					Forward host traffic into internal member container ports. Bind to all interfaces or loopback only.
				</p>
			</div>
			{#if !isAddingPort}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => (isAddingPort = true)}
					class="gap-1.5"
				>
					<Plus size={13} />
					<span>Add port binding</span>
				</Button>
			{/if}
		</div>

		<!-- Add Port Inline Form -->
		{#if isAddingPort}
			<div
				class="mb-4 flex flex-wrap items-center gap-3 rounded-md border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] p-3"
			>
				<div class="flex flex-col gap-1">
					<label for="new-host-port" class="text-xs font-medium text-[var(--color-kumo-subtle)]">
						Host Port
					</label>
					<Input
						id="new-host-port"
						bind:value={newHostPort}
						placeholder="e.g. 8090"
						class="h-8 w-28 font-mono text-xs"
					/>
				</div>

				<span class="mt-4 text-xs font-semibold text-[var(--color-kumo-subtle)]">-&gt;</span>

				<div class="flex flex-col gap-1">
					<label for="new-pod-port" class="text-xs font-medium text-[var(--color-kumo-subtle)]">
						Pod Port
					</label>
					<Input
						id="new-pod-port"
						bind:value={newPodPort}
						placeholder="e.g. 80"
						class="h-8 w-28 font-mono text-xs"
					/>
				</div>

				<div class="flex flex-col gap-1">
					<span class="text-xs font-medium text-[var(--color-kumo-subtle)]">Protocol</span>
					<div class="w-28">
						<Select
							value={newProtocol}
							onValueChange={(v: any) => {
								if (v === 'tcp' || v === 'udp') newProtocol = v;
							}}
							options={protocolOptions}
							size="sm"
						/>
					</div>
				</div>

				<div class="flex flex-col gap-1">
					<span class="text-xs font-medium text-[var(--color-kumo-subtle)]">Bind Address</span>
					<div class="w-56">
						<Select
							value={newHostIp}
							onValueChange={(v: any) => {
								if (v === '0.0.0.0' || v === '127.0.0.1') newHostIp = v;
							}}
							options={bindAddressOptions}
							size="sm"
						/>
					</div>
				</div>

				<div class="mt-4 flex items-center gap-2">
					<Button variant="primary" size="sm" onclick={handleAddPort} disabled={isSaving}>
						Save binding
					</Button>
					<Button variant="ghost" size="sm" onclick={() => (isAddingPort = false)}>
						Cancel
					</Button>
				</div>
			</div>
		{/if}

		<!-- Port Bindings Table (Kumo UI) -->
		<div class="overflow-x-auto md:overflow-x-visible">
			<Table class="w-full min-w-[780px] text-left">
				<TableHeader
					class="sticky top-0 z-20 border-b border-[var(--border)] bg-[var(--bg-table-header)]"
				>
					<TableRow>
						<TableHead class="px-4 py-2.5 text-xs font-semibold normal-case">Host Port</TableHead>
						<TableHead class="px-4 py-2.5 text-xs font-semibold normal-case">Bind Address</TableHead>
						<TableHead class="px-4 py-2.5 text-xs font-semibold normal-case">Target Pod Port</TableHead>
						<TableHead class="px-4 py-2.5 text-xs font-semibold normal-case">Protocol</TableHead>
						<TableHead class="px-4 py-2.5 text-xs font-semibold normal-case">Routing Path</TableHead>
						<TableHead class="px-4 py-2.5 text-right text-xs font-semibold normal-case">Actions</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody class="divide-y divide-[var(--border)]">
					{#if portsList.length === 0}
						<TableRow>
							<TableCell colspan={6} class="py-6 text-center text-[13px]">
								No host port bindings configured. Containers communicate internally via localhost.
							</TableCell>
						</TableRow>
					{:else}
						{#each portsList as item (item.id)}
							<TableRow class="transition-colors hover:bg-[var(--bg-table-row-hover)]">
								{#if editingId === item.id}
									<TableCell class="px-4 py-2">
										<Input
											bind:value={editHostPort}
											class="h-8 w-24 font-mono text-[13px]"
											aria-label="Edit Host Port"
										/>
									</TableCell>
									<TableCell class="px-4 py-2">
										<div class="w-44">
											<Select
												value={editHostIp}
												onValueChange={(v: any) => {
													if (v === '0.0.0.0' || v === '127.0.0.1') editHostIp = v;
												}}
												options={bindAddressOptions}
												size="sm"
											/>
										</div>
									</TableCell>
									<TableCell class="px-4 py-2">
										<Input
											bind:value={editPodPort}
											class="h-8 w-24 font-mono text-[13px]"
											aria-label="Edit Pod Port"
										/>
									</TableCell>
									<TableCell class="px-4 py-2">
										<div class="w-24">
											<Select
												value={editProtocol}
												onValueChange={(v: any) => {
													if (v === 'tcp' || v === 'udp') editProtocol = v;
												}}
												options={protocolOptions}
												size="sm"
											/>
										</div>
									</TableCell>
									<TableCell class="px-4 py-2 font-mono text-xs">
										{editHostIp}:{editHostPort || '?'} -&gt; localhost:{editPodPort || '?'}
									</TableCell>
									<TableCell class="px-4 py-2 text-right">
										<div class="flex items-center justify-end gap-1.5">
											<Button variant="primary" size="sm" onclick={() => saveEdit(item)}>
												Save
											</Button>
											<Button variant="ghost" size="sm" onclick={cancelEdit}>
												Cancel
											</Button>
										</div>
									</TableCell>
								{:else}
									<TableCell class="px-4 py-3 font-mono text-[13px]">
										:{item.hostPort}
									</TableCell>
									<TableCell class="px-4 py-3 font-mono text-[13px]">
										{normalizePortHostIp(item.hostIp)}
									</TableCell>
									<TableCell class="px-4 py-3 font-mono text-[13px]">
										:{item.podPort}
									</TableCell>
									<TableCell class="px-4 py-3">
										<Badge variant="neutral" class="font-mono text-xs uppercase">
											{item.protocol}
										</Badge>
									</TableCell>
									<TableCell class="px-4 py-3 font-mono text-xs">
										{normalizePortHostIp(item.hostIp)}:{item.hostPort} -&gt; localhost:{item.podPort}
									</TableCell>
									<TableCell class="px-4 py-3 text-right">
										<div class="flex items-center justify-end gap-1.5">
											<Button variant="secondary" size="sm" onclick={() => startEdit(item)}>
												Edit
											</Button>
											<Button
												variant="secondary-destructive"
												size="sm"
												onclick={() => handleDeletePort(item.id)}
												aria-label="Delete port binding"
											>
												Delete
											</Button>
										</div>
									</TableCell>
								{/if}
							</TableRow>
						{/each}
					{/if}
				</TableBody>
			</Table>
		</div>
	</div>
</LayerCard>
