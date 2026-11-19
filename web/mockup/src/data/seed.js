// Shared helpers for the synthetic data. Everything is generated from fixed
// seeds; none of it comes from the real database.

export function mulberry32(seed) {
	return function () {
		seed |= 0;
		seed = (seed + 0x6d2b79f5) | 0;
		let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
		t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

export function rng(seed) {
	const r = mulberry32(seed);
	return {
		r,
		pick: (a) => a[Math.floor(r() * a.length)],
		chance: (p) => r() < p,
		int: (lo, hi) => lo + Math.floor(r() * (hi - lo + 1)),
	};
}

export const NOW = Date.UTC(2026, 10, 17, 16, 0, 0);
export const DAY = 86400000;

export function iso(ms) {
	return new Date(ms).toISOString().replace('T', ' ').slice(0, 19);
}

export function fmt(n) {
	return Number(n).toLocaleString('en-US');
}

export function compact(n) {
	if (n >= 1e6) return (n / 1e6).toFixed(n >= 1e7 ? 0 : 1) + 'M';
	if (n >= 1e3) return (n / 1e3).toFixed(n >= 1e4 ? 0 : 1) + 'k';
	return String(n);
}

export function bytes(n) {
	if (n >= 2 ** 40) return (n / 2 ** 40).toFixed(1) + ' TB';
	if (n >= 1 << 30) return (n / (1 << 30)).toFixed(1) + ' GB';
	if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1) + ' MB';
	if (n >= 1 << 10) return (n / (1 << 10)).toFixed(0) + ' KB';
	return n + ' B';
}

export function ago(s) {
	const ms = NOW - Date.parse(s.replace(' ', 'T') + 'Z');
	if (ms < 3600000) return Math.max(1, Math.floor(ms / 60000)) + 'm ago';
	if (ms < DAY) return Math.floor(ms / 3600000) + 'h ago';
	const d = Math.floor(ms / DAY);
	if (d < 30) return d + 'd ago';
	if (d < 365) return Math.floor(d / 30) + 'mo ago';
	return Math.floor(d / 365) + 'y ago';
}

export function monthLabel(ym) {
	return new Date(ym + '-01T00:00:00Z').toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });
}

export function dayLabel(ymd) {
	return new Date(ymd + 'T00:00:00Z').toLocaleDateString('en-US', { weekday: 'short', month: 'long', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
}

// Filtering / sorting / paging in the shape the /v1/search API serves.
function match(row, c) {
	const v = row[c.field];
	switch (c.op) {
	case 'contains': return String(v ?? '').toLowerCase().includes(String(c.value).toLowerCase());
	case '=': return String(v) === String(c.value);
	case '!=': return String(v) !== String(c.value);
	case 'in': return Array.isArray(c.value) && c.value.map(String).includes(String(v));
	case '>': return typeof v === 'number' ? v > Number(c.value) : String(v) > String(c.value);
	case '<': return typeof v === 'number' ? v < Number(c.value) : String(v) < String(c.value);
	case 'is': return Boolean(v) === (c.value === true || c.value === 'yes');
	case 'empty': return !v;
	default: return true;
	}
}

export function queryRows(all, { q = '', conds = [], sort, dir = 'desc', page = 1, per = 25 }, searchText) {
	let rows = all;
	if (q) {
		const s = q.toLowerCase().replace(/^@/, '');
		rows = rows.filter((x) => searchText(x).toLowerCase().includes(s));
	}
	for (const c of conds) rows = rows.filter((x) => match(x, c));
	if (sort) {
		rows = [...rows].sort((a, b) => {
			const x = a[sort], y = b[sort];
			const d = typeof x === 'number' ? x - y : String(x).localeCompare(String(y));
			return dir === 'asc' ? d : -d;
		});
	}
	const total = rows.length;
	const pages = Math.max(1, Math.ceil(total / per));
	const p = Math.min(page, pages);
	return { total, pages, page: p, rows: rows.slice((p - 1) * per, p * per) };
}

export const OPS = {
	number: ['=', '>', '<', '!='],
	text: ['contains', '=', 'empty'],
	enum: ['=', '!='],
	bool: ['is'],
	date: ['>', '<'],
};

export const FIRST = ['Arif', 'Bima', 'Citra', 'Dewi', 'Eko', 'Fajar', 'Gita', 'Hana',
	'Indra', 'Joko', 'Kirana', 'Lukas', 'Maya', 'Nadia', 'Oscar', 'Putri',
	'Raka', 'Sari', 'Tomi', 'Umar', 'Vina', 'Wulan', 'Yoga', 'Zara', 'Alex',
	'Mira', 'Leon', 'Nora', 'Ivan', 'Elsa', 'Hugo', 'Rina', 'Kenji', 'Ayu'];
export const LAST = ['Pratama', 'Wijaya', 'Santoso', 'Halim', 'Kusuma', 'Lestari',
	'Nugroho', 'Hartono', 'Saputra', 'Fischer', 'Novak', 'Tanaka', 'Moreau',
	'', '', '', ''];
