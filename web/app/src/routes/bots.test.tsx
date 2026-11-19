// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { tokenShapeOk, type BotsProps } from './bots.tsx';

beforeEach(setup);
const dialog = () => screen.getByRole('dialog');
const TG = '7000000009:' + 'Q'.repeat(30);
const DC = 'MTIzNDU2Nzg5MDEyMzQ1Njc4.GaBcDe.' + 'x'.repeat(27);

describe('bot token shapes', () => {
	it('take Telegram <digits>:<secret> with 30 or more secret characters, never a fixed length', () => {
		expect(tokenShapeOk('telegram', TG)).toBe(true);
		expect(tokenShapeOk('telegram', '7000000009:' + 'Q'.repeat(60))).toBe(true);
		expect(tokenShapeOk('telegram', '7000000009:' + 'Q'.repeat(29))).toBe(false);
		expect(tokenShapeOk('telegram', 'abc:' + 'Q'.repeat(35))).toBe(false);
	});
	it('take Discord tokens of three base64url parts, the first a Discord id', () => {
		expect(tokenShapeOk('discord', DC)).toBe(true);
		expect(tokenShapeOk('discord', 'bm90LWFuLWlk.GaBcDe.xxxxx')).toBe(false); // "not-an-id"
		expect(tokenShapeOk('discord', 'two.parts')).toBe(false);
	});
});

