// @vitest-environment happy-dom
// "Fetch from Telegram": a big unstored file (url null, fetch set) shows
// the button instead of "not stored" wherever it is drawn (chat media,
// the message page, the files list, card and preview, a group's Files
// tab, edit histories, a new chat photo, a background, a gift's and a
// giveaway's sticker); a click posts fetchFile once and the file then uses the URL
// answered; errors are put in words and are not retried by themselves.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import type { ReactElement } from 'react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { MessageMedia, type Media } from '../routes/chat-media.tsx';
import { PLAIN_KINDS } from '../test/kinds.ts';
import { resetFetches } from './fetchfile.tsx';
import { FILES_GROUP } from '../../standin/groupfiles.ts';
import type { ChatMessage } from '../types.ts';
import type { FilesProps } from '../routes/files.tsx';
import type { GroupFilesPage } from '../api/groupfiles.ts';
import type { MessageProps } from '../routes/message.tsx';
import type { ChatProps } from '../routes/chat.tsx';
import type { ServiceDetail } from './servicedetail.tsx';
import type { S } from '../api/schema.ts';
import { KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';

beforeEach(() => {
	setup();
	resetFetches();
	let meta = document.querySelector<HTMLMetaElement>('meta[name="csrf-token"]');
	if (!meta) { meta = document.createElement('meta'); meta.name = 'csrf-token'; document.head.append(meta); }
	meta.content = 'test-csrf';
});
afterEach(() => vi.useRealTimers());

const FETCH = '/api/files/tok-big/fetch';
const big = (o: Partial<Media> = {}): Media => ({ stored: false, fetchable: true, fetch: FETCH, url: null, thumb: null, size: 300 * 2 ** 20, ...o });
const msg = (content_type: ChatMessage['content_type'], m: Media): ChatMessage => ({
	id: 1, message_id: 1, date: '2026-11-17T10:00:00Z', scope: 'group', sender: null, content_type, text: '',
	media: m, album: [], reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null, ...PLAIN_KINDS,
});
const show = (el: ReactElement) => render(<div className="app">{el}</div>);
const button = () => screen.getByRole('button', { name: /Fetch(ing)? from Telegram/ }) as HTMLButtonElement;
const answer = (state: 'stored' | 'partial', url = '/files/tok-big') => ({ state, url, size: 300 * 2 ** 20 });

describe('chat media of a big unstored file', () => {
	it('fetches a video on a click and plays the stream at once', async () => {
		let release!: () => void;
		const calls = fakeApi('superadmin', { [`POST ${FETCH}`]: () => new Promise((r) => { release = () => r(answer('partial')); }) });
		show(<MessageMedia msg={msg('video', big({ duration: 600 }))} onClick={false} />);
		expect(screen.queryByText('Not stored')).toBeNull();
		expect(screen.getByText('On Telegram')).toBeTruthy();
		fireEvent.click(button());
		await waitFor(() => expect(button().disabled).toBe(true));
		expect(button().textContent).toContain('Fetching');
		const posts = callsTo(calls, 'POST', FETCH);
		expect(posts).toHaveLength(1);
		expect(posts[0]!.headers.get('x-csrf-token')).toBe('test-csrf');
		await act(async () => release());
		const viewer = await screen.findByRole('dialog', { name: 'Media viewer' });
		expect(viewer.querySelector('video')!.getAttribute('src')).toBe('/files/tok-big');
	});

	it('shows a photo once fetched, even with media on click', async () => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('stored') });
		show(<MessageMedia msg={msg('photo', big())} onClick />);
		fireEvent.click(button());
		const link = await screen.findByRole('link', { name: 'Photo' });
		expect(link.querySelector('img')!.getAttribute('src')).toBe('/files/tok-big');
	});

	it('turns a document into its download link, and offers the button for voice and stickers', async () => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('partial') });
		const r = show(<MessageMedia msg={msg('document', big({ name: 'dump.tar' }))} onClick={false} />);
		expect(screen.queryByText(/not stored/)).toBeNull();
		fireEvent.click(button());
		const link = await screen.findByRole('link', { name: /dump\.tar/ });
		expect(link.getAttribute('href')).toBe('/files/tok-big');
		r.unmount();
		resetFetches();
		for (const [ct, m] of [['voice', big({ duration: 9, wave: [] })], ['sticker', big({ format: 'webp', emoji: 'x' })]] as const) {
			const s = show(<MessageMedia msg={msg(ct, m)} onClick={false} />);
			expect(button()).toBeTruthy();
			expect(screen.queryByText(/not stored/i)).toBeNull();
			s.unmount();
		}
	});

	it('shows no button to a viewer without the fetch ability', () => {
		show(<MessageMedia msg={msg('video', big({ fetch: null, fetchable: false }))} onClick={false} />);
		expect(screen.queryByRole('button', { name: /Fetch from Telegram/ })).toBeNull();
		expect(screen.getByText('Not stored')).toBeTruthy();
	});

	it.each([
		[403, { error: 'fetch_not_allowed' }, {}, 'You may not fetch files.'],
		[410, { error: 'gone' }, {}, 'Telegram no longer has this file.'],
		[502, { error: 'daemon_failed' }, {}, 'Fetching from Telegram failed.'],
		[504, { error: 'timeout' }, {}, 'Telegram took too long. Try again in a moment.'],
		[503, { error: 'unavailable' }, { 'retry-after': '30' }, 'Telegram is busy, try again in 30 s.'],
	] as const)('says what a %i means', async (status, body, headers, words) => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json', ...headers } }) });
		show(<MessageMedia msg={msg('video', big())} onClick={false} />);
		fireEvent.click(button());
		expect((await screen.findByRole('alert')).textContent).toBe(words);
	});

	it('does not ask again while Retry-After runs, nor after a refusal', async () => {
		let n = 0;
		const calls = fakeApi('superadmin', { [`POST ${FETCH}`]: () => (++n === 1 ? new Response('{"error":"unavailable"}', { status: 503, headers: { 'content-type': 'application/json', 'retry-after': '1' } }) : json(403, { error: 'fetch_not_allowed' })) });
		show(<MessageMedia msg={msg('video', big())} onClick={false} />);
		fireEvent.click(button());
		await screen.findByRole('alert');
		expect(button().disabled).toBe(true);
		fireEvent.click(button());
		expect(callsTo(calls, 'POST', FETCH)).toHaveLength(1);
		// Enabled again once the wait is over; a refusal then is final.
		await waitFor(() => expect(button().disabled).toBe(false), { timeout: 2500 });
		fireEvent.click(button());
		await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('You may not fetch files.'));
		expect(button().disabled).toBe(true);
		fireEvent.click(button());
		expect(callsTo(calls, 'POST', FETCH)).toHaveLength(2);
	});

	it('lets a 502 be tried again by a click, never by itself', async () => {
		let n = 0;
		const calls = fakeApi('superadmin', { [`POST ${FETCH}`]: () => (++n === 1 ? json(502, { error: 'daemon_failed' }) : answer('partial')) });
		show(<MessageMedia msg={msg('document', big({ name: 'a.iso' }))} onClick={false} />);
		fireEvent.click(button());
		await screen.findByRole('alert');
		await new Promise((r) => setTimeout(r, 50));
		expect(callsTo(calls, 'POST', FETCH)).toHaveLength(1);
		fireEvent.click(button());
		expect((await screen.findByRole('link', { name: /a\.iso/ })).getAttribute('href')).toBe('/files/tok-big');
		expect(callsTo(calls, 'POST', FETCH)).toHaveLength(2);
	});
});

