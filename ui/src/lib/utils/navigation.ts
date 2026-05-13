import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

/**
 * Centralized navigation utilities
 * Provides type-safe navigation functions and pre-resolved routes
 */

// Navigation functions - use these for programmatic navigation
export const navigation = {
	goToLogin: () => goto(resolve('/login')),
	goToHome: () => goto(resolve('/')),
	goToSettings: () => goto(resolve('/settings')),
	goToSettingsAccount: () => goto(resolve('/settings/account')),
	goToSettingsGeneral: () => goto(resolve('/settings/general'))
};

// Pre-resolved routes - use these for href attributes
export const routes = {
	login: resolve('/login'),
	home: resolve('/'),
	settings: resolve('/settings'),
	settingsAccount: resolve('/settings/account'),
	settingsGeneral: resolve('/settings/general')
};
