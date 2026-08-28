/**
 * Authentication service - Thin wrapper that delegates to the api service
 *
 * Provides clean API call structure with error handling.
 * Uses httpOnly cookies for secure JWT storage (handled automatically by fetch).
 *
 * NOTE: Don't use this directly - use the auth store instead (lib/stores/auth.ts)
 */

import { api } from '$lib/services/api';
import type { AuthUserResponse } from '$lib/api/generated/src';

export interface AuthError {
	message: string;
	status?: number;
}

export class AuthService {
	async login(
		username: string,
		password: string,
		rememberMe: boolean = false
	): Promise<AuthUserResponse> {
		try {
			const response = await api.login(username, password, rememberMe);
			return response.user!;
		} catch (error: unknown) {
			// Extract error message from response
			const message = await this.extractErrorMessage(error);
			const status =
				error && typeof error === 'object' && 'status' in error
					? (error.status as number)
					: undefined;
			throw { message, status } as AuthError;
		}
	}

	async getMe(): Promise<AuthUserResponse | null> {
		try {
			const response = await api.getMe();
			return response.user || null;
		} catch {
			return null;
		}
	}

	async logout(): Promise<void> {
		await api.logout();
	}

	async checkSetupRequired(): Promise<boolean> {
		const response = await api.checkSetupRequired();
		return response.required || false;
	}

	async createFirstUser(username: string, password: string): Promise<AuthUserResponse> {
		const response = await api.createFirstUser(username, password);
		return response.user!;
	}

	// Returns nothing on purpose: the endpoint is unauthenticated, so it
	// reports neither whether the username exists nor where on disk the code
	// was written. Reading the code requires access to the server.
	async requestPasswordReset(username: string): Promise<void> {
		await api.requestPasswordReset(username);
	}

	async confirmPasswordReset(code: string, newPassword: string): Promise<AuthUserResponse> {
		const response = await api.confirmPasswordReset(code, newPassword);
		return response.user!;
	}

	private async extractErrorMessage(error: unknown): Promise<string> {
		// If error has a response, try to get text
		if (error && typeof error === 'object' && 'response' in error) {
			try {
				const response = error.response as Response;
				const text = await response.text();
				return text || 'An error occurred';
			} catch {
				return 'An error occurred';
			}
		}
		// Fallback to error message
		if (error instanceof Error) {
			return error.message;
		}
		return 'An error occurred';
	}
}
