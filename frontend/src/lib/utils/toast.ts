// Toast option builders keep notification policy in one place.
// Rule of thumb: toasts announce — long reports belong in a modal opened
// from a toast action, never crammed into the notification itself.
import type { KumoToastOptions } from 'kumo-svelte';

// Errors stay on screen until dismissed so long details can be read.
export function errorToastOptions(title: string, description?: string): KumoToastOptions {
	return { description, title, variant: 'error', timeout: 0 };
}
