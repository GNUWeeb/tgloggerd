// "Fetch from Telegram": the button a file shows when it has a fetch
// path but no url (a big unstored file, which no page view fetches by
// itself), and the state of each fetch. The state is kept per fetch
// path for the whole page, so the same file in a card, a list row and
// its preview fetches once and every place then uses its new URL.
import { useEffect, useState, useSyncExternalStore } from 'react';
import { CloudDownload, LoaderCircle } from 'lucide-react';
import { ApiError } from '../api/client.ts';
import { fetchFile, type FileFetch } from '../api/files.ts';

export type FetchState = {
	busy: boolean;
	got: FileFetch | null;
	err: string | null;
	// Asking again is pointless (not allowed, gone, unknown): the button
	// stays disabled.
	final: boolean;
	// Before this time (ms) the button stays disabled: Retry-After.
	until: number;
};

const IDLE: FetchState = { busy: false, got: null, err: null, final: false, until: 0 };
const states = new Map<string, FetchState>();
const subs = new Set<() => void>();
const put = (path: string, st: FetchState) => {
	states.set(path, st);
	for (const f of subs) f();
};
const subscribe = (f: () => void) => {
	subs.add(f);
	return () => { subs.delete(f); };
};

// For tests: forget every fetch.
export function resetFetches(): void {
	states.clear();
	for (const f of subs) f();
}

// A busy daemon that names no wait is asked again no sooner than this.
const BUSY_WAIT = 10;

// What went wrong, in words, and whether a click may try again.
export function fetchError(e: unknown): { err: string; final: boolean; wait: number } {
	if (!(e instanceof ApiError)) return { err: 'The server could not be reached. Try again.', final: false, wait: 0 };
	switch (e.status) {
		case 401: return { err: 'Sign in to fetch files from Telegram.', final: true, wait: 0 };
		case 403:
			if (e.code === 'fetch_not_allowed') return { err: 'You may not fetch files.', final: true, wait: 0 };
			if (e.code === 'csrf') return { err: 'The page has expired. Reload it and try again.', final: true, wait: 0 };
			return { err: 'Fetching is not allowed now.', final: true, wait: 0 };
		case 404: return { err: 'The file was not found.', final: true, wait: 0 };
		case 410: return { err: 'Telegram no longer has this file.', final: true, wait: 0 };
		case 502: return { err: 'Fetching from Telegram failed.', final: false, wait: 0 };
		case 503: return e.retryAfter
			? { err: `Telegram is busy, try again in ${e.retryAfter} s.`, final: false, wait: e.retryAfter }
			: { err: 'Telegram is busy, try again later.', final: false, wait: BUSY_WAIT };
		case 504: return { err: 'Telegram took too long. Try again in a moment.', final: false, wait: 0 };
		default: return { err: 'Fetching from Telegram failed.', final: false, wait: 0 };
	}
}

const canStart = (st: FetchState | undefined) => !st || (!st.busy && !st.got && !st.final && st.until <= Date.now());

// Ask for the file once: a fetch already running, done, refused for
// good or told to wait is not asked again.
export async function startFetch(path: string): Promise<FileFetch | null> {
	if (!canStart(states.get(path))) return null;
	put(path, { ...IDLE, busy: true });
	try {
		const got = await fetchFile(path);
		put(path, { ...IDLE, got });
		return got;
	} catch (e) {
		const { err, final, wait } = fetchError(e);
		put(path, { ...IDLE, err, final, until: wait ? Date.now() + wait * 1000 : 0 });
		return null;
	}
}

export function useFileFetch(path: string | null | undefined): FetchState {
	return useSyncExternalStore(subscribe, () => (path ? states.get(path) ?? IDLE : IDLE), () => IDLE);
}

type Fetchable = { url?: string | null; stored: boolean; fetch?: string | null };

// The file as it is now: with the URL a fetch answered, once it has.
// A partial file is not stored, but its URL streams it already.
export function useFetched<T extends Fetchable>(f: T): T {
	const st = useFileFetch(f.url ? null : f.fetch);
	return st.got && !f.url ? { ...f, url: st.got.url, stored: st.got.state === 'stored' } : f;
}

// Whether a file waits for the button: no url, but a fetch path.
export const needsFetch = (f: Fetchable) => !f.url && !!f.fetch;

// The button and, under it, what went wrong. onDone gets the answer of
// the fetch this button started. Clicks stop here, so a button inside
// a clickable row or card does not open it too.
export function FetchButton({ path, onDone, primary = false, small = false }: { path: string; onDone?: (got: FileFetch) => void; primary?: boolean; small?: boolean }) {
	const st = useFileFetch(path);
	const [, tick] = useState(0);
	// Wake up when Retry-After has passed, to enable the button again.
	useEffect(() => {
		const left = st.until - Date.now();
		if (left <= 0) return;
		const t = setTimeout(() => tick((x) => x + 1), left + 50);
		return () => clearTimeout(t);
	}, [st.until]);
	const waiting = st.until > Date.now();
	return (
		<span className={`a-fetch ${small ? 'sm' : ''}`}>
			<button type="button" className={`a-btn ${primary ? 'a-btn-primary' : ''} a-fetchbtn`} disabled={st.busy || st.final || waiting} aria-busy={st.busy || undefined}
				onClick={(e) => {
					e.preventDefault();
					e.stopPropagation();
					void startFetch(path).then((got) => { if (got) onDone?.(got); });
				}}>
				{st.busy ? <LoaderCircle size={14} className="a-spin" /> : <CloudDownload size={14} />}
				{st.busy ? 'Fetching from Telegram...' : 'Fetch from Telegram'}
			</button>
			{st.err && <span className="a-fetcherr" role="alert">{st.err}</span>}
		</span>
	);
}
