// Runs the render service process (main.ts) as the container does, on a
// socket of the test's own.
import { afterAll, describe, expect, it } from 'vitest';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = fileURLToPath(new URL('.', import.meta.url));
const dir = mkdtempSync(join(tmpdir(), 'tgd-main-'));
afterAll(() => rmSync(dir, { recursive: true, force: true }));

describe('render service process', () => {
	it('exits 1 without a built entry instead of forking workers', () => {
		const r = spawnSync(process.execPath, [join(here, 'main.ts')], {
			env: { ...process.env, RENDER_SOCKET: join(dir, 'none.sock'), RENDER_ENTRY: join(dir, 'missing.js') },
			encoding: 'utf8',
			timeout: 10000,
		});
		expect(r.status).toBe(1);
		expect(r.stdout).toMatch(/no render entry/);
	});
});
