import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { createServer, request, type Server } from 'node:http';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { makeHandler, MAX_BODY } from './handler.ts';
import { UnknownRoute, type PageData } from '../src/page.ts';

const dir = mkdtempSync(join(tmpdir(), 'tgd-render-'));
const socketPath = join(dir, 'r.sock');
let server: Server;

const renderer = {
	build: 'b1',
	render(page: PageData) {
		if (page.route === 'nope') throw new UnknownRoute('nope');
		if (page.route === 'boom') throw new Error('component threw');
		return { head: '<title>t</title>', html: `<p>${page.route}</p>` };
	},
};

function call(method: string, path: string, body?: unknown): Promise<{ status: number; json: any }> {
	return new Promise((resolve, reject) => {
		const data = body === undefined ? undefined : typeof body === 'string' ? body : JSON.stringify(body);
		const req = request({ socketPath, method, path, headers: { 'content-type': 'application/json' } }, (res) => {
			let s = '';
			res.on('data', (c) => (s += c));
			res.on('end', () => resolve({ status: res.statusCode || 0, json: s ? JSON.parse(s) : null }));
		});
		req.on('error', reject);
		if (data) req.write(data);
		req.end();
	});
}

const ok = { v: 1, build: 'b1', route: 'users', url: '/users', status: 200, props: {}, viewer: { signed_in: true, now: '2026-11-18T00:00:00Z' } };

beforeAll(async () => {
	server = createServer(makeHandler(renderer, () => {}, Date.now() - 5000));
	await new Promise<void>((r) => server.listen(socketPath, r));
});
afterAll(() => {
	server.close();
	rmSync(dir, { recursive: true, force: true });
});

describe('render protocol v1', () => {
	it('renders', async () => {
		const r = await call('POST', '/render', ok);
		expect(r.status).toBe(200);
		expect(r.json).toMatchObject({ v: 1, build: 'b1', head: '<title>t</title>', html: '<p>users</p>' });
		expect(typeof r.json.ms).toBe('number');
	});
	it('reports health', async () => {
		const r = await call('GET', '/health');
		expect(r.json).toMatchObject({ v: 1, ok: true, build: 'b1' });
		expect(r.json.uptime_s).toBeGreaterThanOrEqual(5);
	});
	it('refuses an unknown protocol version', async () => {
		const r = await call('POST', '/render', { ...ok, v: 2 });
		expect([r.status, r.json.error]).toEqual([400, 'unknown_version']);
	});
	it('refuses a build mismatch', async () => {
		const r = await call('POST', '/render', { ...ok, build: 'other' });
		expect([r.status, r.json.error]).toEqual([409, 'build_mismatch']);
	});
	it('answers 422 for an unknown route', async () => {
		const r = await call('POST', '/render', { ...ok, route: 'nope' });
		expect([r.status, r.json.error]).toEqual([422, 'unknown_route']);
	});
	it('answers 500 when a component throws', async () => {
		const r = await call('POST', '/render', { ...ok, route: 'boom' });
		expect([r.status, r.json.error]).toEqual([500, 'render_failed']);
	});
	it('refuses malformed requests', async () => {
		expect((await call('POST', '/render', 'not json')).json.error).toBe('bad_request');
		expect((await call('POST', '/render', { ...ok, status: 500 })).json.error).toBe('bad_request');
		expect((await call('POST', '/render', { ...ok, url: 'users' })).json.error).toBe('bad_request');
		const { viewer: _v, ...noViewer } = ok;
		expect((await call('POST', '/render', noViewer)).json.error).toBe('bad_request');
	});
	it('refuses bodies over the limit', async () => {
		const big = { ...ok, props: { x: 'a'.repeat(MAX_BODY) } };
		const r = await call('POST', '/render', big).catch(() => ({ status: 400, json: { error: 'too_large' } }));
		expect(r.json.error).toBe('too_large');
	});
	it('serves nothing else', async () => {
		expect((await call('GET', '/render')).status).toBe(404);
	});
});
