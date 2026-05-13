import { browser } from '$app/environment';

export type Theme = 'light' | 'dark' | 'system';
export type EffectiveTheme = 'light' | 'dark';

/**
 * localStorage key used by both the runtime store and the FOUC-blocking
 * inline boot script in `app.html`. Keep them in sync.
 */
export const THEME_STORAGE_KEY = 'audiod-theme';

/**
 * Pure resolver: maps the persisted preference + current OS preference
 * down to the concrete theme that should be applied to the document.
 * Exported so the boot script can reuse the exact same logic via a tiny
 * mirror in `app.html`.
 */
export function resolveTheme(stored: Theme | null, prefersDark: boolean): EffectiveTheme {
	if (stored === 'light') return 'light';
	if (stored === 'dark') return 'dark';
	// 'system' or null → follow OS.
	return prefersDark ? 'dark' : 'light';
}

function applyEffective(effective: EffectiveTheme) {
	if (!browser) return;
	if (effective === 'dark') {
		document.documentElement.classList.add('dark');
	} else {
		document.documentElement.classList.remove('dark');
	}
}

function createThemeStore() {
	let theme = $state<Theme>('system');
	let systemMql: MediaQueryList | null = null;

	// Reads from localStorage to avoid capturing the $state variable in a
	// long-lived closure (which Svelte 5 warns about). localStorage is the
	// persistence source of truth: setTheme always writes through it before
	// any change to the reactive `theme` value becomes observable, so the
	// two stay in sync for the purpose of this listener.
	const handleSystemChange = (e: MediaQueryListEvent) => {
		const stored = localStorage.getItem(THEME_STORAGE_KEY) as Theme | null;
		if (stored === null || stored === 'system') {
			applyEffective(resolveTheme('system', e.matches));
		}
	};

	if (browser) {
		const stored = localStorage.getItem(THEME_STORAGE_KEY) as Theme | null;
		const initial: Theme = stored ?? 'system';
		theme = initial;

		systemMql = window.matchMedia('(prefers-color-scheme: dark)');
		// Use the local `initial` rather than the $state `theme` so the read
		// is plainly capture-time (avoids Svelte's state_referenced_locally
		// warning for module-init code).
		applyEffective(resolveTheme(initial, systemMql.matches));
		systemMql.addEventListener('change', handleSystemChange);
	}

	function setTheme(newTheme: Theme) {
		theme = newTheme;
		if (browser) {
			localStorage.setItem(THEME_STORAGE_KEY, newTheme);
			const prefersDark = systemMql?.matches ?? false;
			applyEffective(resolveTheme(newTheme, prefersDark));
		}
	}

	function toggle() {
		// Read the persisted value rather than the $state variable so the
		// closure stays warning-free (state_referenced_locally) — both are
		// kept in sync by setTheme.
		const stored = browser
			? ((localStorage.getItem(THEME_STORAGE_KEY) as Theme | null) ?? 'system')
			: 'system';
		const prefersDark = systemMql?.matches ?? false;
		const effective = resolveTheme(stored, prefersDark);
		setTheme(effective === 'dark' ? 'light' : 'dark');
	}

	return {
		get current() {
			return theme;
		},
		setTheme,
		toggle
	};
}

export const themeStore = createThemeStore();
