// A group's Telegram members as the Go service would give them (the
// group-members contract): the cached member count, the member list
// (GET /api/groups/{id}/members and the group_members page) and the
// superadmin's switch that hides a channel's list. Synthetic: the
// members are sample users, picked by the group's id.
// @ts-nocheck: the mockup data is untyped JavaScript.
import { users } from '../../mockup/src/data/users.js';
import { groups, getGroup, getAdmins } from '../../mockup/src/data/groups.js';

type Role = 'superadmin' | 'admin' | 'member' | 'public';

// The samples, by position in the mockup's groups (stable ids):
// GNU/Weeb has a count and a long list (paged); Linux Kernel Indonesia
// has a list but no count yet (Telegram not asked); Assembly Lounge,
// which the archive account left, has a count only; Homelab is under a
// flood wait; the channel Compiler Nerds does not show members to the
// account (no admin there); the channel Data Study has its list hidden
// by a superadmin; the seventh group's list is unavailable (the daemon
// does not answer).
const NO_COUNT = groups[1].id;
const NOT_MEMBER = groups[4].id;
const FLOOD = groups[5].id;
const NOT_ALLOWED = groups[10].id;
const HIDDEN_AT_START = groups[16].id;
// The daemon is off or does not answer.
const UNAVAILABLE = groups[6].id;

// Channels whose list a superadmin hid (web_hidden_member_lists).
const hidden = new Set<number>([HIDDEN_AT_START]);

const AT = '2026-11-17T15:40:00Z';
export function memberCount(g): { member_count: number | null; member_count_at: string | null } {
	if (g.id === NO_COUNT) return { member_count: null, member_count_at: null };
	// More people are in a group than ever wrote in it.
	return { member_count: Math.round(g.members * 1.3) + (Math.abs(g.id) % 97), member_count_at: AT };
}

const summary = (u) => ({ id: u.id, name: u.name, username: u.username, type: u.type, photo: null });

// Every member the list has for a group: its admins first (the creator
// as creator), then sample users, one of them restricted, and one the
// archive has not stored yet (its id alone).
const lists = new Map<number, unknown[]>();
function listOf(g) {
	if (lists.has(g.id)) return lists.get(g.id);
	const seen = new Set<number>();
	const out = [];
	const day = (n: number) => new Date(Date.parse(AT) - n * 86400000).toISOString().replace(/\.\d+Z$/, 'Z');
	for (const a of getAdmins(g)) {
		if (seen.has(a.user.id)) continue;
		seen.add(a.user.id);
		out.push({ user: summary(a.user), status: a.status, joined_at: a.status === 'creator' ? null : day(400 + out.length) });
	}
	const n = g.id === groups[0].id ? 120 : 30;
	const start = Math.abs(g.id) % 300;
	for (let i = 0; out.length < n && i < users.length; i++) {
		const u = users[(start + i * 7) % users.length];
		if (seen.has(u.id)) continue;
		seen.add(u.id);
		out.push({ user: summary(u), status: out.length === 5 ? 'restricted' : 'member', joined_at: day(out.length * 3) });
	}
	out.push({ user: { id: 7700000000 + (Math.abs(g.id) % 1000), name: '', username: '', type: 'unknown', photo: null }, status: 'member', joined_at: null });
	lists.set(g.id, out);
	return out;
}

const fullView = (role: Role) => role === 'superadmin' || role === 'admin';

// The members a search finds, as the contract has Go search them: a
// leading @ is dropped, the name and username match case insensitively
// anywhere, and a q of digits (a leading - allowed) finds the member
// with that exact id (byId) apart from the others (hits). Each match
// says how it was found (matched_by), as the daemon marks them. No
// search (an empty q, or an @ alone) is null.
export const MAX_Q = 64;
function search(list, q: string) {
	const t = q.trim().replace(/^@/, '').toLowerCase();
	if (!t) return null;
	const id = /^-?\d+$/.test(t) ? list.find((m) => String(m.user.id) === t) : undefined;
	const hit = (m) => m !== id && (m.user.name.toLowerCase().includes(t) || (m.user.username ?? '').toLowerCase().includes(t));
	return { byId: id ? { ...id, matched_by: 'id' } : null, hits: list.filter(hit).map((m) => ({ ...m, matched_by: 'search' })) };
}

