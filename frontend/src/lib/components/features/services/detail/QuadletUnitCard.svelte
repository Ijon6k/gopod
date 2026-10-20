<script lang="ts">
	import {
		Button,
		Input,
		CopyButton,
		LayerCard,
		Switch,
		useKumoToastManager
	} from '$lib/components/ui';
	import { Cube, Package, FileText, WarningCircle, ArrowsClockwise } from 'phosphor-svelte';
	import type { QuadletUnitFile } from '$lib/parsers/quadlet-units';
	import {
		readContainerCard,
		readPodCard,
		containerCardEdits,
		podCardEdits,
		applyUnitEdits,
		unitFallbackName
	} from '$lib/parsers/quadlet-units';

	const toast = useKumoToastManager();

	interface Props {
		unit: QuadletUnitFile;
		onApply: (name: string, nextContent: string) => void;
	}

	let { unit, onApply }: Props = $props();

	// Structured state is re-read whenever the unit body changes externally
	// (fresh preview load or an applied edit round-trip).
	let containerState = $state(readContainerCard(''));
	let podState = $state(readPodCard(''));

	$effect(() => {
		// Tracks unit.content so external changes (preview reload or an applied
		// edit round-trip) reset the form from the current unit text.
		containerState = readContainerCard(unit.content);
		podState = readPodCard(unit.content);
	});

	function addEnvRow() {
		containerState = { ...containerState, env: [...containerState.env, { key: '', value: '' }] };
	}
	function removeEnvRow(index: number) {
		containerState = {
			...containerState,
			env: containerState.env.filter((_, i) => i !== index)
		};
	}
	function addVolumeRow() {
		containerState = {
			...containerState,
			volumes: [...containerState.volumes, { source: '', target: '', options: 'Z' }]
		};
	}
	function removeVolumeRow(index: number) {
		containerState = {
			...containerState,
			volumes: containerState.volumes.filter((_, i) => i !== index)
		};
	}
	function addTmpfsRow() {
		containerState = { ...containerState, tmpfs: [...containerState.tmpfs, ''] };
	}
	function removeTmpfsRow(index: number) {
		containerState = {
			...containerState,
			tmpfs: containerState.tmpfs.filter((_, i) => i !== index)
		};
	}

	function handleApply() {
		const kind = unit.kind;
		let next: string;
		if (kind === 'pod') {
			next = applyUnitEdits(unit.content, podCardEdits(podState));
		} else if (kind === 'container') {
			next = applyUnitEdits(unit.content, containerCardEdits(containerState));
		} else {
			toast.add({
				title: 'Read-only unit',
				description: 'This unit type has no structured fields; edit it in raw mode.',
				variant: 'info'
			});
			return;
		}
		if (next === unit.content) {
			toast.add({ title: 'No changes to apply', variant: 'info' });
			return;
		}
		onApply(
			unit.name || `${unitFallbackName(unit)}${kind === 'pod' ? '.pod' : '.container'}`,
			next
		);
	}
</script>

