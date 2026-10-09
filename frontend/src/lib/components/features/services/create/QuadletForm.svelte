<script lang="ts">
	import { Input, Button } from '$lib/components/primitives';
	import { CodeEditor } from '$lib/components/ui';
	import { CheckCircle, WarningCircle, UploadSimple } from 'phosphor-svelte';

	interface Props {
		name: string;
		description: string;
		config: string;
	}

	let {
		name = $bindable(''),
		description = $bindable(''),
		config = $bindable(`[Unit]
Description=Managed Quadlet Container
After=network-online.target

[Container]
Image=docker.io/library/nginx:alpine
PublishPort=8080:80
AutoUpdate=registry

[Service]
Restart=always

[Install]
WantedBy=default.target`)
	}: Props = $props();

	let validationStatus = $state<'idle' | 'valid' | 'invalid'>('idle');
	let validationMsg = $state('');

	function validateQuadlet() {
		try {
			if (!config.trim()) {
				validationStatus = 'invalid';
				validationMsg = 'Quadlet configuration cannot be empty.';
				return;
			}
			if (!config.includes('[Container]') && !config.includes('[Pod]')) {
				validationStatus = 'invalid';
				validationMsg = 'Configuration must include a [Container] or [Pod] section.';
				return;
			}
			validationStatus = 'valid';
			validationMsg = 'Valid systemd Quadlet specification.';
		} catch (err: any) {
			validationStatus = 'invalid';
			validationMsg = err.message || 'Syntax error.';
		}
	}

	function handleFileUpload(e: Event) {
		const target = e.target as HTMLInputElement;
		const file = target.files?.[0];
		if (!file) return;

		const reader = new FileReader();
		reader.onload = (evt) => {
			if (typeof evt.target?.result === 'string') {
				config = evt.target.result;
				if (!name) {
					// Use filename without extension as service name
					name = file.name.replace(/\.(container|pod|kube|volume|network)$/i, '');
				}
				validateQuadlet();
			}
		};
		reader.readAsText(file);
	}
</script>

<div class="flex flex-col gap-4">
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
		<div class="flex flex-col gap-1.5">
			<label for="quadlet-svc-name" class="text-xs font-medium text-[var(--text-secondary)]">
				Service name <span class="text-[var(--status-red)]">*</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input id="quadlet-svc-name" bind:value={name} placeholder="e.g. system-daemon" />
			</div>
		</div>

		<div class="flex flex-col gap-1.5">
			<label for="quadlet-svc-desc" class="text-xs font-medium text-[var(--text-secondary)]">
				Description <span class="text-[var(--text-tertiary)]">(optional)</span>
			</label>
			<div
				class="rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] px-3 py-2 focus-within:border-[var(--accent)]"
			>
				<Input
					id="quadlet-svc-desc"
					bind:value={description}
					placeholder="Declarative systemd Quadlet unit"
				/>
			</div>
		</div>
	</div>

	<div class="flex flex-col gap-2">
		<div class="flex items-center justify-between">
			<label for="quadlet-editor" class="text-xs font-medium text-[var(--text-secondary)]">
				Quadlet unit configuration (.container) <span class="text-[var(--status-red)]">*</span>
			</label>
			<div class="flex items-center gap-2">
				<label
					class="inline-flex cursor-pointer items-center gap-1 rounded border border-[var(--border)] bg-[var(--bg-surface)] px-2.5 py-1 text-xs text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
				>
					<UploadSimple size={13} />
					Import file
					<input
						type="file"
						accept=".container,.pod,.kube"
						onchange={handleFileUpload}
						class="sr-only"
					/>
				</label>
				{#if validationStatus === 'valid'}
					<span class="flex items-center gap-1 text-[11px] text-[var(--status-green)]">
						<CheckCircle size={14} />
						{validationMsg}
					</span>
				{:else if validationStatus === 'invalid'}
					<span class="flex items-center gap-1 text-[11px] text-[var(--status-red)]">
						<WarningCircle size={14} />
						{validationMsg}
					</span>
				{/if}
				<Button variant="secondary" size="sm" onclick={validateQuadlet}>Validate</Button>
			</div>
		</div>

		<CodeEditor bind:value={config} language="quadlet" height="260px" />
		<span class="text-[11px] text-[var(--text-tertiary)]">
			Quadlets are converted by Podman generator into systemd service units.
		</span>
	</div>
</div>
