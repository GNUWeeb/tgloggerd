// @vitest-environment happy-dom
// The message page shows what the chat does: a message's link preview.
import { beforeEach, describe, expect, it } from 'vitest';
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';

beforeEach(setup);
const page = (id: number) => `/messages/group/${id}`;

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
});
