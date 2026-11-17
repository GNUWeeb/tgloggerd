// The container's health check: exits 0 when the render service on
// RENDER_SOCKET answers GET /health with ok, 1 otherwise.
import { request } from 'node:http';

const socketPath = process.env.RENDER_SOCKET || '/run/tgloggerd-render/render.sock';

const req = request({ socketPath, path: '/health', timeout: 2000 }, (res) => {
	let s = '';
	res.on('data', (c) => (s += c));
	res.on('end', () => {
		let ok = false;
		try {
			ok = res.statusCode === 200 && JSON.parse(s).ok === true;
		} catch {}
		if (!ok) console.error(`unhealthy: ${res.statusCode} ${s}`);
		process.exit(ok ? 0 : 1);
	});
});
req.on('timeout', () => req.destroy(new Error('timed out')));
req.on('error', (e) => {
	console.error(`unhealthy: ${e.message}`);
	process.exit(1);
});
req.end();
