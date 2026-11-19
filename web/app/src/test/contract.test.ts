// The stand-in's dummy data against the API description: every page it
// serves, as each sample account sees it, must match its page schema in
// webapi/openapi.yaml, and so must the chat, search and entity answers.
// The page tests run on this data, so it is the contract the Go loaders
// are held to; a mismatch here means the page tests and the description
// disagree, and one of them must change.
import { describe, expect, it } from 'vitest';
import { setEngine, viewOf, adminApi, dashboardSeries, federationGroups, badPref, groupPhotos, userPhotos, badRange, settingsState, chatFor, chatWindow, entities, historyApi, lookupName, pageData, sampleContacts, searchChats, type Persona, searchCapabilities, fetchFileApi } from '../../standin/fixtures.ts';
import { loadSpec, responseSchema, validate, type Schema } from './schema.ts';
import { FORUM_GROUP, KINDS_GROUP, TOPICS, sampleOf } from '../../standin/kinds.ts';
import { chatStream, privateMessages } from '../../../mockup/src/data/messages.js';

const spec = loadSpec();
const ref = (name: string): Schema => ({ $ref: `#/components/schemas/${name}` });
// What goes over the wire: keys holding undefined are dropped.
const wire = (v: unknown) => JSON.parse(JSON.stringify(v)) as unknown;

// route id -> the page schema whose `route` names it.
const variants = new Map<string, string>();
for (const v of (spec.components.schemas.PageAnswer!.oneOf as { $ref: string }[]).map((x) => x.$ref.split('/').pop()!)) {
	const routeSchema = ((spec.components.schemas[v]!.allOf as Schema[])[1]!.properties as Record<string, Schema>).route!;
	for (const r of (routeSchema.enum ?? [routeSchema.const]) as string[]) variants.set(r, v);
}
variants.set('not_found', 'PageNotFound');

