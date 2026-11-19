// @vitest-environment happy-dom
// A message with no sender is the logging account's: "You" to a
// signed-in viewer, its own name to a signed-out visitor (who is not
// that account), and "The archive account" when the name is unknown.
import { beforeEach, describe, expect, it } from 'vitest';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { pageData } from '../../standin/fixtures.ts';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessageProps } from './message.tsx';
import type { MessagesProps } from './messages.tsx';
import type { ChatMessage } from '../types.ts';
import type { ViewerWithLogger } from '../ui/logger.ts';

beforeEach(setup);

const LOGGER = { id: 5100000009, name: 'Sample Logger', username: null, photo: null };
const signedIn = (): ViewerWithLogger => ({ ...fixture('/').viewer, logger: LOGGER });
const signedOut = (logger?: ViewerWithLogger['logger']): ViewerWithLogger => ({ signed_in: false, now: fixture('/').viewer.now, ...(logger !== undefined && { logger }) });

describe('the chat\'s service line without a sender', () => {
	const chatUrl = () => `/groups/${fixture<GroupsProps>('/groups').props.rows[0]!.id}/chat`;
	function line(viewer: ViewerWithLogger) {
		const p = fixture<ChatProps>(chatUrl()).props;
		const t = p.messages.find((m) => m.content_type === 'text')!;
		const m = { ...t, id: 1e9, message_id: 9e5, content_type: 'service', text: 'pinned a message', service_type: 'pin_message', service: null, sender: null, sender_chat: undefined, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [] } as ChatMessage;
		fakeApi('superadmin');
		renderPage(chatUrl(), 'superadmin', { props: { ...p, messages: [m], at: null }, viewer });
		const el = document.getElementById('m900000')!.firstElementChild!;
		return [...el.childNodes].filter((n) => !(n instanceof Element && n.matches('.c-svctime, .c-deltag'))).map((n) => n.textContent).join('').trim();
	}

	it('says "You" to a signed-in viewer', () => expect(line(signedIn())).toBe('You pinned a message'));
	it('names the logging account to a signed-out visitor', () => expect(line(signedOut(LOGGER))).toBe('Sample Logger pinned a message'));
	it('falls back without the account\'s name', () => {
		expect(line(signedOut(null))).toBe('The archive account pinned a message');
	});
	it('falls back when the server leaves the account out', () => {
		expect(line(signedOut())).toBe('The archive account pinned a message');
	});
});

describe('the message page without a sender', () => {
	const url = () => `/messages/group/${fixture<MessagesProps>('/group_messages').props.rows[0]!.id}`;
	function sender(viewer: ViewerWithLogger) {
		const p = fixture<MessageProps>(url()).props;
		renderPage(url(), 'superadmin', { props: { message: { ...p.message, sender: null, sender_chat: undefined } }, viewer });
		const head = document.querySelector('.a-msgcard-head')!;
		// The name, and the avatar's initials.
		return [head.querySelector('b')!.textContent, head.firstElementChild!.textContent];
	}

	it('says "You" to a signed-in viewer', () => expect(sender(signedIn())[0]).toBe('You (logging account)'));
	it('names the logging account to a signed-out visitor', () => {
		expect(sender(signedOut(LOGGER))).toEqual(['Sample Logger', 'SL']);
	});
	it('falls back without the account\'s name', () => expect(sender(signedOut(null))[0]).toBe('The archive account'));
});

describe('the stand-in', () => {
	it('sends the logging account to every viewer', () => {
		for (const persona of ['public', 'superadmin', 'member'] as const) expect((pageData('/', persona).viewer as ViewerWithLogger).logger?.name).toBeTruthy();
	});
});
