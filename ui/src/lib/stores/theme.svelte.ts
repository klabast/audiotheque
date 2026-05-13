import { browser } from '$app/environment';

type Theme = 'light' | 'dark' | 'system';

function createThemeStore() {
	let theme = $state<Theme>('system'); // Default to system

	// Initialize theme from localStorage or system preference
	if (browser) {
		const stored = localStorage.getItem('audiod-theme') as Theme | null;
		if (stored) {
			theme = stored;
			applyTheme(stored);
		} else {
			theme = 'system';
			applyTheme('system');
		}
	}

	function applyTheme(newTheme: Theme) {
		if (!browser) return;

		if (newTheme === 'system') {
			// Follow system preference
			const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
			if (prefersDark) {
				document.documentElement.classList.add('dark');
			} else {
				document.documentElement.classList.remove('dark');
			}
		} else if (newTheme === 'dark') {
			document.documentElement.classList.add('dark');
		} else {
			document.documentElement.classList.remove('dark');
		}
	}

	function setTheme(newTheme: Theme) {
		theme = newTheme;
		if (browser) {
			localStorage.setItem('audiod-theme', newTheme);
			applyTheme(newTheme);
		}
	}

	function toggle() {
		setTheme(theme === 'dark' ? 'light' : 'dark');
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
