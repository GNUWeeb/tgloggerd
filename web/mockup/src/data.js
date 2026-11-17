// Synthetic data for the design prototype. Everything here is generated
// from a fixed seed; none of it comes from the real database.

function mulberry32(seed) {
	return function () {
		seed |= 0;
		seed = (seed + 0x6d2b79f5) | 0;
		let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
		t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
		return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
	};
}

const rnd = mulberry32(20261117);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const chance = (p) => rnd() < p;
const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));

const FIRST = ['Arif', 'Bima', 'Citra', 'Dewi', 'Eko', 'Fajar', 'Gita', 'Hana',
	'Indra', 'Joko', 'Kirana', 'Lukas', 'Maya', 'Nadia', 'Oscar', 'Putri',
	'Raka', 'Sari', 'Tomi', 'Umar', 'Vina', 'Wulan', 'Yoga', 'Zara', 'Alex',
	'Mira', 'Leon', 'Nora', 'Ivan', 'Elsa', 'Hugo', 'Rina', 'Kenji', 'Ayu'];
const LAST = ['Pratama', 'Wijaya', 'Santoso', 'Halim', 'Kusuma', 'Lestari',
	'Nugroho', 'Hartono', 'Saputra', 'Fischer', 'Novak', 'Tanaka', 'Moreau',
	'', '', '', ''];
const BIOS = ['Kernel hacker. Coffee first.', 'Building things in C and Rust',
	'Student, open source enthusiast', 'Sysadmin by day, gamer by night',
	'Here for the memes', 'Backend dev. Opinions are my own.',
	'Learning assembly the hard way', 'DM for collabs', '', '', ''];
const LANGS = ['en', 'id', 'en', 'id', 'ja', 'de', 'fr', ''];
const GROUP_NAMES = ['GNU/Weeb', 'Linux Kernel Indonesia', 'Rust Programming',
	'C/C++ Study Group', 'Assembly Lounge', 'Homelab & Self-hosting',
	'Open Source Chat', 'Night Owls Dev', 'Distro Hoppers', 'CTF Practice'];

const NOW = Date.UTC(2026, 10, 17, 16, 0, 0);
const DAY = 86400000;

function iso(ms) {
	return new Date(ms).toISOString().replace('T', ' ').slice(0, 19);
}

function makeUser(i) {
	const first = pick(FIRST);
	const last = pick(LAST);
	const deleted = chance(0.06);
	const bot = !deleted && chance(0.05);
	const created = NOW - int(30, 1400) * DAY - int(0, DAY);
	const updated = Math.min(NOW, created + int(0, 900) * DAY);
	const handle = (first + (last ? '_' + last : '') + (chance(0.4) ? int(1, 99) : ''))
		.toLowerCase() + (bot ? '_bot' : '');
	const u = {
		id: 9300000000 - i * int(3000, 90000),
		first_name: deleted ? '' : first,
		last_name: deleted ? '' : last,
		username: deleted || chance(0.25) ? '' : handle,
		type: deleted ? 'deleted' : bot ? 'bot' : 'regular',
		msg_count: chance(0.35) ? 0 : Math.floor(Math.pow(rnd(), 3) * 48000),
		is_verified: chance(0.03),
		is_premium: !deleted && chance(0.3),
		is_scam: chance(0.015),
		is_fake: chance(0.01),
		is_support: chance(0.005),
		phone: chance(0.1) ? '+62 8' + int(10, 99) + ' ' + int(1000, 9999) + ' ' + int(1000, 9999) : '',
		language: deleted ? '' : pick(LANGS),
		bio: deleted ? '' : pick(BIOS),
		hue: int(0, 359),
		has_photo: !deleted && chance(0.7),
		created_at: iso(created),
		updated_at: iso(updated),
		_created: created,
	};
	u.name = deleted ? '(deleted account)' : (first + (last ? ' ' + last : ''));
	return u;
}

export const users = Array.from({ length: 480 }, (_, i) => makeUser(i));
// A well-populated user to land on from the directions page.
Object.assign(users[3], {
	first_name: 'Dewi', last_name: 'Kusuma', name: 'Dewi Kusuma',
	username: 'dewikusuma', type: 'regular', msg_count: 38214,
	is_premium: true, is_verified: false, has_photo: true, hue: 262,
	bio: 'Kernel hacker. Coffee first. She/her.', language: 'id',
	phone: '+62 812 5550 1234',
});
export const featuredUserId = users[3].id;

export const totals = {
	users: 459935,
	groups: 1842,
	private_messages: 2318774,
	group_messages: 48211906,
	files: 912403,
};

export function getUser(id) {
	return users.find((u) => String(u.id) === String(id));
}

