// A minimal History API router. Every screen has a real path (no #
// fragments), which is what the production app needs for server rendering.
import { createContext, useContext, useEffect, useState, useCallback } from 'react';

const Ctx = createContext(null);

export function RouterProvider({ children }) {
	const [loc, setLoc] = useState(() => ({ path: location.pathname, search: location.search }));
	useEffect(() => {
		const on = () => setLoc({ path: location.pathname, search: location.search });
		addEventListener('popstate', on);
		return () => removeEventListener('popstate', on);
	}, []);
	const navigate = useCallback((to, { replace = false } = {}) => {
		const u = new URL(to, location.href);
		if (u.pathname + u.search === location.pathname + location.search) return;
		history[replace ? 'replaceState' : 'pushState'](null, '', u.pathname + u.search);
		setLoc({ path: u.pathname, search: u.search });
		if (!replace) scrollTo(0, 0);
	}, []);
	return <Ctx.Provider value={{ ...loc, navigate }}>{children}</Ctx.Provider>;
}

export function useRouter() {
	return useContext(Ctx);
}

export function useQuery() {
	const { search, path, navigate } = useRouter();
	const params = Object.fromEntries(new URLSearchParams(search));
	const set = (patch, opts) => {
		const p = new URLSearchParams(search);
		for (const [k, v] of Object.entries(patch)) {
			if (v === undefined || v === '' || v === null) p.delete(k);
			else p.set(k, typeof v === 'string' ? v : JSON.stringify(v));
		}
		const s = p.toString();
		navigate(path + (s ? '?' + s : ''), opts);
	};
	return [params, set];
}

export function Link({ to, children, className, onClick, ...rest }) {
	const { navigate } = useRouter();
	return (
		<a
			href={to}
			className={className}
			onClick={(e) => {
				onClick?.(e);
				if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey) return;
				e.preventDefault();
				navigate(to);
			}}
			{...rest}
		>
			{children}
		</a>
	);
}
