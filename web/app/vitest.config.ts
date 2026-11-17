import { defineConfig } from 'vitest/config';

export default defineConfig({
	define: { __TGD_BUILD__: JSON.stringify('test') },
	test: { include: ['src/**/*.test.{ts,tsx}', 'server/**/*.test.ts', 'standin/**/*.test.ts', 'scripts/**/*.test.ts'] },
});
