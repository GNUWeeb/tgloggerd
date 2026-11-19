// @vitest-environment happy-dom
// A forwarding route's content types: presets, groups with a tri-state
// box, an empty set refused, each direction's own options, and a type
// the app does not know shown by its name.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { D2TProps, T2DProps } from './forwarding.tsx';

beforeEach(setup);
const T2D = '/platform-fwd/telegram-discord';
const D2T = '/platform-fwd/discord-telegram';
const dialog = () => screen.getByRole('dialog');
const box = (name: string) => within(dialog()).getByRole('checkbox', { name }) as HTMLInputElement;
const preset = () => within(dialog()).getByRole('radio', { checked: true }).textContent;
const groupRow = (name: string) => box(name).closest('li')!;
const groupNames = () => [...dialog().querySelectorAll('.a-typegroup-name')].map((x) => x.textContent);

function addWebhook() {
	fakeApi('superadmin');
	renderPage(T2D);
	fireEvent.click(screen.getByRole('button', { name: /Add webhook/ }));
}

describe('forwarding content types', () => {
	it('starts a new route with every option, its groups closed', () => {
		addWebhook();
		expect(preset()).toBe('Everything');
		expect(groupNames()).toEqual(['Text', 'Media', 'Audio', 'Polls and games', 'Places and contacts', 'Telegram extras', 'Service messages', 'Everything else']);
		expect(box('Media').checked).toBe(true);
		expect(within(groupRow('Media')).getByText('6 of 6')).toBeTruthy();
		expect(dialog().querySelectorAll('.a-checkchip')).toHaveLength(0);
	});

	it('applies the presets, and names the one the set equals', () => {
		addWebhook();
		fireEvent.click(within(dialog()).getByRole('radio', { name: 'Text only' }));
		expect(preset()).toBe('Text only');
		expect(box('Text').checked).toBe(true);
		expect(box('Media').checked).toBe(false);
		fireEvent.click(within(dialog()).getByRole('radio', { name: 'Messages and media' }));
		expect(preset()).toBe('Messages and media');
		for (const g of ['Text', 'Media', 'Audio', 'Polls and games', 'Places and contacts']) expect(box(g).checked, g).toBe(true);
		for (const g of ['Telegram extras', 'Service messages', 'Everything else']) expect(box(g).checked, g).toBe(false);
		// Turning the missing groups on by hand lands on Everything again.
		for (const g of ['Telegram extras', 'Service messages', 'Everything else']) fireEvent.click(box(g));
		expect(preset()).toBe('Everything');
	});

	it('toggles a whole group, and shows a mixed one open with its count', () => {
		addWebhook();
		fireEvent.click(box('Media'));
		expect(box('Media').checked).toBe(false);
		expect(box('Media').getAttribute('aria-checked')).toBe('false');
		expect(within(groupRow('Media')).getByText('0 of 6')).toBeTruthy();
		expect(preset()).toBe('Custom');
		fireEvent.click(box('Media'));
		expect(box('Media').checked).toBe(true);
		expect(preset()).toBe('Everything');
		fireEvent.click(within(groupRow('Media')).getByRole('button', { name: 'Show Media' }));
		fireEvent.click(box('GIFs'));
		expect(box('Media').getAttribute('aria-checked')).toBe('mixed');
		expect(box('Media').indeterminate).toBe(true);
		expect(within(groupRow('Media')).getByText('5 of 6')).toBeTruthy();
		expect(within(groupRow('Media')).getByRole('button', { name: 'Hide Media' }).getAttribute('aria-expanded')).toBe('true');
		// A mixed group's box turns it all on.
		fireEvent.click(box('Media'));
		expect(box('Media').checked).toBe(true);
		expect(box('GIFs').checked).toBe(true);
	});

	it('opens a group that starts mixed, and keeps a full one closed', () => {
		const r = fixture<T2DProps>(T2D).props.routes.find((x) => x.types.includes('photo') && !x.types.includes('video'))!;
		fakeApi('superadmin');
		renderPage(T2D);
		const row = screen.getAllByRole('button', { name: 'Edit' }).find((b) => b.closest('tr')!.textContent!.includes(r.chat.title))!;
		fireEvent.click(row);
		expect(box('Photos').checked).toBe(true);
		expect(box('Videos').checked).toBe(false);
		expect(within(groupRow('Text')).queryByRole('button')).toBeNull();
	});

	it('refuses an empty set', () => {
		addWebhook();
		fireEvent.click(within(dialog()).getByRole('radio', { name: 'Text only' }));
		fireEvent.click(box('Text'));
		expect(within(dialog()).getByRole('alert').textContent).toContain('Choose at least one kind of message');
		expect((within(dialog()).getByRole('button', { name: 'Save' }) as HTMLButtonElement).disabled).toBe(true);
	});

	it('sends the set in option order', async () => {
		const r = fixture<T2DProps>(T2D).props.routes[0]!;
		const calls = fakeApi('superadmin', { 'PATCH /api/forwarding/telegram-discord/:id': () => undefined });
		renderPage(T2D);
		fireEvent.click(screen.getAllByRole('button', { name: 'Edit' })[0]!);
		fireEvent.click(within(dialog()).getByRole('radio', { name: 'Text only' }));
		fireEvent.click(within(groupRow('Media')).getByRole('button', { name: 'Show Media' }));
		fireEvent.click(box('Stickers'));
		fireEvent.click(box('Service messages'));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Save' }));
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)).toHaveLength(1));
		expect((callsTo(calls, 'PATCH', `/api/forwarding/telegram-discord/${r.id}`)[0]!.body as { types: string[] }).types).toEqual(['text', 'sticker', 'service']);
	});

	it('shows Discord to Telegram only its own short list', () => {
		fakeApi('superadmin');
		renderPage(D2T);
		fireEvent.click(screen.getByRole('button', { name: /Add route/ }));
		expect(groupNames()).toEqual(['Text', 'Media', 'Audio']);
		expect(within(groupRow('Media')).getByText('5 of 5')).toBeTruthy();
		// Messages and media would be everything here.
		expect(within(dialog()).getAllByRole('radio').map((x) => x.textContent)).toEqual(['Everything', 'Text only']);
		fireEvent.click(within(groupRow('Media')).getByRole('button', { name: 'Show Media' }));
		expect([...dialog().querySelectorAll('.a-checkchip')].map((x) => x.textContent)).toEqual(['Photos', 'Videos', 'GIFs', 'Files', 'Stickers']);
	});

	it('shows an option the app does not know under Other kinds by its name', () => {
		const page = fixture<D2TProps>(D2T);
		fakeApi('superadmin');
		renderPage(D2T, 'superadmin', { props: { ...page.props, type_options: [...page.props.type_options, 'hologram' as never] } });
		fireEvent.click(screen.getByRole('button', { name: /Add route/ }));
		expect(groupNames()).toEqual(['Text', 'Media', 'Audio', 'Other kinds']);
		fireEvent.click(within(groupRow('Other kinds')).getByRole('button', { name: 'Show Other kinds' }));
		expect(box('hologram').checked).toBe(true);
	});
});
