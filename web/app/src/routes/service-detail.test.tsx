// @vitest-environment happy-dom
// A service message's stored detail on its line in the chat and on its
// message page: the new photo, the pinned message quoted, a video
// chat's length and start, a poll option, the topic and community
// linked, the old title, a background set for the actor alone.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { ALL, FORUM_GROUP, KINDS_GROUP, PUBLIC_POLLS_GROUP, TOPICS, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';
import type { ServiceDetail } from '../ui/servicedetail.tsx';

beforeEach(setup);
const chatUrl = `/groups/${FORUM_GROUP.id}/chat`;
const ref = (id: number, title: string) => ({ id, title, kind: 'group' as const, photo: null });
const PHOTO = { stored: true, fetchable: false, url: '/files/p1', thumb: '/files/p1t', render: 'image', ext: 'jpg', w: 640, h: 640, name: '', size: 1000 } as unknown as ChatMessage['media'] & object;
const SENDER = { id: 5, name: 'Sample Sender' } as ChatMessage['sender'];

// The chat with one service message, built on a plain text one;
// undefined leaves the detail out, as the API does when there is none.
function line(type: string, text: string, detail: ServiceDetail | null | undefined, o: Partial<ChatMessage> = {}, viewer: object = {}) {
	const page = fixture<ChatProps>(chatUrl);
	const t = page.props.messages.find((m) => m.content_type === 'text')!;
	const service = { type, refs: { users: [], chats: [], message_id: null }, ...(detail !== undefined ? { detail } : {}) } as ChatMessage['service'];
	const m = { ...t, id: 1e9, message_id: 9e5, content_type: 'service', text, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], sender: SENDER, service, ...o } as ChatMessage;
	fakeApi('superadmin');
	renderPage(chatUrl, 'superadmin', { props: { ...page.props, messages: [m], at: null }, viewer: { ...page.viewer, ...viewer } });
	return document.getElementById('m900000')!.firstElementChild as HTMLElement;
}
// The line's words, without its time and deleted tag.
const words = (el: Element) => [...el.childNodes].filter((n) => !(n instanceof Element && n.matches('.c-svctime, .c-deltag'))).map((n) => n.textContent).join('').trim();

describe('a service line in the chat', () => {
	it('shows the new group photo, which opens the viewer', () => {
		const l = line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO });
		const a = within(l).getByRole('link', { name: 'The new photo' });
		expect(a.querySelector('img')!.getAttribute('src')).toBe('/files/p1t');
		fireEvent.click(a);
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/p1');
	});

	it('says a new photo was not stored, and waits to be asked when media load on click', () => {
		const lost = line('suggest_profile_photo', 'suggested a profile photo', { kind: 'photo', photo: { ...PHOTO, stored: false, url: null, thumb: null } });
		expect(within(lost).getByRole('img', { name: 'Photo not stored' })).toBeTruthy();
		document.body.innerHTML = '';
		const l = line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO }, {}, { chat: { media: 'click' } });
		expect(l.querySelector('img')).toBeNull();
		fireEvent.click(within(l).getByRole('button', { name: 'Show the photo' }));
		expect(within(l).getByRole('link', { name: 'The new photo' })).toBeTruthy();
	});

	it('quotes the pinned message', () => {
		const l = line('pin_message', 'pinned a message', { kind: 'pin', snippet: 'Meeting moved to Friday' });
		expect(l.querySelector('q')!.textContent).toBe('Meeting moved to Friday');
		expect(words(l)).toBe('Sample Sender pinned a message Meeting moved to Friday');
	});

	it('says how long a video chat ran, and when one is planned', () => {
		expect(words(line('video_chat_ended', 'ended the video chat', { kind: 'video_chat', duration: 3725, start_date: null }))).toBe('Sample Sender ended the video chat after 1 h 2 min');
		document.body.innerHTML = '';
		const l = line('video_chat_scheduled', 'scheduled a video chat', { kind: 'video_chat', duration: null, start_date: '2026-12-01T12:00:00Z' });
		expect(l.querySelector('time')!.getAttribute('datetime')).toBe('2026-12-01T12:00:00Z');
		expect(words(l)).toMatch(/^Sample Sender scheduled a video chat for .*2026/);
	});

	it('quotes a poll option added or removed', () => {
		expect(words(line('poll_option_added', 'added an option to a poll', { kind: 'poll_option', text: 'Rust' }))).toBe('Sample Sender added an option to a poll: Rust');
		expect(document.querySelector('.c-service q')!.textContent).toBe('Rust');
	});

	it('links the topic a text names, with its colour, and names one it does not', () => {
		const t = TOPICS[1]!;
		const topic = { id: t.id, name: t.name, icon_color: t.icon_color };
		const l = line('forum_topic_created', `created topic "${t.name}"`, { kind: 'topic', topic });
		const a = within(l).getByRole('link', { name: t.name });
		expect(a.getAttribute('href')).toContain(`topic=${t.id}`);
		expect((a.querySelector('.k-topicdot') as HTMLElement).style.background).toBeTruthy();
		expect(words(l)).toBe(`Sample Sender created topic "${t.name}"`);
		document.body.innerHTML = '';
		const e = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic });
		expect(words(e)).toBe(`Sample Sender changed the topic icon: ${t.name}`);
		expect(within(e).getByRole('link', { name: t.name }).getAttribute('href')).toContain(`topic=${t.id}`);
	});

	it('names a topic whose id, name or colour is not known', () => {
		const l = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic: { id: null, name: 'Help', icon_color: null } });
		expect(words(l)).toBe('Sample Sender changed the topic icon: Help');
		expect(within(l).queryByRole('link', { name: 'Help' })).toBeNull();
		expect((l.querySelector('.c-svctopic .k-topicdot') as HTMLElement).style.background).toBeTruthy();
		document.body.innerHTML = '';
		const n = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic: { id: 12, name: null, icon_color: 0xffd67e } });
		expect(words(n)).toBe('Sample Sender changed the topic icon: a topic');
		expect(within(n).getByRole('link', { name: 'a topic' }).getAttribute('href')).toContain('topic=12');
	});

	it('links the community', () => {
		const l = line('chat_added_to_community', 'added the group to a community', { kind: 'community', community: ref(-1007, 'Linux Users') });
		expect(within(l).getByRole('link', { name: 'Linux Users' }).getAttribute('href')).toBe('/groups/-1007');
	});

	it('gives a created group its title, and links the basic group a supergroup came from', () => {
		expect(words(line('chat_create', 'created the group', { kind: 'title', title: 'Study Club', basic_group: null }))).toBe('Sample Sender created the group Study Club');
		document.body.innerHTML = '';
		const l = line('chat_upgrade_from', 'the supergroup was created from a group', { kind: 'title', title: 'Study Club', basic_group: ref(-42, 'Study Club') });
		expect(words(l)).toBe('The supergroup was created from a group Study Club');
		expect(within(l).getByRole('link', { name: 'Study Club' }).getAttribute('href')).toBe('/groups/-42');
	});

	it('says a background was set for the actor alone', () => {
		expect(words(line('chat_set_background', 'changed the chat background', { kind: 'background', only_for_self: true }))).toBe('Sample Sender changed the chat background for themselves');
		document.body.innerHTML = '';
		expect(words(line('chat_set_background', 'changed the chat background', { kind: 'background', only_for_self: true }, { sender: null, sender_chat: undefined }))).toBe('You changed the chat background for yourself');
		document.body.innerHTML = '';
		expect(words(line('chat_set_background', 'changed the chat background', { kind: 'background', only_for_self: false }))).toBe('Sample Sender changed the chat background');
	});

	it('stays as it was without a detail', () => {
		expect(words(line('pin_message', 'pinned a message', null))).toBe('Sample Sender pinned a message');
		document.body.innerHTML = '';
		const pin = line('pin_message', 'pinned a message', undefined);
		expect(words(pin)).toBe('Sample Sender pinned a message');
		expect(pin.querySelector('q')).toBeNull();
		document.body.innerHTML = '';
		const c = line('chat_added_to_community', 'added the group to a community', undefined);
		expect(words(c)).toBe('Sample Sender added the group to a community');
		expect(c.querySelector('a[href^="/groups/"]')).toBeNull();
	});
});

