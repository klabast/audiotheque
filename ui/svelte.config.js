import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

const config = {
	preprocess: vitePreprocess(),
	kit: {
		adapter: adapter({
			fallback: 'index.html' // This makes it a SPA
		}),
		prerender: {
			handleHttpError: 'ignore',
			handleMissingId: 'ignore'
		}
	}
};

export default config;
