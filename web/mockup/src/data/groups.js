import { rng, mulberry32, NOW, DAY, iso, queryRows } from './seed.js';
import { users } from './users.js';

const { r, pick, chance, int } = rng(4242);

const NAMED = [
	['GNU/Weeb', 'gnuweeb', 'Low-level programming, Linux and free software.'],
	['Linux Kernel Indonesia', 'linuxkernelid', 'Discussing kernel development and patches.'],
	['Rust Programming', 'rustlang_id', 'Rust questions, crates and code review.'],
	['C/C++ Study Group', 'cppstudy', 'Learn C and C++ together.'],
	['Assembly Lounge', 'asm_lounge', 'x86-64, ARM64 and RISC-V assembly.'],
	['Homelab & Self-hosting', 'homelab_id', 'Servers, NAS, Proxmox and friends.'],
	['Open Source Chat', '', 'General open source talk.'],
	['Night Owls Dev', '', 'Late night coding sessions.'],
	['Distro Hoppers', 'distrohop', 'Which distro this week?'],
	['CTF Practice', 'ctfpractice', 'Pwn, rev, crypto and web challenges.'],
	['Compiler Nerds', 'compilernerds', 'Parsers, IRs, optimisations.'],
	['Embedded Indonesia', '', 'Microcontrollers, RTOS, PCB design.'],
];
const WORDS = ['Dev', 'Hub', 'Community', 'Study', 'Chat', 'Lab', 'Club', 'Talk', 'Corner', 'Guild'];
const TOPICS = ['Python', 'Go', 'Web', 'Android', 'DevOps', 'Data', 'Security', 'Gaming', 'Music', 'Anime', 'Crypto', 'Design', 'Math', 'Physics', 'Photography'];

function makeGroup(i) {
	const named = NAMED[i];
	const title = named ? named[0] : `${pick(TOPICS)} ${pick(WORDS)}${chance(0.3) ? ' ' + pick(['ID', 'Global', 'JKT', 'Bandung', 'Official']) : ''}`;
	const type = chance(0.12) ? 'channel' : chance(0.15) ? 'basic_group' : 'supergroup';
	const created = NOW - int(60, 1500) * DAY;
	const msg = i < NAMED.length ? int(200000, 9000000) : Math.floor(Math.pow(r(), 2.5) * 2000000);
	return {
		kind: 'group',
		id: -1001000000000 - int(1000000, 999999999),
		title,
		username: named ? named[1] : chance(0.4) ? title.toLowerCase().replace(/[^a-z0-9]+/g, '_').slice(0, 24) : '',
		type,
		description: named ? named[2] : chance(0.6) ? `Welcome to ${title}. Be nice, no spam.` : '',
		msg_count: msg,
		admin_count: type === 'basic_group' ? int(1, 3) : int(1, 14),
		members: int(40, 24000),
		hue: int(0, 359),
		has_photo: chance(0.8),
		created_at: iso(created),
		updated_at: iso(Math.min(NOW, created + int(0, 1400) * DAY)),
		exposed: i === 0 || i === 2 || i === 9,
	};
}

export const groups = Array.from({ length: 220 }, (_, i) => makeGroup(i));
export const featuredGroup = groups[0];

const byId = new Map(groups.map((g) => [String(g.id), g]));
export function getGroup(id) {
	return byId.get(String(id));
}

export function queryGroups(state, keep) {
	return queryRows(keep ? groups.filter(keep) : groups, state, (g) => `${g.title} ${g.username} ${g.id}`);
}

