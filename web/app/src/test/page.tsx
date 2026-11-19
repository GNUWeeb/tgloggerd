// Helpers for page tests: render a page through the real App with the
// stand-in's dummy page data, and fake the API per test. Nothing here is
// shared between test files (vitest isolates them); call setup() in
// beforeEach so tests in one file do not see each other's calls.
import { cleanup, render } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import App from '../App.tsx';
import { dashboardSeries, pageData, type Persona } from '../../standin/fixtures.ts';
import type { PageData } from '../page.ts';

export interface Call { method: string; path: string; query: URLSearchParams; body: unknown; headers: Headers }
type Handler = (call: Call) => Response | unknown | Promise<Response | unknown>;

export const json = (status: number, body?: unknown) => new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } });

// Calls the page made, in order; and `fake`, which answers them. A
// handler is keyed "METHOD /path" with :name for one path segment and
// returns a Response, or a value sent as 200 JSON (undefined: 204).
// Page data (GET /api/page/...) and the dashboard's chart series come
// from the fixtures unless a test answers them; anything else
// unanswered is a 404.
export function fakeApi(persona: Persona, handlers: Record<string, Handler> = {}) {
	const calls: Call[] = [];
	const routes = Object.entries(handlers).map(([k, h]) => {
		const [method, pattern] = k.split(' ') as [string, string];
		const re = new RegExp('^' + pattern.replace(/:[a-z_]+/g, '[^/]+') + '$');
		return { method, re, h };
	});
	vi.stubGlobal('fetch', async (req: Request) => {
		const u = new URL(req.url);
		const path = decodeURIComponent(u.pathname);
		const text = req.method === 'GET' || req.method === 'HEAD' ? '' : await req.text();
		const call: Call = { method: req.method, path, query: u.searchParams, body: text ? JSON.parse(text) : undefined, headers: req.headers };
		calls.push(call);
		const r = routes.find((x) => x.method === req.method && x.re.test(path));
		if (r) {
			const out = await r.h(call);
			return out instanceof Response ? out : out === undefined ? new Response(null, { status: 204 }) : json(200, out);
		}
		if (req.method === 'GET' && path.startsWith('/api/page/')) {
			const p = pageData(path.slice('/api/page'.length) + u.search, persona);
			return json(p.status, p);
		}
		if (req.method === 'GET' && path === '/api/dashboard/series') {
			const [status, body] = dashboardSeries(persona, u.searchParams);
			return json(status, body);
		}
		return json(404, { error: 'not_found' });
	});
	return calls;
}

// The fixtures' page data for `url`, without rendering it.
export const fixture = <P,>(url: string, persona: Persona = 'superadmin') => pageData(url, persona) as PageData<P>;

// Render the page at `url` as `persona` sees it (or with given data).
export function renderPage(url: string, persona: Persona = 'superadmin', data?: Partial<PageData>) {
	const page = { ...pageData(url, persona), ...data } as PageData;
	history.replaceState(null, '', url);
	let meta = document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]');
	if (!meta) { meta = document.createElement('meta'); meta.name = 'csrf-token'; document.head.append(meta); }
	meta.content = 'test-csrf';
	return { page, ...render(<App page={page} />) };
}

export function setup() {
	// No live stream in page tests: pages fall back as without it.
	vi.stubGlobal('EventSource', undefined);
}

// Unstubbing puts back the refusing fetch of no-network.ts, so a request
// started during or after cleanup never reaches the network.
afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

// The calls matching method and path, for assertions.
export const callsTo = (calls: Call[], method: string, path: string | RegExp) =>
	calls.filter((c) => c.method === method && (typeof path === 'string' ? c.path === path : path.test(c.path)));
