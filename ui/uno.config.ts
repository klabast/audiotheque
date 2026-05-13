import {
	defineConfig,
	presetUno,
	presetIcons,
	transformerVariantGroup,
	transformerDirectives
} from 'unocss';

export default defineConfig({
	// Enable variant groups: hover:(bg-black text-white)
	transformers: [transformerVariantGroup(), transformerDirectives()],

	presets: [
		presetUno(),
		presetIcons({
			scale: 1.2,
			extraProperties: {
				display: 'inline-block',
				'vertical-align': 'middle'
			}
		})
	],

	// Theme extends UnoCSS defaults with custom colors
	theme: {
		colors: {
			// These map to CSS variables defined in app.css (RGB channels for opacity support)
			primary: {
				DEFAULT: 'rgb(var(--color-primary) / <alpha-value>)',
				muted: 'rgb(var(--color-primary-muted) / <alpha-value>)'
			},
			accent: {
				DEFAULT: 'rgb(var(--color-accent) / <alpha-value>)',
				muted: 'rgb(var(--color-accent-muted) / <alpha-value>)'
			},
			action: {
				DEFAULT: 'rgb(var(--color-action) / <alpha-value>)',
				muted: 'rgb(var(--color-action-muted) / <alpha-value>)'
			},
			background: 'rgb(var(--color-background) / <alpha-value>)',
			surface: {
				DEFAULT: 'rgb(var(--color-surface) / <alpha-value>)',
				alt: 'rgb(var(--color-surface-alt) / <alpha-value>)',
				hover: 'rgb(var(--color-surface-hover) / <alpha-value>)'
			},
			success: 'rgb(var(--color-success) / <alpha-value>)',
			warning: 'rgb(var(--color-warning) / <alpha-value>)',
			error: 'rgb(var(--color-error) / <alpha-value>)',
			info: 'rgb(var(--color-info) / <alpha-value>)',
			text: {
				primary: 'rgb(var(--color-text-primary) / <alpha-value>)',
				secondary: 'rgb(var(--color-text-secondary) / <alpha-value>)',
				muted: 'rgb(var(--color-text-muted) / <alpha-value>)'
			},
			border: {
				DEFAULT: 'rgb(var(--color-border) / <alpha-value>)',
				hover: 'rgb(var(--color-border-hover) / <alpha-value>)'
			}
		}
	},

	// Design system shortcuts - reusable patterns
	shortcuts: {
		// Action button (play button in player footer)
		'btn-action': 'bg-action text-white hover:bg-action-muted active:opacity-80 transition-colors',

		// Card patterns
		card: 'bg-surface rounded-lg p-8 shadow-lg',
		'card-bordered': 'bg-surface rounded-lg border border-border p-8',

		// Form patterns
		'form-input':
			'w-full rounded-lg border border-border bg-surface px-4 py-3 text-text-primary placeholder:text-text-muted focus:(border-primary outline-none ring-2 ring-primary/20)',
		'form-select':
			'w-full rounded-lg border border-border bg-surface-alt px-4 py-3 text-text-primary focus:(border-primary outline-none ring-2 ring-primary/20)',
		'form-label': 'block text-sm font-medium text-text-secondary mb-2',

		// Layout patterns
		stack: 'flex flex-col',
		'stack-sm': 'stack gap-2',
		'stack-md': 'stack gap-4',
		'stack-lg': 'stack gap-6',
		row: 'flex flex-row items-center',
		'row-between': 'row justify-between',

		// Player icon buttons (desktop - with hover)
		'player-btn':
			'w-8 h-8 flex items-center justify-center rounded-full text-text-secondary hover:text-text-primary hover:bg-surface-hover transition-colors',
		'player-btn-muted':
			'w-8 h-8 flex items-center justify-center rounded-full text-text-muted hover:text-text-primary hover:bg-surface-hover transition-colors',
		// Player icon buttons (mobile - with active, no hover)
		'player-btn-mobile':
			'w-11 h-11 flex items-center justify-center rounded-full text-text-primary active:bg-surface-hover transition-colors',
		'player-btn-mobile-lg':
			'w-14 h-14 flex items-center justify-center rounded-full text-text-secondary active:bg-surface-hover transition-colors',
		'player-btn-mobile-play':
			'w-18 h-18 flex items-center justify-center rounded-full bg-action text-white active:opacity-80 transition-opacity',

		// Album covers
		'player-album-lg':
			'w-16 h-16 rounded-lg overflow-hidden flex-shrink-0 bg-surface-alt shadow-md',
		'player-album-sm': 'w-11 h-11 rounded-lg overflow-hidden flex-shrink-0 bg-surface shadow-md',
		'player-album-md': 'w-12 h-12 rounded-lg overflow-hidden flex-shrink-0 bg-surface shadow-md',

		// Home indicator (iOS)
		'home-indicator': 'w-[134px] h-[5px] bg-text-primary/20 rounded-full',

		// Album grid
		'album-grid': 'grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6',

		// Track list
		'track-item': 'flex items-center gap-4 px-4 py-3 hover:bg-surface-hover transition-colors',

		// Badges
		badge:
			'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium text-text-secondary',
		'badge-primary': 'badge bg-primary/20 text-primary',

		// Dividers
		divider: 'w-px h-5 bg-border flex-shrink-0',
		'divider-horizontal': 'h-px w-full bg-border'
	}
});
