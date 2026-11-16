<script lang="ts">
	import type { Service } from '$lib/types';
	import { SectionCard, Select, Button, Banner, CopyButton } from '$lib/components/ui';
	import { WarningCircle, ShieldCheck, SpinnerGap, Trash, Lightning } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		service: Service;
		lingerInfo: { linger: boolean; user: string } | null;
		restartPolicy: string;
		isDemoting: boolean;
		onDemoteQuadlet: () => void;
		onOpenQuadlet: () => void;
	}

	let {
		open = $bindable(true),
		service,
		lingerInfo,
		restartPolicy = $bindable(),
		isDemoting,
		onDemoteQuadlet,
		onOpenQuadlet
	}: Props = $props();

	let summary = $derived(`Restart: ${restartPolicy}`);
</script>

<SectionCard
	title="Lifecycle"
	description="Restart policy and systemd supervision."
	collapsible={true}
	bind:open
>
	{#snippet headerActions()}
		<span class="max-w-[240px] truncate font-mono text-[11px] text-[var(--text-tertiary)]"
			>{summary}</span
		>
	{/snippet}

	{#if lingerInfo && !lingerInfo.linger}
		<Banner variant="alert" size="sm">
			{#snippet icon()}
				<WarningCircle size={14} weight="fill" class="shrink-0" />
			{/snippet}
			<div class="flex flex-col gap-1 text-xs leading-relaxed">
				<span class="font-semibold">Systemd user linger is disabled for {lingerInfo.user}.</span>
				<span>
					Rootless services stop on logout. Run
					<code class="rounded bg-[var(--bg-hover)] px-1.5 py-0.5 font-mono text-[0.9em]">
						loginctl enable-linger {lingerInfo.user}
					</code>
					on the host.
					<CopyButton
						text={`loginctl enable-linger ${lingerInfo.user}`}
						label="Copy"
						variant="inline"
					/>
				</span>
			</div>
		</Banner>
	{/if}

	{#if service.type === 'quadlet' || service.runtimeTarget === 'quadlet'}
		<!-- Currently supervised: expose the demote escape hatch -->
		<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex flex-col gap-1">
				<div class="flex items-center gap-2">
					<ShieldCheck size={15} class="text-[var(--status-green)]" />
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						Supervised by systemd ({service.quadletUnit || `${service.name}.service`})
					</span>
				</div>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Native systemd user unit with restart supervision.
				</span>
			</div>
			<Button
				variant="secondary"
				size="sm"
				disabled={isDemoting}
				onclick={onDemoteQuadlet}
				class="shrink-0 text-[var(--status-red)]"
			>
				{#if isDemoting}
					<SpinnerGap size={14} class="animate-spin" />
					<span>Demoting…</span>
				{:else}
					<Trash size={14} />
					<span>Demote to Podman</span>
				{/if}
			</Button>
		</div>
	{:else}
		<!-- Not supervised: offer the promotion flow -->
		<div class="flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
			<div class="flex flex-col gap-1">
				<div class="flex items-center gap-2">
					<Lightning size={15} weight="fill" class="text-[var(--accent)]" />
					<span class="text-xs font-semibold text-[var(--text-primary)]">
						Runs without systemd supervision
					</span>
				</div>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Promote to a systemd user unit for restart on crash and boot persistence.
				</span>
			</div>
			<Button variant="secondary" size="sm" onclick={onOpenQuadlet} class="shrink-0">
				<Lightning size={14} />
				<span>Promote to systemd…</span>
			</Button>
		</div>
	{/if}

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1">
			<span class="text-xs font-medium text-[var(--text-secondary)]">Unit file</span>
			<span class="font-mono text-xs text-[var(--text-tertiary)]">
				{service.quadletFile || '~/.config/containers/systemd (on promotion)'}
			</span>
		</div>

		<Select
			id="lc-restart"
			label="Restart policy"
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