<LayerCard class="w-full overflow-hidden shadow-xs">
	<!-- Card Header -->
	<div
		class="flex flex-wrap items-center justify-between gap-3 border-b border-[var(--border-subtle)] px-5 py-3.5"
	>
		<div class="flex items-center gap-2">
			{#if unit.kind === 'pod'}
				<Package size={16} class="text-[var(--accent)]" />
			{:else if unit.kind === 'container'}
				<Cube size={16} class="text-[var(--accent)]" />
			{:else}
				<FileText size={16} class="text-[var(--text-tertiary)]" />
			{/if}
			<div class="flex flex-col">
				<span class="font-mono text-xs font-semibold text-[var(--text-primary)]">
					{unit.name || unitFallbackName(unit)}
				</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					{unit.kind === 'pod'
						? 'Pod unit'
						: unit.kind === 'container'
							? 'Container unit'
							: 'Unit (read-only structured view)'}
				</span>
			</div>
		</div>
		<div class="flex items-center gap-2">
			<CopyButton text={unit.content} title="Copy unit text" variant="icon" size={13} />
			{#if unit.kind !== 'other'}
				<Button variant="secondary" size="sm" onclick={handleApply} class="gap-1.5">
					<ArrowsClockwise size={13} /> Apply
				</Button>
			{/if}
		</div>
	</div>

	{#if unit.kind === 'pod'}
		<div class="grid grid-cols-1 gap-4 px-5 py-4 md:grid-cols-2">
			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Network</span>
				{#each podState.network as net}
					<code
						class="rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--accent)]"
						>Network={net}</code
					>
				{:else}
					<span class="text-[11px] text-[var(--text-tertiary)]">Default (no Network directive)</span
					>
				{/each}
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="pod-stoptimeout"
					>Stop timeout (seconds)</label
				>
				<Input
					id="pod-stoptimeout"
					bind:value={podState.stopTimeout}
					placeholder="10"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>

			<div class="flex flex-col gap-2 md:col-span-2">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]"
						>Published ports (PublishPort)</span
					>
					<Button
						variant="secondary"
						size="xs"
						onclick={() =>
							(podState = { ...podState, publishPorts: [...podState.publishPorts, ''] })}
						>+ Port</Button
					>
				</div>
				{#each podState.publishPorts as port, i}
					<div class="flex items-center gap-2">
						<Input
							bind:value={podState.publishPorts[i]}
							placeholder="8080:80"
							class="text-xs font-[var(--font-mono)]"
						/>
						<Button
							variant="ghost"
							size="xs"
							onclick={() =>
								(podState = {
									...podState,
									publishPorts: podState.publishPorts.filter((_, j) => j !== i)
								})}>Remove</Button
						>
					</div>
				{:else}
					<span class="text-[11px] text-[var(--text-tertiary)]">No published ports.</span>
				{/each}
			</div>

			{#if podState.labels.length > 0}
				<div class="flex flex-col gap-1.5 md:col-span-2">
					<span class="text-xs font-medium text-[var(--text-secondary)]"
						>Labels (managed by GOPOD)</span
					>
					{#each podState.labels as label}
						<code
							class="rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-secondary)]"
							>Label={label.key}={label.value}</code
						>
					{/each}
				</div>
			{/if}
		</div>
	{:else if unit.kind === 'container'}
		<div class="grid grid-cols-1 gap-4 px-5 py-4 md:grid-cols-2">
			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Image</span>
				<code
					class="truncate rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--accent)]"
					>{containerState.image || '—'}</code
				>
			</div>
			<div class="flex flex-col gap-1.5">
				<span class="text-xs font-medium text-[var(--text-secondary)]">Container name</span>
				<code
					class="truncate rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-secondary)]"
					>{containerState.containerName || '—'}</code
				>
			</div>
			{#if containerState.pod}
				<div class="flex flex-col gap-1.5">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Joined pod</span>
					<code
						class="truncate rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-secondary)]"
						>Pod={containerState.pod}</code
					>
				</div>
			{/if}

			<!-- Environment rows -->
			<div class="flex flex-col gap-2 md:col-span-2">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Environment</span>
					<Button variant="secondary" size="xs" onclick={addEnvRow}>+ Env</Button>
				</div>
				{#each containerState.env as row, i}
					<div class="flex items-center gap-2">
						<Input
							bind:value={row.key}
							placeholder="KEY"
							class="w-40 text-xs font-[var(--font-mono)]"
						/>
						<Input
							bind:value={row.value}
							placeholder="value"
							class="flex-1 text-xs font-[var(--font-mono)]"
						/>
						<Button variant="ghost" size="xs" onclick={() => removeEnvRow(i)}>Remove</Button>
					</div>
				{:else}
					<span class="text-[11px] text-[var(--text-tertiary)]">No environment directives.</span>
				{/each}
			</div>

			<!-- Volume rows -->
			<div class="flex flex-col gap-2 md:col-span-2">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Volumes (Volume)</span>
					<Button variant="secondary" size="xs" onclick={addVolumeRow}>+ Volume</Button>
				</div>
				{#each containerState.volumes as row, i}
					<div class="flex items-center gap-2">
						<Input
							bind:value={row.source}
							placeholder="source (name or host path)"
							class="flex-1 text-xs font-[var(--font-mono)]"
						/>
						<Input
							bind:value={row.target}
							placeholder="/container/path"
							class="w-48 text-xs font-[var(--font-mono)]"
						/>
						<Input
							bind:value={row.options}
							placeholder="Z"
							class="w-20 text-xs font-[var(--font-mono)]"
						/>
						<Button variant="ghost" size="xs" onclick={() => removeVolumeRow(i)}>Remove</Button>
					</div>
				{:else}
					<span class="text-[11px] text-[var(--text-tertiary)]">No volume mounts.</span>
				{/each}
			</div>

			<!-- tmpfs rows -->
			<div class="flex flex-col gap-2 md:col-span-2">
				<div class="flex items-center justify-between">
					<span class="text-xs font-medium text-[var(--text-secondary)]">Tmpfs mounts (Tmpfs)</span>
					<Button variant="secondary" size="xs" onclick={addTmpfsRow}>+ Tmpfs</Button>
				</div>
				{#each containerState.tmpfs as row, i}
					<div class="flex items-center gap-2">
						<Input
							bind:value={containerState.tmpfs[i]}
							placeholder="/work"
							class="text-xs font-[var(--font-mono)]"
						/>
						<Button variant="ghost" size="xs" onclick={() => removeTmpfsRow(i)}>Remove</Button>
					</div>
				{:else}
					<span class="text-[11px] text-[var(--text-tertiary)]">No tmpfs mounts.</span>
				{/each}
			</div>

			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="cap-add"
					>capabilities added (CAN_ADD, OTHER)</label
				>
				<Input
					id="cap-add"
					bind:value={containerState.capAdd}
					placeholder="NET_BIND_SERVICE"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="cap-drop"
					>capabilities dropped (CAN_DROP, OTHER)</label
				>
				<Input
					id="cap-drop"
					bind:value={containerState.capDrop}
					placeholder="ALL"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="add-device"
					>devices (added to container)</label
				>
				<Input
					id="add-device"
					bind:value={containerState.addDevice}
					placeholder="/dev/dri"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="userns"
					>User namespace (UserNS)</label
				>
				<Input
					id="userns"
					bind:value={containerState.userNs}
					placeholder="keep-id"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>

			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="health-cmd"
					>health check command (HealthCmd)</label
				>
				<Input
					id="health-cmd"
					bind:value={containerState.healthCmd}
					placeholder="curl -f http://localhost/health"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex items-end gap-4">
				<div class="flex flex-col gap-1.5">
					<label class="text-xs font-medium text-[var(--text-secondary)]" for="health-interval"
						>Health interval</label
					>
					<Input
						id="health-interval"
						bind:value={containerState.healthInterval}
						placeholder="30s"
						class="w-24 text-xs font-[var(--font-mono)]"
					/>
				</div>
				<div class="flex flex-col gap-1.5">
					<label class="text-xs font-medium text-[var(--text-secondary)]" for="health-onfailure"
						>Health on-failure</label
					>
					<Input
						id="health-onfailure"
						bind:value={containerState.healthOnFailure}
						placeholder="kill"
						class="w-24 text-xs font-[var(--font-mono)]"
					/>
				</div>
			</div>

			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="pids-limit"
					>PidsLimit</label
				>
				<Input
					id="pids-limit"
					bind:value={containerState.pidsLimit}
					placeholder="2048"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="cpu-limit"
					>CPU limit (PodmanArgs=--cpu-quota)</label
				>
				<Input
					id="cpu-limit"
					bind:value={containerState.cpuLimit}
					placeholder="200000 (µs, 2 cores)"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<div class="flex flex-col gap-1.5">
				<label class="text-xs font-medium text-[var(--text-secondary)]" for="mem-limit"
					>Memory limit (PodmanArgs=--memory)</label
				>
				<Input
					id="mem-limit"
					bind:value={containerState.memoryLimit}
					placeholder="512MB"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>

			<!-- Switches -->
			<div class="flex items-center gap-3 md:col-span-2">
				<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
					<Switch bind:checked={containerState.readOnly} size="sm" /> Read-only root filesystem (ReadOnly)
				</label>
				{#if containerState.readOnly}
					<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
						<Switch bind:checked={containerState.readOnlyTmpfs} size="sm" /> Read-only tmpfs (ReadOnlyTmpfs)
					</label>
				{/if}
				<label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]">
					<Switch bind:checked={containerState.securityLabelDisable} size="sm" /> Disable SELinux label
					(SecurityLabelDisable)
				</label>
			</div>

			{#if containerState.labels.length > 0}
				<div class="flex flex-col gap-1.5 md:col-span-2">
					<span class="text-xs font-medium text-[var(--text-secondary)]"
						>Labels (managed by GOPOD)</span
					>
					{#each containerState.labels as label}
						<code
							class="rounded bg-[var(--bg-surface)] px-2 py-1 font-mono text-[11px] text-[var(--text-secondary)]"
							>Label={label.key}={label.value}</code
						>
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<div class="flex items-start gap-2 px-5 py-4">
			<WarningCircle size={16} class="mt-0.5 shrink-0 text-kumo-warning" />
			<pre
				class="m-0 flex-1 overflow-x-auto rounded-md border border-[var(--border-subtle)] bg-[var(--bg-panel)] p-3 font-mono text-[11px] leading-relaxed text-[var(--text-secondary)]">{unit.content}</pre>
		</div>
	{/if}
</LayerCard>
