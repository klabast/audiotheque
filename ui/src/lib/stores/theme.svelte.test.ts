import { beforeEach, describe, expect, it, vi } from 'vitest';

beforeEach(() => {
	vi.resetModules();
	vi.clearAllMocks();
	localStorage.clear();
	document.documentElement.classList.remove('dark');
});

// resolveTheme is the pure function shared by the runtime store AND the
// FOUC-blocking inline boot script. Centralising it guarantees both paths
// agree on what 'dark' means.
describe('resolveTheme', () => {
	it('returns "light" when stored value is "light"', async () => {
		const { resolveTheme } = await import('./theme.svelte');
		expect(resolveTheme('light', true)).toBe('light');
		expect(resolveTheme('light', false)).toBe('light');
	});

	it('returns "dark" when stored value is "dark"', async () => {
		const { resolveTheme } = await import('./theme.svelte');
		expect(resolveTheme('dark', true)).toBe('dark');
		expect(resolveTheme('dark', false)).toBe('dark');
	});

	it('follows the system preference when stored value is "system"', async () => {
		const { resolveTheme } = await import('./theme.svelte');
		expect(resolveTheme('system', true)).toBe('dark');
		expect(resolveTheme('system', false)).toBe('light');
	});

	it('falls back to system preference when no value is stored', async () => {
		const { resolveTheme } = await import('./theme.svelte');
		expect(resolveTheme(null, true)).toBe('dark');
		expect(resolveTheme(null, false)).toBe('light');
	});
});

describe('themeStore localStorage key', () => {
	it('writes to the post-rebrand "audiod-theme" key', async () => {
		const { themeStore } = await import('./theme.svelte');
		themeStore.setTheme('dark');
		expect(localStorage.getItem('audiod-theme')).toBe('dark');
		// And the pre-rebrand key must not leak back in.
		expect(localStorage.getItem('flux-theme')).toBeNull();
	});

	it('reads from "audiod-theme" on initialisation', async () => {
		localStorage.setItem('audiod-theme', 'dark');
		const { themeStore } = await import('./theme.svelte');
		expect(themeStore.current).toBe('dark');
		expect(document.documentElement.classList.contains('dark')).toBe(true);
	});
});

describe('themeStore tracks system preference when stored value is "system"', () => {
	it('flips the dark class when prefers-color-scheme:dark fires', async () => {
		// Start with a "system" theme and a media query we can drive.
		const listeners = new Set<(e: MediaQueryListEvent) => void>();
		let matches = false;
		const mql = {
			get matches() {
				return matches;
			},
			media: '(prefers-color-scheme: dark)',
			onchange: null,
			addEventListener: vi.fn((_evt: string, cb: (e: MediaQueryListEvent) => void) => {
				listeners.add(cb);
			}),
			removeEventListener: vi.fn((_evt: string, cb: (e: MediaQueryListEvent) => void) => {
				listeners.delete(cb);
			}),
			dispatchEvent: vi.fn()
		};
		window.matchMedia = vi.fn().mockReturnValue(mql) as unknown as typeof window.matchMedia;

		localStorage.setItem('audiod-theme', 'system');
		await import('./theme.svelte');

		// System starts in light mode — no .dark class.
		expect(document.documentElement.classList.contains('dark')).toBe(false);

		// Simulate the OS flipping to dark mode.
		matches = true;
		listeners.forEach((cb) => cb({ matches: true } as MediaQueryListEvent));
		expect(document.documentElement.classList.contains('dark')).toBe(true);

		// And back to light.
		matches = false;
		listeners.forEach((cb) => cb({ matches: false } as MediaQueryListEvent));
		expect(document.documentElement.classList.contains('dark')).toBe(false);
	});
});
