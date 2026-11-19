// The stand-in's dummy data against the API description: every page it
// serves, as each sample account sees it, must match its page schema in
// webapi/openapi.yaml, and so must the chat, search and entity answers.
// The page tests run on this data, so it is the contract the Go loaders
// are held to; a mismatch here means the page tests and the description
// disagree, and one of them must change.
import { describe, expect, it } from 'vitest';
import { setEngine, viewOf, adminApi, federationGroups, badPref, groupPhotos, userPhotos, perDayStart, settingsState, chatFor, chatWindow, entities, historyApi, lookupName, pageData, sampleContacts, searchChats, type Persona } from '../../standin/fixtures.ts';
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
const sampleIds = (chatStream(KINDS_GROUP) as { id: number; message_id: number; chat: { id: number; kind: string } }[]).filter((m) => sampleOf(m)).map((m) => m.id);

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
		expect(patch(admin.id, { restrict: { ...hide, join_chat: true } })[1]).toMatchObject({ error: 'not_supported' });
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

	it('the first per-day bar\'s date, in the viewer\'s timezone', () => {
		// The stand-in's now is 17 Nov 16:00 UTC: 23:00 in Jakarta (the
		// site's zone), already 18 Nov 05:00 in Auckland.
		expect(perDayStart('superadmin', 30)).toBe('2026-10-19');
		const prefs = settingsState('superadmin').prefs;
		const was = prefs.timezone;
		prefs.timezone = 'Pacific/Auckland';
		try {
			expect(perDayStart('superadmin', 30)).toBe('2026-10-20');
			const p = pageData('/', 'superadmin').props as { per_day: number[]; per_day_start: string };
			expect(p.per_day).toHaveLength(30);
			expect(p.per_day_start).toBe('2026-10-20');
		} finally {
			if (was === undefined) delete prefs.timezone; else prefs.timezone = was;
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
