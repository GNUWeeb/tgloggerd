// The browser suite: real Chromium against the development stand-in.
// Every worker starts its own stand-in on a free port (e2e/fixtures.ts),
// and every test resets it first, so tests run in parallel without
// sharing state. Run it with ./dev e2e.
import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: 'e2e',
	fullyParallel: true,
	workers: process.env.E2E_WORKERS ? Number(process.env.E2E_WORKERS) : 4,
	timeout: 60_000,
	expect: { timeout: 8_000 },
	forbidOnly: !!process.env.CI,
	reporter: [['list']],
	outputDir: 'node_modules/.e2e-results',
	use: { browserName: 'chromium', viewport: { width: 1440, height: 900 }, trace: 'retain-on-failure' },
});