export const GROUP_FIELDS = [
	{ key: 'id', label: 'Group ID', type: 'number' },
	{ key: 'title', label: 'Title', type: 'text' },
	{ key: 'username', label: 'Username', type: 'text' },
	{ key: 'type', label: 'Type', type: 'enum', values: ['supergroup', 'basic_group', 'channel'] },
	{ key: 'msg_count', label: 'Messages', type: 'number' },
	{ key: 'description', label: 'Description', type: 'text' },
	{ key: 'admin_count', label: 'Admins', type: 'number' },
	{ key: 'exposed', label: 'Public (MCP)', type: 'bool' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
	{ key: 'updated_at', label: 'Updated', type: 'date' },
];

// Groups a user wrote in, busiest first.
export function getGroupsOfUser(u) {
	const g = mulberry32(Number(u.id % 7919));
	const n = u.msg_count ? 1 + Math.floor(g() * 6) : 0;
	const start = Math.floor(g() * NAMED.length);
	return Array.from({ length: n }, (_, i) => ({
		...groups[(start + i) % NAMED.length],
		msgs: Math.floor(u.msg_count * g() / n) + 1,
	})).sort((a, b) => b.msgs - a.msgs);
}

const PERMS = ['change info', 'delete messages', 'ban users', 'invite users', 'pin messages', 'manage video chats', 'promote members', 'manage topics', 'post stories'];

const adminCache = new Map();
export function getAdmins(grp) {
	if (adminCache.has(grp.id)) return adminCache.get(grp.id);
	const g = mulberry32(Number(-grp.id % 9973));
	const list = Array.from({ length: grp.admin_count }, (_, i) => {
		const u = users[Math.floor(g() * 200)];
		const creator = i === 0;
		return {
			user: u,
			status: creator ? 'creator' : 'administrator',
			custom_title: g() < 0.4 ? ['Founder', 'Mod', 'Helper', 'Kernel lead', 'Bot wrangler', 'Night shift'][Math.floor(g() * 6)] : '',
			is_anonymous: g() < 0.08,
			perms: creator ? [] : PERMS.filter(() => g() < 0.5),
			since: iso(NOW - Math.floor(g() * 900) * DAY),
		};
	});
	adminCache.set(grp.id, list);
	return list;
}

const histCache = new Map();
export function getGroupHistory(grp) {
	if (histCache.has(grp.id)) return histCache.get(grp.id);
	const g = mulberry32(Number(-grp.id % 104729));
	const kinds = ['title', 'description', 'username', 'photo', 'admin', 'admin', 'admin'];
	const out = [];
	let t = NOW - Math.floor(g() * 4) * DAY;
	for (let i = 0; i < 8 + Math.floor(g() * 20); i++) {
		t -= Math.floor((1 + g() * 50) * DAY * g());
		const kind = kinds[Math.floor(g() * kinds.length)];
		const e = { kind, at: iso(t) };
		if (kind === 'title') e.detail = g() < 0.5 ? grp.title : grp.title + ' ' + ['(old)', 'Official', 'v2', 'Lounge'][Math.floor(g() * 4)];
		if (kind === 'description') e.detail = g() < 0.2 ? '' : grp.description || 'Rules: be nice, stay on topic.';
		if (kind === 'username') { e.action = g() < 0.6 ? 'added' : 'removed'; e.detail = (grp.username || 'group') + (g() < 0.5 ? '' : Math.floor(g() * 99)); }
		if (kind === 'photo') { e.hue = Math.floor(g() * 360); e.removed = g() < 0.1; }
		if (kind === 'admin') {
			e.user = users[Math.floor(g() * 200)];
			e.action = ['promoted', 'demoted', 'rights changed', 'title changed'][Math.floor(g() * 4)];
		}
		out.push(e);
	}
	histCache.set(grp.id, out);
	return out;
}

// Most active members of a group, busiest first.
export function getTopPosters(grp, n = 8) {
	const g = mulberry32(Number(-grp.id % 7907));
	const seen = new Set();
	const out = [];
	let left = grp.msg_count * 0.6;
	while (out.length < n) {
		const u = users[Math.floor(g() * 300)];
		if (seen.has(u.id) || u.type === 'deleted') continue;
		seen.add(u.id);
		const msgs = Math.floor(left * (0.15 + g() * 0.2));
		left -= msgs;
		out.push({ user: u, msgs });
	}
	return out.sort((a, b) => b.msgs - a.msgs);
}