const first = <T,>(url: string, persona: Persona, pick: (props: never) => T) => pick(pageData(url, persona).props as never);
const userId = first('/users', 'superadmin', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
const groupId = first('/groups', 'superadmin', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
const memberGroupId = first('/groups', 'member', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
const exposedId = first('/groups', 'superadmin', (p: { rows: { id: number; exposed?: boolean }[] }) => p.rows.find((g) => g.exposed)!.id);
// An exposed group the member is not granted: its page is the public one.
const exposedHiddenId = first('/groups', 'superadmin', (p: { rows: { id: number; exposed?: boolean }[] }) => {
	const mine = new Set(first('/groups', 'member', (q: { rows: { id: number }[] }) => q.rows.map((g) => g.id)));
	return p.rows.find((g) => g.exposed && !mine.has(g.id))!.id;
});
const groupMsg = first('/group_messages', 'superadmin', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
const privateMsg = first('/private_messages', 'superadmin', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
const fileId = first('/files?view=list', 'superadmin', (p: { rows: { id: number }[] }) => p.rows[0]!.id);
// A file seen in no visible message (chat null), found page by page.
const orphanId = (() => {
	for (let page = 1; page < 100; page++) {
		const rows = (pageData(`/files?view=list&per=200&page=${page}`, 'superadmin').props as { rows: { id: number; chat: unknown }[] }).rows;
		const hit = rows.find((r) => r.chat === null);
		if (hit) return hit.id;
		if (!rows.length) break;
	}
	throw new Error('the stand-in has no file without a visible message');
})();

// The kind samples (standin/kinds.ts): their chat, a forum topic, and
// the message page of each sample.
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const sampleIds = (chatStream(KINDS_GROUP) as Raw[]).filter((m) => sampleOf(m)).map((m) => m.id);
// The first private chat's samples: a call, service rows, gifts, themes.
const privateSampleIds = (chatStream(privateMessages()[0]!.chat) as Raw[]).filter((m) => sampleOf(m)).map((m) => m.id);

const PAGES: [Persona, string][] = [
	...(['superadmin', 'admin', 'member'] as Persona[]).flatMap((a): [Persona, string][] => [
		[a, '/'], [a, '/users'], [a, '/users?q=a&sort=msg_count&dir=asc&page=2'], [a, '/groups'], [a, `/groups/${a === 'member' ? memberGroupId : groupId}`],
		[a, `/groups/${a === 'member' ? memberGroupId : groupId}/admins`], [a, `/groups/${a === 'member' ? memberGroupId : groupId}/history`],
		[a, `/groups/${a === 'member' ? memberGroupId : groupId}/chat`], [a, '/files'], [a, `/files?view=list&file=${fileId}`],
		[a, '/group_messages'], [a, '/group_messages?q=rust&f=' + encodeURIComponent('[{"field":"is_deleted","op":"is","value":"yes"}]')],
		[a, '/settings'], [a, '/settings/preferences'], [a, '/settings/sessions'],
		[a, '/platform-fwd'], [a, '/platform-fwd/telegram-discord'], [a, '/platform-fwd/discord-telegram'],
		[a, '/platform-fwd/bots'], 		[a, '/settings/accounts'], [a, '/settings/federations'], [a, '/settings/federations/1'], [a, '/settings/federations/0'], [a, '/join-chat'], [a, '/mcp-admin'], [a, '/mcp-admin/groups'], [a, '/mcp-admin/tokens'],
		[a, '/no/such/page'],
	]),
	['superadmin', `/files?view=list&file=${orphanId}`], ['admin', `/files?view=list&file=${orphanId}`],
	['superadmin', `/users/${userId}`], ['superadmin', `/users/${userId}/history`], ['superadmin', `/users/${userId}/chat`],
	['member', `/users/${userId}`], ['superadmin', '/private_messages'], ['superadmin', `/messages/group/${groupMsg}`],
	['superadmin', `/messages/private/${privateMsg}`], ['superadmin', `/groups/${groupId}/chat?at=1`],
	['public', `/groups/${exposedId}/chat`], ['public', `/groups/${exposedId}`], ['member', `/groups/${exposedHiddenId}`], ['public', '/login'], ['public', '/login?next=%2Fusers'],
	['public', '/password/forgot'], ['public', '/password/set'], ['public', '/account/email/verify'], ['superadmin', '/password/forgot'],
	['superadmin', `/groups/${KINDS_GROUP.id}/chat`], ['admin', `/groups/${KINDS_GROUP.id}/chat`], ['member', `/groups/${KINDS_GROUP.id}/chat`],
	['superadmin', `/groups/${FORUM_GROUP.id}/chat`], ['superadmin', `/groups/${FORUM_GROUP.id}/chat?topic=${TOPICS[1]!.id}`], ['superadmin', `/groups/${FORUM_GROUP.id}/chat?topic=999`],
	...sampleIds.map((id): [Persona, string] => ['superadmin', `/messages/group/${id}`]), ...sampleIds.map((id): [Persona, string] => ['member', `/messages/group/${id}`]),
	...privateSampleIds.map((id): [Persona, string] => ['superadmin', `/messages/private/${id}`]),
	['superadmin', `/groups/${KINDS_GROUP.id}/files`], ['superadmin', `/groups/${KINDS_GROUP.id}/polls`], ['public', `/groups/${exposedId}/files`], ['public', `/groups/${exposedId}/polls`],
	['noemail', '/users'], ['noemail', '/'],
	['newpw', '/users'], ['newpw', '/settings'], ['newpw', '/password/forgot'],
];

describe('stand-in page data against webapi/openapi.yaml', () => {
	it.each(PAGES)('%s %s', (persona, url) => {
		const page = pageData(url, persona);
		const schema = variants.get(page.route);
		expect(schema, `route ${page.route} has no page schema`).toBeDefined();
		expect(validate(spec, ref(schema!), wire(page))).toEqual([]);
	});
});

describe('stand-in answers against webapi/openapi.yaml', () => {
	const ok = (status: number, body: unknown, path: string, method = 'get') => {
		expect(status).toBe(200);
		expect(validate(spec, responseSchema(spec, path, method), wire(body))).toEqual([]);
	};

	it('a chat window', () => {
		const chat = chatFor('group', String(groupId), 'superadmin');
		for (const anchor of [{}, { before: 1e9 }, { after: 0 }, { at: 1 }, { after_ts: 0 }, { after_ts: 4e9 }]) ok(200, chatWindow(chat, anchor), '/api/chats/{scope}/{id}/messages');
	});

	it('a chat window of every kind, and of one forum topic', () => {
		const kinds = chatFor('group', String(KINDS_GROUP.id), 'superadmin');
		for (const persona of ['superadmin', 'member'] as Persona[]) ok(200, chatWindow(kinds, {}, 60, viewOf(persona, KINDS_GROUP.id)), '/api/chats/{scope}/{id}/messages');
		// The first private chat has a call, a service row and a rich
		// message without its layout.
		const pm = chatWindow(chatFor('user', String(privateMessages()[0]!.chat.id), 'superadmin'), {}) as { messages: { content_type: string }[] };
		expect(pm.messages.some((m) => m.content_type === 'rich')).toBe(true);
		ok(200, pm, '/api/chats/{scope}/{id}/messages');
		const forum = chatFor('group', String(FORUM_GROUP.id), 'superadmin');
		const w = chatWindow(forum, { topic: TOPICS[1]!.id }) as { messages: { topic_id: number | null }[] };
		expect(w.messages.length).toBeGreaterThan(0);
		expect(w.messages.every((m) => m.topic_id === TOPICS[1]!.id)).toBe(true);
		ok(200, w, '/api/chats/{scope}/{id}/messages');
	});

	it.each(['superadmin', 'admin', 'member'] as Persona[])('the chat search, as %s', (p) => {
		for (const q of ['', 'q=rust', 'kind=group', 'kind=user&limit=5', 'federation=0&limit=200', 'cursor=20']) {
			const [status, body] = searchChats(p, new URLSearchParams(q));
			ok(status, body, '/api/chats/search');
		}
	});

	it('account creation and a password reset', () => {
		const [cs, created] = adminApi('POST', 'accounts', { username: 'contract1', name: 'C', email: 'c@example.org', role: 'member', grants: [{ fed: 0, group_ids: [] }] }, 'superadmin');
		expect(cs).toBe(201);
		expect(validate(spec, responseSchema(spec, '/api/accounts', 'post', '201'), wire(created))).toEqual([]);
		const [rs, reset] = adminApi('POST', `accounts/${(created as { id: number }).id}/password-reset`, {}, 'superadmin');
		ok(rs, reset, '/api/accounts/{id}/password-reset', 'post');
	});

	it.each(['superadmin', 'member'] as Persona[])('the bot list for pickers, as %s', (p) => {
		for (const kind of [undefined, 'discord', 'telegram']) {
			const [status, body] = adminApi('GET', 'bots', kind ? { kind } : {}, p);
			ok(status, body, '/api/bots');
		}
	});

	// Before the test below, which takes gw_admin's Join chat grant.
	it('join answers, held filings and dropping one', () => {
		type Props = { recent: { by: string; title: string }[]; pending: { id: number; by: string; federation: { id: number } }[] };
		const props = (p: Persona) => pageData('/join-chat', p).props as Props;
		// An admin sees only its own attempts and filings.
		expect(props('admin').recent.every((r) => r.by === 'gw_admin')).toBe(true);
		expect(props('admin').pending.every((f) => f.by === 'gw_admin')).toBe(true);
		expect(props('superadmin').pending.some((f) => f.by !== 'gw_admin')).toBe(true);
		expect(props('superadmin').recent.length).toBeGreaterThan(props('admin').recent.length);
		const [js, joined] = adminApi('POST', 'join-chat/join', { ref: '@compilernerds_id', federation_ids: [1, 0] }, 'admin');
		ok(js, joined, '/api/join-chat/join', 'post');
		expect(joined).toMatchObject({ outcome: 'joined', federation_ids: [1], filing_pending: false });
		// A federation the admin does not administer: 404, nothing joined.
		const before = props('admin').recent.length;
		expect(adminApi('POST', 'join-chat/join', { ref: '@x', federation_ids: [2] }, 'admin')[0]).toBe(404);
		expect(props('admin').recent).toHaveLength(before);
		const held = props('admin').pending.length;
		const [rs, sent] = adminApi('POST', 'join-chat/join', { ref: 'https://t.me/+request_me', federation_ids: [1] }, 'admin');
		ok(rs, sent, '/api/join-chat/join', 'post');
		expect(sent).toMatchObject({ outcome: 'request_sent', federation_ids: [1], filing_pending: true });
		expect(props('admin').pending).toHaveLength(held + 1);
		// Dropping: another account's filing is 404 to an admin, its own
		// goes, and a superadmin may drop anyone's.
		const others = props('superadmin').pending.find((f) => f.by !== 'gw_admin')!;
		expect(adminApi('DELETE', `join-chat/pending/${others.id}`, {}, 'admin')[0]).toBe(404);
		expect(adminApi('DELETE', `join-chat/pending/${others.id}`, {}, 'member')[0]).toBe(404);
		const own = props('admin').pending.at(-1)!;
		expect(adminApi('DELETE', `join-chat/pending/${own.id}`, {}, 'admin')[0]).toBe(204);
		expect(adminApi('DELETE', `join-chat/pending/${own.id}`, {}, 'admin')[0]).toBe(404);
		expect(adminApi('DELETE', `join-chat/pending/${others.id}`, {}, 'superadmin')[0]).toBe(204);
		expect(props('superadmin').pending.some((f) => f.id === others.id)).toBe(false);
	});

	it('history pages of a user and a group', () => {
		const user = String(sampleContacts()[0]);
		for (const q of ['', 'limit=3', 'limit=3&before=h3']) {
			const [us, ub] = historyApi('users', user, new URLSearchParams(q), 'superadmin');
			ok(us, ub, '/api/users/{id}/history');
			const [gs, gb] = historyApi('groups', String(groupId), new URLSearchParams(q), 'superadmin');
			ok(gs, gb, '/api/groups/{id}/history');
		}
		expect(historyApi('users', user, new URLSearchParams('before=junk'), 'superadmin')[0]).toBe(400);
		expect(historyApi('users', user, new URLSearchParams(''), 'member')[0]).toBe(404);
	});

	it('the preferences a PATCH takes', () => {
		expect(badPref({ theme: 'dark', sidebar: 'collapsed' })).toBeNull();
		expect(badPref({ sidebar: null })).toBeNull();
		expect(badPref({ sidebar: 'hidden' })).toBe('sidebar');
		expect(badPref({ nope: 1 })).toBe('nope');
		// Every key the spec's Prefs has is one the stand-in checks.
		const keys = Object.keys(spec.components.schemas.Prefs?.properties ?? {});
		expect(keys.length).toBeGreaterThan(0);
		for (const k of keys) expect(badPref({ [k]: null }), k).toBeNull();
	});

	it('hiding private messages from an admin, as Go takes it', () => {
		const accts = (adminApi('GET', 'accounts', {}, 'superadmin')[1] as { accounts?: { id: number; role: string }[] }).accounts
			?? (pageData('/settings/accounts', 'superadmin').props as { accounts: { id: number; role: string }[] }).accounts;
		const admin = accts.find((a) => a.id === 3)!;
		const member = accts.find((a) => a.role === 'member')!;
		const patch = (id: number, body: object, who: Persona = 'superadmin') => adminApi('PATCH', `accounts/${id}`, body, who);
		const hide = { hide_private: true, hide_groups: [], join_chat: false };
		expect(admin.role).toBe('admin');
		// Only a superadmin; only for an admin; only that restriction.
		expect(patch(admin.id, { restrict: hide }, 'admin')[0]).toBe(403);
		expect(patch(member.id, { restrict: hide })[0]).toBe(400);
		// The Join chat grant: an admin's alone, and it opens the menu
		// entry, the page and its API; taking it back closes them.
		expect(patch(member.id, { restrict: { hide_private: false, hide_groups: [], join_chat: true } })[0]).toBe(400);
		const [gst, granted] = patch(admin.id, { restrict: { hide_private: false, hide_groups: [], join_chat: true } });
		expect(gst).toBe(200);
		expect((granted as { restrict: { join_chat: boolean } }).restrict.join_chat).toBe(true);
		expect(pageData('/', 'admin').viewer.nav).toContain('join_chat');
		expect(pageData('/join-chat', 'admin').route).toBe('join_chat');
		expect(adminApi('POST', 'join-chat/preview', { link: 'https://t.me/example' }, 'admin')[0]).not.toBe(404);
		expect(patch(admin.id, { restrict: { hide_private: false, hide_groups: [], join_chat: false } })[0]).toBe(200);
		expect(pageData('/', 'admin').viewer.nav).not.toContain('join_chat');
		expect(pageData('/join-chat', 'admin').route).toBe('not_found');
		expect(adminApi('POST', 'join-chat/preview', { link: 'https://t.me/example' }, 'admin')[0]).toBe(404);
		expect(patch(admin.id, { restrict: { ...hide, hide_groups: [-1] } })[1]).toMatchObject({ error: 'bad_request', field: 'hide_groups' });
		const [st, row] = patch(admin.id, { restrict: hide });
		expect(st).toBe(200);
		expect((row as { restrict: { hide_private: boolean } }).restrict.hide_private).toBe(true);
		// The admin then has no private messages: menu, pages, chats.
		expect(pageData('/', 'admin').viewer.nav).not.toContain('private_messages');
		expect(pageData('/private_messages', 'admin').status).toBe(404);
		patch(admin.id, { restrict: { ...hide, hide_private: false } });
		expect(pageData('/', 'admin').viewer.nav).toContain('private_messages');
		// A member made admin in the same PATCH may be restricted; leaving
		// the admin role ends it.
		expect(patch(member.id, { role: 'admin', restrict: hide })[0]).toBe(200);
		const [, back] = patch(member.id, { role: 'member' });
		expect((back as { restrict: { hide_private: boolean } | null }).restrict?.hide_private ?? false).toBe(false);
	});

	it('hiding groups from an admin, as Go takes it', () => {
		const accts = (pageData('/settings/accounts', 'superadmin').props as { accounts: { id: number; role: string }[] }).accounts;
		const admin = accts.find((a) => a.id === 3)!;
		const member = accts.find((a) => a.role === 'member')!;
		const patch = (id: number, body: object, who: Persona = 'superadmin') => adminApi('PATCH', `accounts/${id}`, body, who);
		const g = (pageData('/groups', 'admin').props as { rows: { id: number }[] }).rows[0]!.id;
		const hide = { hide_private: false, hide_groups: [g], join_chat: false };
		expect(patch(admin.id, { restrict: hide }, 'admin')[0]).toBe(403);
		expect(patch(member.id, { restrict: hide })[0]).toBe(400);
		const [st, row] = patch(admin.id, { restrict: hide });
		expect(st).toBe(200);
		expect((row as { restrict: { hide_groups: { id: number }[] } }).restrict.hide_groups.map((x) => x.id)).toEqual([g]);
		ok(200, row, '/api/accounts/{id}', 'patch');
		// The admin then no longer has the group: its page, its list row.
		expect(pageData(`/groups/${g}`, 'admin').status).toBe(404);
		expect((pageData('/groups', 'admin').props as { rows: { id: number }[] }).rows.some((r) => r.id === g)).toBe(false);
		// The list replaces the stored one; leaving the admin role clears it.
		patch(admin.id, { restrict: { ...hide, hide_groups: [] } });
		expect(pageData(`/groups/${g}`, 'admin').status).toBe(200);
		patch(member.id, { role: 'admin', restrict: hide });
		const [, back] = patch(member.id, { role: 'member' });
		expect((back as { restrict: { hide_groups: unknown[] } | null }).restrict?.hide_groups ?? []).toEqual([]);
	});

	it.each(['superadmin', 'member'] as Persona[])('the dashboard series of every period, as %s', (p) => {
		const now = Date.parse('2026-11-17T16:00:00Z') / 1000;
		const ranges: Record<string, number> = { minute: 86400, hour: 48 * 3600, '4h': 180 * 86400, day: 400 * 86400, week: 3000 * 86400, month: now, quarter: now, year: now };
		for (const [period, len] of Object.entries(ranges)) {
			for (const end of ['now', String(now - 3600)]) {
				const [status, body] = dashboardSeries(p, new URLSearchParams({ period, start: String(Math.max(0, (end === 'now' ? now : now - 3600) - len)), end }));
				ok(status, body, '/api/dashboard/series');
			}
		}
	});

	it('the dashboard chart\'s range a PATCH takes, with the stored preferences', () => {
		const day = 86400;
		expect(badRange({}, { dashboard_period: 'minute', dashboard_last: day })).toBeNull();
		expect(badRange({}, { dashboard_period: 'minute', dashboard_last: day + 1 })).toBe('dashboard_last');
		expect(badRange({ dashboard_period: 'minute' }, { dashboard_last: day + 1 })).toBe('dashboard_last');
		expect(badRange({}, { dashboard_start: 100 })).toBe('dashboard_end');
		expect(badRange({}, { dashboard_end: 100 })).toBe('dashboard_start');
		expect(badRange({ dashboard_start: 10 }, { dashboard_end: 100 })).toBeNull();
		expect(badRange({}, { dashboard_start: 100, dashboard_end: 100 })).toBe('dashboard_start');
		expect(badRange({ dashboard_start: 0, dashboard_end: 2 * day }, { dashboard_period: 'minute' })).toBe('dashboard_start');
		expect(badRange({ dashboard_start: 0, dashboard_end: 2 * day }, { dashboard_start: null, dashboard_end: null })).toBeNull();
		// Month and longer have no limit.
		expect(badRange({}, { dashboard_period: 'year', dashboard_start: 0, dashboard_end: 4e9 })).toBeNull();
		expect(badPref({ dashboard_end: 253402300800 })).toBe('dashboard_end');
		expect(badPref({ dashboard_last: 0 })).toBe('dashboard_last');
	});

	it('the dashboard\'s first series follows the viewer\'s chart preferences', () => {
		const prefs = settingsState('superadmin').prefs;
		const was = { ...prefs };
		try {
			let p = pageData('/', 'superadmin') as { viewer: Record<string, unknown>; props: { series: { period: string; live: boolean; points: unknown[] } } };
			expect(p.viewer).toMatchObject({ dashboard_period: 'hour', dashboard_variant: 'bars', dashboard_last: 172800 });
			expect(p.viewer.dashboard_start).toBeUndefined();
			expect(p.props.series).toMatchObject({ period: 'hour', live: true });
			expect(p.props.series.points).toHaveLength(49);
			Object.assign(prefs, { dashboard_period: 'minute' });
			p = pageData('/', 'superadmin') as typeof p;
			expect(p.viewer).toMatchObject({ dashboard_period: 'minute', dashboard_last: 86400 });
			const now = Date.parse('2026-11-17T16:00:00Z') / 1000;
			Object.assign(prefs, { dashboard_period: 'day', dashboard_start: now - 10 * 86400, dashboard_end: now });
			p = pageData('/', 'superadmin') as typeof p;
			expect(p.viewer).toMatchObject({ dashboard_period: 'day', dashboard_start: now - 10 * 86400, dashboard_end: now });
			expect(p.viewer.dashboard_last).toBeUndefined();
			expect(p.props.series).toMatchObject({ period: 'day', live: false });
		} finally {
			for (const k of Object.keys(prefs)) delete prefs[k];
			Object.assign(prefs, was);
		}
	});

	it('the photos of a user and of a group', () => {
		const [us, ub] = userPhotos(String(sampleContacts()[0]), 'superadmin');
		ok(us, ub, '/api/users/{id}/photos');
		const [gs, gb] = groupPhotos(String(groupId), 'superadmin');
		ok(gs, gb, '/api/groups/{id}/photos');
		expect(userPhotos(String(sampleContacts()[0]), 'member')[0]).toBe(404);
	});

	it('a federation, its groups and a change of it', () => {
		ok(...adminApi('GET', 'federations/1', {}, 'superadmin'), '/api/federations/{id}');
		for (const q of ['', 'limit=3', 'limit=3&cursor=3', 'q=dev']) ok(...federationGroups(0, new URLSearchParams(q), 'superadmin'), '/api/federations/{id}/groups');
		ok(...adminApi('PATCH', 'federations/1', { description: 'For the contract' }, 'superadmin'), '/api/federations/{id}', 'patch');
	});

	it('a change of an exposed group\'s note', () => {
		const op = spec.paths['/api/mcp/groups/{group_id}']?.patch as { requestBody: { content: { 'application/json': { schema: Schema } } }; responses: Record<string, unknown> };
		const body = op.requestBody.content['application/json'].schema;
		const id = (pageData('/mcp-admin/groups', 'superadmin').props as { groups: { group: { id: number } }[] }).groups[0]!.group.id;
		for (const note of ['For the contract', '']) {
			expect(validate(spec, body, { note })).toEqual([]);
			expect(adminApi('PATCH', `mcp/groups/${id}`, { note }, 'superadmin')).toEqual([204, null]);
		}
		expect(validate(spec, body, {})).not.toEqual([]);
		const answers = [adminApi('PATCH', `mcp/groups/${id}`, {}, 'superadmin')[0], adminApi('PATCH', 'mcp/groups/-1', { note: '' }, 'superadmin')[0]];
		expect(answers).toEqual([400, 404]);
		for (const st of [204, ...answers]) expect(op.responses).toHaveProperty(String(st));
	});

	it('a fetch from Telegram, of a big file (partial) and of a small one (stored)', () => {
		const rows = (pageData('/files?view=list&per=200&f=' + encodeURIComponent('[{"field":"stored","op":"is","value":"no"}]'), 'superadmin').props as { rows: { url: string | null; fetch?: string | null; size: number }[] }).rows;
		const token = (r: { fetch?: string | null }) => r.fetch!.split('/')[3]!;
		const answers = (r: { fetch?: string | null }) => !!r.fetch && fetchFileApi(token(r))[0] === 200;
		const big = rows.find((r) => r.url === null && answers(r));
		const small = rows.find((r) => r.url !== null && answers(r));
		expect(big && small).toBeTruthy();
		for (const [r, state] of [[big!, 'partial'], [small!, 'stored']] as const) {
			const [status, body] = fetchFileApi(token(r));
			ok(status, body, '/api/files/{token}/fetch', 'post');
			expect((body as { state: string }).state).toBe(state);
		}
		expect(fetchFileApi('nope')[0]).toBe(404);
		// Some file is gone from Telegram.
		expect(rows.some((r) => r.fetch && fetchFileApi(token(r))[0] === 410)).toBe(true);
	});

	it('a name lookup', () => {
		const [status, body] = lookupName(String(sampleContacts()[0]), 'superadmin');
		ok(status, body, '/api/users/{id}/lookup-name', 'post');
	});

	it('adding a bot', () => {
		const [status, body] = adminApi('POST', 'bots', { kind: 'telegram', name: 'Contract', token: '7000000099:' + 'k'.repeat(40) }, 'superadmin');
		expect(status).toBe(201);
		expect(validate(spec, responseSchema(spec, '/api/bots', 'post', '201'), wire(body))).toEqual([]);
	});

	it.each(['superadmin', 'member'] as Persona[])('the entity lookup, as %s', (p) => {
		const [status, body] = entities(p, new URLSearchParams(`users=${userId},1,999999999&groups=${groupId},${memberGroupId}`));
		ok(status, body, '/api/entities');
	});
});

describe('the kind samples', () => {
	it('are each on a message page of their own, for a full viewer and a member', () => {
		expect(sampleIds.length).toBeGreaterThanOrEqual(23);
		for (const persona of ['superadmin', 'member'] as Persona[]) for (const id of sampleIds) expect(pageData(`/messages/group/${id}`, persona).status).toBe(200);
	});
});

describe('the message lists with a search engine', () => {
	const checked = (url: string, persona: Persona) => {
		const page = pageData(url, persona);
		return validate(spec, ref(variants.get(page.route)!), wire(page));
	};
	it.each(['available', 'behind'] as const)('answer as the description says while it is %s', (state) => {
		setEngine(state);
		try {
			for (const persona of ['superadmin', 'member'] as Persona[]) {
				for (const url of ['/group_messages', '/group_messages?q=kernel&highlight=1&facets=1&fuzzy=1', '/group_messages?f=' + encodeURIComponent('[{"field":"text","op":"matches","value":"rust"}]') + '&highlight=1']) {
					expect(checked(url, persona)).toEqual([]);
				}
			}
			expect(checked('/private_messages?q=thanks&highlight=1&facets=1', 'superadmin')).toEqual([]);
			const p = pageData('/group_messages?q=kernel&highlight=1&facets=1', 'superadmin').props as { engine?: { available: boolean }; highlights?: unknown[]; rows: unknown[]; facets?: unknown[] };
			expect(p.engine?.available).toBe(state === 'available');
			if (state === 'available') { expect(p.highlights).toHaveLength(p.rows.length); expect(p.facets?.length).toBeGreaterThan(0); }
		} finally { setEngine('off'); }
	});
});

describe('the users and groups lists with a search engine', () => {
	const checked = (url: string, persona: Persona) => {
		const page = pageData(url, persona);
		return validate(spec, ref(variants.get(page.route)!), wire(page));
	};
	const f = (field: string, value: string) => 'f=' + encodeURIComponent(JSON.stringify([{ field, op: 'matches', value }]));
	it.each(['available', 'behind'] as const)('answer as the description says while it is %s', (state) => {
		setEngine(state);
		try {
			for (const persona of ['superadmin', 'member'] as Persona[]) {
				for (const url of ['/users', `/users?${f('bio', 'kernel')}&highlight=1&fuzzy=1&facets=1`, '/groups', `/groups?${f('description', 'spam')}&highlight=1`]) {
					expect(checked(url, persona)).toEqual([]);
				}
			}
			type P = { engine?: { available: boolean; fields: string[]; options: string[] }; backend?: { ignored?: string[] }; highlights?: string[][]; rows: unknown[] };
			const u = pageData(`/users?${f('bio', 'kernel')}&highlight=1&facets=1`, 'superadmin').props as P;
			expect(u.engine?.fields).toEqual(['bio']);
			expect(u.engine?.options).not.toContain('facets');
			expect(u.backend?.ignored).toEqual(state === 'available' ? ['facets'] : ['highlight', 'facets']);
			const g = pageData(`/groups?${f('description', 'spam')}&highlight=1`, 'superadmin').props as P;
			expect(g.engine?.fields).toEqual(['description']);
			if (state === 'available') {
				expect(u.rows.length).toBeGreaterThan(0);
				expect(u.highlights).toHaveLength(u.rows.length);
				expect(u.highlights!.flat().join(' ')).toContain('<mark>');
				expect(g.highlights!.flat().join(' ')).toContain('<mark>spam</mark>');
			}
			// A member sees no bios, so nothing of the engine on users.
			expect((pageData('/users', 'member').props as P).engine).toBeUndefined();
		} finally { setEngine('off'); }
	});
	it('are listed in the capabilities, users only to full viewers', () => {
		setEngine('available');
		try {
			for (const persona of ['superadmin', 'member', 'public'] as Persona[]) expect(validate(spec, responseSchema(spec, '/api/search/capabilities', 'get'), wire(searchCapabilities(persona)))).toEqual([]);
			const keys = (p: Persona) => Object.keys(searchCapabilities(p).entities);
			expect(keys('superadmin')).toEqual(['group_messages', 'private_messages', 'users', 'groups']);
			expect(keys('member')).toEqual(['group_messages', 'groups']);
			expect((searchCapabilities('superadmin').entities as Record<string, { fields: string[] }>).users!.fields).toEqual(['bio']);
		} finally { setEngine('off'); }
		expect(searchCapabilities('superadmin')).toEqual({ engine: null, entities: {} });
	});
});
