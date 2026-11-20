// In-app navigation over real URLs: a click on a Link fetches the next
// page's data from /api/page and swaps the page without a reload; back
// and forward do the same. Anything else falls back to a full load.
import { createContext, useCallback, useContext, useEffect, useRef, useState, type AnchorHTMLAttributes, type MouseEvent, type ReactNode } from 'react';
import type { PageData, Viewer } from './page.ts';
import { fetchPage } from './api/page.ts';
import { onGate } from './api/client.ts';

interface Nav {
	page: PageData;
	viewer: Viewer;
	loading: boolean;
	navigate: (to: string, opts?: { replace?: boolean; scroll?: boolean }) => void;
	// Refetch the current page's data in place: no history entry, no
	// scrolling, no loading state. Dropped if a navigation starts.
	refresh: () => Promise<void>;
	// Change the address in place, without fetching: for state a page
	// keeps in its URL and draws itself (an open preview). The page data
	// keeps its props and takes the new url, so the next setQuery, a
	// refresh and a reload all see it. No history entry.
	replaceUrl: (to: string) => void;
}

const Ctx = createContext<Nav | null>(null);

// A page's URL without a fragment: fragments stay in the browser.
const noHash = (u: string) => u.split('#')[0] ?? '';

export function NavProvider({ initial, children }: { initial: PageData; children: ReactNode }) {
	const [page, setPage] = useState(initial);
	const [loading, setLoading] = useState(false);
	const abort = useRef<AbortController | null>(null);
	const quiet = useRef<AbortController | null>(null);
	const current = useRef(initial.url);

	const load = useCallback(async (url: string, push: 'push' | 'replace' | 'none', scroll: boolean) => {
		abort.current?.abort();
		quiet.current?.abort();
		const ac = new AbortController();
		abort.current = ac;
		setLoading(true);
		try {
			const r = await fetchPage(url, ac.signal);
			if (r.kind === 'reload') {
				location.assign(url);
				return;
			}
			// The address bar shows the URL the server answered with: it
			// translates old (Drogon) list URLs to the current ones.
			const shown = noHash(r.page.url) || url;
			if (push === 'push') history.pushState(null, '', shown);
			else if (push === 'replace' || shown !== url) history.replaceState(null, '', shown);
			current.current = r.page.url;
			setPage(r.page);
			if (scroll) scrollTo(0, 0);
		} catch (e) {
			if ((e as Error).name !== 'AbortError') location.assign(url);
		} finally {
			if (abort.current === ac) setLoading(false);
		}
	}, []);

	// The first page, rendered by the server for an old URL, names the
	// current one: show that, keeping the address bar's own #anchor (a
	// page may already have consumed and removed it, as the set-password
	// token, so it is never taken from the page data).
	useEffect(() => {
		const want = noHash(initial.url);
		if (want && location.pathname + location.search !== want) history.replaceState(null, '', want + location.hash);
	}, []);

	useEffect(() => {
		const onPop = () => void load(location.pathname + location.search, 'none', false);
		addEventListener('popstate', onPop);
		return () => removeEventListener('popstate', onPop);
	}, [load]);

	const navigate = useCallback((to: string, opts: { replace?: boolean; scroll?: boolean } = {}) => {
		void load(to, opts.replace ? 'replace' : 'push', opts.scroll ?? !opts.replace);
	}, [load]);

	const refresh = useCallback(async () => {
		const url = current.current;
		quiet.current?.abort();
		const ac = new AbortController();
		quiet.current = ac;
		try {
			const r = await fetchPage(url, ac.signal);
			// A page that now needs a full load (signed out, no longer a
			// React page) is left alone until the next navigation.
			if (r.kind === 'page' && !ac.signal.aborted && current.current === url) setPage(r.page);
		} catch { /* the next event or poll tries again */ }
	}, []);

	const replaceUrl = useCallback((to: string) => {
		const url = noHash(to);
		history.replaceState(null, '', url);
		current.current = url;
		setPage((p) => ({ ...p, url }));
	}, []);

	useEffect(() => {
		onGate(() => void refresh());
		return () => onGate(null);
	}, [refresh]);

	return <Ctx.Provider value={{ page, viewer: page.viewer, loading, navigate, refresh, replaceUrl }}>{children}</Ctx.Provider>;
}

export function useNav(): Nav {
	const v = useContext(Ctx);
	if (!v) throw new Error('useNav outside NavProvider');
	return v;
}

// The current URL's query, from the page data so server and client agree.
export function useQueryParams(): URLSearchParams {
	const { page } = useNav();
	return new URL(page.url, 'http://x').searchParams;
}

export function setQuery(url: string, patch: Record<string, string | number | undefined | null>): string {
	const u = new URL(url, 'http://x');
	for (const [k, v] of Object.entries(patch)) {
		if (v === undefined || v === null || v === '') u.searchParams.delete(k);
		else u.searchParams.set(k, String(v));
	}
	const q = u.searchParams.toString();
	return u.pathname + (q ? '?' + q : '');
}

export function Link({ to, children, onClick, ...rest }: { to: string; children?: ReactNode } & AnchorHTMLAttributes<HTMLAnchorElement>) {
	const { navigate } = useNav();
	return (
		<a
			href={to}
			onClick={(e: MouseEvent<HTMLAnchorElement>) => {
				onClick?.(e);
				if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
				e.preventDefault();
				navigate(to);
			}}
			{...rest}
		>
			{children}
		</a>
	);
}
