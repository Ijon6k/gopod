<script lang="ts">
	import { SectionCard, Input, Switch, Banner } from '$lib/components/ui';
	import { WarningCircle } from 'phosphor-svelte';

	interface Props {
		open?: boolean;
		userNamespace: string;
		userNsError?: string;
		devices: string;
		devicesError?: string;
		isPrivileged: boolean;
		selinuxLabel: string;
		apparmorProfile: string;
		capAdd: string;
		capAddError?: string;
		capDrop: string;
		capDropError?: string;
		noNewPrivileges: boolean;
		privilegedCapDropWarning?: string;
		privilegedUserNsWarning?: string;
	}

	let {
		open = $bindable(false),
		userNamespace = $bindable(),
		userNsError = '',
		devices = $bindable(),
		devicesError = '',
		isPrivileged = $bindable(),
		selinuxLabel = $bindable(),
		apparmorProfile = $bindable(),
		capAdd = $bindable(),
		capAddError = '',
		capDrop = $bindable(),
		capDropError = '',
		noNewPrivileges = $bindable(true),
		privilegedCapDropWarning = '',
		privilegedUserNsWarning = ''
	}: Props = $props();

	// Compact header summary of non-default settings.
	let summary = $derived.by(() => {
		const parts: string[] = [];
		const ns = userNamespace.trim();
		if (ns) parts.push(`userns ${ns}`);
		const devCount = devices
			.split(',')
			.map((d) => d.trim())
			.filter(Boolean).length;
		if (devCount) parts.push(`${devCount} device${devCount === 1 ? '' : 's'}`);
		if (isPrivileged) parts.push('privileged');
		if (capAddError === '') {
			const addCount = capAdd
				.split(',')
				.map((c) => c.trim())
				.filter(Boolean).length;
			if (addCount) parts.push(`+${addCount} caps`);
		}
		if (capDropError === '') {
			const dropCount = capDrop
				.split(',')
				.map((c) => c.trim())
				.filter(Boolean).length;
			if (dropCount) parts.push(`-${dropCount} caps`);
		}
		if (selinuxLabel.trim()) parts.push('selinux');
		if (apparmorProfile.trim()) parts.push('apparmor');
		if (!noNewPrivileges) parts.push('nnp off');
		return parts.length ? parts.join(' · ') : 'All defaults';
	});
</script>

<SectionCard
	title="Runtime & Security"
	description="Namespace isolation, host devices, privilege boundaries, and capabilities."
	collapsible={true}
	bind:open
>
	{#snippet headerActions()}
		<span class="max-w-[240px] truncate font-mono text-[11px] text-[var(--text-tertiary)]"
			>{summary}</span
		>
	{/snippet}

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<Input
			id="rt-userns"
			label="User namespace"
			description="Blank uses the podman default."
			bind:value={userNamespace}
			placeholder="keep-id"
			error={userNsError || undefined}
			class="font-mono text-xs"
		/>

		<Input
			id="rt-devices"
			label="Host devices"
			bind:value={devices}
			placeholder="/dev/fuse, /dev/net/tun"
			error={devicesError || undefined}
			class="font-mono text-xs"
		/>
	</div>

	<div class="flex flex-col gap-4">
		<div class="flex items-center justify-between gap-4">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">Privileged</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Access to all host devices; disables isolation. Keep off unless required.
				</span>
			</div>
			<Switch bind:checked={isPrivileged} size="sm" />
		</div>

		<div class="flex items-center justify-between gap-4">
			<div class="flex flex-col gap-0.5">
				<span class="text-xs font-medium text-[var(--text-primary)]">No new privileges</span>
				<span class="text-[11px] text-[var(--text-tertiary)]">
					Blocks setuid escalation inside the container. Enabled by default.
				</span>
			</div>
			<Switch bind:checked={noNewPrivileges} size="sm" />
		</div>
	</div>

	{#if privilegedCapDropWarning || privilegedUserNsWarning}
		<Banner variant="alert" size="sm">
			{#snippet icon()}
				<WarningCircle size={14} weight="fill" class="shrink-0" />
			{/snippet}
			<div class="flex flex-col gap-0.5 text-xs leading-relaxed">
				{#if privilegedCapDropWarning}
					<span>{privilegedCapDropWarning}</span>
				{/if}
				{#if privilegedUserNsWarning}
					<span>{privilegedUserNsWarning}</span>
				{/if}
			</div>
		</Banner>
	{/if}

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<Input
			id="sec-selinux"
			label="SELinux process label"
			description="Process type, e.g. spc_t or disable. Volumes use the Z / z flags."
			bind:value={selinuxLabel}
			placeholder="container_t"
			class="font-mono text-xs"
		/>

		<Input
			id="sec-apparmor"
			label="AppArmor profile"
			description="Set unconfined to disable."
			bind:value={apparmorProfile}
			placeholder="default"
			class="font-mono text-xs"
		/>

		<Input
			id="sec-cap-add"
			label="Add capabilities"
			bind:value={capAdd}
			placeholder="NET_BIND_SERVICE"
			error={capAddError || undefined}
			class="font-mono text-xs"
		/>

		<Input
			id="sec-cap-drop"
			label="Drop capabilities"
			bind:value={capDrop}
			placeholder="ALL"
			error={capDropError || undefined}
			class="font-mono text-xs"
		/>
	</div>
</SectionCard>
