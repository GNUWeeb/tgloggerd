// @vitest-environment happy-dom
// The user and group pages on the fuller profile: new fields, usernames
// with their kind, username events, and timelines paged by cursor.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { historyApi, sampleContacts } from '../../standin/fixtures.ts';
import type { UserProps } from './user.tsx';
import type { GroupProps } from './group.tsx';
import type { GroupsProps } from './groups.tsx';

beforeEach(setup);
type Full = Extract<UserProps, { limited?: false }>;
const [contact, , pending] = sampleContacts() as [number, number, number];
const userPage = (id: number) => fixture<Full>(`/users/${id}`).props;
const groupId = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
// The stand-in's own history operation, as the server answers it.
const serveHistory = (kind: 'users' | 'groups') => ({
	[`GET /api/${kind}/:id/history`]: (c: { path: string; query: URLSearchParams }) => {
		const [status, body] = historyApi(kind, c.path.split('/')[3]!, c.query, 'superadmin');
		return json(status, body);
	},
});

describe('user page', () => {
	it('pages the timeline by cursor until its start', async () => {
		const p = userPage(contact);
		expect(p.history_next).toBeTruthy();
		const calls = fakeApi('superadmin', serveHistory('users'));
		renderPage(`/users/${contact}/history`);
		const before = document.querySelectorAll('.a-timeline > li').length;
		expect(screen.getByRole('tab', { name: 'All' })).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: 'Load older changes' }));
		await waitFor(() => expect(document.querySelectorAll('.a-timeline > li').length).toBeGreaterThan(before));
		expect(callsTo(calls, 'GET', `/api/users/${contact}/history`)[0]!.query.get('before')).toBe(p.history_next);
		while (screen.queryByRole('button', { name: /Load older changes|Loading/ })) {
			const b = screen.queryByRole('button', { name: 'Load older changes' });
			if (b) fireEvent.click(b);
			await new Promise((r) => setTimeout(r, 20));
		}
		expect(screen.getByText('Start of the recorded history')).toBeTruthy();
		expect(screen.queryByText(/load as you scroll/)).toBeNull();
	});

	it('says when an older page cannot be read', async () => {
		fakeApi('superadmin', { 'GET /api/users/:id/history': () => json(503, { error: 'unavailable', message: 'The archive is busy.' }) });
		renderPage(`/users/${contact}/history`);
		fireEvent.click(screen.getByRole('button', { name: 'Load older changes' }));
		expect((await screen.findByRole('alert')).textContent).toContain('The archive is busy.');
		expect(screen.getByRole('button', { name: 'Load older changes' })).toBeTruthy();
	});

	it('shows the profile\'s new fields', () => {
		const p = userPage(contact);
		const u = { ...p.user, birthday: { day: 12, month: 3, year: null }, paid_message_star_count: 50, restricts_new_chats: true, photo_file_id: 77, restriction_reason: '', has_sensitive_content: true,
			emoji_status: { custom_emoji_id: '5368324170671202286', expires_at: null },
			usernames: [{ username: 'dewi', active: true, kind: 'active' as const, collectible: false }, { username: 'dewi777', active: false, kind: 'disabled' as const, collectible: true }] };
		renderPage(`/users/${contact}`, 'superadmin', { props: { ...p, user: u } });
		expect(screen.getByText('12 March')).toBeTruthy();
		expect(screen.getByText('50 Stars')).toBeTruthy();
		expect(screen.getByText('only from contacts and Premium users')).toBeTruthy();
		expect(screen.getByText('marked sensitive')).toBeTruthy();
		expect(screen.getByText('5368324170671202286')).toBeTruthy();
		expect(screen.getByRole('link', { name: 'file 77' }).getAttribute('href')).toBe('/files?file=77');
		expect(screen.getByText('@dewi')).toBeTruthy();
		const off = screen.getByText(/@dewi777/);
		expect(off.textContent).toContain('(off)');
		expect(within(off).getByLabelText('collectible')).toBeTruthy();
	});

	it('links the personal channel', () => {
		const p = fixture<Full>(`/users/${contact}`).props;
		const g = fixture<GroupsProps>('/groups').props.rows[0]!;
		renderPage(`/users/${contact}`, 'superadmin', { props: { ...p, user: { ...p.user, personal_chat_id: g.id, personal_chat: g } } });
		const dd = screen.getByText('Personal channel').nextElementSibling as HTMLElement;
		expect(within(dd).getByRole('link', { name: g.title }).getAttribute('href')).toBe(`/groups/${g.id}`);
	});

	it('names username events beyond added and removed', () => {
		const p = userPage(contact);
		const at = '2026-11-10T10:00:00Z';
		const history = [
			{ kind: 'username' as const, at, detail: 'dewi', action: 'reordered' as const, username_kind: 'active' as const },
			{ kind: 'username' as const, at, detail: 'dewi', action: 'kind_changed' as const, username_kind: 'disabled' as const },
			{ kind: 'username' as const, at, detail: 'dewi777', action: 'collectible_changed' as const, username_kind: 'active' as const, collectible: true },
		];
		renderPage(`/users/${contact}/history`, 'superadmin', { props: { ...p, history, history_next: null } });
		expect(screen.getByText('Usernames reordered')).toBeTruthy();
		expect(screen.getByText('Username turned off')).toBeTruthy();
		expect(screen.getByText('Username collectible status changed')).toBeTruthy();
	});

	it('says since when a lookup left the contact removed, and its last error', () => {
		renderPage(`/users/${pending}`);
		const b = screen.getByText(/removed this contact and has not added it back yet/).closest('.a-banner')!;
		expect(b.textContent).toContain('17 Nov 2026');
		expect(within(b as HTMLElement).getByText('FLOOD_WAIT_120')).toBeTruthy();
	});
});

describe('group page', () => {
	it('pages the timeline by cursor', async () => {
		const id = groupId();
		const p = fixture<GroupProps>(`/groups/${id}/history`).props;
		const calls = fakeApi('superadmin', serveHistory('groups'));
		renderPage(`/groups/${id}/history`);
		if (!p.history_next) { expect(screen.getByText('Start of the recorded history')).toBeTruthy(); return; }
		fireEvent.click(screen.getByRole('button', { name: 'Load older changes' }));
		await waitFor(() => expect(callsTo(calls, 'GET', `/api/groups/${id}/history`)).toHaveLength(1));
		expect(callsTo(calls, 'GET', `/api/groups/${id}/history`)[0]!.query.get('before')).toBe(p.history_next);
	});

	it('lists its usernames with their kind', () => {
		const id = groupId();
		const p = fixture<GroupProps>(`/groups/${id}`).props;
		const group = { ...p.group, usernames: [{ username: 'lkid', active: true, kind: 'active' as const, collectible: false }, { username: 'lkid_old', active: false, kind: 'disabled' as const, collectible: false }] };
		renderPage(`/groups/${id}`, 'superadmin', { props: { ...p, group } });
		expect(screen.getByText('Usernames')).toBeTruthy();
		expect(screen.getByText(/@lkid_old/).textContent).toContain('(off)');
	});
});
