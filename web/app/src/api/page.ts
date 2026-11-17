// Page data for browser navigation: GET /api/page/{path} returns the
// same object the server embedded on first load (web-ssr.md section 7).
// The page's own query string goes along as this request's.
import type { PageData } from '../page.ts';
import { api } from './client.ts';

export type PageAnswer = { kind: 'page'; page: PageData } | { kind: 'reload' };

export async function fetchPage(url: string, signal?: AbortSignal): Promise<PageAnswer> {
	const u = new URL(url, 'http://x');
	const search = u.searchParams.toString();
	const { data, error, response } = await api.GET('/api/page/{path}', {
		params: { path: { path: u.pathname.slice(1) } },
		querySerializer: () => search,
		headers: { accept: 'application/json' },
		signal,
	});
	// A missing page answers 404 with the not_found page's data.
	const body: unknown = data ?? error;
	if (response.status === 401 || !body || typeof body !== 'object') return { kind: 'reload' };
	// A path that is not a React page yet: let the browser load it.
	if ('reload' in body || !('route' in body)) return { kind: 'reload' };
	return { kind: 'page', page: body as PageData };
}
