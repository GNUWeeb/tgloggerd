// @vitest-environment happy-dom
// The chat view's parity with the Drogon one: who a message was posted
// as, forwards, signatures, replies, link previews, service messages,
// and opening the chat at the oldest messages, a time or an old anchor.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
const groupId = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const chatUrl = () => `/groups/${groupId()}/chat`;
const base = () => fixture<ChatProps>(chatUrl()).props;
const pageCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/page\//);

// A window opened before the newest messages loads the next page by
// itself while it is too short to scroll, as it always is without a
// layout. Wait until the load from the window at `url` has been
// answered (each test answers that it reached the newest), so none is
// still on its way once the test's fake fetch is gone.
// The wait is generous: the full suite under load can take well over
// waitFor's default second to render a window and answer its load.
async function newerLoaded(calls: ReturnType<typeof fakeApi>, url: string) {
	const last = String(fixture<ChatProps>(url).props.messages.at(-1)!.message_id);
	await waitFor(() => expect(callsTo(calls, 'GET', /\/messages$/).some((c) => c.query.get('after') === last)).toBe(true), { timeout: 5000 });
	await waitFor(() => expect(screen.queryByText('Loading newer messages...')).toBeNull(), { timeout: 5000 });
}

// The chat with these messages only, each built on a plain text one.
function chatWith(msgs: Partial<ChatMessage>[]) {
	const p = base();
	const t = p.messages.find((m) => m.content_type === 'text')!;
	const messages = msgs.map((o, i) => ({ ...t, id: 1e9 + i, message_id: 9e5 + i, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], ...o }) as ChatMessage);
	return renderPage(chatUrl(), 'superadmin', { props: { ...p, messages, at: null } });
}
const row = (i: number) => document.getElementById(`m${9e5 + i}`)!;

