import { rng, mulberry32, NOW, DAY, iso, queryRows, FIRST, LAST } from './seed.js';

const { r, pick, chance, int } = rng(20261117);

const BIOS = ['Kernel hacker. Coffee first.', 'Building things in C and Rust',
	'Student, open source enthusiast', 'Sysadmin by day, gamer by night',
	'Here for the memes', 'Backend dev. Opinions are my own.',
	'Learning assembly the hard way', 'DM for collabs', '', '', ''];
const LANGS = ['en', 'id', 'en', 'id', 'ja', 'de', 'fr', ''];

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
		kind: 'user',
		id: 9300000000 - i * int(3000, 90000),
		first_name: deleted ? '' : first,
		last_name: deleted ? '' : last,
		username: deleted || chance(0.25) ? '' : handle,
		type: deleted ? 'deleted' : bot ? 'bot' : 'regular',
		msg_count: chance(0.35) ? 0 : Math.floor(Math.pow(r(), 3) * 48000),
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
	};
	u.name = deleted ? 'Deleted Account' : (first + (last ? ' ' + last : ''));
	return u;
}

export const users = Array.from({ length: 480 }, (_, i) => makeUser(i));
Object.assign(users[3], {
	first_name: 'Dewi', last_name: 'Kusuma', name: 'Dewi Kusuma',
	username: 'dewikusuma', type: 'regular', msg_count: 38214,
	is_premium: true, is_verified: false, has_photo: true, hue: 262,
	bio: 'Kernel hacker. Coffee first. She/her.', language: 'id',
	phone: '+62 812 5550 1234',
});
export const featuredUser = users[3];
// A few contacts of the logging account, saved under their own labels.
for (const [i, label] of [[3, 'Dewi (kernel)'], [8, 'Bima kampus'], [21, 'Sari GW']]) {
	Object.assign(users[i], { is_contact: true, contact_name: label });
}
export const TOTAL_USERS = 459935;

const byId = new Map(users.map((u) => [String(u.id), u]));
export function getUser(id) {
	return byId.get(String(id));
}

export function queryUsers(state, keep) {
	return queryRows(keep ? users.filter(keep) : users, state, (u) => `${u.name} ${u.username} ${u.id}`);
}

export const USER_FIELDS = [
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

const histCache = new Map();
export function getUserHistory(u) {
	if (histCache.has(u.id)) return histCache.get(u.id);
	const r = mulberry32(Number(u.id % 100000));
	const kinds = ['name', 'username', 'bio', 'photo', 'name', 'bio', 'phone'];
	const out = [];
	let t = NOW - Math.floor(r() * 3) * DAY;
	const n = u.type === 'deleted' ? 1 : 6 + Math.floor(r() * 18);
	for (let i = 0; i < n; i++) {
		t -= Math.floor((1 + r() * 60) * DAY * r());
		const kind = kinds[Math.floor(r() * kinds.length)];
		const e = { kind, at: iso(t) };
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

export function getUsernames(u) {
	const h = getUserHistory(u).filter((e) => e.kind === 'username' && e.action === 'added');
	const list = h.slice(0, 2).map((e) => ({ username: e.detail, kind: 'past' }));
	if (u.username) list.unshift({ username: u.username, kind: 'active' });
	return list;
}

// Messages per day for the last 30 days.
export function getActivity(seed, total) {
	const r = mulberry32(Number(seed % 104729));
	const base = total / 900;
	return Array.from({ length: 30 }, (_, i) =>
		Math.max(0, Math.round(base * (0.3 + r() * 1.4) * (i % 7 === 5 || i % 7 === 6 ? 0.6 : 1))));
}
