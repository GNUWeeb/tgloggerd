// Page data for browser navigation: GET /api/page/<path> returns the
// same object the server embedded on first load (web-ssr.md section 7).
// Hand-written until the generated client exists (go-web-openapi).
import type { PageData } from '../page.ts';

export type PageAnswer = { kind: 'page'; page: PageData } | { kind: 'reload' };

export async function fetchPage(url: string, signal?: AbortSignal): Promise<PageAnswer> {
	const res = await fetch('/api/page' + url, { headers: { accept: 'application/json' }, credentials: 'same-origin', signal });
	if (res.status === 401) return { kind: 'reload' };
	const body = (await res.json()) as PageData | { reload: true };
	// A path that is not a React page yet: let the browser load it.
	if ('reload' in body) return { kind: 'reload' };
	return { kind: 'page', page: body };
}
