// Development stand-in for the Go service's page side: routes a path to
// fixture page data, renders it with the app's render entry and serves
// the document Go would assemble, plus /api/page/<path> for browser
// navigation. Vite serves the client modules with hot reload. Never
// deployed; the real server is webapi/.
//
//   ?as=superadmin|admin|member   view as a sample account (remembered in a cookie)
//   ?ssr=0                        skip server rendering, as when the render service is down
import { createServer as createHttp, type IncomingMessage, type ServerResponse } from 'node:http';
import { createServer as createVite } from 'vite';
import { documentHtml } from './document.ts';

const PORT = Number(process.env.PORT) || 5173;
const vite = await createVite({
	server: { middlewareMode: true, fs: { allow: ['..'] } },
	appType: 'custom',
});

type Persona = 'superadmin' | 'admin' | 'member';
function persona(req: IncomingMessage, url: URL): Persona {
	const q = url.searchParams.get('as');
	if (q === 'superadmin' || q === 'admin' || q === 'member') return q;
	const c = /(?:^|; )tgd_as=(\w+)/.exec(req.headers.cookie || '')?.[1];
	return c === 'admin' || c === 'member' ? c : 'superadmin';
}

function cleanUrl(url: URL): string {
	url.searchParams.delete('as');
	url.searchParams.delete('ssr');
	const q = url.searchParams.toString();
	return url.pathname + (q ? '?' + q : '');
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
	if (url.pathname.startsWith('/api/page/') || url.pathname === '/api/page') {
		const target = cleanUrl(new URL(url.pathname.slice('/api/page'.length) + url.search || '/', 'http://x'));
		const page = fx.pageData(target, who);
		res.writeHead(page.status, { 'content-type': 'application/json', 'set-cookie': cookie, 'cache-control': 'no-store' });
		res.end(JSON.stringify(page));
		return;
	}
	const target = cleanUrl(url);
	const page = fx.pageData(target, who);
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
