// @vitest-environment happy-dom
// A group's Files tab: sub tabs for the kinds it has, each file drawn as
// the files page draws it with its sender and both message links,
// older pages appended from GET /api/groups/{id}/files, and the preview
// opening in place.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { groupTabApi } from '../../standin/fixtures.ts';
import { CHANNEL_GROUP, FILES_GROUP, SOME_KINDS_GROUP } from '../../standin/groupfiles.ts';
import type { GroupFilesPage } from '../api/groupfiles.ts';

beforeEach(setup);
const pageCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//);
const filesUrl = (id: number, q = '') => `/groups/${id}/files${q}`;
const kindNav = () => screen.getByRole('navigation', { name: 'File kinds' });

describe('group files tab', () => {
	it('is a tab of the group page, with a link from the overview', () => {
		renderPage(`/groups/${FILES_GROUP.id}`);
		const tabs = document.querySelector('.a-tabs')!;
		expect(within(tabs as HTMLElement).getByRole('link', { name: /^Files/ }).getAttribute('href')).toBe(filesUrl(FILES_GROUP.id));
	});

	it('offers sub tabs for the kinds the group has, with their counts', () => {
		const p = fixture<GroupFilesPage>(filesUrl(FILES_GROUP.id)).props;
		renderPage(filesUrl(FILES_GROUP.id));
		const links = within(kindNav()).getAllByRole('link');
		expect(links.map((a) => a.firstChild!.textContent)).toEqual(['All', 'Photos', 'Videos', 'GIF', 'Audio', 'Voice', 'Documents', 'Stickers', 'Video notes']);
		expect(links[0]!.getAttribute('aria-current')).toBe('page');
		const photos = p.kinds.find((k) => k.kind === 'photo')!.count!;
		const link = within(kindNav()).getByRole('link', { name: /^Photos/ });
		expect(link.getAttribute('href')).toBe(filesUrl(FILES_GROUP.id, '?kind=photo'));
		expect(link.querySelector('.a-tabcount')!.textContent).toBe(String(photos));
		// The Files tab counts them all.
		const total = p.kinds.reduce((a, k) => a + k.count!, 0);
		const tab = within(document.querySelector<HTMLElement>('.a-tabs')!).getByRole('link', { name: /^Files/ });
		expect(tab.querySelector('.a-tabcount')!.textContent).toBe(String(total));
	});

	it('leaves out the kinds a group does not have, and uncounted kinds have no badge', () => {
		renderPage(filesUrl(SOME_KINDS_GROUP.id));
		const names = within(kindNav()).getAllByRole('link').map((a) => a.textContent);
		expect(names).not.toContain('GIF');
		expect(names).toContain('Photos');
		expect(kindNav().querySelector('.a-tabcount')).toBeNull();
	});

	it('opens another kind through the page data', async () => {
		const calls = fakeApi('superadmin');
		renderPage(filesUrl(FILES_GROUP.id));
		fireEvent.click(within(kindNav()).getByRole('link', { name: /^Stickers/ }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(pageCalls(calls)[0]!.path).toBe(`/api/page${filesUrl(FILES_GROUP.id)}`);
		expect(pageCalls(calls)[0]!.query.get('kind')).toBe('sticker');
		await waitFor(() => expect(within(kindNav()).getByRole('link', { name: /^Stickers/ }).getAttribute('aria-current')).toBe('page'));
	});

	it('shows each file with its sender and links to its message', () => {
		const { page } = renderPage(filesUrl(FILES_GROUP.id));
		const f = (page.props as GroupFilesPage).rows.find((r) => r.sender)!;
		const i = (page.props as GroupFilesPage).rows.indexOf(f);
		const card = document.querySelectorAll<HTMLElement>('.a-groupfile')[i]!;
		expect(within(card).getByRole('button', { name: new RegExp(f.name.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')) })).toBeTruthy();
		expect(within(card).getByRole('link', { name: f.sender!.name }).getAttribute('href')).toBe(`/users/${f.sender!.id}`);
		expect(within(card).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(`/groups/${FILES_GROUP.id}/chat?at=${f.message_id}`);
		expect(within(card).getByRole('link', { name: 'Message' }).getAttribute('href')).toBe(`/messages/group/${f.message_row_id}`);
	});

	it('names a channel post\'s sender as the channel', () => {
		renderPage(filesUrl(CHANNEL_GROUP.id));
		const card = document.querySelector<HTMLElement>('.a-groupfile')!;
		expect(within(card).getByRole('link', { name: CHANNEL_GROUP.title }).getAttribute('href')).toBe(`/groups/${CHANNEL_GROUP.id}`);
	});

	it('lists them as rows in the list view, with the same links', () => {
		const { page } = renderPage(filesUrl(FILES_GROUP.id, '?view=list'));
		const f = (page.props as GroupFilesPage).rows[0]!;
		const row = document.querySelector<HTMLElement>('.a-table tbody tr')!;
		expect(within(row).getByText(f.name)).toBeTruthy();
		expect(within(row).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(f.chat_link);
		expect(within(row).getByRole('link', { name: 'Message' }).getAttribute('href')).toBe(`/messages/group/${f.message_row_id}`);
		expect(document.querySelector('.a-groupfile')).toBeNull();
	});

	it('appends the next page from next_before, and says when all are shown', async () => {
		const first = fixture<GroupFilesPage>(filesUrl(FILES_GROUP.id)).props;
		const [, second] = groupTabApi('files', String(FILES_GROUP.id), new URLSearchParams(`before=${first.next_before}`), 'superadmin');
		const calls = fakeApi('superadmin', { 'GET /api/groups/:id/files': () => ({ ...(second as GroupFilesPage), next_before: null }) });
		renderPage(filesUrl(FILES_GROUP.id));
		expect(first.next_before).not.toBeNull();
		expect(document.querySelectorAll('.a-groupfile')).toHaveLength(first.rows.length);
		fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
		await waitFor(() => expect(document.querySelectorAll('.a-groupfile')).toHaveLength(first.rows.length + (second as GroupFilesPage).rows.length));
		const call = callsTo(calls, 'GET', `/api/groups/${FILES_GROUP.id}/files`)[0]!;
		expect(call.query.get('before')).toBe(String(first.next_before));
		expect(call.query.get('kind')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull();
		expect(screen.getByText(/^All \d+ files shown$/)).toBeTruthy();
		expect(pageCalls(calls)).toHaveLength(0);
	});

	it('opens a file\'s preview in place, with its sender and links, and closes it', async () => {
		const calls = fakeApi('superadmin');
		const { page } = renderPage(filesUrl(FILES_GROUP.id));
		const f = (page.props as GroupFilesPage).rows.find((r) => r.sender)!;
		const i = (page.props as GroupFilesPage).rows.indexOf(f);
		fireEvent.click(document.querySelectorAll<HTMLElement>('.a-groupfile .a-filecard')[i]!);
		const d = await screen.findByRole('dialog', { name: f.name });
		expect(within(d).getByRole('link', { name: 'In the chat' }).getAttribute('href')).toBe(f.chat_link);
		expect(within(d).getByRole('link', { name: 'Message' }).getAttribute('href')).toBe(`/messages/group/${f.message_row_id}`);
		expect(within(d).getByRole('link', { name: f.sender!.name })).toBeTruthy();
		fireEvent.keyDown(document, { key: 'Escape' });
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(pageCalls(calls)).toHaveLength(0);
	});

	it('shows a member the files of its own group', () => {
		const mine = (fixture<{ rows: { id: number }[] }>('/groups', 'member').props.rows)[0]!.id;
		renderPage(filesUrl(mine), 'member');
		expect(kindNav()).toBeTruthy();
	});
});