describe('the message page', () => {
	it('offers the button for a big file of a kind without a card', async () => {
		const id = fixture<{ rows: { id: number }[] }>('/group_messages').props.rows[0]!.id;
		const base = fixture<MessageProps>(`/messages/group/${id}`).props.message;
		const m = { ...base, content_type: 'unknown', media: { file_id: 9, name: 'blob.bin', size: 2 ** 30, stored: false, thumb: null, url: null, fetch: FETCH } } as unknown as MessageProps['message'];
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('partial') });
		renderPage(`/messages/group/${base.id}`, 'superadmin', { route: 'message', status: 200, props: { message: m } });
		fireEvent.click(button());
		await waitFor(() => expect(screen.queryByRole('button', { name: /Fetch from Telegram/ })).toBeNull());
	});
});

describe('the files page', () => {
	const bigRow = () => {
		const p = fixture<FilesProps>('/files?view=list').props;
		return { p, f: { ...p.rows[0]!, id: 4250, name: 'huge.mkv', ext: 'mkv', file_type: 'video' as const, stored: false, fetchable: true, fetch: FETCH, url: null, thumb: null } };
	};

	it('offers the button in the list row and in the preview, and both use the answer', async () => {
		const { p, f } = bigRow();
		const calls = fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('partial') });
		renderPage('/files?view=list&file=4250', 'superadmin', { props: { ...p, rows: [f], preview: f } });
		const d = await screen.findByRole('dialog', { name: 'huge.mkv' });
		expect(within(d).getByText('no, can be fetched from Telegram')).toBeTruthy();
		expect(within(d).queryByText(/only its details were kept/)).toBeNull();
		const row = screen.getAllByRole('row').find((r) => within(r).queryByText('huge.mkv'))!;
		expect(within(row).queryByText('missing')).toBeNull();
		expect(within(row).getByRole('button', { name: /Fetch from Telegram/ })).toBeTruthy();
		fireEvent.click(within(d).getByRole('button', { name: /Fetch from Telegram/ }));
		const dl = await within(d).findByRole('link', { name: /Download/ });
		expect(dl.getAttribute('href')).toBe('/files/tok-big');
		// The row has the same file: fetched once, now fetchable by URL.
		expect(within(row).getByText('fetchable')).toBeTruthy();
		expect(callsTo(calls, 'POST', FETCH)).toHaveLength(1);
	});

	it('tags a card of a big file as on Telegram', () => {
		const { p, f } = bigRow();
		fakeApi('superadmin');
		renderPage('/files', 'superadmin', { props: { ...p, rows: [f] } });
		const card = document.querySelector<HTMLElement>('.a-filecard')!;
		expect(within(card).getByText('On Telegram')).toBeTruthy();
	});
});

