<script lang="ts">
	import type { Service, Workload } from '$lib/types';
	import { TerminalView } from '$lib/components/ui';
	import { dataStore } from '$lib/data';
	import { parseComposeWorkloads } from '$lib/parsers/compose';

	interface Props {
		service: Service;
	}

	let { service }: Props = $props();

	let workloads = $derived.by<Workload[]>(() => {
		// 1. Podman Pod: populate all member containers in the Pod (excluding infra pause container)
		if (service.type === 'pod') {
			const podKey = service.id;
			const podNameKey = service.name;
			const podPrefixedKey = `pod_${service.name}`;

			const memberSvcs = dataStore.services.filter(
				(s) =>
					s.projectId === service.projectId &&
					s.id !== service.id &&
					(s.podId === podKey ||
						s.podId === podNameKey ||
						s.podId === podPrefixedKey ||
						(s.inPod && s.type !== 'pod'))
			);
			const memberServiceIds = new Set(memberSvcs.map((m) => m.id));

			const podContainers = dataStore.containers.filter(
				(c) =>
					!c.name.endsWith('-infra') &&
					(c.pod === podPrefixedKey ||
						c.pod === podNameKey ||
						c.pod === podKey ||
						memberServiceIds.has(c.serviceId) ||
						(c.labels && c.labels['io.gopod.service'] && memberServiceIds.has(c.labels['io.gopod.service'])) ||
						(c.name.includes(service.name) && !c.name.endsWith('-infra')))
			);

			if (podContainers.length > 0) {
				return podContainers.map((c) => ({
					name: c.name,
					image: c.image || '—',
					status: c.status || 'running'
				}));
			}

			if (memberSvcs.length > 0) {
				return memberSvcs.map((m) => ({
					name: m.name,
					image: m.image || '—',
					status: m.status || 'running'
				}));
			}
		}

		// 2. Live containers matching this service/project
		const matchContainers = dataStore.containers
			.filter(
				(c) =>
					!c.name.endsWith('-infra') &&
					(c.name === service.name ||
						c.name === service.id ||
						(c.labels &&
							(c.labels['io.gopod.service'] === service.id ||
								c.labels['io.gopod.name'] === service.name ||
								c.labels['com.docker.compose.project'] === service.id ||
								c.labels['io.podman.compose.project'] === service.id ||
								c.labels['com.docker.compose.service'] === service.name ||
								c.labels['io.podman.compose.service'] === service.name)) ||
						(service.id &&
							(c.name.startsWith(`${service.id}-`) || c.name.startsWith(`${service.id}_`))) ||
						(service.name && c.name.includes(service.name)))
			)
			.map((c) => ({
				name: c.name,
				image: c.image || '—',
				status: c.status || 'running'
			}));

		if (matchContainers.length > 0) return matchContainers;

		// 2. Explicit workloads
		if (service.workloads && service.workloads.length > 0) return service.workloads;

		// 3. Compose YAML parsed services
		if (service.composeYaml) {
			const parsed = parseComposeWorkloads(service.composeYaml);
			if (parsed.length > 0) return parsed;
		}

		// 4. Default single workload (prefer container name matching service.id)
		return [
			{ name: service.id || service.name, image: service.image ?? '—', status: service.status }
		];
	});

	let containerOverride = $state<string>('');
</script>

<div class="flex w-full flex-col gap-4">
	{#if service.status !== 'running'}
		<div
			class="flex items-center justify-between rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-4 py-2.5 text-xs text-[var(--text-secondary)]"
		>
			<div class="flex items-center gap-2">
				<span class="h-2 w-2 rounded-full bg-[var(--text-tertiary)]"></span>
				<span
					>Service is currently <strong>{service.status || 'stopped'}</strong>. Deploy or start the
					service to execute commands inside the container.</span
				>
			</div>
		</div>
	{/if}

	<TerminalView
		title={service.name}
		{workloads}
		bind:selectedWorkload={containerOverride}
		height="calc(100dvh - 280px)"
	/>
</div>
