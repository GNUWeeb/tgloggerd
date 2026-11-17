// Fixture page data for the stand-in, built from the mockup's synthetic
// data (web/mockup/src/data). Loaded through Vite, so it can import the
// mockup's modules. Development only: the Go service is the real source.
// @ts-nocheck: the mockup data is untyped JavaScript.
import { users, getUser, getUserHistory, getUsernames, getActivity, queryUsers, TOTAL_USERS } from '../../mockup/src/data/users.js';
import { groups, getGroupsOfUser } from '../../mockup/src/data/groups.js';
import { groupMessages, senderOf } from '../../mockup/src/data/messages.js';
import { accounts, federations } from '../../mockup/src/data/accounts.js';
import { buildScope, allowed } from '../../mockup/src/viewer.jsx';
import { TOTALS } from '../../mockup/src/data/totals.js';
import type { PageData, Viewer } from '../src/page.ts';

const NOW = '2026-11-17T16:00:00Z';
const iso = (s: string) => s.replace(' ', 'T') + 'Z';

export const PERSONAS = { superadmin: 1, admin: 3, member: 5 } as const;
export type Persona = keyof typeof PERSONAS;

const PAGE_KEYS = ['dashboard', 'users', 'groups', 'group_messages', 'private_messages', 'files', 'platform_fwd', 'join_chat', 'mcp', 'accounts', 'federations'];

function scopeOf(persona: Persona) {
	return buildScope(accounts.find((a) => a.id === PERSONAS[persona]));
}

export function viewerOf(persona: Persona): Viewer {
	const s = scopeOf(persona);
	return {
		signed_in: true,
		account: { id: s.acct.id, username: s.acct.username, name: s.acct.name, role: s.acct.role },
		now: NOW,
		nav: PAGE_KEYS.filter((k) => allowed(s, k)),
		pinned: groups.slice(0, 12).filter(s.group).slice(0, 3).map((g) => ({ id: g.id, title: g.title })),
	};
}

function userSummary(u, full: boolean) {
	const base = { id: u.id, name: u.name, username: u.username, type: u.type, photo: null, contact_name: u.contact_name };
	if (!full) return base;
	return {
		...base, msg_count: u.msg_count, is_verified: u.is_verified, is_premium: u.is_premium, is_scam: u.is_scam, is_fake: u.is_fake,
		is_support: u.is_support, language: u.language, created_at: iso(u.created_at), updated_at: iso(u.updated_at),
	};
}

const groupSummary = (g) => ({ id: g.id, title: g.title, username: g.username, type: g.type, photo: null, exposed: g.exposed });

type Loaded = { route: string; status: 200 | 404; props: unknown };
const notFound: Loaded = { route: 'not_found', status: 404, props: {} };

function listState(q: URLSearchParams, defSort: string) {
	let conds = [];
	try { conds = q.get('f') ? JSON.parse(q.get('f')) : []; } catch { conds = []; }
	return {
		q: q.get('q') || '', conds, sort: q.get('sort') || defSort, dir: q.get('dir') || 'desc',
		page: Number(q.get('page')) || 1, per: Number(q.get('per')) || 25,
	};
}

// Path -> route and props, with the viewer's scope applied: hidden things
// answer exactly like missing ones.
export function load(url: string, persona: Persona): Loaded {
	const u = new URL(url, 'http://x');
	const path = u.pathname.replace(/\/+$/, '') || '/';
	const s = scopeOf(persona);
	const full = s.fullUser;
	let m;
	if (path === '/') {
		const vis = groups.slice(0, 12).filter(s.group);
		return {
			route: 'dashboard', status: 200, props: {
				kpis: (s.role === 'member'
					? [['users', users.filter(s.user).length, 2], ['groups', vis.length, 0], ['group_messages', vis.reduce((a, g) => a + g.msg_count, 0), 780]]
					: [['users', TOTALS.users, 412], ['groups', TOTALS.groups, 3], ['group_messages', TOTALS.group_messages, 38211], ['private_messages', TOTALS.private_messages, 1290], ['files', TOTALS.files, 2406]]
				).filter(([k]) => allowed(s, k)).map(([key, total, today]) => ({ key, total, today })),
				per_day: getActivity(31337, 38000 * 900 / 30),
				busiest: vis.sort((a, b) => b.msg_count - a.msg_count).slice(0, 6).map((g) => ({ ...groupSummary(g), msg_count: g.msg_count })),
				deleted: groupMessages().filter((x) => x.deleted_at && s.group(x.chat)).slice(0, 5).map((x) => ({
					id: x.id, scope: 'group', chat: groupSummary(x.chat), sender: userSummary(senderOf(x), false),
					snippet: x.text || `[${x.content_type}]`, deleted_at: iso(x.deleted_at), message_id: x.message_id,
				})),
				status: s.role === 'member' ? undefined : { session: 'connected', last_message_at: '2026-11-17T15:59:56Z', forwarding_active: 6, store_used: '5.3 TB', store_total: '8 TB', store_ratio: 0.66 },
			},
		};
	}
	if (path === '/users') {
		const st = listState(u.searchParams, full ? 'msg_count' : 'name');
		const r = queryUsers(st, s.role === 'member' ? s.user : null);
		return { route: 'users', status: 200, props: { ...st, ...r, rows: r.rows.map((x) => userSummary(x, full)), archive_total: full ? TOTAL_USERS : undefined } };
	}
	if ((m = path.match(/^\/users\/(\d+)(\/history)?$/))) {
		const usr = getUser(m[1]);
		if (!usr || !s.user(usr)) return notFound;
		if (!full) {
			if (m[2]) return notFound;
			return { route: 'user', status: 200, props: { limited: true, user: userSummary(usr, false), shared_groups: getGroupsOfUser(usr).filter(s.group).map(groupSummary) } };
		}
		return {
			route: m[2] ? 'user_history' : 'user', status: 200, props: {
				user: { ...userSummary(usr, true), first_name: usr.first_name, last_name: usr.last_name, bio: usr.bio, phone: usr.phone, usernames: getUsernames(usr).map((n) => ({ username: n.username, active: n.kind === 'active' })) },
				groups: getGroupsOfUser(usr).filter(s.group).map((g) => ({ ...groupSummary(g), msgs: g.msgs })),
				history: getUserHistory(usr).map((e) => ({ kind: e.kind, at: iso(e.at), detail: e.detail, action: e.action, photo: null, removed: e.kind === 'photo' ? true : undefined })),
				activity: getActivity(usr.id, usr.msg_count),
				can_private: allowed(s, 'private_messages'),
			},
		};
	}
	return notFound;
}

export function pageData(url: string, persona: Persona): PageData {
	const r = load(url, persona);
	return { v: 1, route: r.route, url, status: r.status, props: r.props, viewer: viewerOf(persona) };
}

export { federations };
