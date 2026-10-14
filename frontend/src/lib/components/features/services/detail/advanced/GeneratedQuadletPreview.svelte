<script lang="ts">
	import type { Service } from '$lib/types';
	import { SectionCard, Button, CodeEditor } from '$lib/components/ui';
	import { DownloadSimple } from 'phosphor-svelte';
	import { getSanitizedImage } from '$lib/parsers/compose-inspect';

	interface Props {
		openGenerated?: boolean;
		service: Service;
		restartPolicy: string;
		userNamespace: string;
		volumes: { source: string; target: string; options: string }[];
		cpuLimit: string;
		memoryLimit: string;
		pidsLimit: string;
		isPrivileged: boolean;
		noNewPrivileges: boolean;
	}

	let {
		openGenerated = $bindable(true),
		service,
		restartPolicy,
		userNamespace,
		volumes,
		cpuLimit,
		memoryLimit,
		pidsLimit,
		isPrivileged,
		noNewPrivileges
	}: Props = $props();

	let genTab = $state<'quadlet' | 'kubernetes' | 'podman'>('quadlet');

	let generatedQuadlet = $derived(`[Unit]
Description=GoPod ${service.name} Service
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
Image=${getSanitizedImage(service)}
ContainerName=${service.projectId}-${service.name}
${service.hostPort && service.hostPort > 0 ? `PublishPort=${service.hostPort}:${service.port || service.hostPort}\n` : ''}AutoUpdate=registry
${userNamespace && userNamespace !== 'default' ? `UserNS=${userNamespace}\n` : ''}${volumes.map((v) => `Volume=${v.source}:${v.target}:${v.options}`).join('\n')}

[Service]
Restart=${restartPolicy}
TimeoutStartSec=300

[Install]
WantedBy=default.target`);

	let generatedKube = $derived(`apiVersion: v1
kind: Pod
metadata:
  name: ${service.projectId}-${service.name}
  labels:
    app.kubernetes.io/name: ${service.name}
    gopod.io/project: ${service.projectId}
spec:
  restartPolicy: Always
  containers:
    - name: ${service.name}
      image: ${service.image || 'localhost/' + service.name + ':latest'}
      ${service.port && service.port > 0 ? `ports:\n        - containerPort: ${service.port}` : ''}
      resources:
        limits:
          cpu: "${cpuLimit}"
          memory: "${memoryLimit}"
      securityContext:
        privileged: ${isPrivileged}
        allowPrivilegeEscalation: ${!noNewPrivileges}`);

	let generatedPodmanRun = $derived(`podman run -d \\
  --name ${service.projectId}-${service.name} \\
${userNamespace && userNamespace !== 'default' ? `  --userns ${userNamespace} \\\n` : ''}  --restart ${restartPolicy} \\
  ${service.hostPort && service.hostPort > 0 ? `-p ${service.hostPort}:${service.port || service.hostPort} \\\n  ` : ''}--cpus ${cpuLimit} \\
  --memory ${memoryLimit} \\
  --pids-limit ${pidsLimit} \\
  ${volumes.map((v) => `-v ${v.source}:${v.target}:${v.options} \\`).join('\n  ')}
  ${service.image || 'localhost/' + service.name + ':latest'}`);

	let currentGenCode = $derived(
		genTab === 'quadlet'
			? generatedQuadlet
			: genTab === 'kubernetes'
				? generatedKube
				: generatedPodmanRun
	);

	function downloadConfig() {
		const ext = genTab === 'quadlet' ? 'container' : genTab === 'kubernetes' ? 'yaml' : 'sh';
		const blob = new Blob([currentGenCode], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${service.name}.${ext}`;
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<SectionCard
	title="Generated Configuration"
	description="Export generated Quadlet unit, Kubernetes YAML, or standalone Podman run command."
	collapsible={true}
	bind:open={openGenerated}
>
	<div class="flex flex-col justify-between gap-3 pt-2 sm:flex-row sm:items-center">
		<!-- Format selector tabs -->
		<div
			class="flex items-center gap-1 rounded-[var(--radius-sm)] border border-[var(--border)] bg-[var(--bg-surface)] p-0.5"
		>
			<button
				type="button"
				onclick={() => (genTab = 'quadlet')}
				class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
				'quadlet'
					? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				Quadlet (.container)
			</button>
			<button
				type="button"
				onclick={() => (genTab = 'kubernetes')}
				class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
				'kubernetes'
					? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				Kubernetes YAML
			</button>
			<button
				type="button"
				onclick={() => (genTab = 'podman')}
				class="cursor-pointer rounded border-0 px-2.5 py-1 text-xs transition-colors {genTab ===
				'podman'
					? 'bg-[var(--bg-panel)] font-medium text-[var(--text-primary)]'
					: 'bg-transparent text-[var(--text-tertiary)] hover:text-[var(--text-secondary)]'}"
			>
				Podman Run CLI
			</button>
		</div>

		<Button variant="secondary" size="sm" onclick={downloadConfig}>
			<DownloadSimple size={13} /> Download
		</Button>
	</div>

	<CodeEditor
		value={currentGenCode}
		readOnly={true}
		language={genTab === 'quadlet' ? 'quadlet' : genTab === 'kubernetes' ? 'yaml' : 'text'}
		height="auto"
	/>
</SectionCard>
