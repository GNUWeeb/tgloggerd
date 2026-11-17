// Fixture page data for the stand-in, built from the mockup's synthetic
// data (web/mockup/src/data). Loaded through Vite, so it can import the
// mockup's modules. Development only: the Go service is the real source.
// @ts-nocheck: the mockup data is untyped JavaScript.
import { users, getUser, getUserHistory, getUsernames, getActivity, queryUsers, TOTAL_USERS } from '../../mockup/src/data/users.js';
import { groups, getGroupsOfUser, queryGroups, getGroup, getAdmins, getGroupHistory, getTopPosters } from '../../mockup/src/data/groups.js';
import { queryFiles, files } from '../../mockup/src/data/files.js';
import { groupMessages, senderOf, queryMessages, getMessage, chatStream, chatOf } from '../../mockup/src/data/messages.js';
import { accounts, federations } from '../../mockup/src/data/accounts.js';
import { buildScope, allowed } from '../../mockup/src/viewer.jsx';
import { TOTALS } from '../../mockup/src/data/totals.js';
import type { PageData, Viewer } from '../src/page.ts';

const NOW = '2026-11-17T16:00:00Z';
const iso = (s: string) => s.replace(' ', 'T') + 'Z';

export const PERSONAS = { superadmin: 1, admin: 3, member: 5 } as const;
// 'public' is a signed-out visitor: only exposed groups' chats exist.
export type Persona = keyof typeof PERSONAS | 'public';

const PAGE_KEYS = ['dashboard', 'users', 'groups', 'group_messages', 'private_messages', 'files', 'platform_fwd', 'join_chat', 'mcp', 'accounts', 'federations'];

function scopeOf(persona: Persona) {
	if (persona === 'public') {
		const none = () => false;
		return { role: 'public', page: none, group: (g) => !!g.exposed, user: none, fullUser: false, canFetch: false };
	}
	return buildScope(accounts.find((a) => a.id === PERSONAS[persona]));
}

