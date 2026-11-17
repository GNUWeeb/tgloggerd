// Runs the render service process (main.ts) and its health check
// (health.ts) as the container does, with a stand-in render entry on a
// socket of the test's own.
import { afterAll, describe, expect, it } from 'vitest';
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = fileURLToPath(new URL('.', import.meta.url));
const dir = mkdtempSync(join(tmpdir(), 'tgd-main-'));
const entry = join(dir, 'entry-server.js');
writeFileSync(entry, `export const BUILD = 'b7';
export function render(page) { return { head: '<title>t</title>', html: '<p>' + page.route + '</p>' }; }
`);
afterAll(() => rmSync(dir, { recursive: true, force: true }));

function health(socket: string) {
	return spawnSync(process.execPath, [join(here, 'health.ts')], { env: { ...process.env, RENDER_SOCKET: socket }, encoding: 'utf8', timeout: 10000 });
}

// Starts main.ts and resolves once it logged that it listens.
function start(socket: string, workers: number): Promise<ChildProcess> {
	const p = spawn(process.execPath, [join(here, 'main.ts')], {
		env: { ...process.env, RENDER_SOCKET: socket, RENDER_ENTRY: entry, RENDER_WORKERS: String(workers) },
		stdio: ['ignore', 'pipe', 'inherit'],
	});
	return new Promise((resolve, reject) => {
		let out = '';
		const timer = setTimeout(() => reject(new Error(`not listening: ${out}`)), 10000);
		p.stdout!.on('data', (c) => {
			out += c;
			if (out.includes('"msg":"listening"')) {
				clearTimeout(timer);
				resolve(p);
			}
		});
		p.on('exit', (code) => reject(new Error(`exited ${code}: ${out}`)));
	});
}

function stop(p: ChildProcess): Promise<number | null> {
	return new Promise((resolve) => {
		p.on('exit', (code) => resolve(code));
		p.kill('SIGTERM');
	});
}

describe('render service process', () => {
	for (const workers of [1, 2]) {
		it(`serves health with ${workers} worker(s), and the check fails once it stopped`, async () => {
			const socket = join(dir, `w${workers}.sock`);
			// A socket a crashed run left behind must not stop the start.
			writeFileSync(socket, '');
			const p = await start(socket, workers);
			const ok = health(socket);
			expect(ok.status, ok.stderr).toBe(0);
			expect(await stop(p)).toBe(0);
			const down = health(socket);
			expect(down.status).toBe(1);
			expect(down.stderr).toMatch(/unhealthy/);
		});
	}

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
