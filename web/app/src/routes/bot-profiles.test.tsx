// @vitest-environment happy-dom
// A Telegram bot the archive has as a user links to its user page, on
// the Bots page and in a Discord to Telegram route; a bot without a
// profile, and every Discord bot, shows as before.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { D2TProps } from './forwarding.tsx';
import type { BotsProps } from './bots.tsx';

beforeEach(setup);

type Profiled = { name: string; kind: string; profile?: number | null };

const bots = () => fixture<BotsProps>('/platform-fwd/bots').props.bots as (BotsProps['bots'][number] & Profiled)[];
const row = (name: string) => [...document.querySelectorAll<HTMLElement>('tbody tr')].find((r) => r.textContent!.includes(name))!;
const profileLinks = (root: Element) => [...root.querySelectorAll('a[href^="/users/"]')];

describe('bot profiles', () => {
	it('links a Telegram bot with a profile on the Bots page, by its name, and no other bot', () => {
		const all = bots();
		const tg = all.find((b) => b.kind === 'telegram' && b.profile != null)!;
		const plain = all.find((b) => b.kind === 'telegram' && b.profile === null)!;
		const discord = all.filter((b) => b.kind === 'discord');
		expect(tg && plain).toBeTruthy();
		expect(discord.length).toBeGreaterThan(0);
		expect(discord.every((b) => b.profile === null)).toBe(true);
		fakeApi('superadmin');
		renderPage('/platform-fwd/bots');
		const r = row(tg.name);
		const link = within(r).getByRole('link', { name: tg.name });
		expect(link.getAttribute('href')).toBe(`/users/${tg.profile}`);
		// The avatar links too, but as a mouse target only: one link in the
		// tab order and for screen readers.
		const links = profileLinks(r);
		expect(links).toHaveLength(2);
		expect(links.every((a) => a.getAttribute('href') === `/users/${tg.profile}`)).toBe(true);
		const avatar = links.find((a) => a !== link)!;
		expect(avatar.getAttribute('tabindex')).toBe('-1');
		expect(avatar.getAttribute('aria-hidden')).toBe('true');
		expect(avatar.querySelector('a')).toBeNull();
		expect(link.querySelector('a')).toBeNull();
		for (const b of [plain, ...discord]) {
			expect(profileLinks(row(b.name)), b.name).toHaveLength(0);
			expect(within(row(b.name)).getByText(b.name).tagName).toBe('SPAN');
		}
	});

	it('links a route\'s Telegram bot with a profile, and no other bot', () => {
		const routes = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes as (D2TProps['routes'][number] & { telegram_bot: Profiled; discord_bot: Profiled | null })[];
		const linked = routes.find((r) => r.telegram_bot.profile != null)!;
		const unlinked = routes.find((r) => r.telegram_bot.profile === null)!;
		expect(linked && unlinked).toBeTruthy();
		fakeApi('superadmin');
		renderPage('/platform-fwd/discord-telegram');
		const cells = [...document.querySelectorAll('.a-botcell')];
		const cell = (name: string) => cells.filter((c) => c.textContent!.includes(name));
		const link = within(cell(linked.telegram_bot.name)[0] as HTMLElement).getByRole('link', { name: linked.telegram_bot.name });
		expect(link.getAttribute('href')).toBe(`/users/${linked.telegram_bot.profile}`);
		for (const c of cell(unlinked.telegram_bot.name)) expect(profileLinks(c)).toHaveLength(0);
		for (const r of routes) if (r.discord_bot) for (const c of cell(r.discord_bot.name)) expect(profileLinks(c)).toHaveLength(0);
	});

	it('opens the bot\'s user page on a click', async () => {
		const tg = bots().find((b) => b.profile != null)!;
		const calls = fakeApi('superadmin');
		renderPage('/platform-fwd/bots');
		fireEvent.click(within(row(tg.name)).getByRole('link', { name: tg.name }));
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/page/users/${tg.profile}`)).toHaveLength(1));
		await waitFor(() => expect(location.pathname).toBe(`/users/${tg.profile}`));
	});

	it('opens the bot\'s user page on a click on its avatar in a route', async () => {
		const routes = fixture<D2TProps>('/platform-fwd/discord-telegram').props.routes as (D2TProps['routes'][number] & { telegram_bot: Profiled })[];
		const { profile } = routes.find((r) => r.telegram_bot.profile != null)!.telegram_bot;
		const calls = fakeApi('superadmin');
		renderPage('/platform-fwd/discord-telegram');
		fireEvent.click(document.querySelector(`.a-botcell a.a-botavatar[href="/users/${profile}"]`)!);
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/page/users/${profile}`)).toHaveLength(1));
	});
});
