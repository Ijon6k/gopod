<script lang="ts">
	import { SectionCard, Input, Switch } from '$lib/components/ui';
	import { WarningCircle } from 'phosphor-svelte';

	interface Props {
		openRuntime?: boolean;
		openSecurity?: boolean;
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
		openRuntime = $bindable(true),
		openSecurity = $bindable(true),
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
</script>

<!-- 1. RUNTIME CONFIGURATION -->
<SectionCard
	title="Runtime Execution"
	description="User namespace isolation and host device access — applied as real podman run flags."
	collapsible={true}
	bind:open={openRuntime}
>
	<div class="grid grid-cols-1 gap-4 pt-2 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="rt-userns" class="text-xs font-medium text-[var(--text-secondary)]"
				>User Namespace</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="rt-userns"
					bind:value={userNamespace}
					placeholder="keep-id (leave blank for podman default)"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if userNsError}
				<span class="text-[11px] text-[var(--status-red)]">{userNsError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--userns</code>; blank, <code>default</code> and <code>none</code> leave the podman
					default untouched.
				</span>
			{/if}
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="rt-devices" class="text-xs font-medium text-[var(--text-secondary)]"
				>Host Devices</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="rt-devices"
					bind:value={devices}
					placeholder="/dev/fuse, /dev/net/tun"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if devicesError}
				<span class="text-[11px] text-[var(--status-red)]">{devicesError}</span>
			{:else}
				<span class="text-[10px] text-[var(--text-tertiary)]">
					Emits <code>--device</code> for each host path.
				</span>
			{/if}
		</div>
	</div>
</SectionCard>

<!-- 3. SECURITY & CAPABILITIES -->
<SectionCard
	title="Security & Capabilities"
	description="SELinux confinement, Linux kernel capabilities, and privilege boundaries."
	collapsible={true}
	bind:open={openSecurity}
>
	<div
		class="mt-2 flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
	>
		<div class="flex flex-col gap-0.5">
			<span class="text-xs font-medium text-[var(--text-primary)]">Privileged Container Mode</span>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Disables SELinux isolation and grants access to all host kernel devices. Keep disabled for
				standard PaaS apps.
			</span>
		</div>
		<Switch bind:checked={isPrivileged} size="sm" />
	</div>

	{#if privilegedCapDropWarning || privilegedUserNsWarning}
		<div
			class="flex items-start gap-2 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
		>
			<WarningCircle size={14} weight="fill" class="mt-0.5 shrink-0 text-kumo-warning" />
			<div class="flex flex-col gap-0.5">
				{#if privilegedCapDropWarning}
					<span class="text-[11px] leading-relaxed text-[var(--text-secondary)]"
						>{privilegedCapDropWarning}</span
					>
				{/if}
				{#if privilegedUserNsWarning}
					<span class="text-[11px] leading-relaxed text-[var(--text-secondary)]"
						>{privilegedUserNsWarning}</span
					>
				{/if}
			</div>
		</div>
	{/if}

	<div
		class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-3"
	>
		<div class="flex flex-col gap-0.5">
			<span class="text-xs font-medium text-[var(--text-primary)]">No New Privileges</span>
			<span class="text-[11px] text-[var(--text-tertiary)]">
				Emits <code>--security-opt no-new-privileges</code> so setuid escalation cannot grant additional
				rights. Enabled by default for hardening.
			</span>
		</div>
		<Switch bind:checked={noNewPrivileges} size="sm" />
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-3">
		<div class="flex flex-col gap-1.5">
			<label for="sec-selinux" class="text-xs font-medium text-[var(--text-secondary)]"
				>SELinux Process Label</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="sec-selinux"
					bind:value={selinuxLabel}
					placeholder="container_t, spc_t, or disable"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<span class="text-[10px] text-[var(--text-tertiary)]">
				Process type (e.g. <code>spc_t</code> for unconfined, or <code>disable</code>). For volumes,
				use <code>:z</code>/<code>:Z</code> below.
			</span>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="sec-apparmor" class="text-xs font-medium text-[var(--text-secondary)]"
				>AppArmor Profile (Ubuntu/Debian)</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="sec-apparmor"
					bind:value={apparmorProfile}
					placeholder="default or unconfined"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			<span class="text-[10px] text-[var(--text-tertiary)]">
				Standard MAC on Ubuntu/Debian. Set to <code>unconfined</code> to disable AppArmor.
			</span>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="sec-cap-add" class="text-xs font-medium text-[var(--text-secondary)]"
				>Add Capabilities</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="sec-cap-add"
					bind:value={capAdd}
					placeholder="NET_BIND_SERVICE"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if capAddError}
				<span class="text-[11px] text-[var(--status-red)]">{capAddError}</span>
			{/if}
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="sec-cap-drop" class="text-xs font-medium text-[var(--text-secondary)]"
				>Drop Capabilities</label
			>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2"
			>
				<Input
					id="sec-cap-drop"
					bind:value={capDrop}
					placeholder="ALL"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
			{#if capDropError}
				<span class="text-[11px] text-[var(--status-red)]">{capDropError}</span>
			{/if}
		</div>
	</div>
</SectionCard>
