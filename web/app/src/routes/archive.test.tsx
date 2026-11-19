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
	it('opens and closes a file preview in place, without reloading the list', async () => {
		const calls = fakeApi('superadmin');
		const { page } = renderPage('/files?view=list');
		const f = (page.props as FilesProps).rows[0]!;
		fireEvent.click(screen.getAllByText(f.name)[0]!);
		await screen.findByRole('dialog', { name: f.name });
		expect(new URLSearchParams(location.search).get('file')).toBe(String(f.id));
		fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close' }));
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(location.search).toBe('?view=list');
		// A mobile row and a grid card open it in place too.
		fireEvent.click(within(document.querySelector('.a-list')!).getAllByText(f.name)[0]!);
		await screen.findByRole('dialog', { name: f.name });
		fireEvent.keyDown(document, { key: 'Escape' });
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(0);
		// The next navigation starts from the URL as it is now.
		fireEvent.click(screen.getByRole('button', { name: 'Grid view' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(lastPageCall(calls).query.get('file')).toBeNull();
		const card = (await screen.findAllByRole('button', { name: new RegExp(f.name) })).find((b) => b.classList.contains('a-filecard'))!;
		fireEvent.click(card);
		await screen.findByRole('dialog', { name: f.name });
		expect(new URLSearchParams(location.search).get('file')).toBe(String(f.id));
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1);
	});

	it('shows the server\'s preview on a direct visit to ?file=, and closes it in place', async () => {
		const calls = fakeApi('superadmin');
		const p = fixture<FilesProps>('/files?view=list').props;
		const far = { ...p.rows[0]!, id: 98765, name: 'not-on-this-page.jpg' };
		renderPage('/files?view=list&file=98765', 'superadmin', { props: { ...p, preview: far } });
		await screen.findByRole('dialog', { name: 'not-on-this-page.jpg' });
		fireEvent.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Close' }));
		await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
		expect(location.search).toBe('?view=list');
		expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(0);
		// A later navigation does not bring the closed preview back.
		fireEvent.click(screen.getByRole('button', { name: 'Grid view' }));
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\//)).toHaveLength(1));
		expect(lastPageCall(calls).query.get('file')).toBeNull();
	});

	it('shows a photo whole in the preview, never cropped to a shape', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4245, name: 'tall.jpg', file_type: 'photo' as const, stored: true, url: '/files/tok-tall', thumb: '/files/tok-tall' };
		renderPage('/files?view=list&file=4245', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'tall.jpg' });
		const img = d.querySelector<HTMLImageElement>('.a-preview img')!;
		expect(img.getAttribute('src')).toBe('/files/tok-tall');
		expect(img.classList.contains('a-preview-img')).toBe(true);
		// Not the list's square thumbnail: no cover fit, no fixed shape.
		expect(img.closest('.a-thumb')).toBeNull();
		expect(img.style.objectFit).toBe('');
		expect(img.style.aspectRatio).toBe('');
	});

	it('opens a preview\'s photo bigger, and closes it by its button, Esc or a click beside it', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4246, name: 'big.jpg', file_type: 'photo' as const, stored: true, url: '/files/tok-big', thumb: '/files/tok-big' };
		renderPage('/files?view=list&file=4246', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'big.jpg' });
		const viewer = () => screen.queryByRole('dialog', { name: 'Media viewer' });
		const zoom = within(d).getByRole('button', { name: 'View bigger' });
		fireEvent.click(zoom);
		expect(viewer()!.querySelector('img')!.getAttribute('src')).toBe('/files/tok-big');
		fireEvent.click(within(viewer()!).getByRole('button', { name: 'Close' }));
		expect(viewer()).toBeNull();
		// Esc closes the bigger view only; the preview stays.
		fireEvent.click(zoom);
		fireEvent.keyDown(document, { key: 'Escape' });
		await waitFor(() => expect(viewer()).toBeNull());
		expect(screen.getByRole('dialog', { name: 'big.jpg' })).toBeTruthy();
		fireEvent.click(zoom);
		fireEvent.mouseDown(viewer()!);
		expect(viewer()).toBeNull();
		expect(screen.getByRole('dialog', { name: 'big.jpg' })).toBeTruthy();
		// With the viewer closed, Esc closes the preview again.
		fireEvent.keyDown(document, { key: 'Escape' });
		await waitFor(() => expect(screen.queryByRole('dialog', { name: 'big.jpg' })).toBeNull());
	});

	it('offers no bigger view for a video\'s poster', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4247, name: 'clip.mp4', file_type: 'video' as const, stored: true, url: '/files/tok-clip', thumb: '/files/tok-clip-poster' };
		renderPage('/files?view=list&file=4247', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'clip.mp4' });
		expect(d.querySelector('.a-preview img')!.getAttribute('src')).toBe('/files/tok-clip-poster');
		expect(within(d).queryByRole('button', { name: 'View bigger' })).toBeNull();
	});

	it('shows a file seen in no visible message without links, and cannot open its message', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const orphan = { ...p.rows[0]!, id: 4242, name: 'lonely.pdf', chat: null, message: null, sender: null };
		const props = { ...p, rows: [orphan, ...p.rows.slice(1)], preview: orphan };
		renderPage('/files?view=list&file=4242', 'superadmin', { props });
		const d = await screen.findByRole('dialog', { name: 'lonely.pdf' });
		expect(within(d).getByText('no visible message')).toBeTruthy();
		const open = within(d).getByRole('button', { name: /Open message/ }) as HTMLButtonElement;
		expect(open.disabled).toBe(true);
		expect(within(d).queryAllByRole('link').some((a) => /\/(groups|users|messages)\//.test(a.getAttribute('href') ?? ''))).toBe(false);
	});

	it('fetches an unstored file from Telegram through its URL when the viewer may', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4243, name: 'far-away.mp4', stored: false, fetchable: true, url: '/files/tok-far', thumb: null };
		renderPage('/files?view=list&file=4243', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'far-away.mp4' });
		expect(within(d).getByRole('link', { name: /Fetch from Telegram/ }).getAttribute('href')).toBe('/files/tok-far');
		expect(within(d).getByText('no, can be fetched from Telegram')).toBeTruthy();
		expect(within(d).queryByRole('link', { name: /^Download/ })).toBeNull();
		const row = screen.getAllByRole('row').find((r) => within(r).queryByText('far-away.mp4'))!;
		expect(within(row).getByText('fetchable')).toBeTruthy();
	});

	it('offers nothing to fetch for an unstored file the viewer may not have fetched', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4244, name: 'gone.pdf', stored: false, fetchable: false, url: null, thumb: null, sha256: null };
		renderPage('/files?view=list&file=4244', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'gone.pdf' });
		expect(within(d).queryByRole('link', { name: /Fetch from Telegram|Download/ })).toBeNull();
		expect(within(d).getByText(/only its details were kept/)).toBeTruthy();
		expect(within(d).getByText('not recorded')).toBeTruthy();
	});

	it('shows a stored file\'s SHA-256 and last access', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = p.rows.find((r) => r.stored && r.sha256)!;
		renderPage(`/files?view=list&file=${f.id}`, 'superadmin', { props: { ...p, preview: f } });
		const d = await screen.findByRole('dialog', { name: f.name });
		expect(within(d).getByText(f.sha256!, { exact: false })).toBeTruthy();
		expect(within(d).getByText('Last access')).toBeTruthy();
	});

	it('sorts by id, extension and last access, and filters a digest only by = and !=', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/files?view=list');
		for (const h of ['ID', 'Ext', 'Last access']) expect(screen.getByRole('columnheader', { name: new RegExp(h) })).toBeTruthy();
		fireEvent.click(screen.getByRole('button', { name: 'Filter' }));
		const field = screen.getByRole('combobox', { name: 'Field' });
		expect([...(field as HTMLSelectElement).options].map((o) => o.value)).toEqual(expect.arrayContaining(['tg_file_id', 'sha256', 'updated_at', 'accessed_at']));
		fireEvent.change(field, { target: { value: 'sha256' } });
		expect([...(screen.getByRole('combobox', { name: 'Condition' }) as HTMLSelectElement).options].map((o) => o.value)).toEqual(['=', '!=']);
		fireEvent.change(screen.getByRole('textbox', { name: 'Value' }), { target: { value: 'ab'.repeat(32) } });
		fireEvent.click(screen.getByRole('button', { name: 'Apply' }));
		await waitFor(() => expect(lastPageCall(calls)?.query.get('f')).toBeTruthy());
		expect(JSON.parse(lastPageCall(calls).query.get('f')!)).toEqual([{ field: 'sha256', op: '=', value: 'ab'.repeat(32) }]);
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

	it('says why a pin is refused, such as the cap on pinned groups', async () => {
		const id = gid();
		fakeApi('superadmin', { 'PUT /api/account/pinned/:id': () => json(400, { error: 'too_many', message: 'You have 50 pinned groups, the most there can be. Unpin one first.' }) });
		renderPage(`/groups/${id}`);
		fireEvent.click(screen.getByRole('button', { name: 'Pin' }));
		expect((await screen.findByRole('alert')).textContent).toMatch(/50 pinned groups/);
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
