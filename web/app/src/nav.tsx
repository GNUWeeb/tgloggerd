// In-app navigation over real URLs: a click on a Link fetches the next
// page's data from /api/page and swaps the page without a reload; back
// and forward do the same. Anything else falls back to a full load.
import { createContext, useCallback, useContext, useEffect, useRef, useState, type AnchorHTMLAttributes, type MouseEvent, type ReactNode } from 'react';
import type { PageData, Viewer } from './page.ts';
import { fetchPage } from './api/page.ts';
import { onEmailRequired } from './api/client.ts';

interface Nav {
	page: PageData;
	viewer: Viewer;
	loading: boolean;
	navigate: (to: string, opts?: { replace?: boolean; scroll?: boolean }) => void;
	// Refetch the current page's data in place: no history entry, no
	// scrolling, no loading state. Dropped if a navigation starts.
	refresh: () => Promise<void>;
}

const Ctx = createContext<Nav | null>(null);

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
			if (push === 'push') history.pushState(null, '', url);
			if (push === 'replace') history.replaceState(null, '', url);
			current.current = r.page.url;
			setPage(r.page);
			if (scroll) scrollTo(0, 0);
		} catch (e) {
			if ((e as Error).name !== 'AbortError') location.assign(url);
		} finally {
			if (abort.current === ac) setLoading(false);
		}
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

	useEffect(() => {
		onEmailRequired(() => void refresh());
		return () => onEmailRequired(null);
	}, [refresh]);

	return <Ctx.Provider value={{ page, viewer: page.viewer, loading, navigate, refresh }}>{children}</Ctx.Provider>;
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
