// Fixture page data for the stand-in, built from the mockup's synthetic
// data (web/mockup/src/data). Loaded through Vite, so it can import the
// mockup's modules. Development only: the Go service is the real source.
// @ts-nocheck: the mockup data is untyped JavaScript.
import { users, getUser, getUserHistory, getUsernames, getActivity, queryUsers, TOTAL_USERS } from '../../mockup/src/data/users.js';
import { groups, getGroupsOfUser, queryGroups, getGroup, getAdmins, getGroupHistory, getTopPosters } from '../../mockup/src/data/groups.js';
import { queryFiles, files } from '../../mockup/src/data/files.js';
import { groupMessages, privateMessages, senderOf, queryMessages, getMessage, chatStream, chatOf } from '../../mockup/src/data/messages.js';
import { accounts, federations, mySessions, SECURITY_NOTICES, allFeds, fedGroups, fedAdmins, fedMembers, activeSuperadmins, auditFor, fedById } from '../../mockup/src/data/accounts.js';
import { buildScope, allowed } from '../../mockup/src/viewer.jsx';
import { TOTALS } from '../../mockup/src/data/totals.js';
import { t2d, d2t, mcpExposed, mcpTokens, chatOptions } from '../../mockup/src/data/admin.js';
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
	const pr = settingsState(persona).prefs;
	return {
		timezone: pr.timezone ?? undefined, date_format: pr.date_format ?? undefined, theme: pr.theme ?? undefined,
		rows_per_page: pr.rows_per_page ?? undefined,
		signed_in: true,
		account: { id: s.acct.id, username: s.acct.username, name: s.acct.name, role: s.acct.role },
		now: NOW,
		nav: PAGE_KEYS.filter((k) => allowed(s, k)),
		pinned: settingsState(persona).pinned.map((g) => ({ id: g.id, title: g.title })),
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

const pick = (c) => ({ id: c.id, title: c.title || c.name, kind: c.kind === 'user' ? 'user' : 'group', sub: c.kind === 'user' ? 'private chat' : c.type, photo: null });
const DISCORD_WEBHOOK = /^https:\/\/(?:(?:canary|ptb)\.)?discord(?:app)?\.com\/api(?:\/v\d+)?\/webhooks\/\d+\/[\w-]+$/;
const JOIN_LOG = [
	{ at: '2026-11-17T13:00:00Z', by: 'gw_admin', ref: '@asm_lounge', title: 'Assembly Lounge', outcome: 'joined' },
	{ at: '2026-11-16T14:00:00Z', by: 'admin', ref: 'https://t.me/+AbCdEfGhIjKl', title: 'Homelab & Self-hosting', outcome: 'request_sent' },
	{ at: '2026-11-13T16:00:00Z', by: 'admin', ref: '@some_big_channel', title: 'Some Big Channel', outcome: 'flood_wait' },
];

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
	if (path === '/login') return { route: 'login', status: 200, props: { next: u.searchParams.get('next') } };
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
	if (path.startsWith('/platform-fwd')) {
		if (!allowed(s, 'platform_fwd')) return notFound;
		const vis = (list) => (s.role === 'member' ? list.filter((r) => r.owner === PERSONAS[persona]) : list.filter((r) => (r.chat.kind === 'group' ? s.group(r.chat) : allowed(s, 'private_messages'))));
		if (path === '/platform-fwd') {
			const n = (list) => ({ active: vis(list).filter((r) => r.enabled).length, total: vis(list).length, sent: vis(list).reduce((a, r) => a + r.sent, 0) });
			return { route: 'platform_fwd', status: 200, props: { t2d: n(t2d), d2t: n(d2t) } };
		}
		const owned = (r) => {
			const o = accounts.find((a) => a.id === r.owner);
			const f = fedById(r.fed);
			const cands = s.role === 'superadmin' ? accounts.filter((a) => a.active) : [...fedAdmins(f), ...fedMembers(f)].filter((a) => a.active && (a.role !== 'member' || a.abilities.includes('forward')));
			return { id: r.id, owner: { id: o.id, name: o.name, active: o.active }, federation: { id: f.id, name: f.name }, transfer_candidates: cands.map((a) => ({ id: a.id, name: a.name, role: a.role })) };
		};
		const myFeds = s.role === 'superadmin' ? allFeds() : s.role === 'admin' ? s.manageableFeds : allFeds().filter((f) => s.acct.grants.some((g) => g.fed === f.id));
		const fedList = myFeds.map((f) => ({ id: f.id, name: f.name }));
		if (path === '/platform-fwd/telegram-discord') {
			return { route: 'fwd_telegram_discord', status: 200, props: { federations: fedList, routes: vis(t2d).map((r) => ({ ...owned(r), chat: pick(r.chat), url_masked: r.url.slice(0, r.url.lastIndexOf('/') + 5) + '...', types: r.types, enabled: r.enabled, sent: r.sent, last_sent: r.last ? iso(r.last) : null })) } };
		}
		if (path === '/platform-fwd/discord-telegram') {
			return { route: 'fwd_discord_telegram', status: 200, props: { federations: fedList, routes: vis(d2t).map((r) => ({ ...owned(r), chat: pick(r.chat), channel_id: r.channel, channel_name: r.channel_name, bot: r.bot, types: r.types, enabled: r.enabled, sent: r.sent })) } };
		}
		return notFound;
	}
	if (path === '/join-chat') {
		if (!allowed(s, 'join_chat')) return notFound;
		const feds = s.role === 'superadmin' ? federations : s.manageableFeds.filter((f) => f.id !== 0);
		return { route: 'join_chat', status: 200, props: { federations: feds.map((f) => ({ id: f.id, name: f.name })), recent: JOIN_LOG } };
	}
	if (path.startsWith('/mcp-admin')) {
		if (!allowed(s, 'mcp')) return notFound;
		const ex = mcpExposed.filter((e) => s.group(e.group));
		if (path === '/mcp-admin') {
			const live = mcpTokens.filter((t) => !t.revoked);
			return { route: 'mcp', status: 200, props: { exposed_groups: ex.length, exposed_messages: ex.reduce((a, e) => a + e.group.msg_count, 0), active_tokens: live.length, last_used: live[0]?.last_used ? iso(live[0].last_used) : null, endpoint: 'https://tgd.example.org/mcp' } };
		}
		if (path === '/mcp-admin/groups') {
			return { route: 'mcp_groups', status: 200, props: { groups: ex.map((e) => ({ group: { ...groupSummary(e.group), msg_count: e.group.msg_count }, public_now: e.public_now, note: e.note, added: iso(e.added) })) } };
		}
		if (path === '/mcp-admin/tokens') {
			return { route: 'mcp_tokens', status: 200, props: { tokens: mcpTokens.map((t) => ({ id: t.id, name: t.name, owner: t.owner, created: iso(t.created), last_used: t.last_used ? iso(t.last_used) : null, revoked: t.revoked ? iso(t.revoked) : null })) } };
		}
		return notFound;
	}
	if (path === '/settings/accounts') {
		if (!allowed(s, 'accounts')) return notFound;
		const list = s.role === 'superadmin' ? accounts : accounts.filter((a) => a.role === 'member' && a.grants.some((g) => s.manageableFeds.some((f) => f.id === g.fed)));
		const feds = s.role === 'superadmin' ? allFeds() : s.manageableFeds;
		return {
			route: 'accounts', status: 200, props: {
				accounts: list.map(accountRow),
				federations: feds.map((f) => ({ id: f.id, name: f.name, default: f.id === 0, group_count: fedGroups(f).filter(s.group).length })),
				all_federations: allFeds().map((f) => ({ id: f.id, name: f.name, default: f.id === 0 })),
			},
		};
	}
	if (path === '/settings/federations') {
		if (!allowed(s, 'federations')) return notFound;
		const feds = s.role === 'superadmin' ? allFeds() : s.manageableFeds;
		const person = (a) => ({ id: a.id, username: a.username, name: a.name });
		return {
			route: 'federations', status: 200, props: {
				editable: s.role === 'superadmin',
				federations: feds.map((f) => {
					const gs = fedGroups(f).filter(s.group);
					return { id: f.id, name: f.name, default: f.id === 0, created: f.created ? iso(f.created) : null, groups: gs.slice(0, 12).map(groupSummary), group_count: gs.length, admins: fedAdmins(f).map(person), members: fedMembers(f).map(person) };
				}),
				admin_options: accounts.filter((a) => a.role === 'admin').map(person),
			},
		};
	}
	if (path === '/settings' || path === '/settings/preferences' || path === '/settings/sessions') {
		const st = settingsState(persona);
		const acct = accounts.find((a) => a.id === PERSONAS[persona]);
		if (path === '/settings') {
			return { route: 'settings', status: 200, props: { account: { username: acct.username, name: acct.name, role: acct.role, email: acct.email || null, email_verified: !!acct.verified }, notices: SECURITY_NOTICES } };
		}
		if (path === '/settings/sessions') return { route: 'settings_sessions', status: 200, props: { sessions: st.sessions } };
		return {
			route: 'settings_preferences', status: 200, props: {
				prefs: st.prefs,
				timezones: ['Asia/Jakarta', 'Asia/Makassar', 'Asia/Jayapura', 'Asia/Singapore', 'Europe/Berlin', 'UTC'],
				landing_options: [['/', 'Dashboard'], ['/groups', 'Groups'], ['/group_messages', 'Group messages'], ...st.pinned.map((g) => [`/groups/${g.id}/chat`, `${g.title} chat`])],
				pinned: st.pinned.map((g) => ({ id: g.id, title: g.title, photo: null })),
				saved_searches: st.saved,
			},
		};
	}
	return notFound;
}

