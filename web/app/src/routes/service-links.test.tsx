// @vitest-environment happy-dom
// The people a service line names link to their pages, in the chat and
// on the message page, from the parts the API spells the text in; one
// the viewer may not see is only counted, with no name and no link.
import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import { NavProvider } from '../nav.tsx';
import { ServiceText } from '../ui/serviceparts.tsx';
import type { S } from '../api/schema.ts';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';

beforeEach(setup);

type Persona = 'superadmin' | 'member';
const chatUrl = `/groups/${KINDS_GROUP.id}/chat`;
const msgs = (persona: Persona, url = chatUrl) => fixture<ChatProps>(url, persona).props.messages;
// The sample service messages, as a superadmin (who sees everyone) and
// a member (who never sees the strangers) get them.
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const sample = (type: string, users: number) => (chatStream(KINDS_GROUP) as Raw[]).filter((m) => sampleOf(m)?.content_type === 'service')
	.map((m) => fixture<MessageProps>(`/messages/group/${m.id}`, 'superadmin').props.message)
	.find((m) => m.service?.type === type && m.service.refs.users.length === users)!;
const lineOf = (m: { message_id: number }) => document.getElementById(`m${m.message_id}`)!;
const userLinks = (el: Element) => [...el.querySelectorAll('a[href^="/users/"]')].map((a) => [a.getAttribute('href'), a.textContent]);
const showChat = (persona: Persona, url = chatUrl) => {
	fakeApi(persona);
	renderPage(url, persona);
};
const openMessage = (id: number, persona: Persona) => {
	fakeApi(persona);
	renderPage(`/messages/group/${id}`, persona);
	return fixture<MessageProps>(`/messages/group/${id}`, persona).props.message;
};

describe('a service line in the chat', () => {
	it('links each member added whom the viewer may see, and counts the others', () => {
		const full = sample('chat_add_members', 3);
		const [b, x, y] = full.service!.refs.users;
		showChat('member');
		const m = msgs('member').find((n) => n.id === full.id)!;
		expect(m.text).toBe(`added ${b!.title} and 2 other members to the group`);
		const line = lineOf(m);
		expect(line.textContent).toContain(m.text);
		expect(within(line).getByRole('link', { name: b!.title }).getAttribute('href')).toBe(`/users/${b!.id}`);
		expect(within(line).queryByRole('link', { name: '2 other members' })).toBeNull();
		expect(within(line).queryByText(/2 other members/)!.closest('a')).toBeNull();
		for (const hidden of [x!, y!]) {
			expect(line.textContent).not.toContain(hidden.title);
			expect(line.querySelector(`a[href="/users/${hidden.id}"]`)).toBeNull();
		}
	});

	it('links all of them for a viewer who sees everyone', () => {
		const full = sample('chat_add_members', 3);
		showChat('superadmin');
		const line = lineOf(full);
		for (const u of full.service!.refs.users) expect(within(line).getByRole('link', { name: u.title }).getAttribute('href')).toBe(`/users/${u.id}`);
	});

	it('shows only the count for a member added whom the viewer may not see', () => {
		const full = sample('chat_add_members', 1);
		const hidden = full.service!.refs.users[0]!;
		showChat('member');
		const m = msgs('member').find((n) => n.id === full.id)!;
		expect(m.text).toBe('added a member to the group');
		const line = lineOf(m);
		expect(line.textContent).toContain('added a member to the group');
		expect(line.textContent).not.toContain(hidden.title);
		// The only user link is the one who added them.
		expect(userLinks(line).map(([href]) => href)).toEqual(m.sender ? [`/users/${m.sender.id}`] : []);
	});

	it('links the member removed and the participants invited', () => {
		for (const type of ['chat_delete_member', 'invite_video_chat_participants']) {
			const m = sample(type, type === 'chat_delete_member' ? 1 : 2);
			const url = `${chatUrl}?at=${m.message_id}`;
			showChat('superadmin', url);
			const line = lineOf(m);
			for (const u of m.service!.refs.users) expect(within(line).getByRole('link', { name: u.title }).getAttribute('href')).toBe(`/users/${u.id}`);
			expect(line.textContent).toContain(m.text);
			document.body.innerHTML = '';
		}
	});
});

