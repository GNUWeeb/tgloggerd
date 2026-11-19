// @vitest-environment happy-dom
// The message page shows what the chat does: a message's link preview,
// and its forum topic by name and colour, linked to the topic's view.
import { beforeEach, describe, expect, it } from 'vitest';
import { screen } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import { FORUM_GROUP, TOPICS } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessageProps } from './message.tsx';

beforeEach(setup);
const page = (id: number) => `/messages/group/${id}`;
const details = () => screen.getByRole('heading', { level: 2, name: 'Details' }).closest('.a-card') as HTMLElement;

// The first message in a group's chat that passes a test.
function chatMessage(groupId: number, ok: (m: ChatProps['messages'][number]) => boolean) {
	return fixture<ChatProps>(`/groups/${groupId}/chat`).props.messages.find(ok);
}

describe('message page parity with the chat', () => {
	it('draws the link preview of a message that has one', () => {
		const groups = fixture<GroupsProps>('/groups').props.rows;
		const m = groups.map((g) => chatMessage(g.id, (x) => x.link_preview !== null)).find((x) => x)!;
		expect(m).toBeTruthy();
		renderPage(page(m.id));
		const a = document.querySelector('a.c-lp')!;
		expect(a.getAttribute('href')).toBe(m.link_preview!.url);
		expect(a.textContent).toContain(m.link_preview!.title);
	});

	it('names a forum message\'s topic, with its colour, linked to the topic', () => {
		const m = chatMessage(FORUM_GROUP.id, (x) => x.content_type !== 'service' && x.topic_id !== null)!;
		const t = TOPICS.find((x) => x.id === m.topic_id)!;
		renderPage(page(m.id));
		const link = screen.getByRole('link', { name: t.name });
		expect(details().contains(link)).toBe(true);
		expect(link.getAttribute('href')).toBe(`/groups/${FORUM_GROUP.id}/chat?topic=${t.id}`);
		expect((link.querySelector('.k-topicdot') as HTMLElement).style.background).toBeTruthy();
		expect(screen.queryByText(`topic ${t.id}`)).toBeNull();
	});

	it('falls back to the topic\'s number when the topic is not known', () => {
		const m = chatMessage(FORUM_GROUP.id, (x) => x.content_type !== 'service' && x.topic_id !== null)!;
		const p = fixture<MessageProps>(page(m.id)).props;
		renderPage(page(m.id), 'superadmin', { props: { message: { ...p.message, topic: null } as MessageProps['message'] } });
		expect(screen.getByRole('link', { name: `topic ${m.topic_id}` }).getAttribute('href')).toBe(`/groups/${FORUM_GROUP.id}/chat?topic=${m.topic_id}`);
	});
});