// The stand-in's forum service messages with their detail.
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const forumSample = (type: string, kind?: string) => (chatStream(FORUM_GROUP) as Raw[]).find((m) => {
	const s = sampleOf(m)?.service?.(ALL);
	return s?.type === type && (kind === undefined || (s.detail as { kind?: string } | null)?.kind === kind);
})!;
const open = (raw: Raw, persona: 'superadmin' | 'member' = 'superadmin') => {
	fakeApi(persona);
	renderPage(`/messages/group/${raw.id}`, persona);
	return fixture<MessageProps>(`/messages/group/${raw.id}`, persona).props.message;
};
const text = () => document.querySelector('.a-msgtext')!;

describe('a service message\'s page', () => {
	it('shows the new photo, which opens the viewer', () => {
		open(forumSample('chat_change_photo'));
		fireEvent.click(within(text() as HTMLElement).getByRole('link', { name: 'The new photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/standin-986010');
	});

	it('quotes the pinned message, and leaves a pin logged without it as it was', () => {
		const m = open(forumSample('pin_message', 'pin'));
		expect(text().querySelector('q')!.textContent).toBe((m.service?.detail as { snippet: string }).snippet);
		expect(screen.getByRole('link', { name: 'The message this is about' })).toBeTruthy();
		document.body.innerHTML = '';
		const old = open((chatStream(FORUM_GROUP) as Raw[]).find((x) => { const s = sampleOf(x)?.service?.(ALL); return s?.type === 'pin_message' && s.detail == null; })!);
		expect('detail' in old.service!).toBe(false);
		expect(document.querySelector('q')).toBeNull();
	});

	it('says how long the video chat ran', () => {
		open(forumSample('video_chat_ended'));
		expect(text().textContent).toContain('after 1 h 2 min');
	});

	it('links its topic', () => {
		open(forumSample('forum_topic_created'));
		expect(within(text() as HTMLElement).getByRole('link', { name: TOPICS[1]!.name }).getAttribute('href')).toBe(`${chatUrl}?topic=${TOPICS[1]!.id}`);
	});

	it('links the community and the basic group the supergroup came from', () => {
		open(forumSample('chat_added_to_community'));
		expect(screen.getByRole('link', { name: PUBLIC_POLLS_GROUP.title }).closest('.a-msgtext')).toBeTruthy();
		document.body.innerHTML = '';
		open(forumSample('chat_upgrade_from'));
		expect(within(text() as HTMLElement).getByRole('link', { name: 'Linux Study Group' }).getAttribute('href')).toBe(`/groups/${KINDS_GROUP.id}`);
	});
});
