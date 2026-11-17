// Smoke test of a running render service: health, then one render.
// Usage: RENDER_SOCKET=... node server/smoke.ts [build]
import { request } from 'node:http';

const socketPath = process.env.RENDER_SOCKET || '/run/tgloggerd-render/render.sock';
const build = process.argv[2] || 'dev';

function call(method: string, path: string, body?: unknown): Promise<string> {
	return new Promise((resolve, reject) => {
		const req = request({ socketPath, method, path, headers: { 'content-type': 'application/json' } }, (res) => {
			let s = '';
			res.on('data', (c) => (s += c));
			res.on('end', () => resolve(`${res.statusCode} ${s}`));
		});
		req.on('error', reject);
		if (body) req.write(JSON.stringify(body));
		req.end();
	});
}

console.log(await call('GET', '/health'));
console.log(await call('POST', '/render', {
	v: 1, build, route: 'not_found', url: '/nope', status: 404, props: {},
	viewer: { signed_in: false, now: new Date().toISOString() },
}));
