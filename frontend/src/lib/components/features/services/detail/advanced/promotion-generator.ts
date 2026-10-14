import type { Service } from '$lib/types';
import {
	slugifyName,
	getSanitizedImage,
	parseAllComposeWorkloads,
	parseComposeWorkload
} from '$lib/parsers/compose-inspect';

export interface PromotionFilesResult {
	podFiles: Record<string, string>;
	activePromoTab: string;
	promoCode: string;
}

export function generatePromotionFiles(
	service: Service,
	restartPolicy: string,
	volumes: { source: string; target: string; options: string }[],
	allServices: Service[]
): PromotionFilesResult {
	const cleanSlug = slugifyName(service.name) || 'service';
	const comp = service.composeYaml ? parseComposeWorkload(service.composeYaml) : null;
	const targetImage = comp?.image ? comp.image : getSanitizedImage(service);
	const targetContainerName = comp?.containerName
		? slugifyName(comp.containerName)
		: slugifyName(service.id) || cleanSlug;
	const execDirective = comp?.exec ? `Exec=${comp.exec}\n` : '';

	let publishDirectives = '';
	if (comp && comp.ports.length > 0) {
		publishDirectives = comp.ports.map((p: string) => `PublishPort=${p}`).join('\n') + '\n';
	} else if (!service.composeYaml && service.hostPort && service.hostPort > 0) {
		publishDirectives = `PublishPort=${service.hostPort}:${service.port || service.hostPort}\n`;
	}

	let volumeDirectives = '';
	if (comp && comp.volumes.length > 0) {
		volumeDirectives = comp.volumes
			.map((v: string) => {
				if (!v.includes(':Z') && !v.includes(':z')) {
					return `Volume=${v}:Z`;
				}
				return `Volume=${v}`;
			})
			.join('\n');
	} else if (volumes.length > 0) {
		volumeDirectives = volumes
			.filter((v) => v.source.trim() && v.target.trim())
			.map((v) => `Volume=${slugifyName(v.source)}:${v.target}:${v.options || 'Z'}`)
			.join('\n');
	}

	const promoRestart = comp?.restart || restartPolicy || 'always';
	const multiCompose = service.composeYaml ? parseAllComposeWorkloads(service.composeYaml) : [];

	if (service.type === 'pod' || multiCompose.length > 1) {
		const files: Record<string, string> = {};
		const podKey = `pod_${cleanSlug}.pod`;

		if (service.type !== 'pod' && multiCompose.length > 1) {
			const allPorts = multiCompose.flatMap((c) => c.ports);
			const podPublishLine =
				allPorts.length > 0
					? allPorts.map((p: string) => `PublishPort=${p}`).join('\n') + '\n'
					: '';
			files[podKey] = `[Unit]
Description=GoPod ${cleanSlug} Pod
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Pod]
PodName=pod_${cleanSlug}
Network=gopod-net
${podPublishLine}
[Install]
WantedBy=default.target`;

			for (const m of multiCompose) {
				const mSlug = slugifyName(m.containerName || m.name) || 'member';
				const memberKey = `${mSlug}.container`;
				const mImg = m.image || `docker.io/library/${mSlug}:latest`;
				const mExec = m.exec ? `Exec=${m.exec}\n` : '';
				const mEnv =
					m.environment.length > 0
						? m.environment.map((e: string) => `Environment=${e}`).join('\n') + '\n'
						: '';
				const mVol =
					m.volumes.length > 0
						? m.volumes
								.map((v: string) => `Volume=${v.includes(':') ? v : `${slugifyName(v)}:${v}:Z`}`)
								.join('\n') + '\n'
						: '';

				files[memberKey] = `[Unit]
Description=GoPod Pod Member ${mSlug}
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=${mSlug}
Image=${mImg}
Pod=pod_${cleanSlug}.pod
${mExec}${mEnv}${mVol}Label=io.gopod.service=${service.id}
Label=io.gopod.project=${service.projectId}

[Service]
Restart=${m.restart || 'always'}
TimeoutStartSec=300

[Install]
WantedBy=default.target`;
			}
		} else {
			const podPublishLine = publishDirectives;
			const podUnit = `[Unit]
Description=GoPod Pod ${cleanSlug}
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Pod]
PodName=pod_${cleanSlug}
Network=gopod-net
${podPublishLine}
[Install]
WantedBy=default.target`;

			const memberServices = allServices.filter(
				(s) =>
					s.projectId === service.projectId &&
					s.id !== service.id &&
					s.type !== 'pod' &&
					(s.podId === service.id ||
						s.podId === service.name ||
						s.podId === `pod_${service.name}` ||
						s.podId === `pod_${cleanSlug}` ||
						(s.inPod &&
							(!s.podId || s.podId === service.name || s.podId === `pod_${service.name}`)))
			);

			files[podKey] = podUnit;

			if (memberServices.length === 0) {
				const memberKey = `${cleanSlug}-app.container`;
				files[memberKey] = `[Unit]
Description=GoPod Pod Member ${cleanSlug}
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=${targetContainerName}
Image=${targetImage}
Pod=pod_${cleanSlug}.pod
${execDirective}${volumeDirectives ? volumeDirectives + '\n' : ''}Label=io.gopod.service=${service.id}
Label=io.gopod.project=${service.projectId}

[Service]
Restart=${promoRestart}
TimeoutStartSec=300

[Install]
WantedBy=default.target`;
			} else {
				for (const m of memberServices) {
					const mSlug = slugifyName(m.name) || 'member';
					const memberKey = `${mSlug}.container`;
					const mImg = getSanitizedImage(m);
					files[memberKey] = `[Unit]
Description=GoPod Pod Member ${mSlug}
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=${mSlug}
Image=${mImg}
Pod=pod_${cleanSlug}.pod
Label=io.gopod.service=${m.id}
Label=io.gopod.project=${service.projectId}

[Service]
Restart=${m.restartPolicy || 'always'}
TimeoutStartSec=300

[Install]
WantedBy=default.target`;
				}
			}
		}

		return {
			podFiles: files,
			activePromoTab: podKey,
			promoCode: files[podKey] || ''
		};
	}

	return {
		podFiles: {},
		activePromoTab: '',
		promoCode: `[Unit]
Description=GoPod Quadlet ${cleanSlug}
After=network-online.target
StartLimitIntervalSec=60s
StartLimitBurst=5

[Container]
ContainerName=${targetContainerName}
Image=${targetImage}
${execDirective}${publishDirectives}${volumeDirectives ? volumeDirectives + '\n' : ''}Label=io.gopod.service=${service.id}
Label=io.gopod.project=${service.projectId}

[Service]
Restart=${promoRestart}
TimeoutStartSec=300

[Install]
WantedBy=default.target`
	};
}

export function validateQuadletSyntax(promoCode: string): { text: string; success: boolean } {
	const code = promoCode.trim();
	if (!code.includes('[Unit]')) {
		return {
			text: 'Validation error: Missing [Unit] section in systemd specification.',
			success: false
		};
	}
	if (!code.includes('[Container]') && !code.includes('[Pod]')) {
		return {
			text: 'Validation error: Quadlet unit must contain either [Container] or [Pod] section.',
			success: false
		};
	}
	if (code.includes('[Container]') && !code.includes('Image=')) {
		return {
			text: 'Validation error: [Container] section must define Image= directive.',
			success: false
		};
	}
	const containerIdx = code.indexOf('[Container]');
	const serviceIdx = code.indexOf('[Service]');
	if (containerIdx !== -1) {
		const containerSection =
			serviceIdx !== -1 ? code.slice(containerIdx, serviceIdx) : code.slice(containerIdx);
		if (containerSection.includes('Restart=')) {
			return {
				text: 'Validation warning: Restart= directive belongs in [Service] section, not [Container].',
				success: false
			};
		}
	}
	return {
		text: 'Syntax valid! Quadlet unit matches systemd specification and podman-system-generator conventions.',
		success: true
	};
}
