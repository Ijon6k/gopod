<script lang="ts">
	import { SectionCard, Radio, Input, Switch } from '$lib/components/ui';

	interface Props {
		openRuntime?: boolean;
		openSecurity?: boolean;
		runtimeMode: 'rootless' | 'rootful';
		userNamespace: string;
		devices: string;
		isPrivileged: boolean;
		selinuxLabel: string;
		apparmorProfile: string;
		capAdd: string;
		capDrop: string;
	}

	let {
		openRuntime = $bindable(true),
		openSecurity = $bindable(true),
		runtimeMode = $bindable(),
		userNamespace = $bindable(),
		devices = $bindable(),
		isPrivileged = $bindable(),
		selinuxLabel = $bindable(),
		apparmorProfile = $bindable(),
		capAdd = $bindable(),
		capDrop = $bindable()
	}: Props = $props();
</script>

<!-- 1. RUNTIME CONFIGURATION -->
<SectionCard
	title="Runtime Execution"
	description="Rootless execution isolation, user namespace mappings, and host devices."
	collapsible={true}
	bind:open={openRuntime}
>
	<div class="flex flex-col gap-1.5 pt-2">
		<span class="text-xs font-medium text-[var(--text-secondary)]">Podman Mode</span>
		<Radio
			value={runtimeMode}
			onValueChange={(val: any) => {
				if (val === 'rootless' || val === 'rootful') runtimeMode = val;
			}}
			appearance="card"
			options={[
				{
					label: 'Rootless (Default)',
					value: 'rootless',
					description:
						'Runs entirely in unprivileged user space without root privileges. Standard for PaaS workloads.'
				},
				{
					label: 'Rootful',
					value: 'rootful',
					description:
						'Runs via system daemon with root permissions. Required for low privileged system ports <1024 or direct raw devices.'
				}
			]}
		/>
	</div>

	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
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
					placeholder="keep-id (leave blank for rootless default)"
					class="text-xs font-[var(--font-mono)]"
				/>
			</div>
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
		</div>
	</div>
</SectionCard>