describe('a service message\'s page', () => {
	it('links the member a viewer may see, and names no one it may not', () => {
		const full = sample('chat_add_members', 3);
		const [b, x, y] = full.service!.refs.users;
		const m = openMessage(full.id, 'member');
		const text = document.querySelector('.a-msgtext')!;
		expect(text.textContent).toBe(m.text);
		expect(userLinks(text)).toEqual([[`/users/${b!.id}`, b!.title]]);
		for (const hidden of [x!, y!]) expect(document.body.textContent).not.toContain(hidden.title);
	});

	it('shows only the count when it names no one the viewer may see', () => {
		const full = sample('chat_add_members', 1);
		const m = openMessage(full.id, 'member');
		const text = document.querySelector('.a-msgtext')!;
		expect(text.textContent).toBe('added a member to the group');
		expect(text.querySelector('a')).toBeNull();
		expect(document.body.textContent).not.toContain(full.service!.refs.users[0]!.title);
		expect(m.service!.refs.users).toEqual([]);
	});

	it('links the participants invited', () => {
		const full = sample('invite_video_chat_participants', 2);
		openMessage(full.id, 'superadmin');
		const text = document.querySelector('.a-msgtext')!;
		expect(text.textContent).toBe(full.text);
		expect(userLinks(text)).toEqual(full.service!.refs.users.map((u) => [`/users/${u.id}`, u.title]));
	});
});

describe('the service text from its parts', () => {
	const ref = (id: number, title: string, kind: 'user' | 'group' = 'user') => ({ id, title, kind, photo: null });
	const draw = (service: S['ServiceInfo'], text: string, extra: Partial<Parameters<typeof ServiceText>[0]> = {}) => render(<NavProvider initial={fixture('/')}><p data-testid="t"><ServiceText service={service} text={text} {...extra} /></p></NavProvider>);

	it('links two people of the same name each to their own page', () => {
		const service: S['ServiceInfo'] = { type: 'chat_add_members', refs: { users: [ref(11, 'Budi'), ref(12, 'Budi')], chats: [], message_id: null },
			parts: [{ text: 'added ' }, { text: 'Budi', user: 0 }, { text: ' and ' }, { text: 'Budi', user: 1 }, { text: ' to the group' }] };
		draw(service, 'added Budi and Budi to the group');
		expect(userLinks(screen.getByTestId('t'))).toEqual([['/users/11', 'Budi'], ['/users/12', 'Budi']]);
	});

	it('links a chat to its group page', () => {
		const service: S['ServiceInfo'] = { type: 'chat_shared', refs: { users: [], chats: [ref(-100, 'Kernel Hackers', 'group')], message_id: null },
			parts: [{ text: 'shared ' }, { text: 'Kernel Hackers', chat: 0 }, { text: ' with the bot' }] };
		draw(service, 'shared Kernel Hackers with the bot');
		expect(screen.getByRole('link', { name: 'Kernel Hackers' }).getAttribute('href')).toBe('/groups/-100');
	});

	it('shows the text as it is without parts, or with parts that do not spell it', () => {
		const refs = { users: [ref(11, 'Budi')], chats: [], message_id: null };
		draw({ type: 'chat_add_members', refs }, 'added Budi to the group');
		expect(screen.getByTestId('t').textContent).toBe('added Budi to the group');
		expect(screen.queryByRole('link')).toBeNull();
		document.body.innerHTML = '';
		draw({ type: 'chat_add_members', refs, parts: [{ text: 'added ' }, { text: 'Budi', user: 0 }] }, 'added Budi to the group');
		expect(screen.getByTestId('t').textContent).toBe('added Budi to the group');
		expect(screen.queryByRole('link')).toBeNull();
	});

	it('starts a sentence with a capital and wraps the text around the names', () => {
		const service: S['ServiceInfo'] = { type: 'giveaway_winners', refs: { users: [ref(11, 'budi')], chats: [], message_id: 7 },
			parts: [{ text: 'budi', user: 0 }, { text: ' won' }] };
		draw(service, 'budi won', { capital: true, plain: (s) => <a href="?at=7">{s}</a> });
		const t = screen.getByTestId('t');
		expect(t.textContent).toBe('Budi won');
		expect(screen.getByRole('link', { name: 'Budi' }).getAttribute('href')).toBe('/users/11');
		expect(screen.getByRole('link', { name: 'won' }).getAttribute('href')).toBe('?at=7');
	});
});
