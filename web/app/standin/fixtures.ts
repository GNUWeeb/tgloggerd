// Fixture page data for the stand-in, built from the mockup's synthetic
// data (web/mockup/src/data). Loaded through Vite, so it can import the
// mockup's modules. Development only: the Go service is the real source.
// @ts-nocheck: the mockup data is untyped JavaScript.
import { users, getUser, getUserHistory, getUsernames, getActivity, queryUsers, TOTAL_USERS } from '../../mockup/src/data/users.js';
import { groups, getGroupsOfUser, queryGroups, getGroup, getAdmins, getGroupHistory, getTopPosters } from '../../mockup/src/data/groups.js';
import { files } from '../../mockup/src/data/files.js';
import { groupMessages, privateMessages, senderOf, queryMessages, getMessage, chatStream, chatOf } from '../../mockup/src/data/messages.js';
import { accounts, federations, mySessions, SECURITY_NOTICES, allFeds, fedGroups, fedAdmins, fedMembers, activeSuperadmins, auditFor, fedById } from '../../mockup/src/data/accounts.js';
import { buildScope, allowed } from '../../mockup/src/data/scope.js';
import { TOTALS } from '../../mockup/src/data/totals.js';
import { t2d, d2t, mcpExposed, mcpTokens, chatOptions } from '../../mockup/src/data/admin.js';
import { createHash, randomBytes } from 'node:crypto';
import { REGISTRY, SLOW, compile, fieldsOf, legacyQuery, type ListError } from './search.ts';
import { queryRows } from '../../mockup/src/data/seed.js';
import type { PageData, Viewer } from '../src/page.ts';

const NOW = '2026-11-17T16:00:00Z';
const iso = (s: string) => s.replace(' ', 'T') + 'Z';

// noemail is the sample superadmin "ops", which has no email address:
// email is optional, and nothing asks for one.
// newpw is the sample member "mod_kernel", which signed in with a
// temporary password and must choose its own first.
export const PERSONAS = { superadmin: 1, admin: 3, member: 5, noemail: 2, newpw: 6 } as const;
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
		sidebar: pr.sidebar ?? 'open',
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

