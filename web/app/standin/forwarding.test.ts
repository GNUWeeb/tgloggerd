// The stand-in's forwarding routes, as the Go service answers them: each
// direction's type options in order, and a write that takes the whole
// set and refuses an unknown type or an empty one; the federations a
// new route may go into, and who may be given one.
import { describe, expect, it } from 'vitest';
import { adminApi, pageData, type Persona } from './fixtures.ts';
import { groups } from '../../mockup/src/data/groups.js';
import { users } from '../../mockup/src/data/users.js';
import { D2T_OPTIONS, T2D_OPTIONS, migrated } from './fwdtypes.ts';

type Props = { type_options: string[]; routes: { id: number; types: string[] }[] };
const props = (url: string) => (pageData(url, 'superadmin') as unknown as { props: Props }).props;

describe('stand-in forwarding types', () => {
	it('sends each direction its options, and route types in that order', () => {
		for (const [url, opts] of [['/platform-fwd/telegram-discord', T2D_OPTIONS], ['/platform-fwd/discord-telegram', D2T_OPTIONS]] as const) {
			const p = props(url);
			expect(p.type_options).toEqual(opts);
			for (const r of p.routes) expect(r.types).toEqual(opts.filter((t) => r.types.includes(t)));
		}
	});

	it('widens an old set as the migration does', () => {
		expect(migrated('t2d', ['text', 'other'])).toEqual(['text', ...T2D_OPTIONS.filter((t) => !['text', 'photo', 'video', 'document', 'sticker'].includes(t))]);
		expect(migrated('t2d', ['photo', 'text'])).toEqual(['text', 'photo']);
		// audio follows document (000048) and voice follows audio (000050).
		expect(migrated('d2t', ['document', 'photo'])).toEqual(['photo', 'animation', 'document', 'audio', 'voice']);
		// Embeds, polls and system messages were posted as text.
		expect(migrated('d2t', ['text'])).toEqual(['text', 'embed', 'poll', 'system']);
	});

	it('refuses an unknown type or an empty set, and stores a set in option order', () => {
		for (const dir of ['telegram-discord', 'discord-telegram']) {
			const id = props(`/platform-fwd/${dir}`).routes[0]!.id;
			expect(adminApi('PATCH', `forwarding/${dir}/${id}`, { types: [] }, 'superadmin')[0]).toBe(400);
			expect(adminApi('PATCH', `forwarding/${dir}/${id}`, { types: ['text', 'hologram'] }, 'superadmin')[0]).toBe(400);
			expect(adminApi('PATCH', `forwarding/${dir}/${id}`, { types: ['sticker', 'text'] }, 'superadmin')[0]).toBe(204);
			expect(props(`/platform-fwd/${dir}`).routes.find((r) => r.id === id)!.types).toEqual(['text', 'sticker']);
		}
		expect(adminApi('PATCH', 'forwarding/discord-telegram/1', { types: ['video_note'] }, 'superadmin')[0]).toBe(400);
		expect(adminApi('POST', 'forwarding/telegram-discord', { chat_id: -1, chat_kind: 'group', webhook_url: 'https://discord.com/api/webhooks/1/x', types: [], enabled: true }, 'superadmin')[0]).toBe(400);
	});
});

type Route = { id: number; owner: { id: number }; federation: { id: number; name: string }; transfer_candidates: { id: number }[] };
const routes = (url: string, persona: Persona = 'superadmin') => (pageData(url, persona) as unknown as { props: { routes: Route[] } }).props.routes;
const choices = (chat: number, persona: Persona) => {
	const [status, out] = adminApi('GET', 'forwarding/federations', { chat_id: String(chat) }, persona);
	return status === 200 ? (out as { federations: { id: number }[] }).federations.map((f) => f.id) : status;
};
const HOOK = 'https://discord.com/api/webhooks/1/abc';