// Per-account settings the stand-in keeps in memory (lost on reload of
// this module); the Go service keeps them in the web database.
const SETTINGS = new Map();
export function settingsState(persona: Persona) {
	if (!SETTINGS.has(persona)) {
		const s = scopeOf(persona);
		SETTINGS.set(persona, {
			prefs: {},
			pinned: groups.slice(0, 12).filter(s.group).slice(0, 3),
			saved: [
				{ id: 1, title: 'Deleted messages', url: '/group_messages?f=' + encodeURIComponent(JSON.stringify([{ field: 'is_deleted', op: 'is', value: 'yes' }])) },
				{ id: 2, title: 'Premium users', url: '/users?f=' + encodeURIComponent(JSON.stringify([{ field: 'is_premium', op: 'is', value: 'yes' }])) },
			],
			sessions: mySessions.map((x) => ({ id: x.id, device: x.device, ip: x.ip, place: x.place, started: iso(x.started), last: iso(x.last), current: !!x.current })),
		});
	}
	return SETTINGS.get(persona);
}

export function accountRow(a) {
	return {
		id: a.id, username: a.username, name: a.name, email: a.email || null, email_verified: !!a.verified, invited: !!a.invited,
		role: a.role, active: a.active, last_signin: a.last ? iso(a.last) : null,
		last_superadmin: a.role === 'superadmin' && a.active && activeSuperadmins().length === 1,
		admin_of: a.adminOf || [], restrict: a.restrict ? { hide_private: a.restrict.hide_private, hide_groups: a.restrict.hide_groups.map((id) => pick(getGroup(id))), join_chat: a.restrict.join_chat } : null,
		grants: (a.grants || []).map((g) => ({ fed: g.fed, groups: g.groupIds.map((id) => pick(getGroup(id))) })), abilities: a.abilities || [],
		audit: auditFor(a).map((r) => ({ ...r, at: iso(r.at) })),
	};
}

