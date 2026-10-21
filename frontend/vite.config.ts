import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({
				pages: '../backend/dist',
				assets: '../backend/dist',
				fallback: 'index.html',
				precompress: false,
				strict: false
			})
		})
	],
	server: {
		port: 5173,
		proxy: {
			'/api': {
				target: process.env.VITE_BACKEND_URL || 'http://localhost:8000',
				changeOrigin: true,
				ws: true
			}
		}
	},
	build: {
		rollupOptions: {
			output: {
				// Heavy editors/charts live in dedicated chunks so route chunks
				// (and the initial shell) stay small; the runtime chunk files are
				// only fetched by routes that actually use them.
				manualChunks(id) {
					if (!id.includes('node_modules')) return undefined;
					if (id.includes('echarts')) return 'vendor-echarts';
					if (id.includes('@xterm')) return 'vendor-terminal';
					if (id.includes('codemirror') || id.includes('@codemirror')) return 'vendor-editor';
					if (id.includes('@xyflow')) return 'vendor-topology';
					return undefined;
				}
			}
		}
	}
});
