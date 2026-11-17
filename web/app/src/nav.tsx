// In-app navigation over real URLs: a click on a Link fetches the next
// page's data from /api/page and swaps the page without a reload; back
// and forward do the same. Anything else falls back to a full load.
import { createContext, useCallback, useContext, useEffect, useRef, useState, type AnchorHTMLAttributes, type MouseEvent, type ReactNode } from 'react';
import type { PageData, Viewer } from './page.ts';
import { fetchPage } from './api/page.ts';

interface Nav {
	page: PageData;
	viewer: Viewer;
	loading: boolean;
	navigate: (to: string, opts?: { replace?: boolean; scroll?: boolean }) => void;
}

const Ctx = createContext<Nav | null>(null);

export function NavProvider({ initial, children }: { initial: PageData; children: ReactNode }) {
	const [page, setPage] = useState(initial);
	const [loading, setLoading] = useState(false);
	const abort = useRef<AbortController | null>(null);

	const load = useCallback(async (url: string, push: 'push' | 'replace' | 'none', scroll: boolean) => {
		abort.current?.abort();
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

	return <Ctx.Provider value={{ page, viewer: page.viewer, loading, navigate }}>{children}</Ctx.Provider>;
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
