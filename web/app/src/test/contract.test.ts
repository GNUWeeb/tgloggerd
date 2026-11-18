// The stand-in's dummy data against the API description: every page it
// serves, as each sample account sees it, must match its page schema in
// webapi/openapi.yaml, and so must the chat, search and entity answers.
// The page tests run on this data, so it is the contract the Go loaders
// are held to; a mismatch here means the page tests and the description
// disagree, and one of them must change.
import { describe, expect, it } from 'vitest';
import { adminApi, chatFor, chatWindow, entities, historyApi, lookupName, pageData, sampleContacts, searchChats, type Persona } from '../../standin/fixtures.ts';
import { loadSpec, responseSchema, validate, type Schema } from './schema.ts';

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

const PAGES: [Persona, string][] = [
	...(['superadmin', 'admin', 'member'] as Persona[]).flatMap((a): [Persona, string][] => [
		[a, '/'], [a, '/users'], [a, '/users?q=a&sort=msg_count&dir=asc&page=2'], [a, '/groups'], [a, `/groups/${a === 'member' ? memberGroupId : groupId}`],
		[a, `/groups/${a === 'member' ? memberGroupId : groupId}/admins`], [a, `/groups/${a === 'member' ? memberGroupId : groupId}/history`],
		[a, `/groups/${a === 'member' ? memberGroupId : groupId}/chat`], [a, '/files'], [a, `/files?view=list&file=${fileId}`],
		[a, '/group_messages'], [a, '/group_messages?q=rust&f=' + encodeURIComponent('[{"field":"is_deleted","op":"is","value":"yes"}]')],
		[a, '/settings'], [a, '/settings/preferences'], [a, '/settings/sessions'],
		[a, '/platform-fwd'], [a, '/platform-fwd/telegram-discord'], [a, '/platform-fwd/discord-telegram'],
		[a, '/platform-fwd/bots'], 		[a, '/settings/accounts'], [a, '/settings/federations'], [a, '/join-chat'], [a, '/mcp-admin'], [a, '/mcp-admin/groups'], [a, '/mcp-admin/tokens'],
		[a, '/no/such/page'],
	]),
	['superadmin', `/files?view=list&file=${orphanId}`], ['admin', `/files?view=list&file=${orphanId}`],
	['superadmin', `/users/${userId}`], ['superadmin', `/users/${userId}/history`], ['superadmin', `/users/${userId}/chat`],
	['member', `/users/${userId}`], ['superadmin', '/private_messages'], ['superadmin', `/messages/group/${groupMsg}`],
	['superadmin', `/messages/private/${privateMsg}`], ['superadmin', `/groups/${groupId}/chat?at=1`],
	['public', `/groups/${exposedId}/chat`], ['public', '/login'], ['public', '/login?next=%2Fusers'],
	['public', '/password/forgot'], ['public', '/password/set'], ['public', '/account/email/verify'], ['superadmin', '/password/forgot'],
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