describe('bots page', () => {
	it('shows every bot with its owner to a superadmin, and only its own to a member', () => {
		const all = fixture<BotsProps>('/platform-fwd/bots').props;
		renderPage('/platform-fwd/bots');
		expect(screen.getAllByRole('columnheader').map((h) => h.textContent)).toContain('Owner');
		expect(document.querySelectorAll('tbody tr')).toHaveLength(all.bots.length);
		const own = fixture<BotsProps>('/platform-fwd/bots', 'member').props;
		expect(own.bots.length).toBeLessThan(all.bots.length);
		expect(own.show_owner).toBe(false);
	});

	it('explains a missing intent and a refused token', () => {
		renderPage('/platform-fwd/bots');
		expect(screen.getByText(/Turn on Message Content Intent/)).toBeTruthy();
		expect(screen.getByText(/refused the token/)).toBeTruthy();
	});

	it('filters by kind', () => {
		const all = fixture<BotsProps>('/platform-fwd/bots').props;
		renderPage('/platform-fwd/bots');
		fireEvent.click(screen.getByRole('tab', { name: 'Telegram' }));
		expect(document.querySelectorAll('tbody tr')).toHaveLength(all.bots.filter((b) => b.kind === 'telegram').length);
	});

	it('adds a bot, refusing a malformed token before asking the server', async () => {
		const calls = fakeApi('superadmin', { 'POST /api/bots': (c) => json(201, { ...(fixture<BotsProps>('/platform-fwd/bots').props.bots[0]!), ...(c.body as object) }) });
		renderPage('/platform-fwd/bots');
		fireEvent.click(screen.getByRole('button', { name: /Add bot/ }));
		fireEvent.click(within(dialog()).getByRole('tab', { name: 'Telegram' }));
		fireEvent.change(dialog().querySelector('#bot-label')!, { target: { value: 'Relay 2' } });
		fireEvent.change(dialog().querySelector('#bot-token')!, { target: { value: '12345:short' } });
		fireEvent.submit(dialog().querySelector('form')!);
		expect(within(dialog()).getByRole('alert').textContent).toMatch(/does not look like a Telegram bot token/);
		expect(callsTo(calls, 'POST', '/api/bots')).toHaveLength(0);
		fireEvent.change(dialog().querySelector('#bot-token')!, { target: { value: TG } });
		fireEvent.submit(dialog().querySelector('form')!);
		await waitFor(() => expect(callsTo(calls, 'POST', '/api/bots')).toHaveLength(1));
		expect(callsTo(calls, 'POST', '/api/bots')[0]!.body).toEqual({ kind: 'telegram', name: 'Relay 2', token: TG });
	});

	it('shows why the server refuses a bot', async () => {
		fakeApi('superadmin', { 'POST /api/bots': () => json(409, { error: 'token_in_use', message: 'Another account has this bot already.' }) });
		renderPage('/platform-fwd/bots');
		fireEvent.click(screen.getByRole('button', { name: /Add bot/ }));
		fireEvent.change(dialog().querySelector('#bot-label')!, { target: { value: 'Mine' } });
		fireEvent.change(dialog().querySelector('#bot-token')!, { target: { value: DC } });
		fireEvent.submit(dialog().querySelector('form')!);
		await within(dialog()).findByText('Another account has this bot already.');
	});

	it('renames a bot without sending a token, and replaces the token only when one is typed', async () => {
		const b = fixture<BotsProps>('/platform-fwd/bots').props.bots[0]!;
		const calls = fakeApi('superadmin', { 'PATCH /api/bots/:id': () => b });
		renderPage('/platform-fwd/bots');
		fireEvent.click(screen.getByRole('button', { name: `Edit ${b.name}` }));
		fireEvent.change(dialog().querySelector('#bot-label')!, { target: { value: 'Renamed' } });
		fireEvent.submit(dialog().querySelector('form')!);
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/bots/${b.id}`)).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', `/api/bots/${b.id}`)[0]!.body).toEqual({ name: 'Renamed' });
	});

	it('switches a bot off, and removes one only after confirming, showing why it cannot be', async () => {
		const b = fixture<BotsProps>('/platform-fwd/bots').props.bots.find((x) => x.enabled)!;
		vi.stubGlobal('confirm', vi.fn().mockReturnValueOnce(false).mockReturnValueOnce(true));
		const calls = fakeApi('superadmin', {
			'PATCH /api/bots/:id': () => b,
			'DELETE /api/bots/:id': () => json(409, { error: 'bot_in_use', message: '2 routes use this bot.' }),
		});
		renderPage('/platform-fwd/bots');
		const row = screen.getByRole('button', { name: `Edit ${b.name}` }).closest('tr')!;
		fireEvent.click(row.querySelector('input.a-switch')!);
		await waitFor(() => expect(callsTo(calls, 'PATCH', `/api/bots/${b.id}`)[0]?.body).toEqual({ enabled: false }));
		fireEvent.click(within(row).getByRole('button', { name: `Remove ${b.name}` }));
		expect(callsTo(calls, 'DELETE', /^\/api\/bots\//)).toHaveLength(0);
		fireEvent.click(within(row).getByRole('button', { name: `Remove ${b.name}` }));
		expect((await screen.findByRole('alert')).textContent).toBe('2 routes use this bot.');
	});

	// A text field followed by a password field looks like a login form:
	// browsers filled the site login into Name and Token, and the token
	// field would then save the admin's password as a bot token.
	it('keeps password managers out of the add and edit dialogs', () => {
		const b = fixture<BotsProps>('/platform-fwd/bots').props.bots[0]!;
		renderPage('/platform-fwd/bots');
		const check = () => {
			const inputs = [...dialog().querySelectorAll('input')];
			expect(inputs.length).toBeGreaterThanOrEqual(2);
			for (const i of inputs) {
				expect(i.type).not.toBe('password');
				expect(i.getAttribute('autocomplete')).toBe('off');
				expect(i.getAttribute('data-1p-ignore')).not.toBeNull();
				expect(i.getAttribute('data-lpignore')).toBe('true');
				expect(i.getAttribute('data-form-type')).toBe('other');
				for (const a of [i.name, i.id]) expect(a).not.toMatch(/^$|name|user|mail|pass|login/i);
			}
			const tok = dialog().querySelector<HTMLInputElement>('#bot-token')!;
			expect(tok.className).toContain('a-secret');
			expect(tok.getAttribute('autocorrect')).toBe('off');
			expect(tok.getAttribute('autocapitalize')).toBe('off');
			expect(tok.getAttribute('spellcheck')).toBe('false');
			const eye = within(dialog()).getByRole('button', { name: 'Show token' });
			expect(eye.getAttribute('aria-pressed')).toBe('false');
			fireEvent.click(eye);
			expect(eye.getAttribute('aria-pressed')).toBe('true');
			expect(tok.className).not.toContain('a-secret');
		};
		fireEvent.click(screen.getByRole('button', { name: /Add bot/ }));
		check();
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
		fireEvent.click(screen.getByRole('button', { name: `Edit ${b.name}` }));
		check();
	});

	it('opens the add dialog empty after typing and cancelling', () => {
		renderPage('/platform-fwd/bots');
		fireEvent.click(screen.getByRole('button', { name: /Add bot/ }));
		fireEvent.change(dialog().querySelector('#bot-label')!, { target: { value: 'Typed' } });
		fireEvent.change(dialog().querySelector('#bot-token')!, { target: { value: TG } });
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Show token' }));
		fireEvent.click(within(dialog()).getByRole('button', { name: 'Cancel' }));
		expect(screen.queryByRole('dialog')).toBeNull();
		fireEvent.click(screen.getByRole('button', { name: /Add bot/ }));
		expect(dialog().querySelector<HTMLInputElement>('#bot-label')!.value).toBe('');
		expect(dialog().querySelector<HTMLInputElement>('#bot-token')!.value).toBe('');
		expect(dialog().querySelector('#bot-token')!.className).toContain('a-secret');
	});

	it('never puts a token on the page', () => {
		renderPage('/platform-fwd/bots');
		expect(document.body.innerHTML).not.toMatch(/\d{6,}:[A-Za-z0-9_-]{30,}|[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{6}\.[A-Za-z0-9_-]{20,}/);
		expect(document.getElementById('__tgd')?.textContent ?? '').not.toContain('token');
	});
});
