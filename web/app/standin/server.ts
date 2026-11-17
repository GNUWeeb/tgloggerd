// Development stand-in for the Go service's page side: routes a path to
// fixture page data, renders it with the app's render entry and serves
// the document Go would assemble, plus /api/page/<path> for browser
// navigation. Vite serves the client modules with hot reload. Never
// deployed; the real server is webapi/.
//
//   ?as=superadmin|admin|member|public   view as a sample account, or signed out
//                                 (remembered in a cookie)
//   ?ssr=0                        skip server rendering, as when the render service is down
//
// Live updates: GET /api/events is a Server-Sent Events stream as
// web-live.md specifies. Events come from POST /api/_standin/emit
// {kind, chat_id?, message_id?, user_id?}, which also makes the change
// in the fixtures; POST /api/_standin/events {"off": true} makes the
// stream refuse and drops open ones, to exercise the polling fallback.
// TGD_STANDIN_TICK=<ms> emits a new message into a subscribed chat that
// often.
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

// The live event stream.
interface LiveClient { res: ServerResponse; who: Persona; topics: Set<string> }
type Ev = { seq: number; kind: string; chat_id?: number; user_id?: number; [k: string]: unknown };
const liveClients = new Set<LiveClient>();
const liveRing: Ev[] = [];
let liveSeq = 1000;
let liveOff = false;

function evTopics(e: Ev): string[] | null {
	if (e.kind.startsWith('message.') || e.kind === 'file.stored') return [`chat:${e.chat_id}`, 'messages'];
	if (e.kind === 'user.updated') return [`user:${e.user_id}`];
	if (e.kind === 'group.updated') return [`group:${e.chat_id}`];
	if (e.kind === 'stats.changed') return ['stats'];
	return null;
}

function sendEv(c: LiveClient, e: Ev): void {
	c.res.write(`id: ${e.seq}\nevent: ${e.kind}\ndata: ${JSON.stringify(e)}\n\n`);
}

async function publish(e: Omit<Ev, 'seq'>): Promise<Ev> {
	const fx = await vite.ssrLoadModule('/standin/fixtures.ts');
	const ev = { ...e, seq: ++liveSeq } as Ev;
	liveRing.push(ev);
	if (liveRing.length > 500) liveRing.shift();
	const ts = evTopics(ev);
	for (const c of liveClients) {
		if (ts && !ts.some((t) => c.topics.has(t))) continue;
		if (!fx.liveVisible(c.who, ev)) continue;
		sendEv(c, ev);
	}
	return ev;
}

async function openStream(req: IncomingMessage, res: ServerResponse, url: URL, who: Persona): Promise<void> {
	if (who === 'public') { send(res, 401, { error: 'unauthenticated' }); return; }
	if (liveOff) { send(res, 503, { error: 'live_off', message: 'Live updates are off.' }); return; }
	const fx = await vite.ssrLoadModule('/standin/fixtures.ts');
	const c: LiveClient = { res, who, topics: new Set((url.searchParams.get('topics') || '').split(',').filter(Boolean)) };
	res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-store', 'x-accel-buffering': 'no' });
	res.write('retry: 2000\n\n');
	const last = Number(req.headers['last-event-id'] || url.searchParams.get('last') || 0);
	if (last) {
		if (liveRing.length && last < liveRing[0]!.seq - 1) sendEv(c, { seq: liveSeq, kind: 'resync' });
		else for (const e of liveRing) if (e.seq > last && (evTopics(e) ?? []).some((t) => c.topics.has(t)) && fx.liveVisible(who, e)) sendEv(c, e);
	}
	liveClients.add(c);
	const beat = setInterval(() => res.write(': heartbeat\n\n'), 15000);
	req.on('close', () => { clearInterval(beat); liveClients.delete(c); });
}

const TICK = Number(process.env.TGD_STANDIN_TICK) || 0;
if (TICK) {
	setInterval(async () => {
		const chats = [...new Set([...liveClients].flatMap((c) => [...c.topics]).filter((t) => t.startsWith('chat:')))];
		if (!chats.length) return;
		const fx = await vite.ssrLoadModule('/standin/fixtures.ts');
		const e = fx.liveChange('message.new', { chat_id: Number(chats[Math.floor(Math.random() * chats.length)]!.slice(5)) });
		if (e) await publish(e);
	}, TICK);
}

