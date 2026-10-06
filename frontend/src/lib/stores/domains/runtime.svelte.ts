import type { Container, Pod, Image, Volume, Network, PortMapping } from '$lib/types';
import { api } from '$lib/api';

export class RuntimeDomainStore {
	containers = $state<Container[]>([]);
	pods = $state<Pod[]>([]);
	images = $state<Image[]>([]);
	volumes = $state<Volume[]>([]);
	networks = $state<Network[]>([]);
	ports = $state<PortMapping[]>([]);

	getServiceContainers(serviceId: string): Container[] {
		return this.containers.filter((c) => c.serviceId === serviceId);
	}

	async startContainer(id: string) {
		const found = this.containers.find((c) => c.id === id);
		if (found) found.status = 'running';
		await api.runtime.containers.start(id);
	}

	async stopContainer(id: string) {
		const found = this.containers.find((c) => c.id === id);
		if (found) found.status = 'stopped';
		await api.runtime.containers.stop(id);
	}

	async restartContainer(id: string) {
		await api.runtime.containers.restart(id);
	}

	async deleteContainer(id: string, force = false) {
		this.containers = this.containers.filter((c) => c.id !== id);
		await api.runtime.containers.delete(id, force);
	}

	async pruneImages(all = false) {
		await api.runtime.images.prune(all);
	}

	async pruneVolumes() {
		await api.runtime.volumes.prune();
	}

	async pruneSystem() {
		await api.system.prune();
	}

	async fetchRuntimeData() {
		try {
			const [podsData, imagesData, volumesData, networksData] = await Promise.all([
				api.runtime.pods.list().catch(() => []),
				api.runtime.images.list().catch(() => []),
				api.runtime.volumes.list().catch(() => []),
				api.runtime.networks.list().catch(() => [])
			]);

			if (Array.isArray(podsData)) {
				this.pods = podsData.map((p: any) => ({
					id: p.id,
					name: p.name,
					projectId: 'system',
					projectName: 'Podman Pod',
					containers: p.containers || [],
					status: p.status?.toLowerCase() === 'running' ? 'running' : 'stopped',
					network: p.network || 'bridge',
					createdAt: p.created || 'Recent'
				}));
			}

			if (Array.isArray(imagesData)) {
				this.images = imagesData.map((img: any) => ({
					id: img.id,
					name: img.name || img.repository || 'image',
					tag: img.tag || 'latest',
					size: img.size || '—',
					usedBy: 'Active',
					createdAt: img.createdAt || 'Recent'
				}));
			}

			if (Array.isArray(volumesData)) {
				this.volumes = volumesData.map((v: any) => ({
					id: v.name,
					name: v.name,
					projectId: 'system',
					projectName: 'Host Storage',
					serviceId: 'volume',
					serviceName: v.driver || 'local',
					mount: v.mount || v.mountPoint || '—',
					size: v.size || 'Active',
					status: 'mounted'
				}));
			}

			if (Array.isArray(networksData)) {
				this.networks = networksData.map((n: any) => ({
					id: n.id,
					name: n.name,
					driver: n.driver || 'bridge',
					projects: [n.name],
					containers: 0,
					subnet: n.subnet || '—',
					gateway: n.gateway || '—'
				}));
			}
		} catch (e) {
			console.warn('[RuntimeDomainStore] Error fetching runtime data:', e);
		}
	}
}
