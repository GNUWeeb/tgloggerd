// @vitest-environment happy-dom
// Photos where forwarding and the menu name chats and bots: a route's
// chat, a pinned group, a Telegram bot; their initials when there is
// none.
import { beforeEach, describe, expect, it } from 'vitest';
import { within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { D2TProps, T2DProps } from './forwarding.tsx';
import type { BotsProps } from './bots.tsx';

beforeEach(setup);

const img = (root: Element) => root.querySelector('img');

describe('photos', () => {
	it('shows each pinned group\'s photo in the menu, or its initials', () => {
		fakeApi('superadmin');
		const page = fixture('/');
		const pinned = [{ id: -1001, title: 'Alpha Group', photo: '/files/tok-alpha' }, { id: -1002, title: 'Beta Group', photo: null }];
		renderPage('/', 'superadmin', { viewer: { ...page.viewer, pinned } });
		const nav = document.querySelector<HTMLElement>('nav[aria-label="Main"]')!;
		const a = within(nav).getByRole('link', { name: /Alpha Group/ });
		expect(img(a)!.getAttribute('src')).toBe('/files/tok-alpha');
		const b = within(nav).getByRole('link', { name: /Beta Group/ });
		expect(img(b)).toBeNull();
		expect(b.textContent).toContain('BG');
	});

	it('shows a route\'s chat photo on both forwarding pages', () => {
		for (const url of ['/platform-fwd/telegram-discord', '/platform-fwd/discord-telegram']) {
			const routes = fixture<T2DProps | D2TProps>(url).props.routes;
			const withPhoto = routes.find((r) => r.chat.photo);
			expect(withPhoto, url).toBeTruthy();
			fakeApi('superadmin');
			renderPage(url);
			const cell = [...document.querySelectorAll(`a[href="/groups/${withPhoto!.chat.id}"]`)].find((x) => x.classList.contains('a-usercell'))!;
			expect(img(cell)!.getAttribute('src')).toBe(withPhoto!.chat.photo);
			document.body.innerHTML = '';
		}
	});

	it('shows a Telegram bot\'s photo on the Bots page and in a route; a Discord bot has its initials', () => {
		const bots = fixture<BotsProps>('/platform-fwd/bots').props.bots as (BotsProps['bots'][number] & { photo?: string | null })[];
		const tg = bots.find((b) => b.kind === 'telegram' && b.photo)!;
		const dc = bots.find((b) => b.kind === 'discord')!;
		expect(tg).toBeTruthy();
		expect(dc.photo).toBeNull();
		fakeApi('superadmin');
		renderPage('/platform-fwd/bots');
		const row = (name: string) => [...document.querySelectorAll('tbody tr')].find((r) => r.textContent!.includes(name))!;
		expect(img(row(tg.name))!.getAttribute('src')).toBe(tg.photo);
		expect(img(row(dc.name))).toBeNull();
		document.body.innerHTML = '';
		const d2t = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes as (D2TProps['routes'][number] & { telegram_bot: { photo?: string | null } })[];
		const r = d2t.find((x) => x.telegram_bot.photo)!;
		renderPage('/platform-fwd/discord-telegram');
		expect([...document.querySelectorAll('.a-botcell img')].map((x) => x.getAttribute('src'))).toContain(r.telegram_bot.photo);
	});
});