// A row of the users list (UserRow): the Drogon list's other columns go
// to full viewers only.
function userRow(u, full: boolean) {
	if (!full) return userSummary(u, false);
	return {
		...userSummary(u, true), phone: u.phone || '', bio: u.bio || '', restriction_reason: u.is_scam ? 'Reported as a scam by many users.' : '',
		has_sensitive_content: false, restricts_new_chats: Number(String(u.id).slice(-4)) % 7 === 0, paid_message_star_count: 0, personal_chat_id: null,
		emoji_status: u.is_premium ? String(5368324170671202286n + BigInt(Number(String(u.id).slice(-4)))) : null,
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

// A message's media as Go sends it (ChatMedia): url when the bytes are
// stored or, for a viewer who may fetch files, fetchable from Telegram;
// thumb with the same URL for a picture. The sample data is varied by
// file id: a third of the stickers each are webp, webm and tgs, every
// fourth video is an animation, and some album items are videos or
// deleted.
const STICKER_FORMATS = ['webp', 'webm', 'tgs'];
// How each content type renders (MediaDetail.render); stickers by format.
const RENDER = { photo: 'image', video: 'video', animation: 'animation', voice: 'audio', audio: 'audio', document: 'file' };
const EXT_OF = { photo: 'jpg', video: 'mp4', animation: 'mp4', voice: 'ogg', audio: 'mp3', document: 'bin' };
export function chatMedia(md, type, fetch = false, extra = {}) {
	if (!md) return null;
	const stored = md.stored !== false;
	const fetchable = !stored && fetch;
	const format = type === 'sticker' ? STICKER_FORMATS[md.fileId % 3] : undefined;
	// A Lottie sticker's URL says so, for the stand-in's /files to answer
	// with an animation instead of a picture.
	const url = stored || fetchable ? `/files/standin-${md.fileId}${format === 'tgs' ? '.tgs' : ''}` : null;
	const render = RENDER[type] ?? (format === 'tgs' ? 'lottie' : format === 'webm' ? 'animation' : 'image');
	const picture = render === 'image';
	return {
		stored, fetchable, thumb: picture ? url : null, url, render, ext: format ?? md.ext ?? EXT_OF[type],
		w: md.w, h: md.h, duration: md.duration, name: md.name, size: md.size, emoji: md.emoji,
		// Telegram's voice waveform is 5-bit: 0 to 31 per bar.
		wave: md.wave?.map((x) => Math.round(x * 31)),
		...(format ? { format } : {}), ...extra,
	};
}

const typeOf = (m) => (m.content_type === 'video' && m.media && m.media.fileId % 4 === 0 ? 'animation' : m.content_type);

// An album item's own message id: in Telegram the next ids after the
// album's first, but the sample messages leave no room there, so the
// stand-in gives them a range of their own.
function albumItem(a, fetch) {
	const kind = a.fileId % 5 === 0 ? 'video' : 'photo';
	const extra = { kind, message_id: 10_000_000 + a.fileId, ...(a.fileId % 7 === 0 ? { deleted_at: iso('2026-11-16 10:00:00') } : {}) };
	return chatMedia({ ...a, w: 4, h: 3, stored: a.fileId % 6 !== 0, ...(kind === 'video' ? { duration: 12 + (a.fileId % 50) } : {}) }, kind, fetch, extra);
}

// Where a sample forward came from (ForwardOrigin): "Someone" hid its
// sender; the bots and news feeds are channels, each a sample group, the
// original post's id with it.
const FORWARD_CHANNELS = ['Linux Weekly News', 'Rust Indonesia', 'Hacker News Bot'];
function forwardFrom(f, m) {
	if (f.from === 'Someone') return { type: 'hidden_user', name: f.from };
	const i = FORWARD_CHANNELS.indexOf(f.from);
	if (i < 0) return { type: 'user', name: f.from };
	const g = groups[i];
	return { type: 'channel', name: f.from, chat_id: g.id, message_id: 1000 + (m.message_id % 5000), date: iso(m.date) };
}

// How a service message without text says what happened.
const SERVICE_TYPES = { 'joined the group': 'chat_join_by_link', 'left the group': 'chat_delete_member', 'pinned a message': 'pin_message', 'changed the group photo': 'chat_change_photo', 'was added by an admin': 'chat_add_members' };

export function chatMessage(m, fetch = false) {
	const s = senderOf(m);
	const ct = typeOf(m);
	// A channel posts as itself; some posts carry their author's name.
	const channel = m.chat.kind !== 'user' && m.chat.type === 'channel';
	const sender = m.is_outgoing ? null : channel ? { id: m.chat.id, name: m.chat.title, username: m.chat.username, type: 'regular', photo: null } : userSummary(s, false);
	// Every third service message has only its type, as rows logged
	// without the rendered text do.
	const bare = ct === 'service' && m.message_id % 3 === 0;
	// A few replies point at a message the archive never logged, and a
	// few at one in another sample chat.
	const r = m.reply_to;
	const reply = !r ? null : m.message_id % 11 === 0
		? { message_id: r.message_id, sender_id: 0, sender_name: '', snippet: '', resolved: false }
		: {
			message_id: r.message_id, sender_id: senderOf(r)?.id ?? 0, sender_name: senderOf(r)?.name ?? 'You', snippet: r.text || `[${r.content_type}]`, resolved: true,
			...(r.deleted_at ? { deleted: true } : {}),
			...(m.message_id % 13 === 0 && m.chat.kind !== 'user' ? { chat_id: groups.find((g) => g.id !== m.chat.id)!.id } : {}),
		};
	const lp = m.link_preview;
	return {
		id: m.id, message_id: m.message_id, date: iso(m.date), scope: m.scope,
		sender, content_type: ct, text: bare ? '' : m.text || '',
		media: chatMedia(m.media, ct, fetch, m.album ? { kind: ct, message_id: m.message_id } : {}), album: (m.album || []).map((a) => albumItem(a, fetch)),
		reply_to: reply,
		// Each version says its type; a media message's first version had
		// another file (the media was replaced in the edit).
		forward: m.forward?.from ?? null, edits: (m.edits || []).map((e, i) => ({
			at: iso(e.at), text: e.text, content_type: ct,
			...(i === 0 && m.media ? { media: chatMedia({ ...m.media, fileId: m.media.fileId + 500000 }, ct, fetch) } : {}),
		})),
		edited_at: m.edited_at ? iso(m.edited_at) : null, deleted_at: m.deleted_at ? iso(m.deleted_at) : null,
		link_preview: lp ? {
			site: lp.site, title: lp.title, desc: lp.desc,
			url: lp.site === 'GitHub' ? 'https://github.com/torvalds/linux/commit/0123456789ab' : 'https://docs.kernel.org/',
			...(lp.site === 'GitHub' ? { media: chatMedia({ fileId: 990000 + (m.message_id % 1000), w: 1200, h: 600, size: 52000 }, 'photo', fetch) } : {}),
		} : null,
		...(m.forward ? { forward_from: forwardFrom(m.forward, m) } : {}),
		...(channel ? { sender_chat: groupSummary(m.chat), is_channel_post: true, ...(m.message_id % 3 === 0 && s ? { author_signature: s.name } : {}) } : {}),
		...(bare ? { service_type: SERVICE_TYPES[m.text] ?? 'custom' } : {}),
	};
}

type Anchor = { at?: number; date?: string; before?: number; after?: number; after_ts?: number };

// A message's position in the sample stream for an anchor, -1 if none.
// The stand-in reads ?date= as UTC; Go reads it in the viewer's zone.
function anchorIndex(all, a: Anchor): number {
	if (a.at !== undefined) return all.findIndex((m) => m.message_id === a.at);
	if (a.after_ts !== undefined) return all.findIndex((m) => Date.parse(iso(m.date)) / 1000 >= a.after_ts);
	if (a.date) {
		const d = a.date.replace(' ', 'T');
		return all.findIndex((m) => iso(m.date).slice(0, d.length) >= d);
	}
	return -1;
}

// A window of a chat, oldest first: before or after a message id
// (after=0: the oldest), around a message, a time or a day, else the
// newest messages. found is the message opened at.
function windowOf(chat, anchor: Anchor, limit: number, fetch: boolean) {
	const all = chatStream(chat);
	let from = Math.max(0, all.length - limit), to = all.length;
	let found: number | null = null;
	if (anchor.before !== undefined) { to = all.findIndex((m) => m.message_id >= anchor.before); if (to < 0) to = all.length; from = Math.max(0, to - limit); }
	else if (anchor.after !== undefined) {
		const i = all.findIndex((m) => m.message_id > anchor.after);
		if (i >= 0) { from = i; to = Math.min(all.length, from + limit); found = all[i].message_id; }
		else { from = all.length; to = all.length; }
	} else {
		const i = anchorIndex(all, anchor);
		if (i >= 0) { from = Math.max(0, i - limit / 2); to = Math.min(all.length, from + limit); found = all[i].message_id; }
	}
	return { w: { messages: all.slice(from, to).map((m) => chatMessage(m, fetch)), has_older: from > 0, has_newer: to < all.length }, found };
}

export function chatWindow(chat, anchor: Anchor, limit = 60, fetch = false) {
	return windowOf(chat, anchor, limit, fetch).w;
}

// The chat page's first window, by the first anchor of its query that
// parses (ChatProps): at, after_ts, date, after; past the end, the
// newest messages.
function chatPageWindow(chat, q: URLSearchParams, fetch: boolean) {
	const n = (k: string) => (q.get(k) !== null && /^\d+$/.test(q.get(k)!) ? Number(q.get(k)) : undefined);
	const limit = Math.min(100, Math.max(1, n('limit') ?? 60));
	const date = q.get('date') && /^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2})?)?$/.test(q.get('date')!) ? q.get('date')! : undefined;
	const anchors: Anchor[] = [{ at: n('at') }, { after_ts: n('after_ts') }, { date }, { after: n('after') }].filter((a) => Object.values(a)[0] !== undefined);
	for (const a of anchors) {
		const r = windowOf(chat, a, limit, fetch);
		if (r.found !== null) return r;
	}
	return windowOf(chat, {}, limit, fetch);
}

export const canFetch = (persona: Persona) => !!scopeOf(persona).canFetch;

export function chatFor(scope: string, id: string, persona: Persona) {
	const s = scopeOf(persona);
	if (scope === 'group') { const g = getGroup(id); return g && s.group(g) ? g : null; }
	if (!allowed(s, 'private_messages')) return null;
	return getUser(id) || null;
}

// Some stored files belong to no message the viewer can see (the spec's
// FileRow: chat and message null together). The stand-in models a fixed
// few that way; members, who see only their groups' files, never get
// them.
const orphan = (f) => f.id % 23 === 0;

// The sample files with what the mockup data lacks: a SHA-256 for
// stored bytes, the last access and the last update (a few days after
// first seen, by id), and the odd file of unknown type.
const DAY_MS = 86400000;
const shift = (at: string, days: number) => new Date(Math.min(Date.parse(iso(at)) + days * DAY_MS, Date.parse(NOW))).toISOString().replace('T', ' ').slice(0, 19);
let fileData = null;
function allFiles() {
	fileData ??= files().map((f) => ({
		...f,
		file_type: f.id % 97 === 0 ? 'unknown' : f.file_type,
		sha256: f.stored ? createHash('sha256').update(`standin-file-${f.id}`).digest('hex') : null,
		updated_at: shift(f.created_at, (f.id % 5) / 2),
		accessed_at: shift(f.created_at, f.id % 9),
	}));
	return fileData;
}
const pictureType = (t: string) => t === 'photo' || t === 'sticker';

