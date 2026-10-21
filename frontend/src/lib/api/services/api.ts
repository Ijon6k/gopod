import apiClient from '../client';
import type {
	Service,
	Deployment,
	DeployResponse,
	JournalResponse,
	QuadletValidationResponse,
	QuadletPreviewResponse,
	GroupPodRequest,
	WebhookTokenResponse,
	WebhookSecretResponse,
	RevealEnvResponse
} from './types';

export const servicesApi = {
	async list(projectId?: string): Promise<Service[]> {
		const res = await apiClient.get<Service[]>('/services', {
			params: projectId ? { projectId } : undefined
		});
		return res.data;
	},

	async get(id: string): Promise<Service> {
		const res = await apiClient.get<Service>(`/services/${id}`);
		return res.data;
	},

	async create(service: Partial<Service>): Promise<Service> {
		const res = await apiClient.post<Service>('/services', service);
		return res.data;
	},

	async update(id: string, service: Partial<Service>): Promise<Service> {
		const res = await apiClient.put<Service>(`/services/${id}`, service);
		return res.data;
	},

	async delete(id: string, deleteVolumes = false): Promise<{ message: string }> {
		const res = await apiClient.delete<{ message: string }>(
			`/services/${id}${deleteVolumes ? '?delete_volumes=true' : ''}`
		);
		return res.data;
	},

	async deploy(
		id: string,
		trigger = 'manual',
		commit?: string,
		image?: string
	): Promise<DeployResponse> {
		const res = await apiClient.post<DeployResponse>(`/services/${id}/deploy`, {
			trigger,
			commit,
			image
		});
		return res.data;
	},

	async start(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/start`);
		return res.data;
	},

	async stop(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/stop`);
		return res.data;
	},

	async restart(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/restart`);
		return res.data;
	},

	async deployments(id: string): Promise<Deployment[]> {
		const res = await apiClient.get<Deployment[]>(`/services/${id}/deployments`);
		return res.data;
	},

	async getDeployment(id: string): Promise<Deployment> {
		const res = await apiClient.get<Deployment>(`/deployments/${id}`);
		return res.data;
	},

	async cancelDeployment(id: string): Promise<Deployment> {
		const res = await apiClient.post<Deployment>(`/deployments/${id}/cancel`);
		return res.data;
	},

	async cancelServiceDeployment(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/cancel`);
		return res.data;
	},

	async deploymentLogs(id: string): Promise<{ deploymentId: string; logs: string }> {
		const res = await apiClient.get<{ deploymentId: string; logs: string }>(
			`/deployments/${id}/logs`
		);
		return res.data;
	},

	getWebhookUrl(token: string): string {
		if (typeof window === 'undefined') return `/api/deploy/webhook/${token}`;
		return `${window.location.protocol}//${window.location.host}/api/deploy/webhook/${token}`;
	},

	/** Fetches the webhook token on demand. Never part of service list/get payloads. */
	async getWebhookToken(id: string): Promise<WebhookTokenResponse> {
		const res = await apiClient.get<WebhookTokenResponse>(`/services/${id}/webhook`);
		return res.data;
	},

	/** Issues a fresh webhook token; the previous URL stops working immediately. */
	async rotateWebhookToken(id: string): Promise<WebhookTokenResponse> {
		const res = await apiClient.post<WebhookTokenResponse>(`/services/${id}/webhook/rotate`);
		return res.data;
	},

	/** Issues a fresh signature secret; the value is returned once and never serialized again. */
	async rotateWebhookSecret(id: string): Promise<WebhookSecretResponse> {
		const res = await apiClient.post<WebhookSecretResponse>(`/services/${id}/webhook/secret`);
		return res.data;
	},

	/** Reveals the decrypted value of the requested env keys (explicit operator action). */
	async revealEnv(id: string, keys: string[]): Promise<Record<string, string>> {
		const res = await apiClient.post<RevealEnvResponse>(`/services/${id}/env/reveal`, { keys });
		return res.data.values ?? {};
	},

	streamDeploymentLogs(
		id: string,
		onMessage: (data: string) => void,
		onError?: (error: Event) => void
	): () => void {
		if (typeof window === 'undefined') return () => {};
		const es = new EventSource(`/api/deployments/${id}/logs/stream`);
		es.onmessage = (event) => {
			if (event.data) onMessage(event.data);
		};
		if (onError) {
			es.onerror = onError;
		} else {
			es.onerror = () => es.close();
		}
		return () => es.close();
	},

	async getJournal(id: string, lines = 100): Promise<JournalResponse> {
		const res = await apiClient.get<JournalResponse>(`/services/${id}/journal`, {
			params: { lines }
		});
		return res.data;
	},

	async validateQuadlet(
		id: string,
		payload?: { content?: string; files?: Record<string, string> }
	): Promise<QuadletValidationResponse> {
		const res = await apiClient.post<QuadletValidationResponse>(
			`/services/${id}/quadlet/validate`,
			payload || {}
		);
		return res.data;
	},

	async validateQuadletGlobal(payload: {
		content?: string;
		files?: Record<string, string>;
	}): Promise<QuadletValidationResponse> {
		const res = await apiClient.post<QuadletValidationResponse>('/quadlet/validate', payload);
		return res.data;
	},

	async demoteQuadlet(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/quadlet/demote`);
		return res.data;
	},

	async quadletPreview(id: string): Promise<QuadletPreviewResponse> {
		const res = await apiClient.post<QuadletPreviewResponse>(`/services/${id}/quadlet/preview`);
		return res.data;
	},

	async groupIntoPod(id: string, data: GroupPodRequest): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/group-pod`, data);
		return res.data;
	},

	async ungroupFromPod(id: string): Promise<Service> {
		const res = await apiClient.post<Service>(`/services/${id}/ungroup-pod`);
		return res.data;
	}
};

export default servicesApi;
