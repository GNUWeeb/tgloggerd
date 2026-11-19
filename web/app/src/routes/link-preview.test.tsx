// @vitest-environment happy-dom
// A link preview's card in the chat and on the message page: its type,
// author and length, its own video, animation or audio played with the
// chat's components, an embedded player's link, and a Telegram message
// link opening the archived message.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';

type LP = NonNullable<ChatMessage['link_preview']>;

beforeEach(setup);

// Every link preview in the first window of each sample group's chat.
const samples = (persona: 'superadmin' | 'member' = 'superadmin') => fixture<GroupsProps>('/groups', persona).props.rows
	.flatMap((g) => fixture<ChatProps>(`/groups/${g.id}/chat`, persona).props.messages)
	.map((m) => m.link_preview).filter((x): x is LP => !!x);

describe('the stand-in\'s link previews', () => {
	it('has a sample of each kind the card draws', () => {
		const all = samples();
		const of = (t: string) => all.filter((x) => x.type === t);
		expect(of('article').some((x) => x.media?.w && x.media.h)).toBe(true);
		expect(of('video').some((x) => x.media?.render === 'video' && x.media.poster && x.duration)).toBe(true);
		expect(of('audio').some((x) => x.media?.render === 'audio' && x.author)).toBe(true);
		expect(of('embedded_video_player').some((x) => x.embed?.url)).toBe(true);
		expect(of('message').some((x) => x.target)).toBe(true);
		expect(of('unsupported').length).toBeGreaterThan(0);
		// A preview's own medium never carries a minithumb.
		expect(all.every((x) => !x.media?.minithumb)).toBe(true);
	});
});

const groupId = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const chatUrl = () => `/groups/${groupId()}/chat`;
const PIC = '/files/lp-photo';

// The chat with one text message carrying this preview, as `chat`
// (the viewer's chat preferences) says to show media.
function chatWith(lp: LP, chat?: { media: 'click' }) {
	const p = fixture<ChatProps>(chatUrl()).props;
	const t = p.messages.find((m) => m.content_type === 'text')!;
	const messages = [{ ...t, id: 1e9, message_id: 9e5, forward: null, reply_to: null, deleted_at: null, edits: [], link_preview: lp } as ChatMessage];
	const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: false, has_newer: false }) });
	const page = fixture<ChatProps>(chatUrl()).viewer;
	renderPage(chatUrl(), 'superadmin', { props: { ...p, messages, at: null }, ...(chat ? { viewer: { ...page, chat: { ...page.chat!, ...chat } } } : {}) });
	return { calls, card: document.querySelector('#m900000 .c-lp') as HTMLElement };
}

const video = (o: Partial<LP> = {}): LP => ({
	type: 'video', site: 'Example Video', title: 'A talk', desc: 'd', url: 'https://video.example.org/w/1', author: 'Some Channel', duration: 312,
	media: { stored: true, url: '/files/lp-video', thumb: PIC, poster: PIC, render: 'video', ext: 'mp4', w: 1280, h: 720, duration: 312 }, ...o,
});