// The list's first page, or a later one; null for viewers who get 404
// (members, signed out). The caller checks the viewer sees the group.
// With a search (q), the page is one of the search's matches, and
// the first page (offset 0) also holds the id match first, in addition
// to limit; total counts every match, the id match included.
export function membersPage(g, role: Role, offset = 0, limit = 50, q = '') {
	if (!fullView(role)) return null;
	const total = memberCount(g).member_count;
	const off = Math.max(0, Math.floor(offset) || 0);
	const lim = Math.min(200, Math.max(1, Math.floor(limit) || 50));
	// As Go answers: the page asked for, how long Telegram wants a
	// flood wait to last, and no total in a search.
	const none = (reason: string) => ({ available: false, reason, total: search([], q) ? null : total, members: [], offset: off, limit: lim, ...(reason === 'flood_wait' ? { retry_after: 30 } : {}) });
	if (g.id === NOT_MEMBER) return none('not_member');
	if (g.id === FLOOD) return none('flood_wait');
	if (g.id === NOT_ALLOWED) return none('not_allowed');
	if (g.id === UNAVAILABLE) return none('unavailable');
	if (hidden.has(g.id) && role !== 'superadmin') return none('hidden');
	const found = search(listOf(g), q);
	if (!found) return { available: true, reason: '', total, members: listOf(g).slice(off, off + lim), offset: off, limit: lim };
	const first = off === 0 && found.byId ? [found.byId] : [];
	return { available: true, reason: '', total: found.hits.length + (found.byId ? 1 : 0), members: [...first, ...found.hits.slice(off, off + lim)], offset: off, limit: lim };
}

// The group page's fields for the switch.
export function memberFlags(g, role: Role) {
	return { hide_member_list: hidden.has(g.id), can_hide_member_list: role === 'superadmin' && g.type === 'channel' };
}

// GET /api/groups/{id}/members. `sees` tells whether the viewer sees
// the group at all.
export function membersApi(id: string, q: URLSearchParams, role: Role, sees: (g) => boolean): [number, unknown] {
	const g = getGroup(id);
	if (!g || !sees(g)) return [404, { error: 'not_found' }];
	const num = (k: string) => (q.get(k) === null ? undefined : Number(q.get(k)));
	for (const k of ['offset', 'limit']) {
		const v = num(k);
		if (v !== undefined && (!Number.isInteger(v) || v < 0 || (k === 'limit' && (v < 1 || v > 200)))) return [400, { error: 'bad_request', message: `${k} is out of range.`, field: k }];
	}
	const s = q.get('q') ?? '';
	if (s.length > MAX_Q) return [400, { error: 'bad_request', message: `q is longer than ${MAX_Q} characters.`, field: 'q' }];
	const page = membersPage(g, role, num('offset') ?? 0, num('limit') ?? 50, s);
	return page ? [200, page] : [404, { error: 'not_found' }];
}

// PATCH /api/groups/{id} {"hide_member_list": bool}: superadmins only,
// channels only.
export function patchGroup(id: string, body, role: Role, sees: (g) => boolean): [number, unknown] {
	const g = getGroup(id);
	if (!g || !sees(g)) return [404, { error: 'not_found' }];
	if (role !== 'superadmin') return [403, { error: 'forbidden', message: 'Only a superadmin may hide a member list.' }];
	if (typeof body?.hide_member_list !== 'boolean') return [400, { error: 'bad_request', message: 'hide_member_list must be true or false.', field: 'hide_member_list' }];
	if (g.type !== 'channel') return [400, { error: 'bad_request', message: 'Only a channel\'s member list can be hidden.', field: 'hide_member_list' }];
	if (body.hide_member_list) hidden.add(g.id); else hidden.delete(g.id);
	return [200, { id: g.id, ...memberFlags(g, role) }];
}

// The samples, for tests.
export const MEMBER_SAMPLES = { NO_COUNT, NOT_MEMBER, FLOOD, NOT_ALLOWED, UNAVAILABLE, HIDDEN: HIDDEN_AT_START, LONG: groups[0].id };
