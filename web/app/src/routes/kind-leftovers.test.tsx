// @vitest-environment happy-dom
// The kinds' smaller gaps (docs/todo/msg-kind-leftovers.md): what a card
// says when its detail is thin or missing, and the fields each card
// shows.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MessageMedia, kindOf, type Media } from './chat-media.tsx';
import { PLAIN_KINDS } from '../test/kinds.ts';
import { TypeIcon } from '../ui/media.tsx';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { KINDS_GROUP } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);

// The stand-in's kind samples (standin/kinds.ts), in the chat and on
// their own pages.
type Persona = 'superadmin' | 'member';
const chatUrl = `/groups/${KINDS_GROUP.id}/chat`;
const showChat = (persona: Persona = 'superadmin') => {
	fakeApi(persona);
	renderPage(chatUrl, persona);
	return fixture<ChatProps>(chatUrl, persona).props.messages;
};
const bubble = (m: ChatMessage) => within(document.getElementById(`m${m.message_id}`)!);
const sample = (msgs: ChatMessage[], pick: (m: ChatMessage) => boolean) => {
	const m = msgs.find(pick);
	if (!m) throw new Error('no such sample');
	return m;
};
const showPage = (m: ChatMessage, persona: Persona = 'superadmin') => {
	const url = `/messages/${m.scope}/${m.id}`;
	fakeApi(persona);
	renderPage(url, persona);
	return fixture<MessageProps>(url, persona).props.message;
};
// Each test renders one page: the chat's samples are looked up from the
// fixture without rendering.
const samples = (persona: Persona = 'superadmin') => fixture<ChatProps>(chatUrl, persona).props.messages;

const show = (el: ReactElement) => render(<div className="app">{el}</div>);
const msg = (content_type: ChatMessage['content_type'], m: Media | null): ChatMessage => ({
	id: 1, message_id: 1, date: '2026-11-17T10:00:00Z', scope: 'group', sender: null, content_type, text: '',
	media: m, album: [], reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null, ...PLAIN_KINDS,
});

describe('a video note', () => {
	it('draws as a round video even when its media has no render', () => {
		const m: Media = { stored: true, thumb: null, url: '/files/tok-note', duration: 9 };
		expect(kindOf('video_note', m)).toBe('video');
		show(<MessageMedia msg={msg('video_note', m)} onClick={false} />);
		const note = screen.getByRole('link', { name: 'Video' });
		expect(note.className).toContain('round');
		expect(note.getAttribute('style')).toContain('aspect-ratio: 1 / 1');
	});
});

describe('the icons', () => {
	it('give an unknown kind its own icon, not the document\'s', () => {
		const cls = (el: ReactElement) => { const r = show(el); const c = r.container.querySelector('svg')!.getAttribute('class'); r.unmount(); return c; };
		const unknown = cls(<TypeIcon type="unknown" />);
		expect(unknown).not.toBe(cls(<TypeIcon type="document" />));
		expect(unknown).not.toBe(cls(<TypeIcon type="unsupported" />));
	});
});

describe('an expired message', () => {
	it('says what it was from its TDLib class in the chat', () => {
		const msgs = showChat();
		const note = sample(msgs, (m) => m.content_type === 'expired' && m.tdlib_type === 'messageExpiredVideoNote');
		expect(bubble(note).getByText('Expired video message')).toBeTruthy();
		// Without a class it keeps the general wording.
		const plain = sample(msgs, (m) => m.content_type === 'expired' && !m.tdlib_type);
		expect(bubble(plain).getByText('Self-destructing media that has expired')).toBeTruthy();
	});

	it('says so on its own page', () => {
		const note = sample(samples(), (m) => m.content_type === 'expired' && m.tdlib_type === 'messageExpiredVideoNote');
		showPage(note);
		expect(within(document.querySelector('.a-msgkind') as HTMLElement).getByText('Expired video message')).toBeTruthy();
	});
});

describe('an unknown kind', () => {
	it('says on its page what the chat says, naming its TDLib class', () => {
		const m = sample(samples(), (x) => x.content_type === 'unknown' && !!x.tdlib_type);
		showPage(m);
		expect(within(document.querySelector('.a-msgkind') as HTMLElement).getByText('A kind of message this view does not show yet (messageFancyNewThing)')).toBeTruthy();
		expect(screen.queryByText('(no text)')).toBeNull();
	});
});

describe('a kind whose detail is missing', () => {
	it('says what was not recorded of it in the chat', () => {
		const msgs = showChat();
		const poll = sample(msgs, (m) => m.content_type === 'poll' && m.detail === null);
		const line = bubble(poll).getByText('Poll (its options were not recorded)');
		expect(line.querySelector('svg')).toBeTruthy();
		const place = sample(msgs, (m) => m.content_type === 'location' && m.detail === null);
		expect(bubble(place).getByText('Location (its place was not recorded)')).toBeTruthy();
	});

	it('says so on its own page instead of "(no text)"', () => {
		showPage(sample(samples(), (m) => m.content_type === 'poll' && m.detail === null));
		expect(within(document.querySelector('.a-msgkind') as HTMLElement).getByText('Poll (its options were not recorded)')).toBeTruthy();
		expect(screen.queryByText('(no text)')).toBeNull();
	});
});

describe('a game and an invoice', () => {
	it('show a game\'s short name and play its animation cover', () => {
		const game = sample(showChat(), (m) => m.detail?.kind === 'game');
		if (game.detail?.kind !== 'game') throw new Error('no game sample');
		const b = bubble(game);
		expect(b.getByText('lumberjack')).toBeTruthy();
		const video = b.getByRole('link', { name: 'Animation' }).querySelector('video')!;
		expect(video.getAttribute('src')).toBe(game.detail.photo!.url);
		expect(video.loop).toBe(true);
		expect(video.muted).toBe(true);
	});

	it('open an invoice\'s photo in the viewer', () => {
		const inv = sample(showChat(), (m) => m.detail?.kind === 'invoice');
		if (inv.detail?.kind !== 'invoice') throw new Error('no invoice sample');
		fireEvent.click(bubble(inv).getByRole('link', { name: 'Photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe(inv.detail.photo!.url);
	});
});

describe('a contact', () => {
	it('shows the user\'s photo as its avatar and links the user', () => {
		const c = sample(showChat(), (m) => m.detail?.kind === 'contact');
		if (c.detail?.kind !== 'contact' || !c.detail.user?.photo) throw new Error('no contact sample with a photo');
		const card = document.getElementById(`m${c.message_id}`)!.querySelector('.k-contact')!;
		expect(card.querySelector('img')!.getAttribute('src')).toBe(c.detail.user.photo);
		expect(card.querySelector('a')!.getAttribute('href')).toBe(`/users/${c.detail.user.id}`);
	});
});
