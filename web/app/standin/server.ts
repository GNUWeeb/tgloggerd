// Development stand-in for the Go service's page side: routes a path to
// fixture page data, renders it with the app's render entry and serves
// the document Go would assemble, plus /api/page/<path> for browser
// navigation. Vite serves the client modules with hot reload. Never
// deployed; the real server is webapi/.
//
//   ?as=superadmin|admin|member|public   view as a sample account, or signed out
//                                 (remembered in a cookie)
//   ?ssr=0                        skip server rendering, as when the render service is down
import { createServer as createHttp, type IncomingMessage, type ServerResponse } from 'node:http';
import { createServer as createVite } from 'vite';
import { documentHtml } from './document.ts';

const PORT = Number(process.env.PORT) || 5173;
const vite = await createVite({
	server: { middlewareMode: true, fs: { allow: ['..'] } },
	appType: 'custom',
});

type Persona = 'superadmin' | 'admin' | 'member' | 'public';
const PERSONAS: Persona[] = ['superadmin', 'admin', 'member', 'public'];
function persona(req: IncomingMessage, url: URL): Persona {
	const q = url.searchParams.get('as') as Persona | null;
	if (q && PERSONAS.includes(q)) return q;
	const c = /(?:^|; )tgd_as=(\w+)/.exec(req.headers.cookie || '')?.[1] as Persona | undefined;
	return c && PERSONAS.includes(c) ? c : 'superadmin';
}

function cleanUrl(url: URL): string {
	url.searchParams.delete('as');
	url.searchParams.delete('ssr');
	const q = url.searchParams.toString();
	return url.pathname + (q ? '?' + q : '');
}

function send(res: ServerResponse, status: number, body: unknown, cookies: string[] = []): void {
	res.writeHead(status, { 'content-type': 'application/json', 'cache-control': 'no-store', ...(cookies.length ? { 'set-cookie': cookies } : {}) });
	res.end(JSON.stringify(body));
}

function readBody(req: IncomingMessage): Promise<string> {
	return new Promise((resolve, reject) => {
		let s = '';
		req.on('data', (c) => { s += c; if (s.length > 1 << 16) req.destroy(); });
		req.on('end', () => resolve(s));
		req.on('error', reject);
	});
}

async function handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
	const url = new URL(req.url || '/', 'http://x');
	if (url.pathname.startsWith('/app/') || url.pathname.startsWith('/@') || url.pathname.startsWith('/node_modules/')) {
		vite.middlewares(req, res);
		return;
	}
	if (url.pathname === '/favicon.ico') {
		res.writeHead(204);
		res.end();
		return;
	}
	const who = persona(req, url);
	const cookie = `tgd_as=${who}; Path=/; SameSite=Lax`;
	const fx = await vite.ssrLoadModule('/standin/fixtures.ts');
	// Sign-in, as webapi/README "Authentication" describes it, with
	// fixed tokens. Usernames admin, gw_admin and researcher sign in as
	// the sample superadmin, admin and member; the password is "standin".
	if (url.pathname === '/api/login' && req.method === 'GET') {
		send(res, 200, who === 'public' ? { signed_in: false, csrf: 'standin-login' } : { signed_in: true }, ['tgw_csrf=standin-login; Path=/api/login; HttpOnly; SameSite=Lax']);
		return;
	}
	if (url.pathname === '/api/login' && req.method === 'POST') {
		if (req.headers['x-csrf-token'] !== 'standin-login') { send(res, 403, { error: 'csrf' }); return; }
		const body = JSON.parse((await readBody(req)) || '{}') as { username?: string; password?: string };
		const as = ({ admin: 'superadmin', gw_admin: 'admin', researcher: 'member' } as Record<string, Persona>)[body.username ?? ''];
		if (!as || body.password !== 'standin') {
			send(res, 401, { error: 'invalid_credentials', message: 'Wrong username or password.', csrf: 'standin-login' });
			return;
		}
		send(res, 200, { user: { id: 1, username: body.username, role: as }, csrf: 'standin-csrf' }, [`tgd_as=${as}; Path=/; SameSite=Lax`]);
		return;
	}
	if (url.pathname === '/api/logout' && req.method === 'POST') {
		if (req.headers['x-csrf-token'] !== 'standin-csrf') { send(res, 403, { error: 'csrf' }); return; }
		res.writeHead(204, { 'set-cookie': 'tgd_as=public; Path=/; SameSite=Lax' });
		res.end();
		return;
	}
	if (url.pathname === '/api/session') {
		if (who === 'public') send(res, 401, { error: 'unauthenticated' });
		else send(res, 200, { user: fx.viewerOf(who).account, csrf: 'standin-csrf' });
		return;
	}
	let cm;
	if ((cm = url.pathname.match(/^\/api\/chats\/(group|private)\/(-?\d+)\/messages$/))) {
		const chat = fx.chatFor(cm[1], cm[2], who);
		if (!chat) { res.writeHead(404, { 'content-type': 'application/json' }); res.end('{"error":"not_found"}'); return; }
		const num = (k: string) => (url.searchParams.get(k) ? Number(url.searchParams.get(k)) : undefined);
		const w = fx.chatWindow(chat, { before: num('before'), after: num('after'), at: num('around') }, num('limit') || 60);
		res.writeHead(200, { 'content-type': 'application/json', 'cache-control': 'no-store' });
		res.end(JSON.stringify(w));
		return;
	}
	if (url.pathname.startsWith('/api/page/') || url.pathname === '/api/page') {
		const target = cleanUrl(new URL(url.pathname.slice('/api/page'.length) + url.search || '/', 'http://x'));
		const page = fx.pageData(target, who);
		res.writeHead(page.status, { 'content-type': 'application/json', 'set-cookie': cookie, 'cache-control': 'no-store' });
		res.end(JSON.stringify(page));
		return;
	}
	const target = cleanUrl(url);
	const page = fx.pageData(target, who);
	// Signed out: every page but the public chats sends to the sign-in page.
	if (who === 'public' && page.route === 'not_found' && url.pathname !== '/login') {
		res.writeHead(302, { location: `/login?next=${encodeURIComponent(target)}`, 'set-cookie': cookie });
		res.end();
		return;
	}
	let head: string | null = null;
	let html: string | null = null;
	if (url.searchParams.get('ssr') !== '0') {
		try {
			const entry = await vite.ssrLoadModule('/src/entry-server.tsx');
			({ head, html } = entry.render(page));
		} catch (e) {
			vite.ssrFixStacktrace(e as Error);
			console.error(e);
		}
	}
	let doc = documentHtml({ page, head, html, csrf: 'standin-csrf', assets: { css: [], preload: [], entry: '/src/entry-client.tsx' } });
	doc = await vite.transformIndexHtml(target, doc);
	res.writeHead(page.status, { 'content-type': 'text/html; charset=utf-8', 'set-cookie': cookie, 'cache-control': 'no-store' });
	res.end(doc);
}

createHttp((req, res) => {
	handle(req, res).catch((e) => {
		console.error(e);
		res.writeHead(500, { 'content-type': 'text/plain' });
		res.end(String(e));
	});
}).listen(PORT, '0.0.0.0', () => console.log(`stand-in on :${PORT}`));
