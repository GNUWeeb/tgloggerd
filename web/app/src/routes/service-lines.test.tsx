// @vitest-environment happy-dom
// The chat's service lines read as sentences: the logging account's own
// actions as "You ...", and a notice no one did without an actor.
import { beforeEach, describe, expect, it } from 'vitest';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
const chatUrl = () => `/groups/${fixture<GroupsProps>('/groups').props.rows[0]!.id}/chat`;

// The chat with one service message, built on a plain text one.
function chatWith(o: Partial<ChatMessage>) {
	const p = fixture<ChatProps>(chatUrl()).props;
	const t = p.messages.find((m) => m.content_type === 'text')!;
	const m = { ...t, id: 1e9, message_id: 9e5, content_type: 'service', forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], service: undefined, ...o } as ChatMessage;
	fakeApi('superadmin');
	renderPage(chatUrl(), 'superadmin', { props: { ...p, messages: [m], at: null } });
	return document.getElementById('m900000')!.firstElementChild!;
}
// The line's words, without its time and deleted tag.
const words = (el: Element) => [...el.childNodes].filter((n) => !(n instanceof Element && n.matches('.c-svctime, .c-deltag'))).map((n) => n.textContent).join('').trim();

describe('the chat\'s service lines', () => {
	it('name the logging account "You"', () => {
		const line = chatWith({ text: 'pinned a message', service_type: 'pin_message', sender: null, sender_chat: undefined });
		expect(words(line)).toBe('You pinned a message');
	});

	it('keep a sender as the actor', () => {
		const line = chatWith({ text: 'pinned a message', service_type: 'pin_message', sender: { id: 5, name: 'Sample Sender' } as ChatMessage['sender'] });
		expect(words(line)).toBe('Sample Sender pinned a message');
	});

	it('give a sentence no actor and a capital', () => {
		const line = chatWith({ text: 'the supergroup was created from a group', service_type: 'chat_upgrade_from', sender: { id: 5, name: 'Sample Sender' } as ChatMessage['sender'] });
		expect(words(line)).toBe('The supergroup was created from a group');
		expect(line.querySelector('a[href^="/users/"]')).toBeNull();
	});

	it('read an expired gift offer as a sentence', () => {
		const line = chatWith({ text: 'the offer to buy a gift expired', service_type: 'upgraded_gift_purchase_offer_rejected', sender: null, sender_chat: undefined });
		expect(words(line)).toBe('The offer to buy a gift expired');
	});
});