// A file as Go sends it (FileRow): url when the bytes are stored or the
// viewer may have them fetched from Telegram; thumb only for a stored
// photo or sticker.
function fileRow(f, fetch: boolean) {
	const fetchable = !f.stored && fetch;
	const url = f.stored || fetchable ? `/files/standin-${f.id}` : null;
	const base = {
		id: f.id, file_type: f.file_type, name: f.name, ext: f.ext, size: f.size, hits: f.hits, stored: f.stored, fetchable,
		created_at: iso(f.created_at), accessed_at: iso(f.accessed_at), duration: f.duration, tg_file_id: f.tg_file_id, sha256: f.sha256,
		url, thumb: f.stored && pictureType(f.file_type) ? url : null,
	};
	if (orphan(f)) return { ...base, chat: null, sender: null, message: null };
	return {
		...base,
		chat: { id: f.msg.chat.id, title: f.chat_title, kind: f.msg.chat.kind }, sender: f.sender_id ? { id: f.sender_id, name: f.sender_name } : null,
		message: { scope: f.msg.scope, id: f.msg.id },
	};
}

const q = (u: URL, k: string) => u.searchParams.get(k);

type Loaded = { route: string; status: 200 | 404; props: unknown; url?: string };
const LISTS = ['/users', '/groups', '/group_messages', '/private_messages', '/files'];
const notFound: Loaded = { route: 'not_found', status: 404, props: {} };

// A list's state from its query: per defaults to the viewer's rows per
// page (else the list's own default) and goes up to 1000.
function listState(q: URLSearchParams, defSort: string, defPer = 25, persona?: Persona) {
	let conds = [];
	try { conds = q.get('f') ? JSON.parse(q.get('f')) : []; } catch { conds = []; }
	const pref = persona && persona !== 'public' ? settingsState(persona).prefs.rows_per_page : null;
	return {
		q: q.get('q') || '', conds, sort: q.get('sort') || defSort, dir: q.get('dir') || 'desc',
		page: Number(q.get('page')) || 1, per: Math.min(1000, Math.max(1, Number(q.get('per')) || pref || defPer)),
	};
}

// A list with the advanced search: its own filter keys (the rows' own
// keys) and its registry's fields, as one chain. A refused search, or one
// containing the sample word for a slow one, answers the plain list with
// the reason; q and conds are then empty, as applied.
function searchList(st, registry: string, rows: Record<string, unknown>[], keep: ((x) => boolean) | null, query: (st, keep) => { rows: unknown[]; total: number; page: number; pages: number }, estimate = false) {
	const fields = REGISTRY[registry]!;
	let error: ListError | null = null;
	let test: ((r) => boolean) | null = null;
	if (st.q.includes(SLOW)) error = { code: 'search_timeout', message: 'The search ran out of its time limit.' };
	else {
		const t = compile(st.conds, Object.keys(rows[0] ?? {}), fields);
		if (typeof t === 'function') test = t; else error = t;
	}
	const applied = error ? { ...st, q: '', conds: [] } : st;
	const r = query({ ...applied, conds: [] }, (x) => (!keep || keep(x)) && (!test || test(x)));
	// Without a search the biggest list counts by the table's estimate.
	const exact = !(estimate && !applied.q && !applied.conds.length);
	return { st: applied, r: exact ? r : { ...r, total: Math.round(r.total / 100) * 100 || r.total }, extra: { total_exact: exact, fields: fieldsOf(fields), ...(error ? { error } : {}) } };
}

