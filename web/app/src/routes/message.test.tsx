// @vitest-environment happy-dom
// The message page's parity with the Drogon one: channel posts, who a
// message was posted as, signatures, where a forward came from, and
// edits with their former type and media.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { MessageProps } from './message.tsx';
import type { MessagesProps } from './messages.tsx';
import type { GroupsProps } from './groups.tsx';

beforeEach(setup);
const url = () => `/messages/group/${fixture<MessagesProps>('/group_messages').props.rows[0]!.id}`;
const render = (patch: Partial<MessageProps['message']>) => {
	const p = fixture<MessageProps>(url()).props;
	return renderPage(url(), 'superadmin', { props: { message: { ...p.message, ...patch } } });
};
const card = (title: string) => screen.getByRole('heading', { level: 2, name: title }).closest('.a-card') as HTMLElement;

describe('message page', () => {
	it('marks a channel post, links the chat it was posted as, and shows its signature', () => {
		const g = fixture<GroupsProps>('/groups').props.rows[1]!;
		render({ is_channel_post: true, sender_chat: g, author_signature: 'Dewi Kusuma', text: 'a post' });
		expect(screen.getByText('channel post')).toBeTruthy();
		expect(screen.getAllByRole('link', { name: g.title }).every((a) => a.getAttribute('href') === `/groups/${g.id}`)).toBe(true);
		expect(screen.getByText('- Dewi Kusuma')).toBeTruthy();
		expect(screen.getByText('Posted as')).toBeTruthy();
	});

	it('says where a forward came from, with links to the channel and its post', () => {
		render({ forwarded: 'Linux Weekly News', forward_from: { type: 'channel', name: 'Linux Weekly News', chat_id: -100123, message_id: 77, date: '2026-11-01T08:00:00Z' } });
		const c = card('Forwarded from');
		expect(within(c).getByText('a channel')).toBeTruthy();
		expect(within(c).getByRole('link', { name: 'Linux Weekly News' }).getAttribute('href')).toBe('/groups/-100123');
		expect(within(c).getByRole('link', { name: '#77 in the chat' }).getAttribute('href')).toBe('/groups/-100123/chat?at=77');
		expect(within(c).getByText('Originally sent')).toBeTruthy();
	});

	it('links a forward from a user to that user', () => {
		render({ forwarded: 'Ayu', forward_from: { type: 'user', name: 'Ayu', user_id: 42 } });
		expect(within(card('Forwarded from')).getByRole('link', { name: 'Ayu' }).getAttribute('href')).toBe('/users/42');
	});

	it('lists earlier versions with their former type and media', () => {
		render({
			content_type: 'photo', text: 'now', edited: true, edited_at: '2026-11-10T11:00:00Z',
			edits: [
				{ at: '2026-11-10T10:00:00Z', text: 'was a file', content_type: 'document', media: { stored: true, url: '/files/doc', thumb: null, name: 'notes.pdf' }, entities: [] },
				{ at: '2026-11-10T10:30:00Z', text: '', content_type: 'photo', media: { stored: true, url: '/files/old', thumb: '/files/old' }, entities: [] },
			],
		});
		const c = card('Edit history');
		expect(within(c).getByText('Earlier version (document)')).toBeTruthy();
		expect(within(c).getByRole('link', { name: 'notes.pdf' }).getAttribute('href')).toBe('/files/doc');
		expect(within(c).getByRole('link', { name: 'Earlier file' }).querySelector('img')!.getAttribute('src')).toBe('/files/old');
	});

	it('names a chat the viewer may not see without linking it', () => {
		render({ sender: { id: 0, name: '(hidden chat)', username: '', type: 'unknown', photo: null } });
		expect(screen.getAllByText('(hidden chat)').every((e) => e.closest('a') === null)).toBe(true);
		expect(document.querySelector('a[href="/users/0"]')).toBeNull();
	});

	it('names a kind of message it cannot show yet', () => {
		render({ content_type: 'unknown', text: '', media: null, tdlib_type: 'messageVideoNote' });
		expect(screen.getByText('messageVideoNote')).toBeTruthy();
	});
});
