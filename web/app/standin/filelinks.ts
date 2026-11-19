// A file's links as Go gives them (web/docs/media.md).

// The store's per-file cap, as the stand-in has it: a fetchable file
// over it has no url, only its fetch path ("Fetch from Telegram"), so
// no page view fetches it by itself.
export const STORE_CAP = 20 * 2 ** 20;

// A file's url and fetch path, as Go gives them (ChatMedia, FileRow).
export function fileLinks(id: number, tgs: boolean, size: number | undefined, stored: boolean, fetchable: boolean) {
	const token = `standin-${id}${tgs ? '.tgs' : ''}`;
	return { url: stored || (fetchable && (size ?? 0) <= STORE_CAP) ? `/files/${token}` : null, fetch: fetchable ? `/api/files/${token}/fetch` : null };
}
