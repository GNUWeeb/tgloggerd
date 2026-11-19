// @vitest-environment happy-dom
// Per-account forwarding: a new route's federation, asked for once its
// chat is picked and only when the chat has more than one to go into,
// and the refusals of a create said in words.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, json, renderPage, setup, type Call } from '../test/page.tsx';
import type { PickItem } from '../types.ts';

beforeEach(setup);
const T2D = '/platform-fwd/telegram-discord';
const D2T = '/platform-fwd/discord-telegram';
const dialog = () => screen.getByRole('dialog');
const popup = () => waitFor(() => dialog().querySelector('.a-picker-pop') as HTMLElement);
const fill = (sel: string, value: string) => fireEvent.change(dialog().querySelector(sel)!, { target: { value } });
const save = () => within(dialog()).getByRole('button', { name: 'Save' }) as HTMLButtonElement;
const fedSelect = () => within(dialog()).queryByRole('combobox', { name: /Federation/ }) as HTMLSelectElement | null;
const RUST: PickItem = { id: -100777, title: 'Rust Programming', kind: 'group' };
const TWO = [{ id: 0, name: 'Default' }, { id: 2, name: 'Rust and systems' }];
const chats = { 'GET /api/chats/search': () => ({ items: [RUST], next_cursor: null }) };

async function pickChat() {
	fireEvent.click(dialog().querySelector('.a-picker-btn')!);
	fireEvent.click(await within(await popup()).findByRole('button', { name: /Rust Programming/ }));
}

async function newWebhook(feds: (c: Call) => unknown, post: (c: Call) => unknown = () => json(201, { id: 50 })) {
	const calls = fakeApi('superadmin', { ...chats, 'GET /api/forwarding/federations': feds, 'POST /api/forwarding/telegram-discord': post });
	renderPage(T2D);
	fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
	await pickChat();
	fill('#webhook-url', 'https://discord.com/api/webhooks/1/abc');
	return calls;
}

describe('a new route\'s federation', () => {
	it('asks for the chat\'s federations once it is picked, and files it in the only one without asking', async () => {
		const calls = await newWebhook(() => ({ federations: [{ id: 1, name: 'GNU/Weeb network' }] }));
		await waitFor(() => expect(save().disabled).toBe(false));
		expect(callsTo(calls, 'GET', '/api/forwarding/federations').map((c) => c.query.get('chat_id'))).toEqual([String(RUST.id)]);
		expect(fedSelect()).toBeNull();
		fireEvent.click(save());
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')[0]!.body).toMatchObject({ federation_id: 1 });
	});

	it('waits for a pick between several, and sends the one picked', async () => {
		const calls = await newWebhook(() => ({ federations: TWO }));
		const sel = await within(dialog()).findByRole('combobox', { name: /Federation/ }) as HTMLSelectElement;
		expect([...sel.options].map((o) => o.textContent)).toEqual(['Choose a federation...', 'Default', 'Rust and systems']);
		expect(save().disabled).toBe(true);
		// Default is id 0: going back to the placeholder is no pick.
		fireEvent.change(sel, { target: { value: '2' } });
		fireEvent.change(sel, { target: { value: '' } });
		expect(save().disabled).toBe(true);
		fireEvent.change(sel, { target: { value: '2' } });
		expect(save().disabled).toBe(false);
		fireEvent.click(save());
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/telegram-discord')[0]!.body).toMatchObject({ chat_id: RUST.id, federation_id: 2 });
	});

	it('does not ask when editing a route', async () => {
		const calls = fakeApi('superadmin');
		renderPage(T2D);
		fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0]!);
		expect(fedSelect()).toBeNull();
		expect(callsTo(calls, 'GET', '/api/forwarding/federations')).toHaveLength(0);
	});

	it.each([
		['federation_required', 'This chat is in more than one federation: choose the one it goes into.'],
		['bad_federation', 'This chat cannot go into that federation. Choose one from the list.'],
	])('says what %s means', async (error, words) => {
		await newWebhook(() => ({ federations: [{ id: 0, name: 'Default' }] }), () => json(400, { error }));
		await waitFor(() => expect(save().disabled).toBe(false));
		fireEvent.click(save());
		await within(dialog()).findByText(words);
	});

	it('says when the federations could not be loaded, and leaves Save to the server', async () => {
		await newWebhook(() => json(500, { error: 'internal', message: 'Something broke.' }));
		await within(dialog()).findByText(/The federations of this chat could not be loaded: Something broke\./);
		expect(save().disabled).toBe(false);
	});

	it('asks on Discord to Telegram too', async () => {
		const ref = (id: number, kind: 'discord' | 'telegram', name: string) => ({ id, kind, name, identity: null, status: 'connected' as const, enabled: true });
		const BOTS = [ref(2, 'discord', 'Discord reader'), ref(1, 'telegram', 'Logger relay')];
		const calls = fakeApi('superadmin', {
			...chats,
			'GET /api/bots': (c) => ({ bots: BOTS.filter((b) => b.kind === c.query.get('kind')) }),
			'GET /api/forwarding/federations': () => ({ federations: TWO }),
			'POST /api/forwarding/discord-telegram': () => json(201, { id: 9 }),
		});
		renderPage(D2T);
		fireEvent.click(screen.getByRole('button', { name: /Add route/ }));
		fill('input[inputmode=numeric]', '1093447120571433021');
		fireEvent.change(await within(dialog()).findByRole('combobox', { name: 'Discord bot' }), { target: { value: '2' } });
		fireEvent.change(within(dialog()).getByRole('combobox', { name: 'Telegram bot' }), { target: { value: '1' } });
		await pickChat();
		const sel = await within(dialog()).findByRole('combobox', { name: /Federation/ }) as HTMLSelectElement;
		expect(save().disabled).toBe(true);
		fireEvent.change(sel, { target: { value: '0' } });
		fireEvent.click(save());
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/forwarding/discord-telegram')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/forwarding/discord-telegram')[0]!.body).toMatchObject({ chat_id: RUST.id, federation_id: 0 });
	});
});
