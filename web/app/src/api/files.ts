// "Fetch from Telegram" (fetchFile, web/docs/media.md): POST the fetch
// path a media object or file row carries, and get the file's URL back.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

export type FileFetch = S['FileFetch'];

// The token in a fetch path (/api/files/<token>/fetch), or null for a
// path of another shape, which is never posted.
export function fetchToken(path: string): string | null {
	const m = /^\/api\/files\/([^/]+)\/fetch$/.exec(path);
	if (!m) return null;
	try {
		return decodeURIComponent(m[1]!);
	} catch {
		return null;
	}
}

export function fetchFile(path: string): Promise<FileFetch> {
	const token = fetchToken(path);
	if (token === null) return Promise.reject(new Error(`not a fetch path: ${path}`));
	return ok(api.POST('/api/files/{token}/fetch', { params: { path: { token } } }));
}
