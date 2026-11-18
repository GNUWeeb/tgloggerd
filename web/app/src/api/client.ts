// The one way the application calls the API: openapi-fetch over the
// types generated from webapi/openapi.yaml, so a path, parameter or
// body the description does not have fails to compile.
//
// State-changing requests carry the session's CSRF token in
// X-CSRF-Token (the csrf security scheme), added here; a call that
// sets the header itself (signing in, with the pre-login token) keeps
// its own. Errors are {"error": "<code>", "message"?: "..."}.
import createClient, { type Middleware } from 'openapi-fetch';
import type { paths } from './schema.ts';

// retryAfter: the Retry-After the server sent, in seconds, if any.
export class ApiError extends Error {
	constructor(public status: number, public code: string, message?: string, public retryAfter?: number) {
		super(message || code);
	}
}

let csrfOverride: string | null = null;

// The session's token: the csrf-token meta tag the server put in the
// page, until the server issues a new one (a password change moves the
// session to a new cookie, and the old token stops working).
export function sessionCsrf(): string {
	if (csrfOverride !== null) return csrfOverride;
	return typeof document === 'undefined' ? '' : document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]')?.content ?? '';
}

export function setSessionCsrf(token: string): void {
	csrfOverride = token;
	const meta = typeof document === 'undefined' ? null : document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]');
	if (meta) meta.content = token;
}

const csrf: Middleware = {
	onRequest({ request }) {
		if (request.method !== 'GET' && request.method !== 'HEAD' && !request.headers.has('x-csrf-token')) request.headers.set('x-csrf-token', sessionCsrf());
		return request;
	},
};

export const api = createClient<paths>({
	baseUrl: typeof location === 'undefined' ? 'http://localhost' : location.origin,
	credentials: 'same-origin',
	// Looked up per call, so tests can replace fetch after import.
	fetch: (req) => globalThis.fetch(req),
});
api.use(csrf);

// A gate (the spec's Gates): an account that must first choose its own
// password may only do that, and every other call answers 403
// password_change_required. The application then fetches the page data
// again, which brings up the page for the gate.
const GATES = new Set(['password_change_required']);
let onGateHit: (() => void) | null = null;
export function onGate(fn: (() => void) | null): void {
	onGateHit = fn;
}
api.use({
	async onResponse({ response }) {
		if (response.status === 403 && onGateHit) {
			const body = (await response.clone().json().catch(() => null)) as { error?: string } | null;
			if (body?.error && GATES.has(body.error)) onGateHit();
		}
		return response;
	},
});

// The data of a successful call, or an ApiError.
export async function ok<T>(call: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<T> {
	const { data, error, response } = await call;
	if (!response.ok) {
		const e = (error ?? {}) as { error?: string; message?: string };
		const ra = Number(response.headers.get('retry-after') ?? '');
		throw new ApiError(response.status, e.error || 'error', e.message, Number.isFinite(ra) && ra > 0 ? ra : undefined);
	}
	return data as T;
}