describe('stand-in forwarding federations and transfers', () => {
	// groups[3] is in federations 1 and 2, groups[2] in 2 only, groups[1]
	// in none; gw_admin administers 1 and Default, the member is granted
	// groups[0] in 1.
	it('answers the federations holding a chat that are the viewer\'s own, else all holders', () => {
		expect(choices(groups[3]!.id, 'superadmin')).toEqual([1, 2]);
		expect(choices(groups[1]!.id, 'superadmin')).toEqual([0]);
		expect(choices(users[3]!.id, 'superadmin')).toEqual([0]);
		expect(choices(groups[3]!.id, 'admin')).toEqual([1]);
		expect(choices(groups[2]!.id, 'admin')).toEqual([2]);
		expect(choices(groups[0]!.id, 'member')).toEqual([1]);
		expect(choices(groups[3]!.id, 'member')).toBe(404);
		expect(choices(-1, 'superadmin')).toBe(404);
	});

	it('needs a federation on create only when there is a choice, and one of the choices', () => {
		const post = (chat_id: number, federation_id?: number) => adminApi('POST', 'forwarding/telegram-discord', { chat_id, chat_kind: 'group', webhook_url: HOOK, types: ['text'], enabled: true, federation_id }, 'superadmin');
		expect(post(groups[3]!.id)).toEqual([400, expect.objectContaining({ error: 'federation_required' })]);
		expect(post(groups[3]!.id, 3)).toEqual([400, expect.objectContaining({ error: 'bad_federation' })]);
		expect(post(groups[0]!.id, 2)).toEqual([400, expect.objectContaining({ error: 'bad_federation' })]);
		const [s2, made] = post(groups[3]!.id, 2);
		expect(s2).toBe(201);
		expect(routes('/platform-fwd/telegram-discord').find((r) => r.id === (made as { id: number }).id)!.federation).toEqual({ id: 2, name: 'Rust and systems' });
		const [s1, one] = post(groups[0]!.id);
		expect(s1).toBe(201);
		expect(routes('/platform-fwd/telegram-discord').find((r) => r.id === (one as { id: number }).id)!.federation.id).toBe(1);
		const d2t = adminApi('POST', 'forwarding/discord-telegram', { discord_channel_id: '1093447120571433999', chat_id: groups[3]!.id, discord_bot_id: 2, telegram_bot_id: 1, types: ['text'], enabled: true }, 'superadmin');
		expect(d2t).toEqual([400, expect.objectContaining({ error: 'federation_required' })]);
	});

	it('offers transfer candidates by role, the owner left out', () => {
		const t2d = (persona: Persona) => routes('/platform-fwd/telegram-discord', persona);
		const first = t2d('superadmin').find((r) => r.id === 1)!; // owner 1, in federation 1
		expect(first.transfer_candidates.map((a) => a.id)).toEqual([2, 3, 4, 5, 6]);
		// gw_admin: federation 1's admins and its members holding forward
		// to whom the group is granted; nobody for a federation not its own.
		expect(t2d('admin').find((r) => r.id === 1)!.transfer_candidates.map((a) => a.id)).toEqual([3, 5]);
		expect(t2d('admin').find((r) => r.id === 2)!.transfer_candidates.map((a) => a.id)).toEqual([]);
		for (const r of t2d('member')) expect(r.transfer_candidates).toEqual([]);
	});

	it('transfers to a candidate only, and a route only to a member owning both bots', () => {
		const give = (dir: string, id: number, owner_id: number, persona: Persona = 'superadmin') => adminApi('POST', `forwarding/${dir}/${id}/transfer`, { owner_id }, persona);
		expect(give('telegram-discord', 5, 1, 'member')[0]).toBe(404);
		expect(give('telegram-discord', 1, 7)[0]).toBe(400); // disabled
		expect(give('telegram-discord', 1, 1)[0]).toBe(400); // already its owner
		// Route 1 reads with gw_admin's Discord bot and sends with the
		// superadmin's Telegram bot: not both the member's.
		expect(give('discord-telegram', 1, 5)).toEqual([409, expect.objectContaining({ error: 'bot_not_owned' })]);
		expect(give('discord-telegram', 1, 2)).toEqual([204, null]);
		expect(routes('/platform-fwd/discord-telegram').find((r) => r.id === 1)!.owner.id).toBe(2);
		expect(give('telegram-discord', 1, 3, 'admin')).toEqual([204, null]);
		expect(routes('/platform-fwd/telegram-discord').find((r) => r.id === 1)!.owner.id).toBe(3);
	});
});
