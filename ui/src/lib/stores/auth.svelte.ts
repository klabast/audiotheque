/**
 * Auth store - Svelte 5 runes-based authentication state management
 *
 * Usage in components:
 *   import { auth } from '$lib/stores/auth.svelte';
 *
 *   // Read state (reactive via $state/$derived)
 *   auth.user
 *   auth.loading
 *   auth.error
 *
 *   // Actions
 *   await auth.login(username, password);
 *   await auth.logout();
 */

import { type AuthError, AuthService } from '$lib/services/authService';
import type { AuthUserResponse } from '$lib/api/client';

// Service instance (private to this module)
const authService = new AuthService();

export const auth = (() => {
	let user = $state<AuthUserResponse | null>(null);
	let loading = $state(false);
	let error = $state<AuthError | null>(null);

	return {
		get user() {
			return user;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},

		/**
		 * Login action
		 *
		 * Sets loading state, calls service, updates store with result or error.
		 */
		async login(username: string, password: string): Promise<void> {
			loading = true;
			error = null;

			try {
				user = await authService.login(username, password);
				loading = false;
			} catch (e) {
				user = null;
				loading = false;
				error = e as AuthError;
				throw e; // Re-throw so caller can handle (e.g., stay on page)
			}
		},

		/**
		 * Logout action
		 *
		 * Clears local state and server cookie.
		 */
		async logout(): Promise<void> {
			await authService.logout();
			user = null;
			loading = false;
			error = null;
		},

		/**
		 * Clear error
		 *
		 * Useful for dismissing error messages.
		 */
		clearError(): void {
			error = null;
		},

		/**
		 * Initialize session from httpOnly cookie
		 *
		 * Call this on app load to restore user session.
		 * If valid cookie exists, populates store with user.
		 */
		async initializeSession(): Promise<void> {
			loading = true;

			try {
				user = await authService.getMe();
				loading = false;
			} catch {
				// No valid session - stay logged out
				user = null;
				loading = false;
				error = null;
			}
		}
	};
})();