describe('a group\'s Files tab', () => {
	it('offers the button in the list row and the preview of a big file', async () => {
		const p = fixture<GroupFilesPage>(`/groups/${FILES_GROUP.id}/files`);
		const f = { ...p.props.rows[0]!, name: 'big.mp4', file_type: 'video' as const, stored: false, fetchable: true, fetch: FETCH, url: null, thumb: null };
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('stored') });
		const props = { ...p.props, rows: [f], next_before: null };
		const { unmount } = renderPage(`/groups/${FILES_GROUP.id}/files?view=list`, 'superadmin', { props });
		const row = document.querySelector<HTMLElement>('.a-table tbody tr')!;
		expect(within(row).queryByText('missing')).toBeNull();
		expect(within(row).getByRole('button', { name: /Fetch from Telegram/ })).toBeTruthy();
		unmount();
		renderPage(`/groups/${FILES_GROUP.id}/files`, 'superadmin', { props });
		fireEvent.click(document.querySelector<HTMLElement>('.a-groupfile .a-filecard')!);
		const d = await screen.findByRole('dialog', { name: 'big.mp4' });
		expect(within(d).getByText('no, can be fetched from Telegram')).toBeTruthy();
		fireEvent.click(within(d).getByRole('button', { name: /Fetch from Telegram/ }));
		expect((await within(d).findByRole('link', { name: /Download/ })).getAttribute('href')).toBe('/files/tok-big');
	});
});

// A chat with one message, built on a plain text one.
const CHAT = `/groups/${KINDS_GROUP.id}/chat`;
function chatWith(o: Partial<ChatMessage>) {
	const page = fixture<ChatProps>(CHAT);
	const t = page.props.messages.find((m) => m.content_type === 'text')!;
	const m = { ...t, id: 1e9, message_id: 9e5, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], ...o } as ChatMessage;
	renderPage(CHAT, 'superadmin', { props: { ...page.props, messages: [m], at: null } });
	return document.getElementById('m900000')!;
}
const serviceLine = (type: string, detail: ServiceDetail) => chatWith({ content_type: 'service', text: 'did something', sender: { id: 5, name: 'Sample Sender' } as ChatMessage['sender'], service: { type, refs: { users: [], chats: [], message_id: null }, detail } as ChatMessage['service'] });
const EDIT = { at: '2026-11-17T09:00:00Z', text: 'before', entities: [], content_type: 'document' as const, detail: null };