// Path -> route and props, with the viewer's scope applied: hidden things
// answer exactly like missing ones.
export function load(url: string, persona: Persona): Loaded {
	const u = new URL(url, 'http://x');
	const path = u.pathname.replace(/\/+$/, '') || '/';
	// A Drogon list URL is read as the same list; the page names the
	// translated URL, for the address bar.
	if (LISTS.includes(path)) {
		const lq = legacyQuery(u.searchParams);
		if (lq) {
			u.search = lq.toString() ? '?' + lq.toString() : '';
			const r = load(u.pathname + u.search, persona);
			return { ...r, url: u.pathname + u.search };
		}
	}
	const s = scopeOf(persona);
	const full = s.fullUser;
	let m;
	if (path === '/login') return { route: 'login', status: 200, props: { next: u.searchParams.get('next') } };
	if (path === '/password/forgot') return { route: 'password_forgot', status: 200, props: {} };
	if (path === '/password/set') return { route: 'password_set', status: 200, props: {} };
	if (path === '/account/email/verify') return { route: 'email_verify', status: 200, props: {} };
	const acct = persona === 'public' ? null : accounts.find((a) => a.id === PERSONAS[persona]);
	if (acct && mustChange.has(acct.id)) return { route: 'password_change', status: 200, props: { username: acct.username, next: path + u.search } };
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
		const { st, r, extra } = searchList(listState(u.searchParams, full ? 'msg_count' : 'name', 25, persona), full ? 'users' : 'users_member', users, s.role === 'member' ? s.user : null, queryUsers);
		return { route: 'users', status: 200, props: { ...st, ...r, ...extra, rows: r.rows.map((x) => userRow(x, full)), archive_total: full ? TOTAL_USERS : undefined } };
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
				limited: false,
				user: {
					...userSummary(usr, true), first_name: usr.first_name, last_name: usr.last_name, bio: usr.bio, phone: usr.phone,
					usernames: getUsernames(usr).map((n) => ({ username: n.username, active: n.kind === 'active', kind: n.kind === 'active' ? 'active' : 'disabled', collectible: /\d$/.test(n.username) })),
					...contactFields(usr), ...profileExtras(usr, s),
				},
				groups: getGroupsOfUser(usr).filter(s.group).map((g) => ({ ...groupSummary(g), msgs: g.msgs })),
				...(() => { const h = historyPage(userHistory(usr), q(u, 'before'), HISTORY_FIRST) ?? historyPage(userHistory(usr), null, HISTORY_FIRST)!; return { history: h.entries, history_next: h.next }; })(),
				activity: getActivity(usr.id, usr.msg_count),
				can_private: allowed(s, 'private_messages'),
				can_lookup_name: !!s.canLookupName && !!usr.is_contact, lookup_pending: lookupPending.has(usr.id),
				lookup_pending_since: lookupPending.has(usr.id) ? '2026-11-17T09:12:00Z' : null,
				lookup_pending_error: lookupPending.has(usr.id) ? 'FLOOD_WAIT_120' : null,
			},
		};
	}
	if (path === '/groups') {
		const { st, r, extra } = searchList(listState(u.searchParams, 'msg_count', 25, persona), 'groups', groups, s.role === 'superadmin' ? null : s.group, queryGroups);
		return { route: 'groups', status: 200, props: { ...st, ...r, ...extra, rows: r.rows.map((g) => ({ ...groupSummary(g), msg_count: g.msg_count, admin_count: g.admin_count, description: g.description, created_at: iso(g.created_at), updated_at: iso(g.updated_at) })), archive_total: s.role === 'member' ? undefined : TOTALS.groups } };
	}
	if ((m = path.match(/^\/groups\/(-\d+)(\/admins|\/history)?$/))) {
		const g = getGroup(m[1]);
		if (!g || !s.group(g)) return notFound;
		return {
			route: m[2] === '/admins' ? 'group_admins' : m[2] === '/history' ? 'group_history' : 'group', status: 200, props: {
				group: {
					...groupSummary(g), description: g.description, members: g.members, msg_count: g.msg_count, admin_count: g.admin_count, created_at: iso(g.created_at), updated_at: iso(g.updated_at),
					usernames: g.username ? [{ username: g.username, active: true, kind: 'active', collectible: false }] : [], photo_file_id: null,
				},
				admins: getAdmins(g).map((a) => ({ user: userSummary(a.user, false), status: a.status, custom_title: a.custom_title, is_anonymous: a.is_anonymous, perms: a.perms, since: iso(a.since) })),
				...(() => { const h = historyPage(groupHistory(g), q(u, 'before'), HISTORY_FIRST) ?? historyPage(groupHistory(g), null, HISTORY_FIRST)!; return { history: h.entries, history_next: h.next }; })(),
				top_posters: getTopPosters(g).map((p) => ({ user: userSummary(p.user, false), msgs: p.msgs })),
				activity: getActivity(-g.id, g.msg_count / 40),
			},
		};
	}
	if (path === '/files') {
		const st = listState(u.searchParams, 'created_at');
		const canSee = (f) => (orphan(f) ? s.role !== 'member' : f.msg.scope === 'group' ? s.group(f.msg.chat) : allowed(s, 'private_messages'));
		// A digest matches in either case, as in Go.
		const conds = st.conds.map((c) => (c.field === 'sha256' ? { ...c, value: String(c.value).toLowerCase() } : c));
		const r = queryRows(allFiles().filter(canSee), { ...st, conds, per: Number(u.searchParams.get('per')) || 50 }, (f) => `${f.name} ${f.id}`);
		const want = u.searchParams.get('file');
		const pv = want ? allFiles().find((f) => String(f.id) === want && canSee(f)) : null;
		return { route: 'files', status: 200, props: { ...st, ...r, total_exact: true, per: Number(u.searchParams.get('per')) || 50, rows: r.rows.map((f) => fileRow(f, s.canFetch)), preview: pv ? fileRow(pv, s.canFetch) : null, can_fetch: s.canFetch, archive_total: s.role === 'member' ? undefined : TOTALS.files, stored_bytes: s.role === 'member' ? undefined : Math.round(5.3 * 2 ** 40) } };
	}
	if (path === '/group_messages' || path === '/private_messages' || path === '/messages/group' || path === '/messages/private') {
		const scope = path.includes('private') ? 'private' : 'group';
		if (!allowed(s, scope === 'group' ? 'group_messages' : 'private_messages')) return notFound;
		const all = scope === 'group' ? groupMessages() : privateMessages();
		const { st, r, extra } = searchList(listState(u.searchParams, 'date', 50, persona), 'messages', all, scope === 'group' && s.role !== 'superadmin' ? (x) => s.group(x.chat) : null, (state, keep) => queryMessages(scope, state, keep), scope === 'group');
		return { route: scope === 'group' ? 'group_messages' : 'private_messages', status: 200, props: { ...st, ...r, ...extra, scope, rows: r.rows.map(messageRow), archive_total: s.role === 'member' ? undefined : scope === 'group' ? TOTALS.group_messages : TOTALS.private_messages } };
	}
	if ((m = path.match(/^\/messages\/(group|private)\/(\d+)$/))) {
		if (!allowed(s, m[1] === 'group' ? 'group_messages' : 'private_messages')) return notFound;
		const x = getMessage(m[1], m[2]);
		if (!x || (m[1] === 'group' && !s.group(x.chat))) return notFound;
		// The message as the chat shows it gives the parts both pages share
		// (MessageExtra, the media's detail, the edits' types and media).
		const c = chatMessage(x, s.canFetch);
		const media = c.media ? { ...c.media, file_id: x.media.fileId, name: x.media.name || '', size: x.media.size || 0 } : null;
		const extra = Object.fromEntries(['sender_chat', 'forward_from', 'author_signature', 'is_channel_post', 'service_type', 'tdlib_type'].filter((k) => c[k] !== undefined).map((k) => [k, c[k]]));
		return {
			route: 'message', status: 200, props: {
				message: {
					...messageRow(x), ...extra, text: c.text, sender: c.sender ?? messageRow(x).sender,
					reply_to: x.reply_to ? { scope: x.scope, id: x.reply_to.id, sender_name: senderOf(x.reply_to)?.name ?? '', snippet: x.reply_to.text || `[${x.reply_to.content_type}]` } : null,
					edits: c.edits, edited_at: x.edited_at ? iso(x.edited_at) : null, media,
				},
			},
		};
	}
	if ((m = path.match(/^\/(groups|users)\/(-?\d+)\/chat$/))) {
		const kind = m[1] === 'groups' ? 'group' : 'private';
		const c = chatFor(kind, m[2], persona);
		if (!c) return notFound;
		const { w, found } = chatPageWindow(c, u.searchParams, scopeOf(persona).canFetch);
		return {
			route: kind === 'group' ? 'group_chat' : 'user_chat', status: 200, props: {
				chat: kind === 'group'
					? { kind: 'group', group: { ...groupSummary(c), members: c.members, msg_count: c.msg_count } }
					: { kind: 'user', user: { id: c.id, name: c.name, photo: null }, count: chatStream(c).length },
				...w, at: found,
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
		if (path === '/platform-fwd/bots') {
			return { route: 'bots', status: 200, props: { bots: botsFor(persona).map((b) => botRow(b, persona)), show_owner: s.role !== 'member' } };
		}
		if (path === '/platform-fwd/discord-telegram') {
			return { route: 'fwd_discord_telegram', status: 200, props: { federations: fedList, routes: vis(d2t).map((r) => ({ ...owned(r), chat: pick(r.chat), channel_id: r.channel, channel_name: r.channel_name, discord_bot: botRefOf(r.discord_bot_id), telegram_bot: botRefOf(r.telegram_bot_id), types: r.types, enabled: r.enabled, sent: r.sent })) } };
		}
		return notFound;
	}
	if (path === '/join-chat') {
		if (!allowed(s, 'join_chat')) return notFound;
		const feds = s.role === 'superadmin' ? federations : s.manageableFeds.filter((f) => f.id !== 0);
		return { route: 'join_chat', status: 200, props: { federations: feds.map((f) => ({ id: f.id, name: f.name })), recent: JOIN_LOG, api_configured: true } };
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
				// Everything the full design has; Go turns these on as the
				// federation, restriction and grant items land.
				editable: { federations: true, group_grants: true, restrictions: true, abilities: true },
				// The stand-in has mail set up.
				mail: true,
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
// The preference a PATCH or PUT body gets wrong, if any: an unknown key
// or a value its key does not take (null is always taken).
const PREF_VALUES: Record<string, (v: unknown) => boolean> = {
	timezone: (v) => typeof v === 'string' && v.length > 0, date_format: (v) => ['24h', '12h', 'iso'].includes(v as string),
	lang: (v) => ['en', 'id'].includes(v as string), theme: (v) => ['system', 'light', 'dark'].includes(v as string),
	sidebar: (v) => ['open', 'collapsed'].includes(v as string), rows_per_page: (v) => [25, 50, 100].includes(v as number),
	landing: (v) => typeof v === 'string' && /^\/(?!\/)/.test(v) && v.length <= 255, chat_media: (v) => ['auto', 'click'].includes(v as string),
	chat_auto_load: (v) => typeof v === 'boolean', chat_live: (v) => typeof v === 'boolean', chat_show_deleted: (v) => typeof v === 'boolean',
	chat_show_edited: (v) => typeof v === 'boolean', chat_show_service: (v) => typeof v === 'boolean',
};
export function badPref(body: Record<string, unknown>): string | null {
	for (const [k, v] of Object.entries(body)) {
		const ok = PREF_VALUES[k];
		if (!ok || (v !== null && !ok(v))) return k;
	}
	return null;
}

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
// "Look up real name" (web/docs/contact-names.md). Of the logging
// account's sample contacts (users.js), the second's own name was never
// seen, and the third is left removed by an unfinished lookup. Lookups
// are at least LOOKUP_INTERVAL seconds apart, as the daemon keeps them.
const CONTACTS = users.filter((u) => u.is_contact);
const ownUnknown = new Set([CONTACTS[1]?.id]);
const lookupPending = new Set([CONTACTS[2]?.id]);
const ownChecked = new Map<number, string>();
const LOOKUP_INTERVAL = 300;
// The sample contacts' ids, in that order (for tests).
export const sampleContacts = () => CONTACTS.map((u) => u.id as number);
let lastLookup = 0;

function contactFields(u) {
	return {
		is_contact: !!u.is_contact, is_mutual_contact: u.id === CONTACTS[0]?.id,
		own_name_known: !ownUnknown.has(u.id), own_name_checked_at: ownChecked.get(u.id) ?? null,
	};
}

// [status, body, Retry-After seconds]
export function lookupName(id: string, persona: Persona): [number, unknown, number?] {
	const s = scopeOf(persona);
	const u = getUser(id);
	if (!s.canLookupName || !u || !s.user(u)) return [404, { error: 'not_found', message: 'No such user.' }];
	if (!u.is_contact) return [409, { error: 'not_a_contact', message: 'This user is not a contact of the logging account.' }];
	if (lookupPending.has(u.id)) return [503, { error: 'restore_pending', message: 'A contact removed by an earlier lookup is not added back yet' }, 60];
	const now = Date.now();
	if (now - lastLookup < LOOKUP_INTERVAL * 1000) return [429, { error: 'rate_limited', message: 'Another lookup ran a moment ago' }, Math.ceil(LOOKUP_INTERVAL - (now - lastLookup) / 1000)];
	lastLookup = now;
	ownUnknown.delete(u.id);
	ownChecked.set(u.id, new Date(now).toISOString().replace(/\.\d+Z$/, 'Z'));
	const [first, ...rest] = String(u.contact_name ?? '').split(' ');
	return [200, { user_id: u.id, first_name: u.first_name, last_name: u.last_name, contact_first_name: first, contact_last_name: rest.join(' '), checked_at: Math.floor(now / 1000) }];
}

// Histories, page by page (UserHistoryPage, GroupHistoryPage). The
// cursor is the offset of the next entry. The sample timelines are
// short, so the pages' first page is too, for paging to show.
const HISTORY_FIRST = 10;
function historyPage<T>(all: T[], before: string | null, limit: number): { entries: T[]; next: string | null } | null {
	let from = 0;
	if (before !== null) {
		if (!/^h\d+$/.test(before)) return null;
		from = Number(before.slice(1));
	}
	const entries = all.slice(from, from + limit);
	return { entries, next: from + limit < all.length ? `h${from + limit}` : null };
}

// A username event's details; every fifth added name is a reorder, and
// names ending in a digit were bought (collectible).
function usernameEvent(e, i: number) {
	if (e.kind !== 'username') return {};
	const action = e.action === 'added' && i % 5 === 4 ? 'reordered' : e.action;
	return { action, ...(action !== 'removed' ? { username_kind: 'active' } : {}), ...(/\d$/.test(e.detail) ? { collectible: true } : {}) };
}

const userHistory = (usr) => getUserHistory(usr).map((e, i) => ({ kind: e.kind, at: iso(e.at), detail: e.detail, action: e.action, photo: null, removed: e.kind === 'photo' ? true : undefined, ...usernameEvent(e, i) }));
const groupHistory = (g) => getGroupHistory(g).map((e, i) => ({ kind: e.kind, at: iso(e.at), detail: e.detail, action: e.action, photo: null, removed: e.kind === 'photo' ? true : undefined, user: e.user ? userSummary(e.user, false) : undefined, ...usernameEvent(e, i) }));

// GET /api/users/{id}/history and /api/groups/{id}/history.
export function historyApi(kind: 'users' | 'groups', id: string, q: URLSearchParams, persona: Persona): [number, unknown] {
	const s = scopeOf(persona);
	const limit = q.get('limit') === null ? 100 : Number(q.get('limit'));
	if (!Number.isInteger(limit) || limit < 1 || limit > 500) return [400, { error: 'bad_request', message: 'limit is 1 to 500.' }];
	const all = kind === 'users'
		? (() => { const u = getUser(id); return u && s.user(u) && s.fullUser ? userHistory(u) : null; })()
		: (() => { const g = getGroup(id); return g && s.group(g) ? groupHistory(g) : null; })();
	if (!all) return [404, { error: 'not_found' }];
	const page = historyPage(all, q.get('before'), limit);
	return page ? [200, page] : [400, { error: 'bad_request', message: 'That cursor does not parse.' }];
}

// The profile fields the sample users lack, derived from the id: some
// share a birthday (one without the year), the featured user a personal
// channel, premium users an emoji status; a few charge Stars for
// messages or take chats from contacts only.
function profileExtras(usr, s) {
	const n = Number(String(usr.id).slice(-4));
	const channel = usr === users[3] ? groups.find((g) => g.type === 'channel') : null;
	return {
		photo_file_id: null,
		birthday: n % 5 === 0 ? { day: 1 + (n % 28), month: 1 + (n % 12), year: n % 2 ? 1990 + (n % 20) : null } : null,
		personal_chat_id: channel ? channel.id : null,
		personal_chat: channel && s.group(channel) ? groupSummary(channel) : null,
		paid_message_star_count: n % 17 === 0 ? 50 : 0,
		emoji_status: usr.is_premium ? { custom_emoji_id: String(5368324170671202286n + BigInt(n)), expires_at: n % 3 ? null : '2026-12-31T00:00:00Z' } : null,
		accent_color_id: n % 7, profile_accent_color_id: usr.is_premium ? n % 16 : null,
		background_custom_emoji_id: null, profile_background_custom_emoji_id: null,
		restriction_reason: usr.is_scam ? 'Reported as a scam by many users.' : '',
		has_sensitive_content: false, restricts_new_chats: n % 7 === 0,
	};
}

export function adminApi(method: string, p: string, body, persona: Persona): [number, unknown] {
	const s = scopeOf(persona);
	let m;
	if (p === 'accounts' && method === 'POST') {
		if (!allowed(s, 'accounts')) return [404, { error: 'not_found' }];
		if (!body.username) return [400, { error: 'bad_request', message: 'A username is required.' }];
		const email = String(body.email ?? '').trim();
		// Email is optional; a first password is typed, or generated, or a
		// set-password link goes out instead (the stand-in has mail).
		if (body.invite && body.password) return [400, { error: 'bad_request', message: 'Either a password or an invitation, not both.' }];
		if (body.invite && !email) return [409, { error: 'no_email', message: 'An invitation needs an email address.' }];
		if (body.password && new TextEncoder().encode(String(body.password)).length < 8) return [400, { error: 'weak_password', message: 'The password needs at least 8 characters.' }];
		if (accounts.some((a) => a.username === body.username)) return [409, { error: 'taken', message: 'That username is taken.' }];
		if (s.role !== 'superadmin' && body.role !== 'member') return [403, { error: 'forbidden', message: 'Admins create members only.' }];
		const grants = (body.grants || []).map((g) => ({ fed: g.fed, groupIds: g.group_ids || [] }));
		// An admin's new member must land in a federation that admin manages.
		if (s.role !== 'superadmin' && !grants.some((g) => s.manageableFeds.some((f) => f.id === g.fed))) return [400, { error: 'grants_required', message: 'Grant at least one federation you administer.' }];
		const id = Math.max(...accounts.map((a) => a.id)) + 1;
		accounts.push({ id, username: body.username, name: body.name || body.username, email, verified: false, invited: true, role: body.role, active: true, last: '', created: '2026-11-17 16:00:00', hue: 200, adminOf: [], restrict: { hide_private: false, hide_groups: [], join_chat: false }, grants, abilities: [] });
		if (body.invite) return [201, { id, invited: true }];
		mustChange.add(id);
		return [201, body.password ? { id, invited: false } : { id, invited: false, temporary_password: temporaryPassword() }];
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
		if (m[2] === 'password-reset' && method === 'POST') {
			// A verified address gets a link (the stand-in has mail); else a
			// temporary password.
			if (a.email && a.verified) return [200, { link_sent: true }];
			mustChange.add(a.id); a.invited = true;
			return [200, { link_sent: false, temporary_password: temporaryPassword() }];
		}
		if (m[2] === 'resend-invite' && method === 'POST') {
			if (!a.invited) return [409, { error: 'not_invited', message: 'This account has chosen its password already.' }];
			if (!a.email) return [409, { error: 'no_email', message: 'This account has no email address.' }];
			return [204, null];
		}
		if (m[2] === 'sign-out-everywhere' && method === 'POST') return [204, null];
	}
	// Bots: /api/bots[/<id>[/transfer]]
	if ((m = p.match(/^bots(?:\/(\d+))?(?:\/(transfer))?$/))) {
		if (!allowed(s, 'platform_fwd')) return [404, { error: 'not_found' }];
		const mine = botsFor(persona);
		if (!m[1] && method === 'GET') return [200, { bots: mine.filter((b) => !body.kind || b.kind === body.kind).map(botRef) }];
		if (!m[1] && method === 'POST') {
			const name = String(body.name ?? '').trim();
			if (!name || name.length > 64) return [400, { error: 'bad_request', message: 'Give the bot a name of 1 to 64 characters.' }];
			if (!botTokenShape(body.kind, String(body.token ?? ''))) return [400, { error: 'bad_token', message: body.kind === 'discord' ? 'That does not look like a Discord bot token.' : 'That does not look like a Telegram bot token: <digits>:<secret>.' }];
			const same = BOTS.find((b) => b.token === body.token);
			if (same && same.owner !== PERSONAS[persona]) return [409, { error: 'token_in_use', message: 'Another account has this bot already.' }];
			if (same) return [201, botRow(same, persona)];
			const id = Math.max(0, ...BOTS.map((b) => b.id)) + 1;
			const b = { id, kind: body.kind, name, token: body.token, owner: PERSONAS[persona], status: 'connecting', status_at: null, status_detail: null, enabled: true, user_id: null, username: null };
			BOTS.push(b);
			return [201, botRow(b, persona)];
		}
		const b = mine.find((x) => String(x.id) === m[1]);
		if (!b) return [404, { error: 'not_found' }];
		if (m[2] === 'transfer') {
			if (s.role === 'member') return [404, { error: 'not_found' }];
			b.owner = body.owner_id; return [204, null];
		}
		if (method === 'PATCH') {
			if (body.name !== undefined) { const n = String(body.name).trim(); if (!n || n.length > 64) return [400, { error: 'bad_request', message: 'Give the bot a name of 1 to 64 characters.' }]; b.name = n; }
			if (body.token !== undefined) {
				if (!botTokenShape(b.kind, String(body.token))) return [400, { error: 'bad_token', message: 'That does not look like a bot token of this kind.' }];
				b.token = body.token; b.status = 'connecting'; b.status_detail = null;
			}
			if (body.enabled !== undefined) { b.enabled = !!body.enabled; b.status = b.enabled ? 'connecting' : 'disabled'; if (b.enabled) b.status_detail = null; }
			return [200, botRow(b, persona)];
		}
		if (method === 'DELETE') {
			const n = d2t.filter((r) => r.discord_bot_id === b.id || r.telegram_bot_id === b.id).length;
			if (n) return [409, { error: 'bot_in_use', message: `${n} route${n > 1 ? 's use' : ' uses'} this bot. Give ${n > 1 ? 'them' : 'it'} another bot, or delete ${n > 1 ? 'them' : 'it'}, first.` }];
			BOTS.splice(BOTS.indexOf(b), 1); return [204, null];
		}
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
		// A chat as T2D names it (chat_kind) or as D2T does (a positive id
		// is a private chat).
		const chatOf = (id, kind) => {
			const c = (kind ?? (id > 0 ? 'user' : 'group')) === 'user' ? chatOptions.find((x) => x.id === id && x.kind === 'user') : getGroup(id);
			return c && (c.kind !== 'group' || s.group(c)) ? c : null;
		};
		// A D2T route's bots: ones the viewer may use, of the right kind.
		const botErr = (b) => {
			for (const [k, kind] of [['discord_bot_id', 'discord'], ['telegram_bot_id', 'telegram']] as const) {
				if (b[k] === undefined) continue;
				const bot = botsFor(persona).find((x) => x.id === b[k]);
				if (!bot) return [404, { error: 'not_found', message: 'No such bot.' }];
				if (bot.kind !== kind) return [400, { error: 'bad_request', message: `That is not a ${kind === 'discord' ? 'Discord' : 'Telegram'} bot.` }];
			}
			if (b.discord_channel_id !== undefined && !/^\d{15,25}$/.test(String(b.discord_channel_id))) return [400, { error: 'bad_channel', message: 'That is not a Discord channel id.' }];
			return null;
		};
		const dup = (channel, chat, self?) => list.some((x) => x !== self && x.channel === channel && x.chat.id === chat.id);
		if (!m[2] && method === 'POST') {
			const chat = chatOf(body.chat_id, m[1] === 'telegram-discord' ? body.chat_kind : undefined);
			if (!chat) return [404, { error: 'not_found', message: 'No such chat.' }];
			const id = Math.max(0, ...list.map((r) => r.id)) + 1;
			const fed = body.federation_id ?? 0;
			if (m[1] === 'telegram-discord') {
				list.push({ id, owner: PERSONAS[persona], fed, chat, url: body.webhook_url || '', types: body.types, enabled: body.enabled, last: null, sent: 0 });
				return [201, { id }];
			}
			if (body.discord_bot_id === undefined || body.telegram_bot_id === undefined) return [400, { error: 'bad_request', message: 'Choose both bots.' }];
			const e = botErr(body);
			if (e) return e;
			if (dup(body.discord_channel_id, chat)) return [409, { error: 'duplicate', message: 'That channel already forwards to this chat.' }];
			list.push({ id, owner: PERSONAS[persona], fed, channel: body.discord_channel_id, channel_name: '#new-channel', chat, discord_bot_id: body.discord_bot_id, telegram_bot_id: body.telegram_bot_id, types: body.types, enabled: body.enabled, sent: 0 });
			return [201, { id }];
		}
		const r = list.find((x) => String(x.id) === m[2]);
		if (!r || (s.role === 'member' && r.owner !== PERSONAS[persona])) return [404, { error: 'not_found' }];
		if (m[3] === 'transfer') {
			if (s.role === 'member') return [404, { error: 'not_found' }];
			r.owner = body.owner_id; return [204, null];
		}
		if (method === 'PATCH') {
			// The chat (either direction), and a D2T route's channel and bots,
			// change too, checked as on creation.
			let chat = r.chat;
			if (body.chat_id !== undefined) {
				chat = chatOf(body.chat_id, m[1] === 'telegram-discord' ? body.chat_kind : undefined);
				if (!chat) return [404, { error: 'not_found', message: 'No such chat.' }];
			}
			if (m[1] === 'discord-telegram') {
				const e = botErr(body);
				if (e) return e;
				if (dup(body.discord_channel_id ?? r.channel, chat, r)) return [409, { error: 'duplicate', message: 'That channel already forwards to this chat.' }];
				if (body.discord_channel_id !== undefined) r.channel = body.discord_channel_id;
				if (body.discord_bot_id !== undefined) r.discord_bot_id = body.discord_bot_id;
				if (body.telegram_bot_id !== undefined) r.telegram_bot_id = body.telegram_bot_id;
			}
			Object.assign(r, { chat, ...(body.types ? { types: body.types } : {}), ...(body.enabled !== undefined ? { enabled: body.enabled } : {}) });
			return [204, null];
		}
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
	return { v: 1, route: r.route, url: r.url ?? url, status: r.status, props: r.props, viewer: viewerOf(persona) };
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

export function liveChange(kind: string, b: { chat_id?: number; message_id?: number; user_id?: number; bot_id?: number; status?: string }): Record<string, unknown> | null {
	const b2 = b;
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
	if (kind === 'bot.updated') {
		// discordd reports a bot's state: the given status, or the next in a
		// round of the ones it reports.
		const b = BOTS.find((x) => x.id === Number(b2.bot_id));
		if (!b) return null;
		const cycle = ['connected', b.kind === 'discord' ? 'intent_missing' : 'token_refused', 'connected'];
		b.status = b2.status ?? cycle[(cycle.indexOf(b.status) + 1) % cycle.length];
		b.status_at = NOW;
		return { kind, bot_id: b.id };
	}
	if (kind === 'stats.changed') {
		// Counters move: one more message for the named chat and user.
		const g = b.chat_id ? getGroup(Number(b.chat_id)) : null;
		const u = b.user_id ? getUser(Number(b.user_id)) : null;
		if (g) g.msg_count += 1;
		if (u) u.msg_count += 1;
		return { kind, chat_ids: g ? [g.id] : [], user_ids: u ? [u.id] : [] };
	}
	return null;
}

// Whether a persona may see an event (web-live.md section 3).
export function liveVisible(persona: Persona, e: { kind: string; chat_id?: number; user_id?: number; bot_id?: number }): boolean {
	const s = scopeOf(persona);
	if (s.role === 'public') return false;
	if (e.bot_id !== undefined) return botsFor(persona).some((b) => b.id === e.bot_id);
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

// Temporary passwords (until set-password links exist): accounts
// created or reset get one, answered once, and must choose their own
// at the next sign-in; mod_kernel starts that way.
const mustChange = new Set<number>([6]);
const temporaryPassword = () => randomBytes(12).toString('base64url');
export function needsPasswordChange(persona: Persona): boolean {
	return persona !== 'public' && mustChange.has(PERSONAS[persona]);
}
export function passwordChanged(persona: Persona): void {
	if (persona === 'public') return;
	mustChange.delete(PERSONAS[persona]);
	const a = accounts.find((x) => x.id === PERSONAS[persona]);
	if (a) a.invited = false;
}

// A changed address waits for its verification link.
export function changeEmail(persona: Persona, email: string): boolean {
	const a = persona === 'public' ? null : accounts.find((x) => x.id === PERSONAS[persona]);
	if (!a) return false;
	if (!email) { a.email = ''; a.verified = false; a.pendingEmail = null; return false; }
	a.pendingEmail = email;
	return true;
}
export function setPendingEmail(persona: Persona, email: string): void {
	const a = persona === 'public' ? null : accounts.find((x) => x.id === PERSONAS[persona]);
	if (a) a.pendingEmail = email;
}
export function verifyPendingEmails(): void {
	for (const a of accounts) if (a.pendingEmail) { a.email = a.pendingEmail; a.verified = true; a.pendingEmail = null; }
}

// Stand-in for GET /api/entities: display fields and counters of the
// users and groups the persona may see; the others are left out.
export function entities(persona: Persona, q: URLSearchParams): [number, unknown] {
	const s = scopeOf(persona);
	if (s.role === 'public') return [401, { error: 'unauthenticated' }];
	const ids = (k: string) => (q.get(k) || '').split(',').filter(Boolean).map(Number);
	const users = ids('users'), groups = ids('groups');
	if (users.length > 200 || groups.length > 200 || [...users, ...groups].some((n) => !Number.isFinite(n))) return [400, { error: 'bad_request', message: 'At most 200 numeric ids per list.' }];
	return [200, {
		users: users.map((id) => getUser(id)).filter((u) => u && s.user(u)).map((u) => userSummary(u, s.fullUser)),
		groups: groups.map((id) => getGroup(id)).filter((g) => g && s.group(g)).map((g) => ({ ...groupSummary(g), ...(s.role === 'member' ? {} : { msg_count: g.msg_count }) })),
	}];
}

// The landing page of a persona: its preference, else the dashboard.
export function landingOf(persona: Persona): string {
	return persona === 'public' ? '/' : settingsState(persona).prefs.landing || '/';
}

// Bots: every account's own
// Telegram and Discord bots. Tokens stay here and are never answered.
const BOTS = [
	{ id: 1, kind: 'telegram', name: 'Logger relay', token: '7000000001:' + 'a'.repeat(35), owner: 1, status: 'connected', status_at: '2026-11-17 15:58:00', status_detail: null, enabled: true, user_id: '7000000001', username: 'tgd_relay_bot' },
	{ id: 2, kind: 'discord', name: 'Discord reader', token: 'MTIzNDU2Nzg5MDEyMzQ1Njc4.GaBcDe.' + 'x'.repeat(38), owner: 1, status: 'connected', status_at: '2026-11-17 15:59:00', status_detail: null, enabled: true, user_id: '123456789012345678', username: 'tgd-reader' },
	{ id: 3, kind: 'discord', name: 'GNU/Weeb mirror', token: 'MjM0NTY3ODkwMTIzNDU2Nzg5.HbCdEf.' + 'y'.repeat(38), owner: 3, status: 'intent_missing', status_at: '2026-11-17 14:00:00', status_detail: 'Disallowed intent(s): MESSAGE_CONTENT', enabled: true, user_id: '234567890123456789', username: 'gw-mirror' },
	{ id: 4, kind: 'telegram', name: 'Research bot', token: '7000000004:' + 'b'.repeat(35), owner: 5, status: 'token_refused', status_at: '2026-11-17 12:00:00', status_detail: 'Unauthorized', enabled: false, user_id: '7000000004', username: null },
	{ id: 5, kind: 'discord', name: 'Research reader', token: 'MzQ1Njc4OTAxMjM0NTY3ODkw.IcDeFg.' + 'z'.repeat(38), owner: 5, status: 'disabled', status_at: null, status_detail: null, enabled: false, user_id: null, username: null },
];

// A member with forward sees its own bots; admins and superadmins all.
function botsFor(persona: Persona) {
	if (persona === 'public') return [];
	const s = scopeOf(persona);
	return s.role === 'member' ? BOTS.filter((b) => b.owner === PERSONAS[persona]) : BOTS;
}

const botRefOf = (id) => {
	const b = BOTS.find((x) => x.id === id);
	return b ? botRef(b) : null;
};

function botRef(b) {
	return { id: b.id, kind: b.kind, name: b.name, identity: b.user_id ? { user_id: b.user_id, username: b.username } : null, status: b.status, enabled: b.enabled };
}

function botRow(b, persona: Persona) {
	const o = accounts.find((a) => a.id === b.owner);
	const s = scopeOf(persona);
	return {
		...botRef(b),
		owner: o ? { id: o.id, name: o.name, active: o.active } : { id: 0, name: 'Shared', active: true },
		transfer_candidates: s.role === 'member' ? [] : accounts.filter((a) => a.active && a.id !== b.owner && (a.role !== 'member' || a.abilities.includes('forward'))).map((a) => ({ id: a.id, name: a.name, role: a.role })),
		status_at: b.status_at ? iso(b.status_at) : null,
		status_detail: b.status_detail,
		routes: d2t.filter((r) => r.discord_bot_id === b.id || r.telegram_bot_id === b.id).length,
	};
}

// The token shapes the API checks (whether one works shows in the status).
export function botTokenShape(kind: string, token: string): boolean {
	if (kind === 'telegram') return /^\d+:[A-Za-z0-9_-]{30,}$/.test(token);
	if (kind !== 'discord') return false;
	const parts = token.split('.');
	if (parts.length !== 3 || !parts.every((x) => /^[A-Za-z0-9_-]+$/.test(x))) return false;
	try { return /^\d{15,25}$/.test(Buffer.from(parts[0], 'base64url').toString('utf8')); } catch { return false; }
}

// The sample Discord to Telegram routes: the first reads with a bot
// whose intent is missing and sends with the relay; the second was made
// before Discord bots were stored (no Discord bot yet) and sends with a
// disabled bot.
const D2T_BOTS = [[3, 1], [null, 4]];
d2t.forEach((r, i) => {
	const [d, t] = D2T_BOTS[i] ?? [2, 1];
	r.discord_bot_id = d; r.telegram_bot_id = t;
	delete r.bot;
});
