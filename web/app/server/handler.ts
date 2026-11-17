// The render service's HTTP handler, protocol version 1
// (docs/design/web-ssr.md section 4). Kept apart from the socket set-up
// so tests can drive it with a stand-in renderer.
import type { IncomingMessage, ServerResponse } from 'node:http';
import type { PageData, RenderError, RenderRequest, RenderResult } from '../src/page.ts';

export const PROTOCOL = 1;
export const MAX_BODY = 8 << 20;

export interface Renderer {
	build: string;
	render(page: PageData): RenderResult;
}

export interface Log {
	(entry: Record<string, unknown>): void;
}

function send(res: ServerResponse, status: number, body: unknown): void {
	const json = JSON.stringify(body);
	res.writeHead(status, { 'content-type': 'application/json', 'content-length': Buffer.byteLength(json) });
	res.end(json);
}

function fail(res: ServerResponse, status: number, error: RenderError['error'], detail: string): void {
	send(res, status, { v: PROTOCOL, error, detail } satisfies RenderError);
}

function readBody(req: IncomingMessage): Promise<Buffer | null> {
	return new Promise((resolve, reject) => {
		const parts: Buffer[] = [];
		let size = 0;
		req.on('data', (c: Buffer) => {
			size += c.length;
			if (size > MAX_BODY) {
				resolve(null);
				req.destroy();
				return;
			}
			parts.push(c);
		});
		req.on('end', () => resolve(Buffer.concat(parts)));
		req.on('error', reject);
	});
}

function isObject(x: unknown): x is Record<string, unknown> {
	return typeof x === 'object' && x !== null && !Array.isArray(x);
}

// Returns the request, or the error to answer with.
export function parseRequest(raw: unknown): RenderRequest | [number, RenderError['error'], string] {
	if (!isObject(raw)) return [400, 'bad_request', 'body is not a JSON object'];
	if (raw.v !== PROTOCOL) return [400, 'unknown_version', `protocol version ${String(raw.v)} is not ${PROTOCOL}`];
	if (typeof raw.route !== 'string' || !raw.route) return [400, 'bad_request', 'route missing'];
	if (typeof raw.url !== 'string' || !raw.url.startsWith('/')) return [400, 'bad_request', 'url must be a path'];
	if (raw.status !== 200 && raw.status !== 404) return [400, 'bad_request', 'status must be 200 or 404'];
	if (typeof raw.build !== 'string') return [400, 'bad_request', 'build missing'];
	if (!('props' in raw)) return [400, 'bad_request', 'props missing'];
	if (!isObject(raw.viewer) || typeof raw.viewer.now !== 'string') return [400, 'bad_request', 'viewer missing or without now'];
	return raw as unknown as RenderRequest;
}

export function makeHandler(renderer: Renderer, log: Log, started = Date.now()) {
	return async (req: IncomingMessage, res: ServerResponse): Promise<void> => {
		if (req.method === 'GET' && req.url === '/health') {
			send(res, 200, { v: PROTOCOL, ok: true, build: renderer.build, uptime_s: Math.floor((Date.now() - started) / 1000) });
			return;
		}
		if (req.method !== 'POST' || req.url !== '/render') {
			fail(res, 404, 'bad_request', `${req.method} ${req.url} is not served`);
			return;
		}
		const t0 = performance.now();
		const body = await readBody(req);
		if (body === null) {
			fail(res, 400, 'too_large', `request body over ${MAX_BODY} bytes`);
			return;
		}
		let raw: unknown;
		try {
			raw = JSON.parse(body.toString('utf8'));
		} catch {
			fail(res, 400, 'bad_request', 'body is not valid JSON');
			return;
		}
		const r = parseRequest(raw);
		if (Array.isArray(r)) {
			fail(res, r[0], r[1], r[2]);
			return;
		}
		// Markup from another build would not hydrate with the client
		// code the Go service links; refuse so it falls back instead.
		if (r.build !== renderer.build) {
			fail(res, 409, 'build_mismatch', `render service is build ${renderer.build}, request wants ${r.build}`);
			return;
		}
		let out: RenderResult;
		try {
			out = renderer.render(r);
		} catch (e) {
			const err = e as Error;
			if (err?.name === 'UnknownRoute') {
				fail(res, 422, 'unknown_route', `no route ${r.route}`);
			} else {
				log({ request_id: r.request_id, route: r.route, error: String(err?.stack || err) });
				fail(res, 500, 'render_failed', String(err?.message || err));
			}
			return;
		}
		const ms = Math.round(performance.now() - t0);
		const json = JSON.stringify({ v: PROTOCOL, build: renderer.build, head: out.head, html: out.html, ms });
		if (Buffer.byteLength(json) > MAX_BODY) {
			fail(res, 500, 'too_large', 'rendered page over the response limit');
			return;
		}
		res.writeHead(200, { 'content-type': 'application/json', 'content-length': Buffer.byteLength(json) });
		res.end(json);
		log({ request_id: r.request_id, route: r.route, status: r.status, bytes: json.length, ms });
	};
}
