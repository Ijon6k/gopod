import { clsx, type ClassValue } from 'clsx';

/**
 * Utility for conditional class names.
 * Wraps clsx for consistent usage across all components.
 */
export function cn(...inputs: ClassValue[]): string {
	return clsx(inputs);
}
