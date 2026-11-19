// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
const groupId = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const chatUrl = () => `/groups/${groupId()}/chat`;
const windowCalls = (calls: ReturnType<typeof fakeApi>) => callsTo(calls, 'GET', /^\/api\/chats\/group\/-?\d+\/messages$/);

describe('chat', () => {
	it('shows the window of messages with the group in the header', () => {
		const { page } = renderPage(chatUrl());
		const p = page.props as ChatProps;
		const shown = p.messages.filter((m) => m.content_type !== 'service');
		expect(document.querySelectorAll('.c-bubble').length).toBe(shown.length);
		expect(screen.getByRole('heading', { level: 1 }).textContent).toBe(p.chat.kind === 'group' ? p.chat.group.title : '');
	});

	it('without the event stream, polls for newer messages from the cursor and appends them', async () => {
		const p = fixture<ChatProps>(chatUrl()).props;
		const last = p.messages.at(-1)!;
		const fresh: ChatMessage = { ...last, id: last.id + 1, message_id: last.message_id + 1, text: 'a brand new message', content_type: 'text', media: null, album: [] };
		let served = false;
		const calls = fakeApi('superadmin', {
			'GET /api/chats/group/:id/messages': (c) => (c.query.get('after') === String(last.message_id) && !served ? (served = true, { messages: [fresh], has_older: true, has_newer: false }) : { messages: [], has_older: true, has_newer: false }),
		});
		renderPage(chatUrl());
		await screen.findByText('a brand new message', undefined, { timeout: 3000 });
		expect(windowCalls(calls)[0]!.query.get('after')).toBe(String(last.message_id));
	});

	it('stops following when paused', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: false }) });
		renderPage(chatUrl());
		fireEvent.click(screen.getByRole('button', { name: /Live/ }));
		expect(screen.getByRole('button', { name: /Paused/ }).getAttribute('aria-pressed')).toBe('false');
		const n = windowCalls(calls).length;
		await new Promise((r) => setTimeout(r, 1500));
		expect(windowCalls(calls).length).toBe(n);
		expect(screen.getByText(/Live updates paused/)).toBeTruthy();
	});

	it('searches in the chat through the message list, filtered to this chat', async () => {
		const id = groupId();
		const calls = fakeApi('superadmin');
		renderPage(chatUrl());
		fireEvent.click(screen.getByRole('button', { name: 'Search in chat' }));
		fireEvent.change(screen.getByPlaceholderText(/Search in /), { target: { value: 'kernel' } });
		fireEvent.submit(screen.getByPlaceholderText(/Search in /).closest('form')!);
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/group_messages')).toHaveLength(1));
		const c = callsTo(calls, 'GET', '/api/page/group_messages')[0]!;
		expect(c.query.get('q')).toBe('kernel');
		expect(JSON.parse(c.query.get('f')!)).toEqual([{ field: 'chat_id', op: '=', value: String(id) }]);
	});

	it('jumps to a replied-to message outside the window with ?at=', async () => {
		const p = fixture<ChatProps>(chatUrl()).props;
		const withReply = p.messages.find((m) => m.reply_to && !p.messages.some((x) => x.message_id === m.reply_to!.message_id));
		const target = withReply ?? p.messages.find((m) => m.reply_to)!;
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: false }) });
		const page = { props: { ...p, messages: p.messages.map((m) => (m.id === target.id ? { ...m, reply_to: { ...m.reply_to!, message_id: 1 } } : m)) } };
		renderPage(chatUrl(), 'superadmin', page);
		fireEvent.click(document.querySelector('.c-reply')!);
		await waitFor(() => expect(callsTo(calls, 'GET', /^\/api\/page\/groups\//).at(-1)?.query.get('at')).toBe('1'));
	});

	it('shows a signed-out visitor the public page of an exposed group, read only', () => {
		const exposed = fixture<GroupsProps>('/groups').props.rows.find((g) => g.exposed)!;
		renderPage(`/groups/${exposed.id}/chat`, 'public');
		expect(screen.queryByRole('button', { name: /Live/ })).toBeNull();
		expect(screen.getByRole('link', { name: /Sign in/ }).getAttribute('href')).toContain('/login?next=');
		expect(screen.getByText(/Public archive of/)).toBeTruthy();
	});

	describe('the viewer\'s chat preferences', () => {
		const withPrefs = (chat: Record<string, unknown>) => {
			const page = fixture<ChatProps>(chatUrl());
			return { ...page, viewer: { ...page.viewer, chat: { ...page.viewer.chat, ...chat } } };
		};

		it('hide deleted and service messages and edit markers when turned off', () => {
			const page = withPrefs({ show_deleted: false, show_service: false, show_edited: false });
			const p = page.props;
			const msgs = [...p.messages];
			msgs[0] = { ...msgs[0]!, content_type: 'service', text: 'joined the group' };
			msgs[1] = { ...msgs[1]!, content_type: 'text', deleted_at: '2026-11-17T10:00:00Z', text: 'gone message' };
			msgs[2] = { ...msgs[2]!, content_type: 'text', edits: [{ at: '2026-11-17T09:00:00Z', text: 'before', entities: [] }], edited_at: '2026-11-17T09:05:00Z', deleted_at: null };
			renderPage(chatUrl(), 'superadmin', { viewer: page.viewer, props: { ...p, messages: msgs } });
			expect(screen.queryByText(/joined the group/)).toBeNull();
			expect(screen.queryByText('gone message')).toBeNull();
			expect(document.querySelector(`#m${msgs[2]!.message_id} .c-edited`)).toBeNull();
		});

		it('load media only on click when asked to', () => {
			const page = withPrefs({ media: 'click' });
			const photo = page.props.messages.find((m) => m.content_type === 'photo' && m.media?.stored)!;
			renderPage(chatUrl(), 'superadmin', { viewer: page.viewer });
			const row = document.querySelector(`#m${photo.message_id}`)!;
			const show = within(row as HTMLElement).getAllByRole('button', { name: /Show/ })[0]!;
			fireEvent.click(show);
			expect(row.querySelector('a.c-media')).not.toBeNull();
		});

		it('load older messages on a button instead of on scrolling', async () => {
			const page = withPrefs({ auto_load: false });
			const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: false, has_newer: false }) });
			renderPage(chatUrl(), 'superadmin', { viewer: page.viewer });
			const scroller = document.querySelector('.c-scroll') as HTMLElement;
			scroller.scrollTop = 0;
			fireEvent.scroll(scroller);
			expect(windowCalls(calls).filter((c) => c.query.get('before'))).toHaveLength(0);
			fireEvent.click(screen.getByRole('button', { name: 'Load older messages' }));
			await waitFor(() => expect(windowCalls(calls).filter((c) => c.query.get('before'))).toHaveLength(1));
		});
	});
});
