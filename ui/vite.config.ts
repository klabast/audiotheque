import { paraglideVitePlugin } from '@inlang/paraglide-js';
import UnoCSS from 'unocss/vite';
import { svelteTesting } from '@testing-library/svelte/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig(({ mode }) => {
	const isDev = mode === 'development';
	const apiPort = process.env.AUDIOD_PORT ?? '8880';

	return {
		plugins: [
			UnoCSS(),
			sveltekit(),
			paraglideVitePlugin({
				project: './project.inlang',
				outdir: './src/lib/paraglide'
			})
		],
		server: isDev
			? {
					proxy: {
						'/api': {
							target: `http://localhost:${apiPort}`,
							changeOrigin: true,
							ws: true // Enable WebSocket proxying
						}
					}
				}
			: {},
		test: {
			projects: [
				{
					extends: './vite.config.ts',
					plugins: [svelteTesting()],
					test: {
						name: 'client',
						environment: 'jsdom',
						clearMocks: true,
						include: ['src/**/*.svelte.{test,spec}.{js,ts}'],
						exclude: ['src/lib/server/**'],
						setupFiles: ['./vitest-setup-client.ts']
					}
				},
				{
					extends: './vite.config.ts',
					test: {
						name: 'server',
						environment: 'node',
						include: ['src/**/*.{test,spec}.{js,ts}'],
						exclude: ['src/**/*.svelte.{test,spec}.{js,ts}']
					}
				}
			]
		}
	};
});
