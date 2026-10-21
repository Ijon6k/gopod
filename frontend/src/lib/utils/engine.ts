import type { Server } from '$lib/types';

/**
 * Human label for the host container engine. "Podman" on podman hosts,
 * "Docker" on docker hosts; "Runtime" keeps speaking honestly until
 * system info reports which one is attached.
 */
export function engineLabel(server: Pick<Server, 'engine'> | null | undefined): string {
	if (server?.engine === 'docker') return 'Docker';
	if (server?.engine) return 'Podman';
	return 'Runtime';
}

export function engineVersionLabel(
	server:
		| Partial<Pick<Server, 'engineVersion' | 'podmanVersion'>>
		| null
		| undefined
): string {
	return server?.engineVersion || server?.podmanVersion || '';
}
