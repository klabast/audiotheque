import { browser } from '$app/env';

export function getMainScrollContainer(): HTMLElement | null {
	if (!browser) return null;
	return document.querySelector('main');
}
