import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// The client bundle (hashed, with a manifest the Go service reads) goes
// to dist/client; `vite build --ssr` puts the render entry in
// dist/server. Both come from one commit, so their markup matches.
export default defineConfig({
	plugins: [react()],
	base: '/app/',
	build: {
		outDir: 'dist/client',
		manifest: true,
		rollupOptions: { input: 'src/entry-client.tsx' },
	},
	define: {
		__TGD_BUILD__: JSON.stringify(process.env.TGD_BUILD || 'dev'),
	},
	server: { allowedHosts: ['tgd-app-dev'] },
});
