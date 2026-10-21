export interface SSHKey {
	id: string;
	name: string;
	publicKey: string;
	privateKey?: string;
	fingerprint: string;
	type: 'ed25519' | 'rsa';
	createdAt: string;
}

export interface ContainerRegistry {
	id: string;
	name: string;
	url: string;
	username: string;
	token?: string;
	createdAt: string;
}

export interface PodmanSecret {
	id: string;
	name: string;
	createdAt: string;
	driver?: string;
}

export interface GitToken {
	id: string;
	name: string;
	provider: 'github' | 'gitlab' | 'gitea' | 'custom';
	username: string;
	token?: string;
	createdAt: string;
}
