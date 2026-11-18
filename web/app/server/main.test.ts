// Runs the render service process (main.ts) and its health check
// (health.ts) as the container does, with a stand-in render entry on a
// socket of the test's own.
import { afterAll, describe, expect, it } from 'vitest';
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { request } from 'node:http';
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

	it('gives up with exit 1 when workers keep failing to load the entry', () => {
		const broken = join(dir, 'broken.js');
		writeFileSync(broken, "throw new Error('a broken build');\n");
		const r = spawnSync(process.execPath, [join(here, 'main.ts')], {
			env: { ...process.env, RENDER_SOCKET: join(dir, 'broken.sock'), RENDER_ENTRY: broken, RENDER_WORKERS: '2' },
			encoding: 'utf8',
			timeout: 20000,
		});
		expect(r.signal).toBeNull(); // it stopped by itself, not at the timeout
		expect(r.status).toBe(1);
		expect(r.stdout).toMatch(/workers keep exiting; giving up/);
	});

	it('lets a worker finish a render in progress when stopped', async () => {
		const slow = join(dir, 'slow.js');
		writeFileSync(slow, `export const BUILD = 'b7';
export function render(page) { const t = Date.now() + 700; while (Date.now() < t); return { head: '', html: '<p>' + page.route + '</p>' }; }
`);
		const socket = join(dir, 'slow.sock');
		const p = spawn(process.execPath, [join(here, 'main.ts')], { env: { ...process.env, RENDER_SOCKET: socket, RENDER_ENTRY: slow, RENDER_WORKERS: '2' }, stdio: ['ignore', 'pipe', 'inherit'] });
		await new Promise<void>((resolve) => { let out = ''; p.stdout!.on('data', (c) => { out += c; if (out.includes('"msg":"listening"')) resolve(); }); });
		const body = JSON.stringify({ v: 1, build: 'b7', route: 'dashboard', url: '/', status: 200, props: {}, viewer: { signed_in: false, now: '2026-11-18T00:00:00Z' } });
		const answer = new Promise<number>((resolve, reject) => {
			const req = request({ socketPath: socket, path: '/render', method: 'POST', headers: { 'content-type': 'application/json' } }, (res) => { res.resume(); res.on('end', () => resolve(res.statusCode ?? 0)); });
			req.on('error', reject);
			req.end(body);
		});
		await new Promise((r) => setTimeout(r, 200)); // the render is under way
		const exited = stop(p);
		expect(await answer).toBe(200);
		expect(await exited).toBe(0);
	});

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
