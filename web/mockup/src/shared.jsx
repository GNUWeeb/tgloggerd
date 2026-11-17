import { useEffect, useState } from 'react';
import { useQuery } from './router.jsx';
import { queryUsers, initials } from './data.js';

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

// The users list state lives in the URL query, so every view is a real,
// shareable, server-renderable address.
export function useUserList(defaults = {}) {
	const [p, set] = useQuery();
	let conds = [];
	try { conds = p.f ? JSON.parse(p.f) : []; } catch { conds = []; }
	const state = {
		q: p.q || '',
		conds,
		sort: p.sort || defaults.sort || 'msg_count',
		dir: p.dir || 'desc',
		page: Number(p.page) || 1,
		per: Number(p.per) || defaults.per || 25,
	};
	const result = queryUsers(state);
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

export function Avatar({ u, size = 32, radius = '50%', className = '' }) {
	const style = {
		width: size, height: size, borderRadius: radius, flex: 'none',
		display: 'inline-grid', placeItems: 'center',
		fontSize: Math.round(size * 0.38), fontWeight: 600, color: '#fff',
		letterSpacing: 0.2, userSelect: 'none',
	};
	if (u.type === 'deleted') {
		return <span className={className} style={{ ...style, background: 'var(--avatar-deleted, #8a8f98)' }}>?</span>;
	}
	const bg = u.has_photo === false
		? `hsl(${u.hue} 35% 55%)`
		: `linear-gradient(135deg, hsl(${u.hue} 70% 62%), hsl(${(u.hue + 40) % 360} 65% 48%))`;
	return <span className={className} style={{ ...style, background: bg }}>{initials(u)}</span>;
}

export function GroupAvatar({ g, size = 32, radius = '50%' }) {
	return (
		<span style={{
			width: size, height: size, borderRadius: radius, flex: 'none',
			display: 'inline-grid', placeItems: 'center', color: '#fff',
			fontSize: Math.round(size * 0.4), fontWeight: 600,
			background: `linear-gradient(135deg, hsl(${g.hue} 60% 58%), hsl(${(g.hue + 30) % 360} 60% 45%))`,
		}}>{g.title[0]}</span>
	);
}

export function Sparkline({ data, width = 220, height = 44, bars = false, className = '' }) {
	const max = Math.max(1, ...data);
	if (bars) {
		const w = width / data.length;
		return (
			<svg className={className} width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
				{data.map((v, i) => {
					const h = Math.max(1, (v / max) * (height - 2));
					return <rect key={i} x={i * w + 1} y={height - h} width={w - 2} height={h} rx="1.5" fill="currentColor" opacity={i === data.length - 1 ? 1 : 0.55} />;
				})}
			</svg>
		);
	}
	const pts = data.map((v, i) => [(i / (data.length - 1)) * width, height - 2 - (v / max) * (height - 6)]);
	const d = pts.map((p, i) => (i ? 'L' : 'M') + p[0].toFixed(1) + ' ' + p[1].toFixed(1)).join(' ');
	return (
		<svg className={className} width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none" aria-hidden="true">
			<path d={`${d} L ${width} ${height} L 0 ${height} Z`} fill="currentColor" opacity="0.12" />
			<path d={d} fill="none" stroke="currentColor" strokeWidth="1.6" vectorEffect="non-scaling-stroke" />
		</svg>
	);
}

export function userFlags(u) {
	const f = [];
	if (u.is_verified) f.push(['verified', 'ok']);
	if (u.is_premium) f.push(['premium', 'accent']);
	if (u.type === 'bot') f.push(['bot', 'info']);
	if (u.is_support) f.push(['support', 'info']);
	if (u.is_scam) f.push(['scam', 'danger']);
	if (u.is_fake) f.push(['fake', 'danger']);
	if (u.type === 'deleted') f.push(['deleted', 'muted']);
	return f;
}

export function pageList(page, pages) {
	const out = new Set([1, pages, page, page - 1, page + 1]);
	return [...out].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b)
		.flatMap((n, i, a) => (i && n - a[i - 1] > 1 ? ['...', n] : [n]));
}

// Parses a direction's sub-path into a screen name and params.
export function parseSub(sub) {
	let m;
	if ((m = sub.match(/^\/users\/(-?\d+)\/history\/?$/))) return { screen: 'history', id: m[1] };
	if ((m = sub.match(/^\/users\/(-?\d+)\/?$/))) return { screen: 'user', id: m[1] };
	if (/^\/users\/?$/.test(sub)) return { screen: 'users' };
	return { screen: 'stub', name: sub.replace(/^\//, '').split('/')[0] || 'dashboard' };
}
