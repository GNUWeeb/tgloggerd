import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
	plugins: [react()],
	// The dev server container is reached as tgd-mockup-dev by the
	// screenshot container (see ./dev).
	server: { host: '127.0.0.1', strictPort: false, allowedHosts: ['tgd-mockup-dev'] },
});
