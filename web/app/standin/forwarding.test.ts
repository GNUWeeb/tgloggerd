// The stand-in's forwarding routes, as the Go service answers them: each
// direction's type options in order, and a write that takes the whole
// set and refuses an unknown type or an empty one.
import { describe, expect, it } from 'vitest';
import { adminApi, pageData } from './fixtures.ts';
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
