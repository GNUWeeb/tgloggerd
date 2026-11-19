// @vitest-environment happy-dom
// An exposed group's Files and Polls tabs are public: a signed-out
// visitor sees them in the public frame, from a tab row on the public
// group page, with senders named but not linked, the chat link and no
// message page link. A group that is not exposed is unchanged.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { groupTabApi, pageData } from '../../standin/fixtures.ts';
import { PUBLIC_POLLS_GROUP } from '../../standin/kinds.ts';
import { FILES_GROUP } from '../../standin/groupfiles.ts';
import type { GroupFilesPage, GroupPollsPage } from '../api/groupfiles.ts';

beforeEach(setup);
const G = PUBLIC_POLLS_GROUP;
const tabRow = () => document.querySelector<HTMLElement>('.a-tabs')!;
const pageCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//);

describe('public group tabs', () => {
	it('give the public group page a tab row of Overview, Files and Polls', () => {
		renderPage(`/groups/${G.id}`, 'public');
		expect(document.querySelector('.c-publicpage')).not.toBeNull();
		const links = within(tabRow()).getAllByRole('link');
		expect(links.map((a) => a.textContent)).toEqual(['Overview', 'Files', 'Polls']);
		expect(links.map((a) => a.getAttribute('href'))).toEqual([`/groups/${G.id}`, `/groups/${G.id}/files`, `/groups/${G.id}/polls`]);
		expect(links[0]!.getAttribute('aria-current')).toBe('page');
		// The overview and the chat button stay.
		expect(screen.getByText('About')).toBeTruthy();
		expect(screen.getByRole('link', { name: 'Open chat' }).getAttribute('href')).toBe(`/groups/${G.id}/chat`);
	});

	it('show the files in the public frame: senders not linked, the chat link, no message link', () => {
		const { page } = renderPage(`/groups/${G.id}/files`, 'public');
		expect(document.querySelector('.c-publicpage')).not.toBeNull();
		expect(within(tabRow()).getByRole('link', { name: /^Files/ }).getAttribute('aria-current')).toBe('page');
		const rows = (page.props as GroupFilesPage).rows;
		const cards = [...document.querySelectorAll<HTMLElement>('.a-groupfile')];
		expect(cards).toHaveLength(rows.length);
		const card = cards[0]!;
		const f = rows[0]!;
		expect(within(card).getByText(f.sender.name).closest('a')).toBeNull();
		expect(within(card).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(f.chat_link);
		expect(document.querySelector('a[href^="/messages/"]')).toBeNull();
		expect(document.querySelector('a[href^="/users/"]')).toBeNull();
		expect(screen.queryByRole('link', { name: 'Message' })).toBeNull();
	});

	it('open a public file\'s preview without the message link', async () => {
		fakeApi('public');
		const { page } = renderPage(`/groups/${G.id}/files`, 'public');
		const f = (page.props as GroupFilesPage).rows[0]!;
		fireEvent.click(document.querySelector<HTMLElement>('.a-groupfile .a-filecard')!);
		const d = await screen.findByRole('dialog', { name: f.name });
		expect(within(d).getByRole('link', { name: 'In the chat' })).toBeTruthy();
		expect(within(d).queryByRole('link', { name: 'Message' })).toBeNull();
		expect(within(d).getByText(f.sender.name).closest('a')).toBeNull();
	});

	it('page the public files from the JSON twin', async () => {
		const first = fixture<GroupFilesPage>(`/groups/${G.id}/files`, 'public').props;
		expect(first.next_before).not.toBeNull();
		const calls = fakeApi('public', { 'GET /api/groups/:id/files': (c) => groupTabApi('files', String(G.id), c.query, 'public')[1] });
		renderPage(`/groups/${G.id}/files`, 'public');
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(document.querySelectorAll('.a-groupfile').length).toBeGreaterThan(first.rows.length));
		expect(callsTo(calls, 'GET', `/api/groups/${G.id}/files`)[0]!.query.get('before')).toBe(String(first.next_before));
		expect(pageCalls(calls)).toHaveLength(0);
		expect(document.querySelector('a[href^="/messages/"]')).toBeNull();
	});

	it('show the polls in the public frame, without message or user links', () => {
		const { page } = renderPage(`/groups/${G.id}/polls`, 'public');
		expect(document.querySelector('.c-publicpage')).not.toBeNull();
		const rows = (page.props as GroupPollsPage).rows;
		const cards = [...document.querySelectorAll<HTMLElement>('.a-pollcard')];
		expect(cards).toHaveLength(rows.length);
		expect(cards[0]!.querySelector('.k-poll')).not.toBeNull();
		expect(within(cards[0]!).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(rows[0]!.chat_link);
		expect(within(cards[0]!).getByText(rows[0]!.sender.name).closest('a')).toBeNull();
		expect(document.querySelector('a[href^="/messages/"], a[href^="/users/"]')).toBeNull();
	});

	it('draw the public shape in the usual shell for a signed-in viewer without the group', () => {
		// The member is not granted the public polls group.
		const p = pageData(`/groups/${G.id}/files`, 'member');
		expect((p.props as { public?: boolean }).public).toBe(true);
		renderPage(`/groups/${G.id}/files`, 'member');
		expect(document.querySelector('.c-publicpage')).toBeNull();
		expect(document.querySelector('.a-crumbs, nav[aria-label="Main"]')).not.toBeNull();
		expect(within(tabRow()).getAllByRole('link').map((a) => a.textContent!.replace(/\d.*$/, ''))).toEqual(['Overview', 'Files', 'Polls']);
		const f = (p.props as GroupFilesPage).rows[0]!;
		expect(within(document.querySelector<HTMLElement>('.a-groupfile')!).getByText(f.sender.name).closest('a')).toBeNull();
		expect(document.querySelector('.a-groupfile a[href^="/messages/"], .a-groupfile a[href^="/users/"]')).toBeNull();
		expect(screen.getAllByRole('link', { name: 'In the chat' }).length).toBeGreaterThan(0);
	});

	it('leave a group that is not exposed as it was, and the group\'s own viewers their full tabs', () => {
		expect(FILES_GROUP.exposed).toBeFalsy();
		expect(pageData(`/groups/${FILES_GROUP.id}/files`, 'public').route).toBe('not_found');
		expect(pageData(`/groups/${FILES_GROUP.id}/polls`, 'public').route).toBe('not_found');
		renderPage(`/groups/${G.id}/files`, 'superadmin');
		expect(document.querySelector('.c-publicpage')).toBeNull();
		expect(within(tabRow()).getByRole('link', { name: /^History/ })).toBeTruthy();
		expect(screen.getAllByRole('link', { name: 'Message' }).length).toBeGreaterThan(0);
	});
});
