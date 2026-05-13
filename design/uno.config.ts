import { defineConfig, mergeConfigs } from 'unocss';
import baseConfig from '../ui/uno.config';

// Get base shortcuts to spread into design shortcuts
const baseShortcuts = baseConfig.shortcuts || {};

export default mergeConfigs([
	baseConfig,
	defineConfig({
		// Design workbench specific shortcuts (include base shortcuts)
		shortcuts: {
			...baseShortcuts,

			// Device frame styles
			'device-frame': 'bg-gray-900 rounded-2xl shadow-2xl overflow-hidden',
			'device-screen': 'bg-background overflow-auto',
			'device-notch': 'absolute top-0 left-1/2 -translate-x-1/2 bg-black rounded-b-xl',
			'device-home': 'absolute bottom-1 left-1/2 -translate-x-1/2 bg-gray-700 rounded-full',

			// Showcase layout
			'showcase-section': 'mb-12',
			'showcase-title': 'text-2xl font-bold mb-6',
			'showcase-grid': 'grid gap-8',

			// Component preview
			'preview-box': 'border border-border rounded-lg overflow-hidden',
			'preview-label': 'text-xs text-text-muted font-mono mb-2',
		},
	}),
]);
