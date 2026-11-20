import { useEffect, useState } from 'react';
import { useQuery } from '../router.jsx';

// Theme: 'system' follows prefers-color-scheme; 'light'/'dark' force it.
export function useTheme() {
	const [mode, setMode] = useState(() => {
		try { return localStorage.getItem('tgd-theme') || 'system'; } catch { return 'system'; }
	});
	useEffect(() => {
		const root = document.documentElement;
		if (mode === 'system') delete root.dataset.theme;
		else root.dataset.theme = mode;
		try { localStorage.setItem('tgd-theme', mode); } catch { /* private mode */ }
	}, [mode]);
	const cycle = () => setMode((m) => (m === 'system' ? 'light' : m === 'light' ? 'dark' : 'system'));
	return [mode, cycle];
}

// List state (search, filters, sort, page) lives in the URL query, so
// every list view is a real, shareable, server-renderable address.
export function useList(query, defaults = {}) {
	const [p, set] = useQuery();
	let conds = [];
	try { conds = p.f ? JSON.parse(p.f) : []; } catch { conds = []; }
	const state = {
		q: p.q || '',
		conds,
		sort: p.sort || defaults.sort,
		dir: p.dir || defaults.dir || 'desc',
		page: Number(p.page) || 1,
		per: Number(p.per) || defaults.per || 25,
	};
	const result = query(state);
	const update = (patch, opts) => {
		const out = { ...patch };
		if ('conds' in out) { out.f = out.conds.length ? out.conds : undefined; delete out.conds; }
		if (!('page' in patch)) out.page = undefined;
		set(out, opts);
	};
	const toggleSort = (key) => update({
		sort: key,
		dir: state.sort === key && state.dir === 'desc' ? 'asc' : 'desc',
	});
	return { ...state, ...result, update, toggleSort };
}
