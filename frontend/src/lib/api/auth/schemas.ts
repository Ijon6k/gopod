import { z } from 'zod';

export const loginSchema = z.object({
	email: z
		.string()
		.trim()
		.min(1, 'Email address is required')
		.email('Please enter a valid email address'),
	password: z
		.string()
		.min(1, 'Password is required')
});

export type LoginSchema = z.infer<typeof loginSchema>;

export const setupSchema = z
	.object({
		name: z
			.string()
			.trim()
			.min(2, 'Administrator name must be at least 2 characters')
			.max(64, 'Administrator name cannot exceed 64 characters'),
		email: z
			.string()
			.trim()
			.min(1, 'Administrator email address is required')
			.email('Please enter a valid administrator email address'),
		password: z
			.string()
			.min(8, 'Password must be at least 8 characters long')
			.max(128, 'Password cannot exceed 128 characters'),
		confirmPassword: z.string().min(1, 'Please confirm your password')
	})
	.refine((data) => data.password === data.confirmPassword, {
		message: 'Passwords do not match. Please re-enter.',
		path: ['confirmPassword']
	});

export type SetupSchema = z.infer<typeof setupSchema>;
