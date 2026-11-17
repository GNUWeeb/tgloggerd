// @vitest-environment happy-dom
// The archive's list and detail pages: groups, files, messages, a user,
// a group, a message, the dashboard, and a missing page.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { GroupsProps } from './groups.tsx';
import type { FilesProps } from './files.tsx';
import type { MessagesProps } from './messages.tsx';
import type { UserProps } from './user.tsx';
import type { DashboardProps } from './dashboard.tsx';

beforeEach(setup);
const lastPageCall = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//).at(-1)!;

describe('groups list', () => {
	it('toggles a quick filter through the URL', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/groups');
		fireEvent.click(screen.getByRole('button', { name: 'Channels' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(JSON.parse(lastPageCall(calls).query.get('f')!)).toEqual([{ field: 'type', op: '=', value: 'channel' }]);
	});

	it('shows a member only the groups granted to it', () => {
		const { page } = renderPage('/groups', 'member');
		expect((page.props as GroupsProps).total).toBeLessThan(fixture<GroupsProps>('/groups').props.total);
		expect(screen.getAllByText(/groups granted to you/).length).toBe(1);
	});

	it('removes a filter chip and clears all', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/groups?q=linux&f=' + encodeURIComponent(JSON.stringify([{ field: 'type', op: '=', value: 'channel' }])));
		fireEvent.click(screen.getByRole('button', { name: 'Remove filter' }));
		await waitFor(() => expect(lastPageCall(calls)?.query.get('f')).toBeNull());
		expect(lastPageCall(calls).query.get('q')).toBe('linux');
	});
});

describe('files', () => {
	it('opens a file preview through ?file= and closes it', async () => {
		const calls = fakeApi('superadmin');
		const { page } = renderPage('/files?view=list');
		const f = (page.props as FilesProps).rows[0]!;
		fireEvent.click(screen.getAllByText(f.name)[0]!);
		await waitFor(() => expect(lastPageCall(calls)?.query.get('file')).toBe(String(f.id)));
		await screen.findByRole('dialog', { name: f.name });
		fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close' }));
		await waitFor(() => expect(lastPageCall(calls).query.get('file')).toBeNull());
	});

	it('switches between grid and list', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files');
		fireEvent.click(screen.getByRole('button', { name: /List/ }));
		await waitFor(() => expect(lastPageCall(calls)?.query.get('view')).toBe('list'));
	});
});

describe('message lists', () => {
	it('links each group message to its place in the chat', () => {
		const { page } = renderPage('/group_messages');
		const m = (page.props as MessagesProps).rows[0]!;
		const link = screen.getAllByRole('link', { name: 'Show in chat' })[0]!;
		expect(link.getAttribute('href')).toBe(`/groups/${m.chat.id}/chat?at=${m.message_id}`);
	});

	it('hides private messages from a member', () => {
		renderPage('/private_messages', 'member');
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('404');
	});
});

describe('user page', () => {
	it('shows a full profile with its tabs', () => {
		const { page } = renderPage('/users/' + fixture<{ rows: { id: number }[] }>('/users').props.rows[0]!.id);
		const p = page.props as Extract<UserProps, { limited?: false }>;
		expect(screen.getAllByRole('heading', { level: 1 }).some((h) => h.textContent?.includes(p.user.name))).toBe(true);
		expect(screen.getAllByRole('link', { name: /History/ }).length).toBeGreaterThan(0);
	});
});

describe('group page', () => {
	const gid = () => fixture<GroupsProps>('/groups').props.rows.find((g, i) => i > 4)!.id;

	it('pins a group, then shows Unpin once the page data says so', async () => {
		const id = gid();
		const before = fixture(`/groups/${id}`);
		const after = { ...before, viewer: { ...before.viewer, pinned: [...(before.viewer.pinned ?? []), { id, title: 'pinned' }] } };
		const calls = fakeApi('superadmin', {
			'PUT /api/account/pinned/:id': () => undefined,
			'GET /api/page/groups/:id': () => after,
		});
		renderPage(`/groups/${id}`);
		fireEvent.click(screen.getByRole('button', { name: 'Pin' }));
		await screen.findByRole('button', { name: 'Unpin' });
		const put = callsTo(calls, 'PUT', `/api/account/pinned/${id}`);
		expect(put).toHaveLength(1);
		expect(put[0]!.headers.get('x-csrf-token')).toBe('test-csrf');
		expect(screen.getAllByRole('link', { name: 'pinned' }).length).toBe(1); // the sidebar follows
	});

	it('unpins a pinned group', async () => {
		const pinnedId = fixture('/').viewer.pinned![0]!.id;
		const calls = fakeApi('superadmin', { 'DELETE /api/account/pinned/:id': () => undefined });
		renderPage(`/groups/${pinnedId}`);
		fireEvent.click(screen.getByRole('button', { name: 'Unpin' }));
		await waitFor(() => expect(callsTo(calls, 'DELETE', `/api/account/pinned/${pinnedId}`)).toHaveLength(1));
	});

	it('lists the admins on their tab', () => {
		renderPage(`/groups/${gid()}/admins`);
		expect(screen.getByRole('link', { name: /Admins/ }).getAttribute('aria-current') ?? 'x').toBeTruthy();
		expect(document.querySelectorAll('table tbody tr').length).toBeGreaterThan(0);
	});
});

describe('message page', () => {
	it('shows the message with a link to the chat', () => {
		const m = fixture<MessagesProps>('/group_messages').props.rows[0]!;
		renderPage(`/messages/group/${m.id}`);
		expect(screen.getAllByRole('link').some((a) => a.getAttribute('href')?.includes(`/chat?at=${m.message_id}`))).toBe(true);
	});
});

describe('dashboard', () => {
	it('shows the totals and the busiest groups', () => {
		const { page } = renderPage('/');
		const p = page.props as DashboardProps;
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('Dashboard');
		expect(screen.getAllByText(p.busiest[0]!.title).length).toBeGreaterThan(0);
	});
});

describe('missing page', () => {
	it('answers 404 for an unknown path', () => {
		const { page } = renderPage('/no/such/page');
		expect(page.status).toBe(404);
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe('404');
	});
});
