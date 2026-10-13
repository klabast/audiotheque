import { goto } from '$app/navigation';
import { resolve } from '$app/paths';

/**
 * Centralized navigation utilities
 * Provides type-safe navigation functions and pre-resolved routes
 */

// Navigation functions - use these for programmatic navigation
export const navigation = {
	goToLogin: () => goto(resolve('/(public)/login')),
	goToHome: () => goto(resolve('/(authenticated)')),
	goToSettings: () => goto(resolve('/(authenticated)/settings')),
	goToSettingsAccount: () =>
		goto(resolve('/(authenticated)/settings/[category]', { category: 'account' })),
	goToSettingsGeneral: () =>
		goto(resolve('/(authenticated)/settings/[category]', { category: 'general' }))
};

// Pre-resolved routes - use these for href attributes
export const routes = {
	login: resolve('/(public)/login'),
	home: resolve('/(authenticated)'),
	settings: resolve('/(authenticated)/settings'),
	stats: resolve('/(authenticated)/stats'),
	settingsAccount: resolve('/(authenticated)/settings/[category]', { category: 'account' }),
	settingsGeneral: resolve('/(authenticated)/settings/[category]', { category: 'general' })
};
