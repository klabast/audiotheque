import { defineConfig } from 'vite';
import UnoCSS from 'unocss/vite';
import { resolve } from 'path';

export default defineConfig({
	plugins: [UnoCSS()],
	server: {
		port: 3001,
		open: true,
	},
	build: {
		outDir: 'dist',
		rollupOptions: {
			input: {
				main: resolve(__dirname, 'index.html'),
				// Components
				playerBar: resolve(__dirname, 'components/player-bar.html'),
				albumGrid: resolve(__dirname, 'components/album-grid.html'),
				albumDetail: resolve(__dirname, 'components/album-detail.html'),
				sidebar: resolve(__dirname, 'components/sidebar.html'),
				forms: resolve(__dirname, 'components/forms.html'),
				// Building Blocks
				buttons: resolve(__dirname, 'building-blocks/buttons.html'),
				colors: resolve(__dirname, 'building-blocks/colors.html'),
				typography: resolve(__dirname, 'building-blocks/typography.html'),
				alerts: resolve(__dirname, 'building-blocks/alerts.html'),
			},
		},
	},
});
