export const credentialKeys = {
	all: ['credentials'] as const,
	sshKeys: ['credentials', 'ssh-keys'] as const,
	registries: ['credentials', 'registries'] as const,
	secrets: ['credentials', 'secrets'] as const
};