describe('a link preview card', () => {
	it('plays a video preview with the chat\'s video box, its picture as the poster', () => {
		const { card } = chatWith(video());
		const box = card.querySelector('a.c-videowrap')!;
		expect(box.getAttribute('aria-label')).toBe('Video');
		const v = box.querySelector('video')!;
		expect(v.getAttribute('poster')).toBe(PIC);
		// A poster: none of the video loads before a click.
		expect(v.getAttribute('preload')).toBe('none');
		expect(v.getAttribute('src')).toBeNull();
		// The picture is the poster, not drawn a second time.
		expect(card.querySelector('img.c-lp-img')).toBeNull();
		// The medium is beside the page's link, not inside it.
		expect(card.querySelector('a.c-lp-main')!.contains(box)).toBe(false);
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Video . Some Channel . 5:12');
		fireEvent.click(box);
		const viewer = screen.getByRole('dialog', { name: 'Media viewer' });
		expect(viewer.querySelector('video')!.getAttribute('src')).toBe('/files/lp-video');
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.queryByRole('dialog', { name: 'Media viewer' })).toBeNull();
	});

	it('loads no poster when media load on click', () => {
		const { card } = chatWith(video(), { media: 'click' });
		expect(card.querySelector('a.c-videowrap video')!.getAttribute('poster')).toBeNull();
	});

	it('plays an animation preview looping and muted', () => {
		const { card } = chatWith(video({ type: 'animation', duration: undefined, author: undefined, media: { stored: true, url: '/files/lp-gif', thumb: PIC, poster: PIC, render: 'animation', ext: 'mp4', w: 480, h: 270 } }));
		const v = card.querySelector('a.c-videowrap video') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe('/files/lp-gif');
		expect(v.loop && v.muted && v.autoplay).toBe(true);
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Animation');
	});

	it('plays an audio preview with the chat\'s audio player', () => {
		const { card } = chatWith({
			type: 'audio', site: 'Example Podcasts', title: 'Episode 1', desc: '', url: 'https://podcasts.example.org/1', author: 'Host', duration: 1820,
			media: { stored: true, url: '/files/lp-audio', thumb: PIC, render: 'audio', ext: 'mp3', name: 'ep1.mp3', size: 1000, duration: 1820 },
		});
		const a = card.querySelector('audio.c-audio')!;
		expect(a.getAttribute('src')).toBe('/files/lp-audio');
		expect(a.hasAttribute('controls')).toBe(true);
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Audio . Host . 30:20');
		expect(card.querySelector('img.c-lp-img')).toBeNull();
	});

	it('draws an article\'s photo at its own shape, whole, inside the link', () => {
		const { card } = chatWith({ type: 'article', site: 'GitHub', title: 'A commit', desc: 'd', url: 'https://github.com/x/y', author: 'Someone', media: { stored: true, url: PIC, thumb: PIC, render: 'image', w: 1200, h: 600 } });
		const img = card.querySelector('a.c-lp-main img.c-lp-img') as HTMLImageElement;
		expect(img.getAttribute('src')).toBe(PIC);
		expect(img.style.aspectRatio).toBe('1200 / 600');
		// An article says no type, only its author.
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Someone');
		expect(card.querySelector('video, audio')).toBeNull();
	});

	it('opens an embedded player\'s URL in a new tab and never runs it', () => {
		const { card } = chatWith({
			type: 'embedded_video_player', site: 'Example Tube', title: 'A talk', desc: '', url: 'https://tube.example.org/watch?v=1', duration: 65,
			embed: { url: 'https://tube.example.org/embed/1', w: 1280, h: 720 }, media: { stored: true, url: PIC, thumb: PIC, render: 'image', w: 1280, h: 720 },
		});
		const open = within(card).getByRole('link', { name: 'Open the player in a new tab' });
		expect(open.getAttribute('href')).toBe('https://tube.example.org/embed/1');
		expect(open.getAttribute('target')).toBe('_blank');
		expect(open.getAttribute('rel')).toContain('noopener');
		expect(card.querySelector('iframe, video, embed, object')).toBeNull();
		expect(card.querySelector('a.c-lp-main')!.getAttribute('href')).toBe('https://tube.example.org/watch?v=1');
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Video player . 1:05');
	});

	it('links an embedded player or a page only by a safe URL', () => {
		const { card } = chatWith({ type: 'embedded_video_player', site: 's', title: 't', desc: '', url: 'javascript:alert(1)', embed: { url: 'javascript:alert(2)' } });
		expect(document.querySelector('#m900000 [href^="javascript"]')).toBeNull();
		expect(card.querySelector('a')).toBeNull();
		expect(within(card).getByText('t')).toBeTruthy();
	});

	it('opens a Telegram message link at the archived message', async () => {
		const { calls, card } = chatWith({ type: 'message', site: 'Telegram', title: 'A group', desc: 'hi', url: 'https://t.me/agroup/77', target: { chat_id: -100777, message_id: 77 } });
		const a = card.querySelector('a.c-lp-main')!;
		expect(a.getAttribute('href')).toBe('/groups/-100777/chat?at=77');
		expect(a.getAttribute('target')).toBeNull();
		expect(card.querySelector('.c-lp-meta')!.textContent).toBe('Telegram message');
		fireEvent.click(a);
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/page/groups/-100777/chat').some((c) => c.query.get('at') === '77')).toBe(true));
	});

	it('links a Telegram message not in the archive to its URL', () => {
		const { card } = chatWith({ type: 'message', site: 'Telegram', title: 'A group', desc: 'hi', url: 'https://t.me/agroup/77' });
		const a = card.querySelector('a.c-lp-main')!;
		expect(a.getAttribute('href')).toBe('https://t.me/agroup/77');
		expect(a.getAttribute('target')).toBe('_blank');
	});

	it('draws a type it does not know as a plain card', () => {
		const { card } = chatWith({ type: 'unsupported', site: 'Example', title: 'Something new', desc: '', url: 'https://example.org/x' });
		expect(card.querySelector('a.c-lp-main')!.getAttribute('href')).toBe('https://example.org/x');
		expect(card.querySelector('.c-lp-meta')).toBeNull();
		expect(within(card).getByText('Something new')).toBeTruthy();
	});
});

describe('a link preview on the message page', () => {
	// The first sample message whose preview passes a test.
	const find = (ok: (lp: LP) => boolean) => fixture<GroupsProps>('/groups').props.rows
		.flatMap((g) => fixture<ChatProps>(`/groups/${g.id}/chat`).props.messages)
		.find((m) => m.link_preview && ok(m.link_preview))!;

	it('plays a video preview with the chat\'s video box', () => {
		const m = find((lp) => lp.type === 'video' && !!lp.media?.url);
		renderPage(`/messages/group/${m.id}`);
		const card = document.querySelector('.c-lp')!;
		expect(card.querySelector('a.c-videowrap video')!.getAttribute('poster')).toBe(m.link_preview!.media!.poster);
		expect(card.querySelector('.c-lp-meta')!.textContent).toContain('Video');
	});

	it('links a Telegram message preview to the archived message', () => {
		const m = find((lp) => !!lp.target);
		const t = m.link_preview!.target!;
		const page = fixture<MessageProps>(`/messages/group/${m.id}`);
		expect(page.status).toBe(200);
		renderPage(`/messages/group/${m.id}`);
		expect(document.querySelector('.c-lp a.c-lp-main')!.getAttribute('href')).toBe(`/groups/${t.chat_id}/chat?at=${t.message_id}`);
	});
});
