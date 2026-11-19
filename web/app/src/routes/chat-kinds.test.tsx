// @vitest-environment happy-dom
// Each message kind in the chat, from the stand-in's samples
// (standin/kinds.ts): what the bubble shows of it, and what a member
// does not see of the people it names.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP } from '../../standin/kinds.ts';
import { money } from './chat-kinds.tsx';
import type { ChatProps } from './chat.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);

const url = `/groups/${KINDS_GROUP.id}/chat`;
const page = (persona: 'superadmin' | 'member' = 'superadmin') => fixture<ChatProps>(url, persona);
const show = (persona: 'superadmin' | 'member' = 'superadmin') => {
	fakeApi(persona);
	renderPage(url, persona);
	return page(persona).props.messages;
};
const bubbleOf = (m: ChatMessage) => document.getElementById(`m${m.message_id}`)!;
const find = (msgs: ChatMessage[], pick: (m: ChatMessage) => boolean) => msgs.find(pick)!;
const detail = <K extends string>(msgs: ChatMessage[], kind: K) => find(msgs, (m) => m.detail?.kind === kind);

describe('message kinds in the chat', () => {
	it('shows a poll with its results, a quiz\'s answer, and how many options are left out', () => {
		const msgs = show();
		const poll = detail(msgs, 'poll');
		const b = within(bubbleOf(poll));
		expect(b.getByText('Which editor do you use?')).toBeTruthy();
		expect(b.getByText('Anonymous poll')).toBeTruthy();
		expect(b.getByText('Neovim')).toBeTruthy();
		const quiz = find(msgs, (m) => m.detail?.kind === 'poll' && m.detail.type === 'quiz');
		expect(within(bubbleOf(quiz)).getByLabelText('correct answer').closest('li')!.textContent).toContain('Linus Torvalds');
		const long = find(msgs, (m) => m.detail?.kind === 'poll' && m.detail.options_total > m.detail.options.length);
		expect(bubbleOf(long).querySelectorAll('.k-options li')).toHaveLength(20);
		expect(within(bubbleOf(long)).getByText(/5 more options not shown/)).toBeTruthy();
	});

	it('shows the reactions, the counts and the pin a message was last seen with', () => {
		const msgs = show();
		const poll = detail(msgs, 'poll');
		const b = bubbleOf(poll);
		const chips = [...b.querySelectorAll('.k-reaction')].map((c) => c.textContent);
		expect(chips).toEqual(['👍12', '🔥5', '◆3', '2']);
		expect(b.querySelector('[title="1,534 views"]')!.textContent).toBe('1.5k');
		expect(b.querySelector('[title="Pinned"]')).toBeTruthy();
	});

	it('shows a checklist with its title, who did what, and a contact with the phone', () => {
		const msgs = show();
		const list = detail(msgs, 'checklist');
		const b = within(bubbleOf(list));
		expect(b.getByText('Release checklist')).toBeTruthy();
		// The title is the card's, not repeated as the message's text.
		expect(bubbleOf(list).querySelector('.c-text')).toBeNull();
		expect(bubbleOf(list).querySelectorAll('.k-checklist li.done')).toHaveLength(2);
		const contact = detail(msgs, 'contact');
		expect(within(bubbleOf(contact)).getByText('+62 812 0000 0101')).toBeTruthy();
	});

	it('names calls, places and the kinds it cannot show', () => {
		const msgs = show();
		expect(within(bubbleOf(detail(msgs, 'call'))).getByText('Video chat . live now')).toBeTruthy();
		expect(within(bubbleOf(find(msgs, (m) => m.detail?.kind === 'location' && !!m.detail.live))).getByText(/17 earlier ones kept/)).toBeTruthy();
		const venue = bubbleOf(detail(msgs, 'venue'));
		expect(venue.querySelector('a[href^="https://www.openstreetmap.org/"]')!.getAttribute('rel')).toContain('noopener');
		for (const text of ['A widget, which this archive does not show', 'Self-destructing media that has expired', 'A message Telegram itself could not read']) expect(screen.getByText(text)).toBeTruthy();
		expect(screen.getByText(/messageFancyNewThing/)).toBeTruthy();
		expect(within(bubbleOf(detail(msgs, 'invoice'))).getByText(/IDR\s?150,000\.00/)).toBeTruthy();
		expect(bubbleOf(find(msgs, (m) => m.content_type === 'video_note')).querySelector('.c-videowrap.round')).toBeTruthy();
	});

	it('shows a service message\'s text as the server wrote it, and links a pin to its message', () => {
		const msgs = show();
		const add = find(msgs, (m) => m.service?.type === 'chat_add_members' && m.text.startsWith('added'));
		expect(bubbleOf(add).textContent).toContain(add.text);
		const pin = find(msgs, (m) => m.service?.type === 'pin_message' && m.service.refs.message_id !== null);
		const a = bubbleOf(pin).querySelector('a[href^="?at="]')!;
		expect(a.getAttribute('href')).toBe(`?at=${pin.service!.refs.message_id}`);
		expect(a.textContent).toBe('pinned a message');
	});

	it('opens a paid photo in the viewer, plays a paid video, keeps an unpaid item locked and names an unsupported one', () => {
		const msgs = show();
		const paid = bubbleOf(detail(msgs, 'paid_media'));
		const b = within(paid);
		expect(b.getByText('Behind the scenes')).toBeTruthy();
		fireEvent.click(b.getByRole('link', { name: 'Photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/standin-985001');
		fireEvent.keyDown(window, { key: 'Escape' });
		fireEvent.click(b.getByRole('link', { name: 'Video' }));
		const video = screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('video')!;
		expect(video.getAttribute('src')).toBe('/files/standin-985002');
		expect(video.hasAttribute('controls')).toBe(true);
		fireEvent.keyDown(window, { key: 'Escape' });
		const locked = b.getByTitle('Not paid for: only a blurred preview is known');
		expect(within(locked).getByText('Not paid for')).toBeTruthy();
		expect(locked.className).toContain('c-blurred');
		expect(locked.closest('a')).toBeNull();
		expect(within(b.getByTitle('Unsupported item: TDLib could not read it')).getByText('Unsupported item')).toBeTruthy();
	});
});

describe('what a member sees of whom a message names', () => {
	it('gets no phone, and the hidden people only as a count', () => {
		const full = page('superadmin').props.messages;
		const msgs = show('member');
		const contact = detail(msgs, 'contact');
		expect(contact.detail).toMatchObject({ phone: null, vcard: null });
		expect(within(bubbleOf(contact)).queryByText('+62 812 0000 0101')).toBeNull();
		// The call names the participants a member may see, and says how
		// many others there were.
		const call = detail(msgs, 'call');
		const callFull = detail(full, 'call');
		if (call.detail?.kind !== 'call' || callFull.detail?.kind !== 'call') throw new Error('no call sample');
		expect(call.detail.participants_total).toBe(callFull.detail.participants_total);
		expect(callFull.detail.participants).toHaveLength(3);
		expect(call.detail.participants).toHaveLength(2);
		expect(within(bubbleOf(call)).getByText('and 1 more')).toBeTruthy();
		// The service text names only the members it may see.
		const addFull = find(full, (m) => m.service?.type === 'chat_add_members' && m.service.refs.users.length === 3);
		const add = find(msgs, (m) => m.id === addFull.id);
		expect(addFull.text).toMatch(/^added .+, .+ and .+ to the group$/);
		expect(add.text).toMatch(/^added .+ and 2 other members to the group$/);
		expect(add.service!.refs.users).toHaveLength(1);
	});
});

describe('money', () => {
	it('reads an amount in its currency\'s smallest unit', () => {
		expect(money(15000000, 'IDR')).toMatch(/IDR\s?150,000\.00/);
		expect(money(1500, 'JPY')).toMatch(/¥1,500/);
		expect(money(1234, 'KWD')).toMatch(/1\.234/);
		expect(money(50, 'XTR')).toBe('50 Stars');
	});
});
