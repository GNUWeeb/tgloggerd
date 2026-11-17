// @vitest-environment happy-dom
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
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
});
