/**
 * Root layout load function
 *
 * Runs on every navigation, including initial page load.
 * Initializes the session and checks setup status.
 */

import { auth } from '$lib/stores/auth.svelte';
import { api } from '$lib/services/api';

export const ssr = false;

export async function load() {
	await auth.initializeSession();

	let requiresAdminUser: boolean;
	try {
		const response = await api.getSystemStatus();
		requiresAdminUser = response.requiresAdminUser || false;
	} catch {
		requiresAdminUser = false;
	}

	return {
		setupRequired: requiresAdminUser
	};
}