// Stand-in versions of the proposed /api/accounts and /api/federations.
export function adminApi(method: string, p: string, body, persona: Persona): [number, unknown] {
	const s = scopeOf(persona);
	let m;
	if (p === 'accounts' && method === 'POST') {
		if (!allowed(s, 'accounts')) return [404, { error: 'not_found' }];
		if (!body.username || !body.email) return [400, { error: 'bad_request', message: 'Username and email address are required.' }];
		if (accounts.some((a) => a.username === body.username)) return [409, { error: 'taken', message: 'That username is taken.' }];
		if (s.role !== 'superadmin' && body.role !== 'member') return [403, { error: 'forbidden', message: 'Admins create members only.' }];
		const grants = (body.grants || []).map((g) => ({ fed: g.fed, groupIds: g.group_ids || [] }));
		// An admin's new member must land in a federation that admin manages.
		if (s.role !== 'superadmin' && !grants.some((g) => s.manageableFeds.some((f) => f.id === g.fed))) return [400, { error: 'grants_required', message: 'Grant at least one federation you administer.' }];
		const id = Math.max(...accounts.map((a) => a.id)) + 1;
		accounts.push({ id, username: body.username, name: body.name || body.username, email: body.email, verified: false, invited: true, role: body.role, active: true, last: '', created: '2026-11-17 16:00:00', hue: 200, adminOf: [], restrict: { hide_private: false, hide_groups: [], join_chat: false }, grants, abilities: [] });
		return [201, { id }];
	}
	if ((m = p.match(/^accounts\/(\d+)(?:\/([\w-]+))?$/))) {
		const a = accounts.find((x) => String(x.id) === m[1]);
		if (!a || !allowed(s, 'accounts')) return [404, { error: 'not_found' }];
		if (!m[2] && method === 'PATCH') {
			if (a.role === 'superadmin' && body.role && body.role !== 'superadmin' && activeSuperadmins().length === 1) return [409, { error: 'last_superadmin', message: 'The last active superadmin cannot be demoted.' }];
			if (body.name) a.name = body.name;
			if (body.role) a.role = body.role;
			if (body.email !== undefined) { a.email = body.email; a.verified = false; }
			if (body.admin_of) a.adminOf = body.admin_of;
			if (body.restrict) a.restrict = body.restrict;
			if (body.grants) a.grants = body.grants.map((g) => ({ fed: g.fed, groupIds: g.group_ids }));
			if (body.abilities) a.abilities = body.abilities;
			return [200, accountRow(a)];
		}
		if (m[2] === 'disable') {
			if (a.role === 'superadmin' && activeSuperadmins().length === 1) return [409, { error: 'last_superadmin', message: 'The last active superadmin cannot be disabled.' }];
			a.active = false; return [204, null];
		}
		if (m[2] === 'enable') { a.active = true; return [204, null]; }
		if (['password-reset', 'resend-invite', 'sign-out-everywhere'].includes(m[2]) && method === 'POST') return [204, null];
	}
	// Forwarding: /api/forwarding/<telegram-discord|discord-telegram>[/<id>[/transfer]|/test]
	if ((m = p.match(/^forwarding\/(telegram-discord|discord-telegram)(?:\/(\d+|test))?(?:\/(transfer))?$/))) {
		if (!allowed(s, 'platform_fwd')) return [404, { error: 'not_found' }];
		const list = m[1] === 'telegram-discord' ? t2d : d2t;
		const vis = (l) => (s.role === 'member' ? l.filter((r) => r.owner === PERSONAS[persona]) : l);
		// The server POSTs to a webhook URL it is given, so a raw URL must be
		// a Discord webhook, on test as on save.
		const badHook = (u) => u !== undefined && m[1] === 'telegram-discord' && !DISCORD_WEBHOOK.test(String(u));
		if (badHook(body.webhook_url)) return [400, { error: 'bad_webhook', message: 'That is not a Discord webhook URL.' }];
		if (m[2] === 'test') {
			if (body.id !== undefined && !vis(list).some((r) => r.id === body.id)) return [404, { error: 'not_found' }];
			return [204, null];
		}
		if (!m[2] && method === 'POST') {
			const chat = body.chat_kind === 'user' ? chatOptions.find((c) => c.id === body.chat_id && c.kind === 'user') : getGroup(body.chat_id);
			if (!chat || (chat.kind === 'group' && !s.group(chat))) return [400, { error: 'bad_request', message: 'Choose a chat.' }];
			const id = Math.max(0, ...list.map((r) => r.id)) + 1;
			const fed = body.federation_id ?? 0;
			list.push(m[1] === 'telegram-discord'
				? { id, owner: PERSONAS[persona], fed, chat, url: body.webhook_url || '', types: body.types, enabled: body.enabled, last: null, sent: 0 }
				: { id, owner: PERSONAS[persona], fed, channel: body.discord_channel_id, channel_name: '#new-channel', chat, bot: '@standin_bot', types: body.types, enabled: body.enabled, sent: 0 });
			return [201, { id }];
		}
		const r = list.find((x) => String(x.id) === m[2]);
		if (!r || (s.role === 'member' && r.owner !== PERSONAS[persona])) return [404, { error: 'not_found' }];
		if (m[3] === 'transfer') {
			if (s.role === 'member') return [404, { error: 'not_found' }];
			r.owner = body.owner_id; return [204, null];
		}
		if (method === 'PATCH') { Object.assign(r, { ...(body.types ? { types: body.types } : {}), ...(body.enabled !== undefined ? { enabled: body.enabled } : {}) }); return [204, null]; }
		if (method === 'DELETE') { list.splice(list.indexOf(r), 1); return [204, null]; }
	}
	if (p === 'join-chat/preview' || p === 'join-chat/join') {
		if (!allowed(s, 'join_chat')) return [404, { error: 'not_found' }];
		const ref = String(body.ref || '').trim();
		if (!ref) return [400, { error: 'bad_request', message: 'Enter an invite link or a username.' }];
		const chat = { chat_id: -1001999000001, title: 'Compiler Nerds ID', type: 'supergroup', member_count: 1843, username: ref.replace(/^@|^https?:\/\/t\.me\//, ''), is_member: false, needs_approval: false };
		if (p === 'join-chat/preview') return [200, { outcome: 'ok', message: '', chat }];
		JOIN_LOG.unshift({ at: NOW, by: s.acct.username, ref, title: chat.title, outcome: 'joined' });
		return [200, { outcome: 'joined', message: `Joined. The chat is now logged, media included${(body.federation_ids || []).length ? ', and added to the chosen federations' : ''}.`, chat: { ...chat, is_member: true } }];
	}
	if ((m = p.match(/^mcp\/groups(?:\/(-?\d+))?$/))) {
		if (!allowed(s, 'mcp')) return [404, { error: 'not_found' }];
		if (!m[1] && method === 'POST') {
			const g = groups.find((x) => x.id === body.group_id);
			if (!g) return [400, { error: 'bad_request', message: 'Choose a group.' }];
			g.exposed = true; mcpExposed.push({ group: g, public_now: true, note: body.note || '', added: '2026-11-17 16:00:00' });
			return [201, { group_id: g.id }];
		}
		const i = mcpExposed.findIndex((e) => String(e.group.id) === m[1]);
		if (i < 0) return [404, { error: 'not_found' }];
		mcpExposed[i].group.exposed = false; mcpExposed.splice(i, 1); return [204, null];
	}
	if ((m = p.match(/^mcp\/tokens(?:\/(\d+)\/(revoke))?$/))) {
		if (!allowed(s, 'mcp')) return [404, { error: 'not_found' }];
		if (!m[1] && method === 'POST') {
			const id = Math.max(...mcpTokens.map((t) => t.id)) + 1;
			mcpTokens.push({ id, name: body.name || '', owner: s.acct.username, created: '2026-11-17 16:00:00', last_used: '', revoked: null });
			return [201, { id, token: 'tgmcp_standin_' + id + '_shown_once' }];
		}
		const t = mcpTokens.find((x) => String(x.id) === m[1]);
		if (!t) return [404, { error: 'not_found' }];
		t.revoked = '2026-11-17 16:00:00'; return [204, null];
	}
	if (p === 'federations' && method === 'POST') {
		if (s.role !== 'superadmin') return [404, { error: 'not_found' }];
		const id = Math.max(...federations.map((f) => f.id)) + 1;
		federations.push({ id, name: body.name, groupIds: body.group_ids || [], created: '2026-11-17 16:00:00' });
		for (const a of accounts) if ((body.admin_ids || []).includes(a.id) && a.role === 'admin') a.adminOf.push(id);
		return [201, { id }];
	}
	if ((m = p.match(/^federations\/(\d+)$/)) && method === 'PATCH') {
		if (s.role !== 'superadmin') return [404, { error: 'not_found' }];
		const f = fedById(Number(m[1]));
		if (!f) return [404, { error: 'not_found' }];
		if (f.id !== 0) { if (body.name) f.name = body.name; if (body.group_ids) f.groupIds = body.group_ids; }
		for (const a of accounts.filter((x) => x.role === 'admin')) {
			const want = (body.admin_ids || []).includes(a.id);
			a.adminOf = want ? [...new Set([...a.adminOf, f.id])] : a.adminOf.filter((x) => x !== f.id);
		}
		return [200, { id: f.id }];
	}
	return [404, { error: 'not_found' }];
}

export function pageData(url: string, persona: Persona): PageData {
	const r = load(url, persona);
	return { v: 1, route: r.route, url, status: r.status, props: r.props, viewer: viewerOf(persona) };
}

export { federations };

// Stand-in for GET /api/chats/search: the viewer's chats, busiest first,
// matched on title or username, paged by an offset cursor.
export function searchChats(persona: Persona, q: URLSearchParams): [number, unknown] {
	const s = scopeOf(persona);
	if (s.role === 'public') return [401, { error: 'unauthenticated' }];
	const kind = q.get('kind') || 'any';
	const text = (q.get('q') || '').toLowerCase();
	const fed = q.get('federation');
	const limit = Math.min(200, Math.max(1, Number(q.get('limit')) || 20));
	const off = Number(q.get('cursor')) || 0;
	let pool = kind === 'user' ? [] : (fed !== null ? (fedById(Number(fed)) ? fedGroups(fedById(Number(fed))) : []) : groups).filter(s.group);
	if (kind !== 'group' && fed === null && s.role !== 'member' && allowed(s, 'private_messages')) pool = [...pool, ...chatOptions.filter((c) => c.kind === 'user')];
	const hits = pool
		.filter((c) => !text || (c.title || c.name || '').toLowerCase().includes(text) || (c.username || '').toLowerCase().includes(text))
		.sort((x, y) => (y.msg_count || 0) - (x.msg_count || 0));
	const page = hits.slice(off, off + limit);
	return [200, { items: page.map(pick), next_cursor: off + limit < hits.length ? String(off + limit) : null }];
}

// Stand-in changes behind the live event stream: each makes a change in
// the fixtures and returns the event the daemon would emit for it.
const chatById = (id: number) => (id < 0 ? getGroup(id) : getUser(id));
// Fixture dates are 'YYYY-MM-DD HH:MM:SS' (UTC), as the mockup data has them.
const stamp = (ms: number) => new Date(ms).toISOString().slice(0, 19).replace('T', ' ');
const parseStamp = (s: string) => Date.parse(s.replace(' ', 'T') + 'Z');

export function liveChange(kind: string, b: { chat_id?: number; message_id?: number; user_id?: number }): Record<string, unknown> | null {
	if (kind === 'message.new') {
		const chat = chatById(Number(b.chat_id));
		if (!chat) return null;
		const all = chatStream(chat);
		const prev = all[all.length - 1];
		const n = all.length;
		const date = stamp(parseStamp(prev.date) + 30000);
		const m = { ...prev, id: prev.id + 100000 + n, message_id: prev.message_id + 1, date, text: `Live message ${n + 1} from the stand-in`, content_type: 'text', media: null, album: [], reply_to: null, forward: null, edits: null, edited_at: null, deleted_at: null, link_preview: null };
		all.push(m);
		// The message search lists are built once; put it on top of its list.
		const list = chat.kind === 'user' ? privateMessages() : groupMessages();
		if (!list.includes(m) && list.some((x) => x.chat === chat)) list.unshift(Object.assign(m, { is_deleted: false, is_edited: false, is_forwarded: false }));
		return { kind, chat_id: chat.id, message_id: m.message_id };
	}
	if (kind === 'message.edited' || kind === 'message.deleted') {
		const chat = chatById(Number(b.chat_id));
		const m = chat && chatStream(chat).find((x) => x.message_id === Number(b.message_id));
		if (!m) return null;
		if (kind === 'message.edited') { m.edits = [...(m.edits || []), { at: m.edited_at || m.date, text: m.text }]; m.text = `${m.text} (edited live)`; m.edited_at = stamp(parseStamp(NOW.replace('T', ' ').slice(0, 19))); }
		else m.deleted_at = stamp(parseStamp(NOW.replace('T', ' ').slice(0, 19)));
		return { kind, chat_id: chat.id, message_id: m.message_id };
	}
	if (kind === 'user.updated') {
		const u = getUser(Number(b.user_id));
		if (!u) return null;
		u.name = u.name.replace(/ \(renamed \d+\)$/, '') + ` (renamed ${Math.floor(Math.random() * 900 + 100)})`;
		return { kind, user_id: u.id, fields: ['name'] };
	}
	if (kind === 'group.updated') {
		const g = getGroup(Number(b.chat_id));
		if (!g) return null;
		g.title = g.title.replace(/ \(renamed \d+\)$/, '') + ` (renamed ${Math.floor(Math.random() * 900 + 100)})`;
		return { kind, chat_id: g.id, fields: ['title'] };
	}
	if (kind === 'stats.changed') return { kind, chat_ids: b.chat_id ? [b.chat_id] : [], user_ids: b.user_id ? [b.user_id] : [] };
	return null;
}

// Whether a persona may see an event (web-live.md section 3).
export function liveVisible(persona: Persona, e: { kind: string; chat_id?: number; user_id?: number }): boolean {
	const s = scopeOf(persona);
	if (s.role === 'public') return false;
	if (e.chat_id !== undefined) {
		if (e.chat_id < 0) { const g = getGroup(e.chat_id); return !!g && s.group(g); }
		return allowed(s, 'private_messages');
	}
	if (e.user_id !== undefined) { const u = getUser(e.user_id); return !!u && s.user(u); }
	return true;
}

// A group the persona may see, for the pin endpoint.
export function groupFor(persona: Persona, id: string) {
	const g = getGroup(id);
	return g && scopeOf(persona).group(g) ? g : null;
}