export function viewerOf(persona: Persona): Viewer {
	if (persona === 'public') return { signed_in: false, now: NOW };
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

const chatRef = (c) => ({ id: c.id, title: c.title || c.name, kind: c.kind, photo: null });

function messageRow(m) {
	const s = senderOf(m);
	return {
		id: m.id, scope: m.scope, message_id: m.message_id, date: iso(m.date), chat: chatRef(m.chat),
		sender: m.is_outgoing ? null : userSummary(s, false), content_type: m.content_type,
		snippet: m.content_type === 'service' ? m.text : m.text || m.media?.name || `[${m.content_type}]`,
		forwarded: m.forward?.from ?? null, edited: !!m.edits, deleted_at: m.deleted_at ? iso(m.deleted_at) : null,
	};
}

function chatMedia(md, type) {
	if (!md) return null;
	return {
		stored: md.stored !== false, thumb: null, url: md.stored !== false ? `/files/standin-${md.fileId}` : null,
		w: md.w, h: md.h, duration: md.duration, name: md.name, size: md.size, emoji: md.emoji, wave: md.wave,
	};
}

export function chatMessage(m) {
	const s = senderOf(m);
	return {
		id: m.id, message_id: m.message_id, date: iso(m.date), scope: m.scope,
		sender: m.is_outgoing ? null : userSummary(s, false), content_type: m.content_type, text: m.text || '',
		media: chatMedia(m.media, m.content_type), album: (m.album || []).map((a) => chatMedia({ ...a, w: 4, h: 3, stored: true })),
		reply_to: m.reply_to ? { message_id: m.reply_to.message_id, sender_id: senderOf(m.reply_to)?.id ?? 0, sender_name: senderOf(m.reply_to)?.name ?? 'You', snippet: m.reply_to.text || `[${m.reply_to.content_type}]` } : null,
		forward: m.forward?.from ?? null, edits: (m.edits || []).map((e) => ({ at: iso(e.at), text: e.text })),
		edited_at: m.edited_at ? iso(m.edited_at) : null, deleted_at: m.deleted_at ? iso(m.deleted_at) : null,
		link_preview: m.link_preview ? { site: m.link_preview.site, title: m.link_preview.title, desc: m.link_preview.desc } : null,
	};
}

// A window of a chat: the latest messages, or those around a message
// id or the start of a day, oldest first.
export function chatWindow(chat, anchor: { at?: number; date?: string; before?: number; after?: number }, limit = 60) {
	const all = chatStream(chat);
	let from = Math.max(0, all.length - limit), to = all.length;
	if (anchor.before !== undefined) { to = all.findIndex((m) => m.message_id >= anchor.before); if (to < 0) to = all.length; from = Math.max(0, to - limit); }
	else if (anchor.after !== undefined) { from = all.findIndex((m) => m.message_id > anchor.after); if (from < 0) from = all.length; to = Math.min(all.length, from + limit); }
	else if (anchor.at !== undefined || anchor.date) {
		const i = anchor.at !== undefined ? all.findIndex((m) => m.message_id === anchor.at) : all.findIndex((m) => iso(m.date).slice(0, 10) >= anchor.date);
		if (i >= 0) { from = Math.max(0, i - limit / 2); to = Math.min(all.length, from + limit); }
	}
	return { messages: all.slice(from, to).map(chatMessage), has_older: from > 0, has_newer: to < all.length };
}

export function chatFor(scope: string, id: string, persona: Persona) {
	const s = scopeOf(persona);
	if (scope === 'group') { const g = getGroup(id); return g && s.group(g) ? g : null; }
	if (!allowed(s, 'private_messages')) return null;
	return getUser(id) || null;
}

function fileRow(f) {
	return {
		id: f.id, file_type: f.file_type, name: f.name, ext: f.ext, size: f.size, hits: f.hits, stored: f.stored,
		created_at: iso(f.created_at), duration: f.duration, tg_file_id: f.tg_file_id, url: f.stored ? `/files/standin-${f.id}` : null, thumb: null,
		chat: { id: f.msg.chat.id, title: f.chat_title, kind: f.msg.chat.kind }, sender: f.sender_id ? { id: f.sender_id, name: f.sender_name } : null,
		message: { scope: f.msg.scope, id: f.msg.id },
	};
}

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
	if (persona === 'public' && !/^\/groups\/-?\d+\/chat$/.test(path)) return notFound;
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
	if (path === '/groups') {
		const st = listState(u.searchParams, 'msg_count');
		const r = queryGroups(st, s.role === 'superadmin' ? null : s.group);
		return { route: 'groups', status: 200, props: { ...st, ...r, rows: r.rows.map((g) => ({ ...groupSummary(g), msg_count: g.msg_count, admin_count: g.admin_count, description: g.description, created_at: iso(g.created_at) })), archive_total: s.role === 'member' ? undefined : TOTALS.groups } };
	}
	if ((m = path.match(/^\/groups\/(-\d+)(\/admins|\/history)?$/))) {
		const g = getGroup(m[1]);
		if (!g || !s.group(g)) return notFound;
		return {
			route: m[2] === '/admins' ? 'group_admins' : m[2] === '/history' ? 'group_history' : 'group', status: 200, props: {
				group: { ...groupSummary(g), description: g.description, members: g.members, msg_count: g.msg_count, admin_count: g.admin_count, created_at: iso(g.created_at), updated_at: iso(g.updated_at) },
				admins: getAdmins(g).map((a) => ({ user: userSummary(a.user, false), status: a.status, custom_title: a.custom_title, is_anonymous: a.is_anonymous, perms: a.perms, since: iso(a.since) })),
				history: getGroupHistory(g).map((e) => ({ kind: e.kind, at: iso(e.at), detail: e.detail, action: e.action, photo: null, removed: e.kind === 'photo' ? true : undefined, user: e.user ? userSummary(e.user, false) : undefined })),
				top_posters: getTopPosters(g).map((p) => ({ user: userSummary(p.user, false), msgs: p.msgs })),
				activity: getActivity(-g.id, g.msg_count / 40),
			},
		};
	}
	if (path === '/files') {
		const st = listState(u.searchParams, 'created_at');
		const canSee = (f) => (f.msg.scope === 'group' ? s.group(f.msg.chat) : allowed(s, 'private_messages'));
		const r = queryFiles({ ...st, per: Number(u.searchParams.get('per')) || 50 }, canSee);
		const want = u.searchParams.get('file');
		const pv = want ? files().find((f) => String(f.id) === want && canSee(f)) : null;
		return { route: 'files', status: 200, props: { ...st, ...r, per: Number(u.searchParams.get('per')) || 50, rows: r.rows.map(fileRow), preview: pv ? fileRow(pv) : null, can_fetch: s.canFetch, archive_total: s.role === 'member' ? undefined : TOTALS.files, stored_bytes: s.role === 'member' ? undefined : 5.3 * 2 ** 40 } };
	}
	if (path === '/group_messages' || path === '/private_messages' || path === '/messages/group' || path === '/messages/private') {
		const scope = path.includes('private') ? 'private' : 'group';
		if (!allowed(s, scope === 'group' ? 'group_messages' : 'private_messages')) return notFound;
		const st = listState(u.searchParams, 'date');
		const per = Number(u.searchParams.get('per')) || 50;
		const r = queryMessages(scope, { ...st, per }, scope === 'group' && s.role !== 'superadmin' ? (x) => s.group(x.chat) : null);
		return { route: scope === 'group' ? 'group_messages' : 'private_messages', status: 200, props: { ...st, ...r, per, scope, rows: r.rows.map(messageRow), archive_total: s.role === 'member' ? undefined : scope === 'group' ? TOTALS.group_messages : TOTALS.private_messages } };
	}
	if ((m = path.match(/^\/messages\/(group|private)\/(\d+)$/))) {
		if (!allowed(s, m[1] === 'group' ? 'group_messages' : 'private_messages')) return notFound;
		const x = getMessage(m[1], m[2]);
		if (!x || (m[1] === 'group' && !s.group(x.chat))) return notFound;
		const media = x.media ? { file_id: x.media.fileId, name: x.media.name || '', size: x.media.size || 0, duration: x.media.duration, stored: x.media.stored !== false, thumb: null, url: null } : null;
		return {
			route: 'message', status: 200, props: {
				message: {
					...messageRow(x), text: x.content_type === 'service' ? x.text : x.text,
					reply_to: x.reply_to ? { scope: x.scope, id: x.reply_to.id, sender_name: senderOf(x.reply_to)?.name ?? '', snippet: x.reply_to.text || `[${x.reply_to.content_type}]` } : null,
					edits: (x.edits || []).map((e) => ({ at: iso(e.at), text: e.text })), edited_at: x.edited_at ? iso(x.edited_at) : null, media,
				},
			},
		};
	}
	if ((m = path.match(/^\/(groups|users)\/(-?\d+)\/chat$/))) {
		const kind = m[1] === 'groups' ? 'group' : 'private';
		const c = chatFor(kind, m[2], persona);
		if (!c) return notFound;
		const at = u.searchParams.get('at') ? Number(u.searchParams.get('at')) : undefined;
		const w = chatWindow(c, { at, date: u.searchParams.get('date') || undefined });
		return {
			route: kind === 'group' ? 'group_chat' : 'user_chat', status: 200, props: {
				chat: kind === 'group'
					? { kind: 'group', group: { ...groupSummary(c), members: c.members, msg_count: c.msg_count } }
					: { kind: 'user', user: { id: c.id, name: c.name, photo: null }, count: chatStream(c).length },
				...w, at: at ?? null,
			},
		};
	}
	if (persona === 'public') return notFound;
	return notFound;
}

export function pageData(url: string, persona: Persona): PageData {
	const r = load(url, persona);
	return { v: 1, route: r.route, url, status: r.status, props: r.props, viewer: viewerOf(persona) };
}

export { federations };