const histCache = new Map();
export function getHistory(u) {
	if (histCache.has(u.id)) return histCache.get(u.id);
	const r = mulberry32(Number(u.id % 100000));
	const kinds = ['name', 'username', 'bio', 'photo', 'name', 'bio', 'phone'];
	const out = [];
	let t = NOW - Math.floor(r() * 3) * DAY;
	const n = 6 + Math.floor(r() * 18);
	for (let i = 0; i < n; i++) {
		t -= Math.floor((1 + r() * 60) * DAY * r());
		const kind = kinds[Math.floor(r() * kinds.length)];
		const e = { kind, at: iso(t), ms: t };
		if (kind === 'name') e.detail = FIRST[Math.floor(r() * FIRST.length)] + ' ' + LAST[Math.floor(r() * 9)];
		if (kind === 'username') {
			e.action = r() < 0.6 ? 'added' : 'removed';
			e.detail = (u.first_name || 'user').toLowerCase() + Math.floor(r() * 999);
		}
		if (kind === 'bio') e.detail = r() < 0.2 ? '' : BIOS[Math.floor(r() * 7)];
		if (kind === 'phone') e.detail = '+62 8' + Math.floor(10 + r() * 89) + ' 5550 ' + Math.floor(1000 + r() * 8999);
		if (kind === 'photo') { e.hue = Math.floor(r() * 360); e.removed = r() < 0.15; }
		out.push(e);
	}
	histCache.set(u.id, out);
	return out;
}

export function getGroupsOf(u) {
	const r = mulberry32(Number(u.id % 7919));
	const n = u.msg_count ? 1 + Math.floor(r() * 6) : 0;
	const start = Math.floor(r() * GROUP_NAMES.length);
	return Array.from({ length: n }, (_, i) => ({
		id: -1001000000000 - Math.floor(r() * 900000000),
		title: GROUP_NAMES[(start + i) % GROUP_NAMES.length],
		msgs: Math.floor(u.msg_count * r() / n) + 1,
		hue: Math.floor(r() * 360),
	})).sort((a, b) => b.msgs - a.msgs);
}

export function getUsernames(u) {
	const h = getHistory(u).filter((e) => e.kind === 'username' && e.action === 'added');
	const list = h.slice(0, 2).map((e) => ({ username: e.detail, kind: 'past' }));
	if (u.username) list.unshift({ username: u.username, kind: 'active' });
	return list;
}

// Messages-per-day for the last 30 days, for the activity sparkline.
export function getActivity(u) {
	const r = mulberry32(Number(u.id % 104729));
	const base = u.msg_count / 900;
	return Array.from({ length: 30 }, (_, i) =>
		Math.max(0, Math.round(base * (0.3 + r() * 1.4) * (i % 7 === 5 || i % 7 === 6 ? 0.6 : 1))));
}

// Filtering / sorting / paging, the same shape the /v1/search API serves.
export const FIELDS = [
	{ key: 'id', label: 'User ID', type: 'number' },
	{ key: 'name', label: 'Name', type: 'text' },
	{ key: 'username', label: 'Username', type: 'text' },
	{ key: 'type', label: 'Type', type: 'enum', values: ['regular', 'bot', 'deleted'] },
	{ key: 'msg_count', label: 'Messages', type: 'number' },
	{ key: 'is_premium', label: 'Premium', type: 'bool' },
	{ key: 'is_verified', label: 'Verified', type: 'bool' },
	{ key: 'is_scam', label: 'Scam', type: 'bool' },
	{ key: 'is_fake', label: 'Fake', type: 'bool' },
	{ key: 'language', label: 'Language', type: 'text' },
	{ key: 'bio', label: 'Bio', type: 'text' },
	{ key: 'phone', label: 'Phone', type: 'text' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
	{ key: 'updated_at', label: 'Updated', type: 'date' },
];

function match(u, c) {
	const v = u[c.field];
	switch (c.op) {
	case 'contains': return String(v).toLowerCase().includes(String(c.value).toLowerCase());
	case '=': return String(v) === String(c.value);
	case '!=': return String(v) !== String(c.value);
	case '>': return Number(v) > Number(c.value);
	case '<': return Number(v) < Number(c.value);
	case 'is': return Boolean(v) === (c.value === true || c.value === 'yes');
	case 'empty': return !v;
	default: return true;
	}
}

export function queryUsers({ q = '', conds = [], sort = 'msg_count', dir = 'desc', page = 1, per = 25 }) {
	let rows = users;
	if (q) {
		const s = q.toLowerCase().replace(/^@/, '');
		rows = rows.filter((u) => u.name.toLowerCase().includes(s) || u.username.includes(s) || String(u.id).includes(s));
	}
	for (const c of conds) rows = rows.filter((u) => match(u, c));
	rows = [...rows].sort((a, b) => {
		const x = a[sort], y = b[sort];
		const d = typeof x === 'number' ? x - y : String(x).localeCompare(String(y));
		return dir === 'asc' ? d : -d;
	});
	const total = rows.length;
	const pages = Math.max(1, Math.ceil(total / per));
	return { total, pages, page: Math.min(page, pages), rows: rows.slice((page - 1) * per, page * per) };
}

export function fmt(n) {
	return n.toLocaleString('en-US');
}

export function compact(n) {
	if (n >= 1e6) return (n / 1e6).toFixed(n >= 1e7 ? 0 : 1) + 'M';
	if (n >= 1e3) return (n / 1e3).toFixed(n >= 1e4 ? 0 : 1) + 'k';
	return String(n);
}

export function ago(s) {
	const ms = NOW - Date.parse(s.replace(' ', 'T') + 'Z');
	const d = Math.floor(ms / DAY);
	if (d < 1) return 'today';
	if (d < 30) return d + 'd ago';
	if (d < 365) return Math.floor(d / 30) + 'mo ago';
	return Math.floor(d / 365) + 'y ago';
}

export function initials(u) {
	const s = (u.first_name?.[0] || '') + (u.last_name?.[0] || '');
	return s || '?';
}
