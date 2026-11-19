// @vitest-environment happy-dom
// The kinds' smaller gaps (docs/todo/msg-kind-leftovers.md): what a card
// says when its detail is thin or missing, and the fields each card
// shows.
import { beforeEach, describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
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
