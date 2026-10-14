<script lang="ts">
	import type { Service } from '$lib/types';
	import { Input } from '$lib/components/ui';
	import { Info } from 'phosphor-svelte';

	interface Props {
		service: Service;
		containerPort: string;
		hostPort: string;
	}

	let { service, containerPort = $bindable(), hostPort = $bindable() }: Props = $props();
</script>

<div class="flex flex-col gap-3 border-t border-kumo-hairline pt-5">
	<div class="flex flex-col gap-0.5">
		<h3 class="m-0 text-xs font-semibold text-kumo-default">Networking & ports</h3>
		<p class="m-0 text-[11px] text-kumo-subtle">
			Configure container listening port and optional host bindings.
		</p>
	</div>

	<!-- Compact 2-Column Port Inputs -->
	<div class="grid max-w-xl grid-cols-1 gap-4 sm:grid-cols-2">
		<!-- Internal Port -->
		<div class="flex flex-col gap-1.5">
			<label for="internal-port" class="text-xs font-medium text-kumo-default">
				Internal container port <span class="font-normal text-kumo-subtle">(optional)</span>
			</label>
			<Input
				id="internal-port"
				bind:value={containerPort}
				placeholder="e.g. 80, 8080 (optional)"
				class="h-8.5 w-full font-mono text-xs"
			/>
			<span class="text-[11px] leading-normal text-kumo-subtle">
				Port your application listens on. Leave empty if this is a background worker or non-HTTP
				service.
			</span>
		</div>

		<!-- Published Host Port -->
		<div class="flex flex-col gap-1.5">
			<label for="host-port" class="text-xs font-medium text-kumo-default">
				Published host port <span class="font-normal text-kumo-subtle">(optional)</span>
			</label>
			<Input
				id="host-port"
				bind:value={hostPort}
				placeholder="e.g. 8080 (optional)"
				class="h-8.5 w-full font-mono text-xs"
			/>
			<span class="text-[11px] leading-normal text-kumo-subtle">
				Direct host exposure (0.0.0.0:port). Leave empty to isolate container on <code
					>gopod-net</code
				>.
			</span>
		</div>
	</div>

	<!-- Compact Inline Tip Banner -->
	<div
		class="flex items-center gap-2 rounded-md border border-kumo-hairline bg-kumo-base px-3 py-2 text-xs text-kumo-subtle"
	>
		<Info size={14} class="shrink-0 text-[var(--accent)]" />
		<span>
			Containers on <strong>gopod-net</strong> are reachable by service name (e.g.
			<code class="font-mono text-kumo-default">http://{service.name}:{containerPort || '80'}</code
			>) without publishing host ports.
		</span>
	</div>
</div>
