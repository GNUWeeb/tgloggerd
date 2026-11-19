// @vitest-environment happy-dom
// The archive's list and detail pages: groups, files, messages, a user,
// a group, a message, the dashboard, and a missing page.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import type { GroupsProps } from './groups.tsx';
import type { FilesProps } from './files.tsx';
import type { MessagesProps } from './messages.tsx';
import type { UserProps } from './user.tsx';
import type { DashboardProps } from './dashboard.tsx';
import { readFileSync } from 'node:fs';

// A .tgs preview plays with the Lottie player, which is not under test.
vi.mock('../ui/lottie.ts', () => ({ playTgs: vi.fn(() => () => {}) }));

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
	// The grid loads its next batch when its end is near the window's
	// bottom. happy-dom lays nothing out (every box is at 0), so place the
	// end far below, as a first batch that fills the window does: these
	// tests count only the page loads they make themselves.
	beforeEach(() => {
		const real = Element.prototype.getBoundingClientRect;
		vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (this: Element) {
			return this.classList.contains('a-filegrid-foot') ? new DOMRect(0, 10000, 100, 40) : real.call(this);
		});
	});
	afterEach(() => { vi.restoreAllMocks(); });

	// Renders the list in grid, list and mobile form: in a loaded full run
	// this takes longer than the default five seconds.
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
	}, 20_000);

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

	it('offers no picture\'s bigger view for a video\'s poster', async () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		const f = { ...p.rows[0]!, id: 4247, name: 'clip.mp4', file_type: 'video' as const, stored: true, url: '/files/tok-clip', thumb: '/files/tok-clip-poster' };
		renderPage('/files?view=list&file=4247', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'clip.mp4' });
		expect(d.querySelector('.a-preview video')!.getAttribute('poster')).toBe('/files/tok-clip-poster');
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

	it('keeps every grid card\'s picture box at 4:3, showing a tall picture whole', async () => {
		// The app's own stylesheet, so the rules are what is checked.
		const style = document.createElement('style');
		style.textContent = readFileSync('src/styles/app.css', 'utf8');
		document.head.append(style);
		try {
			const p = fixture<FilesProps>('/files').props;
			const tall = { ...p.rows[0]!, id: 4248, name: 'long-screenshot.png', file_type: 'photo' as const, stored: true, url: '/files/tok-long', thumb: '/files/tok-long' };
			renderPage('/files', 'superadmin', { props: { ...p, rows: [tall, ...p.rows.slice(1)] } });
			const img = document.querySelector<HTMLImageElement>('.a-filecard img[src="/files/tok-long"]')!;
			const box = img.closest<HTMLElement>('.a-filecard-media')!;
			const thumb = img.closest<HTMLElement>('.a-thumb')!;
			const cs = (e: Element) => getComputedStyle(e);
			expect(cs(box).aspectRatio.replace(/\s/g, '')).toBe('4/3');
			expect(cs(box).overflow).toBe('hidden');
			expect(cs(box).position).toBe('relative');
			// The picture sits over the box and cannot size it.
			expect(cs(thumb).position).toBe('absolute');
			expect(img.style.objectFit).toBe('contain');
			expect(cs(img).maxHeight).toBe('100%');
			expect(cs(img).maxWidth).toBe('100%');
			expect(cs(img).position).toBe('absolute');
		} finally {
			style.remove();
		}
	});

	it('draws a .webm video and a .tgs sticker as their type icons, never as a broken <img>', async () => {
		const p = fixture<FilesProps>('/files').props;
		// thumb null as Go sends them; and an older server's own-URL thumb.
		const webm = { ...p.rows[0]!, id: 4249, name: 'clip.webm', ext: 'webm', file_type: 'video' as const, stored: true, url: '/files/tok-webm', thumb: null };
		const oldWebm = { ...webm, id: 4250, name: 'old.webm', url: '/files/tok-old', thumb: '/files/tok-old' };
		const tgs = { ...p.rows[0]!, id: 4251, name: 'duck.tgs', ext: 'tgs', file_type: 'sticker' as const, stored: true, url: '/files/tok-tgs.tgs', thumb: '/files/tok-tgs.tgs' };
		const rows = [webm, oldWebm, tgs];
		const { unmount } = renderPage('/files', 'superadmin', { props: { ...p, rows } });
		for (const f of rows) {
			const card = screen.getAllByRole('button').find((b) => b.classList.contains('a-filecard') && within(b).queryByText(f.name))!;
			expect(card.querySelector('img')).toBeNull();
			expect(card.querySelector(f.file_type === 'video' ? '.a-thumb-video' : '.a-thumb-sticker')).toBeTruthy();
		}
		unmount();
		for (const f of rows) {
			const r = renderPage(`/files?view=list&file=${f.id}`, 'superadmin', { props: { ...p, rows, preview: f } });
			const d = await screen.findByRole('dialog', { name: f.name });
			expect(d.querySelector('img')).toBeNull();
			// It plays with the chat's player, with no poster to draw.
			if (f.file_type === 'video') expect(d.querySelector('.a-preview video')!.hasAttribute('poster')).toBe(false);
			else expect(d.querySelector('.a-preview .c-lottie')).toBeTruthy();
			expect(within(d).queryByRole('button', { name: 'View bigger' })).toBeNull();
			// Nor the list's small thumbnails.
			expect(document.querySelectorAll('.a-table img[src*="tok-"]')).toHaveLength(0);
			r.unmount();
		}
	});

	describe('video stills', () => {
		const MINI = 'data:image/jpeg;base64,AA';
		const base = () => fixture<FilesProps>('/files').props;
		const row = (o: Partial<FilesProps['rows'][number]>) => ({ ...base().rows[0]!, stored: true, duration: 30, ...o });
		const still = row({ id: 4260, name: 'clip.mp4', ext: 'mp4', file_type: 'video', url: '/files/tok-v1', thumb: '/files/tok-v1-still', minithumb: MINI });
		const blurOnly = row({ id: 4261, name: 'blur.mp4', ext: 'mp4', file_type: 'animation', url: '/files/tok-v2', thumb: null, minithumb: MINI });
		const neither = row({ id: 4262, name: 'bare.mp4', ext: 'mp4', file_type: 'video', url: '/files/tok-v3', thumb: null });
		const webmSticker = row({ id: 4263, name: 'cat.webm', ext: 'webm', file_type: 'sticker', url: '/files/tok-v4', thumb: '/files/tok-v4-still' });
		const webmVideo = row({ id: 4264, name: 'clip.webm', ext: 'webm', file_type: 'video', url: '/files/tok-v5', thumb: '/files/tok-v5-still' });
		const unstored = row({ id: 4265, name: 'gone.mp4', ext: 'mp4', file_type: 'video_note', stored: false, url: null, thumb: '/files/tok-v6-still', minithumb: MINI });
		const rows = [still, blurOnly, neither, webmSticker, webmVideo, unstored];
		const card = (name: string) => screen.getAllByRole('button').find((b) => b.classList.contains('a-filecard') && within(b).queryByText(name))!;
		const picOf = (el: Element) => el.querySelector('img:not(.a-thumb-mini)')?.getAttribute('src') ?? null;
		const miniOf = (el: Element) => el.querySelector('img.a-thumb-mini')?.getAttribute('src') ?? null;

		it('shows each grid card\'s still with the play mark, the blurred preview without one, and a .webm\'s still', () => {
			renderPage('/files', 'superadmin', { props: { ...base(), rows } });
			const c1 = card('clip.mp4');
			expect(picOf(c1)).toBe('/files/tok-v1-still');
			expect(miniOf(c1)).toBe(MINI);
			expect(c1.querySelector('.a-thumb-play')).toBeTruthy();
			const c2 = card('blur.mp4');
			expect(picOf(c2)).toBeNull();
			expect(miniOf(c2)).toBe(MINI);
			expect(c2.querySelector('.a-thumb-blurred .a-thumb-play')).toBeTruthy();
			const c3 = card('bare.mp4');
			expect(c3.querySelector('img')).toBeNull();
			expect(c3.querySelector('.a-thumb-video')).toBeTruthy();
			// Keyed on the thumb, not the file: a .webm with a still shows it.
			expect(picOf(card('cat.webm'))).toBe('/files/tok-v4-still');
			expect(picOf(card('clip.webm'))).toBe('/files/tok-v5-still');
			// An unstored video keeps its still, marked as not stored.
			const c6 = card('gone.mp4');
			expect(picOf(c6)).toBe('/files/tok-v6-still');
			expect(c6.querySelector('.a-thumb')!.getAttribute('title')).toBe('Not stored');
			// No card asks for a video's own bytes.
			for (const f of rows) expect(document.querySelector(`.a-filegrid img[src="${f.url}"]`)).toBeNull();
			expect(document.querySelector('.a-filegrid video')).toBeNull();
		});

		it('shows the stills in the list view too', () => {
			renderPage('/files?view=list', 'superadmin', { props: { ...base(), rows } });
			const table = document.querySelector('.a-table')!;
			expect(table.querySelector('img[src="/files/tok-v1-still"]')).toBeTruthy();
			expect(table.querySelector(`img.a-thumb-mini[src="${MINI}"]`)).toBeTruthy();
		});

		it('shows the still or blurred preview of a video it cannot play in the preview, which opens no bigger view', async () => {
			// Neither stored nor fetchable: nothing to play, only its still.
			for (const f of rows.map((x) => ({ ...x, stored: false, fetchable: false, url: null }))) {
				const r = renderPage(`/files?view=list&file=${f.id}`, 'superadmin', { props: { ...base(), rows, preview: f } });
				const d = await screen.findByRole('dialog', { name: f.name });
				const pane = d.querySelector('.a-preview')!;
				expect(within(d).queryByRole('button', { name: 'View bigger' })).toBeNull();
				if (f.thumb) {
					expect(pane.querySelector('img.a-preview-img')!.getAttribute('src')).toBe(f.thumb);
					if (f.file_type !== 'sticker') expect(pane.querySelector('.a-thumb-play')).toBeTruthy();
				} else if (f.minithumb) {
					expect(miniOf(pane)).toBe(MINI);
					expect(pane.querySelector('.a-thumb-play')).toBeTruthy();
				} else {
					expect(pane.querySelector('img')).toBeNull();
				}
				expect(pane.querySelector('video')).toBeNull();
				// The still shows beside the note that the video is not stored.
				expect(pane.querySelector('.a-preview-missing')).toBeTruthy();
				r.unmount();
			}
		});
	});

	it('lists a .tgs sticker in the stand-in with no thumb, and a .webm with a still, never its own URL, as Go sends them', () => {
		const rows = fixture<FilesProps>('/files?per=1000').props.rows;
		const tgs = rows.find((r) => r.ext === 'tgs' && r.stored)!;
		expect(tgs, 'a stored .tgs file').toBeTruthy();
		expect(tgs.thumb).toBeNull();
		for (const type of ['video', 'sticker']) {
			const f = rows.find((r) => r.ext === 'webm' && r.file_type === type && r.stored && r.thumb)!;
			expect(f, `a stored .webm ${type} with a still`).toBeTruthy();
			expect(f.thumb).toMatch(/^\/files\//);
			expect(f.thumb).not.toBe(f.url);
		}
	});

	it('gives the stand-in\'s videos a still and a blurred preview, some only the preview, some neither', () => {
		const vids = fixture<FilesProps>('/files?per=1000').props.rows.filter((r) => r.file_type === 'video');
		const mini = /^data:image\/jpeg;base64,/;
		expect(vids.some((r) => r.thumb && mini.test(r.minithumb ?? ''))).toBe(true);
		expect(vids.some((r) => !r.thumb && mini.test(r.minithumb ?? ''))).toBe(true);
		expect(vids.some((r) => !r.thumb && r.minithumb === undefined)).toBe(true);
		// Unstored videos keep their still too.
		expect(vids.some((r) => !r.stored && r.thumb)).toBe(true);
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