// Session CSRF tokens. Pages carry standin-csrf; a password change
// issues a new one, as the Go service does when it reissues the session.
const csrfTokens = new Set(['standin-csrf']);
const csrfOk = (req: IncomingMessage) => csrfTokens.has(String(req.headers['x-csrf-token'] || ''));

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
		if (!csrfOk(req)) { send(res, 403, { error: 'csrf' }); return; }
		res.writeHead(204, { 'set-cookie': 'tgd_as=public; Path=/; SameSite=Lax' });
		res.end();
		return;
	}
	if (url.pathname === '/api/session') {
		if (who === 'public') send(res, 401, { error: 'unauthenticated' });
		else send(res, 200, { user: fx.viewerOf(who).account, csrf: 'standin-csrf' });
		return;
	}
	// The account endpoints proposed for the Go service (settings pages).
	if (url.pathname.startsWith('/api/account/')) {
		if (who === 'public') { send(res, 401, { error: 'unauthenticated' }); return; }
		if (!csrfOk(req)) { send(res, 403, { error: 'csrf' }); return; }
		// Fixture state is untyped JavaScript-shaped data.
		const st: any = fx.settingsState(who);
		const body = req.method === 'GET' || req.method === 'DELETE' ? {} : JSON.parse((await readBody(req)) || '{}');
		const p = url.pathname.slice('/api/account/'.length);
		let am: RegExpMatchArray | null;
		if (p === 'password' && req.method === 'POST') {
			if (body.current_password !== 'standin') { send(res, 400, { error: 'wrong_password', message: 'The current password is wrong.' }); return; }
			st.sessions = st.sessions.filter((x: { current: boolean }) => x.current);
			const csrf = `standin-csrf-${csrfTokens.size}`;
			csrfTokens.add(csrf);
			send(res, 200, { csrf }); return;
		}
		if (p === 'email' && req.method === 'POST') {
			if (body.password !== 'standin') { send(res, 400, { error: 'wrong_password', message: 'The current password is wrong.' }); return; }
			send(res, 202, { pending: body.email }); return;
		}
		if (p === 'preferences' && req.method === 'PUT') {
			st.prefs = Object.fromEntries(Object.entries(body).filter(([, v]) => v !== null));
			send(res, 200, { prefs: st.prefs }); return;
		}
		if ((am = p.match(/^pinned\/(-?\d+)$/)) && req.method === 'DELETE') { st.pinned = st.pinned.filter((g: { id: number }) => String(g.id) !== am![1]); res.writeHead(204); res.end(); return; }
		if ((am = p.match(/^pinned\/(-?\d+)$/)) && req.method === 'PUT') {
			const g = fx.groupFor(who, am[1]);
			if (!g) { send(res, 404, { error: 'not_found' }); return; }
			if (!st.pinned.some((x: { id: number }) => x.id === g.id)) st.pinned.push(g);
			res.writeHead(204); res.end(); return;
		}
		if (p === 'saved-searches' && req.method === 'POST') {
			if (!body.title || typeof body.url !== 'string' || !body.url.startsWith('/')) { send(res, 400, { error: 'bad_request', message: 'A title and a search path are required.' }); return; }
			const id = Math.max(0, ...st.saved.map((x: { id: number }) => x.id)) + 1;
			st.saved.push({ id, title: String(body.title), url: body.url });
			send(res, 201, { id }); return;
		}
		if ((am = p.match(/^saved-searches\/(\d+)$/)) && req.method === 'DELETE') { st.saved = st.saved.filter((x: { id: number }) => String(x.id) !== am![1]); res.writeHead(204); res.end(); return; }
		if ((am = p.match(/^sessions\/(\w+)$/)) && req.method === 'DELETE') { st.sessions = st.sessions.filter((x: { id: string; current: boolean }) => x.current || x.id !== am![1]); res.writeHead(204); res.end(); return; }
		if (p === 'sessions/sign-out-others' && req.method === 'POST') { st.sessions = st.sessions.filter((x: { current: boolean }) => x.current); res.writeHead(204); res.end(); return; }
		send(res, 404, { error: 'not_found' }); return;
	}
	if (url.pathname === '/api/events' && req.method === 'GET') { await openStream(req, res, url, who); return; }
	if (url.pathname === '/api/_standin/emit' && req.method === 'POST') {
		const b = JSON.parse((await readBody(req)) || '{}');
		const e = fx.liveChange(String(b.kind), b);
		if (!e) { send(res, 400, { error: 'bad_request' }); return; }
		send(res, 200, await publish(e));
		return;
	}
	if (url.pathname === '/api/_standin/events' && req.method === 'POST') {
		liveOff = !!JSON.parse((await readBody(req)) || '{}').off;
		if (liveOff) for (const c of liveClients) c.res.end();
		send(res, 200, { off: liveOff });
		return;
	}
	if (/^\/api\/(accounts|federations|forwarding|join-chat|mcp)(\/|$)/.test(url.pathname)) {
		if (who === 'public') { send(res, 401, { error: 'unauthenticated' }); return; }
		if (req.method !== 'GET' && !csrfOk(req)) { send(res, 403, { error: 'csrf' }); return; }
		const body = req.method === 'GET' ? {} : JSON.parse((await readBody(req)) || '{}');
		const [status, out] = fx.adminApi(req.method || 'GET', url.pathname.slice('/api/'.length), body, who);
		if (status === 204) { res.writeHead(204); res.end(); } else send(res, status, out);
		return;
	}
	if (url.pathname === '/api/chats/search' && req.method === 'GET') {
		const [status, out] = fx.searchChats(who, url.searchParams);
		send(res, status, out);
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
		// The path's slashes may come encoded as %2F, as the client sends them.
		const target = cleanUrl(new URL(url.pathname.slice('/api/page'.length).replace(/%2F/gi, '/') + url.search || '/', 'http://x'));
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
