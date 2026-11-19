import { defineConfig } from 'vitest/config';

export default defineConfig({
	define: { __TGD_BUILD__: JSON.stringify('test') },
	test: {
		include: ['src/**/*.test.{ts,tsx}', 'server/**/*.test.ts', 'standin/**/*.test.ts', 'scripts/**/*.test.ts'],
		// Page tests opt into a DOM with `// @vitest-environment happy-dom`.
		environmentOptions: { happyDOM: { url: 'http://localhost/' } },
		// Every file runs in its own worker with its own modules, so the
		// fixtures' in-memory state is never shared between files.
		isolate: true,
		// No test may reach the network, not even after it ends.
		setupFiles: ['src/test/no-network.ts'],
	},
});