describe('edit-history media of a big unstored file', () => {
	it('offers the button in the chat\'s edit list', async () => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('stored') });
		const b = chatWith({ content_type: 'document', media: big({ name: 'now.pdf', url: '/files/now' }), edits: [{ ...EDIT, media: big({ name: 'old.pdf' }) }], edited_at: '2026-11-17T09:05:00Z' });
		fireEvent.click(within(b).getByRole('button', { name: /edited/ }));
		const list = b.querySelector('.c-edits') as HTMLElement;
		expect(list.textContent).toContain('(on Telegram)');
		expect(list.textContent).not.toContain('not stored');
		fireEvent.click(within(list).getByRole('button', { name: 'Fetch from Telegram' }));
		await waitFor(() => expect(within(list).queryByRole('button', { name: /Fetch(ing)? from Telegram/ })).toBeNull());
		expect(list.textContent).not.toContain('on Telegram');
	});

	it('offers the button in the message page\'s edit history, then links the file', async () => {
		const id = fixture<{ rows: { id: number }[] }>('/group_messages').props.rows[0]!.id;
		const p = fixture<MessageProps>(`/messages/group/${id}`).props;
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('partial') });
		renderPage(`/messages/group/${id}`, 'superadmin', { route: 'message', status: 200, props: { message: { ...p.message, edits: [{ ...EDIT, media: big({ name: 'old.pdf' }) }], edited_at: '2026-11-17T09:05:00Z' } } });
		expect(screen.getByText('old.pdf (on Telegram)')).toBeTruthy();
		fireEvent.click(button());
		expect((await screen.findByRole('link', { name: 'old.pdf' })).getAttribute('href')).toBe('/files/tok-big');
	});
});

describe('service-detail media of a big unstored file', () => {
	it('offers the button beside a new chat photo, and plays its animation once fetched', async () => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('partial', '/files/tok-anim') });
		const photo = big({ render: 'image', ext: 'jpg', fetch: '/api/files/tok-photo/fetch' });
		const anim = big({ render: 'animation', ext: 'mp4' });
		const l = serviceLine('chat_change_photo', { kind: 'photo', photo, animation: anim } as ServiceDetail);
		expect(within(l).getByRole('img', { name: 'Photo on Telegram' })).toBeTruthy();
		fireEvent.click(within(l).getByRole('button', { name: 'Fetch from Telegram' }));
		const viewer = await screen.findByRole('dialog', { name: 'Media viewer' });
		expect(viewer.querySelector('video')!.getAttribute('src')).toBe('/files/tok-anim');
	});

	it('offers the button beside a chat background image', async () => {
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('stored') });
		const bg = { kind: 'background', only_for_self: false, type: 'wallpaper', dark: false, image: big({ render: 'image', ext: 'jpg', mime: 'image/jpeg' }), fill: null, intensity: null, is_inverted: false, is_moving: false, is_blurred: false, theme_name: null };
		const l = serviceLine('chat_set_background', bg as ServiceDetail);
		expect(l.querySelector('[aria-label$="(on Telegram)"]')).toBeTruthy();
		fireEvent.click(within(l).getByRole('button', { name: 'Fetch from Telegram' }));
		await waitFor(() => expect(l.querySelector('a.c-bgswatch')?.getAttribute('href')).toBe('/files/tok-big'));
	});

	it('offers the button for a gift\'s sticker, not for its backdrop symbol', () => {
		fakeApi('superadmin');
		const st = big({ render: 'image', ext: 'webp', format: 'webp', emoji: 'x' });
		const l = serviceLine('gift', { kind: 'gift', sticker: st, symbol_sticker: big({ render: 'image', ext: 'webp', format: 'webp', fetch: '/api/files/tok-sym/fetch' }), star_count: null, price: null, sender: null, receiver: null, text: '', entities: [], upgraded: null } as ServiceDetail);
		const c = l.querySelector('.c-gift') as HTMLElement;
		expect(within(c).getAllByRole('button', { name: 'Fetch from Telegram' })).toHaveLength(1);
		expect(c.querySelector('.c-giftsymbol button')).toBeNull();
	});

	it('offers the button for a giveaway\'s prize sticker', async () => {
		const raw = (chatStream(KINDS_GROUP) as { id: number }[]).find((m) => sampleOf(m as never)?.content_type === 'giveaway')!;
		const page = fixture<MessageProps>(`/messages/group/${raw.id}`);
		const m = page.props.message;
		fakeApi('superadmin', { [`POST ${FETCH}`]: () => answer('stored') });
		renderPage(`/messages/group/${raw.id}`, 'superadmin', { props: { ...page.props, message: { ...m, detail: { ...(m.detail as S['GiveawayDetail']), sticker: big({ render: 'image', ext: 'webp', format: 'webp', emoji: 'x' }) } } } });
		const box = document.querySelector('.a-msgkind .k-giftsticker') as HTMLElement;
		fireEvent.click(within(box).getByRole('button', { name: 'Fetch from Telegram' }));
		await waitFor(() => expect(box.querySelector('img')?.getAttribute('src')).toBe('/files/tok-big'));
	});
});