describe('chat parity', () => {
	it('links a channel post to its channel and shows its signature', () => {
		const g = fixture<GroupsProps>('/groups').props.rows[1]!;
		chatWith([{ sender: { id: g.id, name: g.title, username: '', type: 'regular', photo: null }, sender_chat: g, is_channel_post: true, author_signature: 'Dewi Kusuma', text: 'a post' }]);
		// The avatar and the name both link to the channel.
		const links = within(row(0)).getAllByRole('link', { name: g.title });
		expect(links.length).toBe(2);
		expect(links.every((a) => a.getAttribute('href') === `/groups/${g.id}`)).toBe(true);
		expect(within(row(0)).getByText('- Dewi Kusuma')).toBeTruthy();
	});

	it('names a chat the viewer may not see without linking it', () => {
		const hidden = { id: 0, name: '(hidden chat)', username: '', type: 'unknown' as const, photo: null };
		chatWith([
			{ sender: hidden, text: 'posted as a hidden chat' },
			{ forward: '(hidden chat)', forward_from: { type: 'channel', name: '(hidden chat)', date: '2026-11-01T08:00:00Z' } },
			{ reply_to: { message_id: 5, sender_id: 0, sender_name: '(hidden chat)', snippet: 'hi', resolved: true } },
		]);
		expect(within(row(0)).getByText('(hidden chat)').closest('a')).toBeNull();
		expect(row(0).querySelector('a[href="/users/0"]')).toBeNull();
		expect(within(row(1)).getByText('(hidden chat)').closest('a')).toBeNull();
		expect(row(2).querySelector('a[href*="/users/0"], a[href*="/groups/0"]')).toBeNull();
	});

	it('links a forward to where it came from', () => {
		chatWith([
			{ forward: 'Linux Weekly News', forward_from: { type: 'channel', name: 'Linux Weekly News', chat_id: -100123, message_id: 77 } },
			{ forward: 'Ayu', forward_from: { type: 'user', name: 'Ayu', user_id: 42 } },
			{ forward: 'Someone', forward_from: { type: 'hidden_user', name: 'Someone' } },
			{ forward: 'Old style' },
		]);
		expect(within(row(0)).getByRole('link', { name: 'Linux Weekly News' }).getAttribute('href')).toBe('/groups/-100123/chat?at=77');
		expect(within(row(1)).getByRole('link', { name: 'Ayu' }).getAttribute('href')).toBe('/users/42');
		expect(within(row(2)).getByText('Someone').closest('a')).toBeNull();
		expect(within(row(3)).getByText('Old style')).toBeTruthy();
	});

	it('opens a reply in another chat there, and marks unknown and deleted targets', () => {
		chatWith([
			{ reply_to: { message_id: 5, sender_id: 1, sender_name: 'Ayu', snippet: 'hi', resolved: true, chat_id: -100555 } },
			{ reply_to: { message_id: 6, sender_id: 0, sender_name: '', snippet: '', resolved: false } },
			{ reply_to: { message_id: 7, sender_id: 1, sender_name: 'Ayu', snippet: 'gone', resolved: true, deleted: true } },
			{ reply_to: { message_id: 8, sender_id: 1, sender_name: 'Ayu', snippet: 'dm', resolved: true, chat_id: 4242 } },
		]);
		expect(row(0).querySelector('.c-reply')!.getAttribute('href')).toBe('/groups/-100555/chat?at=5');
		expect(within(row(0)).getByText(/in another chat/)).toBeTruthy();
		expect(within(row(1)).getByText('message #6')).toBeTruthy();
		expect(within(row(1)).getByText('not in the archive')).toBeTruthy();
		expect(within(row(2)).getByText('gone (deleted)')).toBeTruthy();
		expect(row(3).querySelector('.c-reply')!.getAttribute('href')).toBe('/users/4242/chat?at=8');
	});

	it('links a link preview to its page, with its picture', () => {
		chatWith([{ link_preview: { site: 'GitHub', title: 'A commit', desc: 'd', url: 'https://github.com/x/y', media: { stored: true, url: '/files/t', thumb: '/files/t', render: 'image', w: 1200, h: 600 } } }]);
		const a = row(0).querySelector('a.c-lp')!;
		expect(a.getAttribute('href')).toBe('https://github.com/x/y');
		expect(a.getAttribute('target')).toBe('_blank');
		expect(a.getAttribute('rel')).toContain('noopener');
		expect(a.querySelector('img')!.getAttribute('src')).toBe('/files/t');
	});

	it('says what a service message without text did, by whom and when', () => {
		const p = base();
		const s = p.messages.find((m) => m.content_type === 'text')!;
		chatWith([{ content_type: 'service', text: '', service_type: 'pin_message', sender: s.sender }]);
		const svc = row(0);
		expect(svc.className).toContain('c-service');
		expect(svc.textContent).toContain('Message pinned');
		expect(within(svc).getByRole('link', { name: s.sender!.name }).getAttribute('href')).toBe(`/users/${s.sender!.id}`);
		expect(svc.querySelector('.c-svctime')).toBeTruthy();
	});

	it('names a kind of message it cannot show yet', () => {
		chatWith([{ content_type: 'unknown', text: '', tdlib_type: 'messageVideoNote' }]);
		expect(within(row(0)).getByText(/messageVideoNote/)).toBeTruthy();
	});

	it('lists earlier versions with their media and former type', () => {
		chatWith([{
			content_type: 'photo', text: 'now a photo', edited_at: '2026-11-10T11:00:00Z',
			media: { stored: true, url: '/files/now', thumb: '/files/now', render: 'image' },
			edits: [
				{ at: '2026-11-10T10:00:00Z', text: 'was a document', content_type: 'document', media: { stored: true, url: '/files/doc', thumb: null, name: 'notes.pdf' }, entities: [] },
				{ at: '2026-11-10T10:30:00Z', text: 'an older photo', content_type: 'photo', media: { stored: true, url: '/files/old', thumb: '/files/old' }, entities: [] },
			],
		}]);
		fireEvent.click(within(row(0)).getByRole('button', { name: /edited/ }));
		const list = row(0).querySelector('.c-edits') as HTMLElement;
		expect(within(list).getByText(/was document/)).toBeTruthy();
		expect(within(list).getByText('document: notes.pdf')).toBeTruthy();
		expect(within(list).getByRole('link', { name: 'Earlier photo' }).querySelector('img')!.getAttribute('src')).toBe('/files/old');
	});

	it('marks a message edited even when no earlier version was recorded', () => {
		chatWith([{ text: 'changed', edits: [], edited_at: '2026-11-10T11:00:00Z' }]);
		const mark = within(row(0)).getByText('edited');
		expect(mark.closest('button')).toBeNull();
		expect(mark.closest('[title]')!.getAttribute('title')).toMatch(/no earlier version recorded/);
	});

	it('jumps to the oldest messages and to a date and time', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: false }) });
		renderPage(chatUrl());
		fireEvent.click(screen.getByRole('button', { name: 'Jump to the oldest messages' }));
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(pageCalls(calls)[0]!.query.get('after')).toBe('0');
		await newerLoaded(calls, `${chatUrl()}?after=0`);
		const input = document.querySelector('.c-datebtn input')!;
		expect(input.getAttribute('type')).toBe('datetime-local');
		fireEvent.change(input, { target: { value: '2026-11-10T12:30' } });
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(2));
		expect(pageCalls(calls)[1]!.query.get('date')).toBe('2026-11-10T12:30');
		expect(pageCalls(calls)[1]!.query.get('after')).toBeNull();
		await newerLoaded(calls, `${chatUrl()}?date=2026-11-10T12%3A30`);
	});

	it('opens an old #msg-N link at that message', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: false }) });
		renderPage(`${chatUrl()}#msg-555422`);
		await waitFor(() => expect(pageCalls(calls)).toHaveLength(1));
		expect(pageCalls(calls)[0]!.query.get('at')).toBe('555422');
		await newerLoaded(calls, `${chatUrl()}?at=555422`);
	});

	it('counts the members the archive has seen, and leaves out none', () => {
		const p = base();
		if (p.chat.kind !== 'group') throw new Error('group chat expected');
		const r = renderPage(chatUrl(), 'superadmin', { props: { ...p, chat: { ...p.chat, group: { ...p.chat.group, members: 12 } } } });
		expect(screen.getByText(/12 members seen/)).toBeTruthy();
		r.unmount();
		renderPage(chatUrl(), 'superadmin', { props: { ...p, chat: { ...p.chat, group: { ...p.chat.group, members: 0 } } } });
		expect(screen.queryByText(/members/)).toBeNull();
	});
});
