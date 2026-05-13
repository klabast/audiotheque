import { browser } from '$app/environment';

export function getMainScrollContainer(): HTMLElement | null {
	if (!browser) return null;
	return document.querySelector('main');
}
