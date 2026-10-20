<script lang="ts">
	import type { Service } from '$lib/types';
	import { SectionCard, Select, Button, Banner } from '$lib/components/ui';
	import { WarningCircle, ShieldCheck, SpinnerGap, Trash, Lightning } from 'phosphor-svelte';

	interface Props {
		openLifecycle?: boolean;
		service: Service;
		lingerInfo: { linger: boolean; user: string } | null;
		restartPolicy: string;
		isDemoting: boolean;
		onDemoteQuadlet: () => void;
		onOpenQuadlet: () => void;
	}

	let {
		openLifecycle = $bindable(true),
		service,
		lingerInfo,
		restartPolicy = $bindable(),
		isDemoting,
		onDemoteQuadlet,
		onOpenQuadlet
	}: Props = $props();
</script>

<SectionCard
	title="Lifecycle & Systemd"
	description="Quadlet unit registration, systemd supervisor integration, and restart policies."
	collapsible={true}
	bind:open={openLifecycle}
>
	<!-- Systemd Linger Warning Banner -->
	{#if lingerInfo && !lingerInfo.linger}
		<Banner variant="alert" class="mb-4 shadow-xs">
			{#snippet icon()}
				<WarningCircle size={18} weight="fill" class="shrink-0 text-kumo-warning" />
			{/snippet}
			<div class="flex flex-col gap-1 text-xs">
				<strong class="font-semibold text-kumo-warning">
					Systemd user linger disabled ({lingerInfo.user})
				</strong>
				<span class="leading-relaxed text-kumo-warning/90">
					Rootless systemd services stop when the session logs out. Run
					<code class="rounded bg-black/20 px-1.5 py-0.5 font-mono text-[0.9em]"
						>loginctl enable-linger {lingerInfo.user}</code
					>
					on the host to keep background Quadlet services running across reboots and logouts.
				</span>
			</div>
		</Banner>
	{/if}

	{#if service.type === 'quadlet' || service.runtimeTarget === 'quadlet'}
		<!-- Quadlet Active Supervision Card with Demote / Detach Action -->
		<div
			class="mb-4 flex flex-col justify-between gap-3 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] p-4 text-xs sm:flex-row sm:items-center"
		>
			<div class="flex flex-col gap-1">
				<div class="flex items-center gap-2">
					<ShieldCheck size={16} class="text-[var(--status-green)]" />
					<span class="text-sm font-semibold text-[var(--color-kumo-default)]">
						Supervised by Systemd Quadlet
					</span>
					<span
						class="rounded bg-[var(--status-green-muted)] px-2 py-0.5 font-mono text-[10px] font-semibold text-[var(--status-green)]"
					>
						{service.name}.service
					</span>
				</div>
				<span class="text-[var(--color-kumo-subtle)]">
					This workload is managed as a native systemd user unit with anti-loop guards and
					auto-restart.
				</span>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={isDemoting}
				onclick={onDemoteQuadlet}
				class="shrink-0 gap-1.5 text-[var(--status-red)] hover:bg-[var(--status-red)]/10"
			>
				{#if isDemoting}
					<SpinnerGap size={14} class="animate-spin" />
					<span>Demoting…</span>
				{:else}
					<Trash size={14} />
					<span>Demote / Detach to Standalone</span>
				{/if}
			</Button>
		</div>
	{:else}
		<!-- Systemd Quadlet Promotion Flow -->
		<div
			class="mb-4 flex flex-col gap-3 rounded-[var(--radius-sm)] border border-[var(--color-kumo-line)] bg-[var(--color-kumo-elevated)] p-4 text-xs"
		>
			<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
				<div class="flex flex-col gap-1">
					<div class="flex items-center gap-2">
						<Lightning size={16} weight="fill" class="text-[var(--accent)]" />
						<span class="text-sm font-semibold text-[var(--color-kumo-default)]">
							⚡ Systemd Quadlet Supervision
						</span>
					</div>
					<span class="text-[var(--color-kumo-subtle)]">
						Promote this workload to a native systemd user service managed by Podman Quadlet with
						automatic crash restart, boot persistence, and systemctl controls.
					</span>
				</div>
				<Button variant="primary" size="sm" onclick={onOpenQuadlet} class="shrink-0 gap-1.5">
					<Lightning size={14} weight="bold" />
					<span>⚡ Generate & Promote to Systemd Quadlet…</span>
				</Button>
			</div>
		</div>
	{/if}

	<div class="grid grid-cols-1 gap-4 pt-2 sm:grid-cols-2">
		<div
			class="mt-2 flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
		>
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]"
					>Systemd Unit Specification</span
				>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					{#if service.quadletFile}
						Unit file: {service.quadletFile}
					{:else}
						Units are written under ~/.config/containers/systemd on promotion.
					{/if}
				</span>
			</div>
			<span class="font-mono text-xs font-semibold text-[var(--accent)]">
				{service.quadletUnit || 'not promoted yet'}
			</span>
		</div>

		<Select
			id="lc-restart"
			label="Restart Policy"
			value={restartPolicy}
			onValueChange={(val) => {
				if (typeof val === 'string') restartPolicy = val as any;
			}}
			options={[
				{ label: 'always', value: 'always' },
				{ label: 'on-failure', value: 'on-failure' },
				{ label: 'unless-stopped', value: 'unless-stopped' },
				{ label: 'no', value: 'no' }
			]}
			size="sm"
		/>
	</div>
</SectionCard>
