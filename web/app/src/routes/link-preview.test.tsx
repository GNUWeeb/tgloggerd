// @vitest-environment happy-dom
// A link preview's card in the chat and on the message page: its type,
// author and length, its own video, animation or audio played with the
// chat's components, an embedded player's link, and a Telegram message
// link opening the archived message.
import { beforeEach, describe, expect, it } from 'vitest';
import { cleanup, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMedia, ChatMessage } from '../types.ts';

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
		// One of each type named since, with what it carries.
		// A .txt document, which renders as text by its extension.
		expect(of('document').some((x) => x.media?.render === 'text')).toBe(true);
		expect(of('sticker').some((x) => x.media?.format)).toBe(true);
		expect(of('video_note').some((x) => x.media?.round)).toBe(true);
		expect(of('voice_note').some((x) => x.media?.render === 'audio' && x.media.wave?.length)).toBe(true);
		expect(of('album').some((x) => x.album && x.album.items.some((i) => i === null) && x.album.items.some((i) => i?.render === 'video'))).toBe(true);
		expect(of('background').some((x) => x.background?.image)).toBe(true);
		expect(of('sticker_set').some((x) => (x.stickers?.length ?? 0) > 1 && x.stickers!.length <= 5)).toBe(true);
		expect(of('gift_collection').some((x) => x.stickers?.length)).toBe(true);
		expect(of('gift_auction').some((x) => x.gift?.auction && x.gift.star_count)).toBe(true);
		expect(of('upgraded_gift').some((x) => x.gift?.upgraded)).toBe(true);
		expect(of('story').some((x) => x.story?.poster && x.story.username)).toBe(true);
		expect(of('live_story').some((x) => x.story)).toBe(true);
		expect(of('story_album').some((x) => x.media?.w && x.media.h)).toBe(true);
		expect(of('theme').some((x) => x.theme?.background)).toBe(true);
		expect(of('text_composition_style').some((x) => x.custom_emoji_id && x.custom_emoji)).toBe(true);
		for (const t of ['direct_messages_chat', 'premium_gift_code', 'request_managed_bot', 'shareable_chat_folder']) expect(of(t).length, t).toBeGreaterThan(0);
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

// The later types (docs/todo/msg-link-preview-types.md), each with
// what it carries beside its card.
const card = (o: Partial<LP>): LP => ({ type: 'unsupported', site: 'Telegram', title: 'T', desc: '', url: 'https://t.me/x/1', ...o });
const stk = (url: string, format: 'webp' | 'tgs' | 'webm', emoji = '🙂'): ChatMedia => ({ stored: true, url, thumb: format === 'webp' ? url : null, render: format === 'tgs' ? 'lottie' : format === 'webm' ? 'animation' : 'image', ext: format, format, emoji, w: 512, h: 512 });
const fill = { type: 'gradient' as const, colors: [0x112233, 0x445566], rotation_angle: 0 };
const bgOf = (o: Partial<NonNullable<LP['background']>> = {}): NonNullable<LP['background']> => ({ type: 'pattern', image: { stored: true, url: '/files/lp-bg', thumb: '/files/lp-bg', render: 'image', ext: 'png', w: 1440, h: 2560 }, fill, intensity: 40, is_inverted: false, is_moving: false, is_blurred: false, theme_name: null, ...o });

describe('a link preview of a later type', () => {
	it('links a document preview\'s file with the chat\'s document row', () => {
		const { card: c } = chatWith(card({ type: 'document', title: 'notes.txt', media: { stored: true, url: '/files/lp-doc', thumb: null, render: 'file', ext: 'txt', name: 'notes.txt', size: 2048 } }));
		const a = c.querySelector('a.c-doc')!;
		expect(a.getAttribute('href')).toBe('/files/lp-doc?download=1');
		expect(c.querySelector('a.c-lp-main')!.contains(a)).toBe(false);
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Document');
	});

	it('draws a document preview\'s medium by its render: a PDF to open, a picture with its download', () => {
		const pdf = chatWith(card({ type: 'document', title: 'guide.pdf', media: { stored: true, url: '/files/lp-pdf', thumb: null, render: 'pdf', ext: 'pdf', name: 'guide.pdf', size: 4096 } })).card;
		const open = within(pdf).getByRole('link', { name: 'Open' });
		expect([open.getAttribute('href'), open.getAttribute('target'), open.getAttribute('rel')]).toEqual(['/files/lp-pdf', '_blank', 'noopener']);
		expect(pdf.querySelector('a.c-doc')!.getAttribute('href')).toBe('/files/lp-pdf?download=1');
		expect(pdf.querySelector('iframe, object, embed')).toBeNull();
		cleanup();
		const png = chatWith(card({ type: 'document', title: 'shot.png', media: { stored: true, url: '/files/lp-png', thumb: '/files/lp-png', render: 'image', ext: 'png', name: 'shot.png', size: 90000 } })).card;
		expect(within(png).getByRole('link', { name: 'Photo' }).querySelector('img')!.getAttribute('src')).toBe('/files/lp-png');
		expect(within(png).getByRole('link', { name: 'Download' }).getAttribute('href')).toBe('/files/lp-png?download=1');
	});

	it('draws a sticker preview\'s sticker, and an older one\'s picture as before', () => {
		const { card: c } = chatWith(card({ type: 'sticker', media: stk('/files/lp-stk.webm', 'webm') }));
		expect(c.querySelector('video.c-sticker')!.getAttribute('src')).toBe('/files/lp-stk.webm');
		expect(c.querySelector('img.c-lp-img')).toBeNull();
	});

	it('keeps an older sticker preview\'s picture in the card', () => {
		const { card: c } = chatWith(card({ type: 'sticker', media: { stored: true, url: PIC, thumb: PIC, render: 'image' } }));
		expect(c.querySelector('a.c-lp-main img.c-lp-img')!.getAttribute('src')).toBe(PIC);
		expect(c.querySelector('.c-sticker')).toBeNull();
	});

	it('draws a video note preview as a round video', () => {
		const { card: c } = chatWith(card({ type: 'video_note', media: { stored: true, url: '/files/lp-round', thumb: PIC, poster: PIC, render: 'video', ext: 'mp4', w: 384, h: 384, duration: 21 } }));
		const box = c.querySelector('a.c-videowrap')!;
		expect(box.classList.contains('round')).toBe(true);
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Video message');
	});

	it('plays a voice note preview with the chat\'s voice player', () => {
		const { card: c } = chatWith(card({ type: 'voice_note', media: { stored: true, url: '/files/lp-voice', thumb: null, render: 'audio', ext: 'ogg', duration: 14, wave: [1, 20, 31] } }));
		expect(c.querySelector('.c-voice audio')!.getAttribute('src')).toBe('/files/lp-voice');
		expect(c.querySelectorAll('.c-voice .c-wave span')).toHaveLength(3);
		expect(c.querySelector('audio.c-audio')).toBeNull();
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Voice message');
	});

	it('shows a video preview\'s kept cover as its poster', () => {
		const { card: c } = chatWith(video({ media: { stored: true, url: '/files/lp-video', thumb: PIC, poster: '/files/lp-cover', render: 'video', ext: 'mp4', w: 1280, h: 720 } }));
		expect(c.querySelector('a.c-videowrap video')!.getAttribute('poster')).toBe('/files/lp-cover');
	});

	it('draws an album as the chat\'s album grid with its caption', () => {
		const { card: c } = chatWith(card({ type: 'album', desc: 'd', album: { caption: 'From the meetup', items: [
			{ stored: true, url: '/files/al-1', thumb: '/files/al-1', render: 'image', w: 4, h: 3 },
			null,
			{ stored: true, url: '/files/al-2', thumb: PIC, poster: PIC, render: 'video', ext: 'mp4', w: 16, h: 9 },
		] } }));
		const grid = c.querySelector('.c-lp-album .c-album.n2')!;
		expect(grid.querySelector('a[aria-label="Photo"]')!.getAttribute('href')).toBe('/files/al-1');
		expect(grid.querySelector('a[aria-label="Video"]')!.getAttribute('href')).toBe('/files/al-2');
		expect(within(c).getByText('From the meetup')).toBeTruthy();
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Album');
	});

	it('draws a background preview with the background swatch', () => {
		const { card: c } = chatWith(card({ type: 'background', background: bgOf() }));
		const sw = c.querySelector('.c-lp-bg .c-bgswatch')!;
		expect(sw.getAttribute('aria-label')).toBe('Pattern over a gradient, 40% intensity');
		expect(sw.querySelector('img')!.getAttribute('src')).toBe('/files/lp-bg');
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Chat background');
	});

	it('names a chat theme background', () => {
		const { card: c } = chatWith(card({ type: 'background', background: bgOf({ type: 'chat_theme', image: null, fill: null, intensity: null, theme_name: '🎄' }) }));
		expect(c.querySelector('.c-bgswatch')).toBeNull();
		expect(c.querySelector('q.c-svcquote')!.textContent).toBe('🎄');
	});

	it('shows a sticker set\'s stickers, leaving out one not recorded', () => {
		const { card: c } = chatWith(card({ type: 'sticker_set', stickers: [stk('/files/s1', 'webp', '😀'), null, stk('/files/s2.webm', 'webm')] }));
		const row = c.querySelector('.c-lp-stickers')!;
		expect(row.children).toHaveLength(2);
		expect(row.querySelector('button.c-sticker-btn img')!.getAttribute('src')).toBe('/files/s1');
		expect(row.querySelector('video.c-sticker')!.getAttribute('src')).toBe('/files/s2.webm');
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Sticker set');
	});

	it('draws a gift auction\'s gift with the gift card, its price and its rounds', () => {
		const { card: c } = chatWith(card({ type: 'gift_auction', gift: { sticker: stk('/files/g1', 'webp', '🎁'), symbol_sticker: null, star_count: 2500, upgraded: null, auction: { start_date: '2026-12-01T10:00:00Z', end_date: null, gifts_per_round: 50 } } }));
		expect(c.querySelector('.c-gift .c-giftstars')!.textContent).toBe('2,500 Stars');
		const line = c.querySelector('.c-lp-auction')!;
		expect(line.querySelector('time')!.getAttribute('dateTime')).toBe('2026-12-01T10:00:00Z');
		expect(line.textContent).toContain('50 gifts per round');
		expect(line.textContent).not.toContain('until');
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Gift auction');
	});

	it('draws an upgraded gift with its title linked to its page', () => {
		const upgraded = { id: '1', regular_gift_id: null, title: 'Plush Pepe', name: 'PlushPepe-1', number: 1, owner: null, model: null, symbol: null, backdrop: null, original_details: null } as unknown as NonNullable<NonNullable<LP['gift']>['upgraded']>;
		const { card: c } = chatWith(card({ type: 'upgraded_gift', gift: { sticker: stk('/files/g2', 'webp'), symbol_sticker: null, star_count: null, upgraded, auction: null } }));
		expect(c.querySelector('a.c-gifttitle')!.getAttribute('href')).toBe('https://t.me/nft/PlushPepe-1');
		expect(c.querySelector('.c-lp-auction')).toBeNull();
	});

	it('shows a story\'s poster and links the story on Telegram', () => {
		const { card: c } = chatWith(card({ type: 'story', story: { poster: { id: 42, title: 'Some Poster', kind: 'user', photo: null }, story_id: 7, username: 'some_poster' } }));
		const st = c.querySelector('.c-lp-story')!;
		expect(st.querySelector('a.k-ref')!.getAttribute('href')).toBe('/users/42');
		const open = within(st as HTMLElement).getByRole('link', { name: 'Open the story on Telegram' });
		expect(open.getAttribute('href')).toBe('https://t.me/some_poster/s/7');
		expect(open.getAttribute('target')).toBe('_blank');
	});

	it('links no story without a username, and keeps its card', () => {
		const { card: c } = chatWith(card({ type: 'live_story', story: { poster: null, story_id: 7, username: null } }));
		expect(c.querySelector('.c-lp-story')).toBeNull();
		expect(c.querySelector('a.c-lp-main')!.getAttribute('href')).toBe('https://t.me/x/1');
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Live story');
	});

	it('draws a theme as a swatch of its colours', () => {
		const { card: c } = chatWith(card({ type: 'theme', theme: { base_theme: 'night', accent_color: 0x3e88f7, background: { ...bgOf({ type: 'fill', image: null, intensity: null }), name: 'Midnight', dark: true }, outgoing_message_fill: { type: 'solid', colors: [0x2b5278], rotation_angle: 0 }, animate_outgoing_message_fill: false, outgoing_message_accent_color: null } }));
		const sw = c.querySelector('.c-lp-theme') as HTMLElement;
		expect(sw.getAttribute('aria-label')).toBe('Based on Night, accent #3e88f7');
		expect(sw.classList.contains('dark')).toBe(true);
		expect(sw.style.background).toContain('linear-gradient');
		expect((sw.querySelector('.c-lp-theme-out') as HTMLElement).style.background).toMatch(/#2b5278|rgb\(43, 82, 120\)/);
		expect(c.querySelector('.c-lp-themewrap')!.textContent).toContain('Midnight');
	});

	it('shows a text composition style\'s custom emoji, and none when unresolved', () => {
		const e = { emoji: '🎉', format: 'webp' as const, url: '/files/ce-1', needs_repainting: false };
		const { card: c } = chatWith(card({ type: 'text_composition_style', title: 'Party', custom_emoji_id: '1', custom_emoji: e }));
		expect(c.querySelector('.c-lp-emoji img.ce')!.getAttribute('src')).toBe('/files/ce-1');
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe('Text style');
	});

	it('shows no icon for an unresolved custom emoji', () => {
		const { card: c } = chatWith(card({ type: 'text_composition_style', custom_emoji_id: '1', custom_emoji: null }));
		expect(c.querySelector('.c-lp-emoji')).toBeNull();
	});

	it.each([
		['direct_messages_chat', 'Direct messages'], ['premium_gift_code', 'Premium gift code'],
		['request_managed_bot', 'Managed bot request'], ['shareable_chat_folder', 'Chat folder'],
		['gift_collection', 'Gift collection'], ['story_album', 'Story album'],
	])('names a %s preview', (type, label) => {
		const { card: c } = chatWith(card({ type }));
		expect(c.querySelector('.c-lp-meta')!.textContent).toBe(label);
	});

	it('draws a type and fields it does not know as the plain card', () => {
		const { card: c } = chatWith({ ...card({ type: 'some_future_kind', title: 'New' }), some_future_field: { x: 1 } } as LP);
		expect(c.querySelector('.c-lp-meta')).toBeNull();
		expect(c.children).toHaveLength(1);
		expect(c.querySelector('a.c-lp-main')!.getAttribute('href')).toBe('https://t.me/x/1');
	});
});

describe('a link preview on the message page', () => {
	// The first sample message with a page whose preview passes a test.
	const find = (ok: (lp: LP) => boolean) => fixture<GroupsProps>('/groups').props.rows
		.flatMap((g) => fixture<ChatProps>(`/groups/${g.id}/chat`).props.messages)
		.find((m) => m.link_preview && ok(m.link_preview) && fixture<MessageProps>(`/messages/group/${m.id}`).status === 200)!;

	it('plays a video preview with the chat\'s video box', () => {
		const m = find((lp) => lp.type === 'video' && !!lp.media?.url);
		renderPage(`/messages/group/${m.id}`);
		const card = document.querySelector('.c-lp')!;
		expect(card.querySelector('a.c-videowrap video')!.getAttribute('poster')).toBe(m.link_preview!.media!.poster);
		expect(card.querySelector('.c-lp-meta')!.textContent).toContain('Video');
	});

	it('draws a later type\'s fields, as the chat does', () => {
		const m = find((lp) => !!lp.gift);
		renderPage(`/messages/group/${m.id}`);
		expect(document.querySelector('.c-lp .c-lp-gift .c-gift')).not.toBeNull();
		expect(document.querySelector('.c-lp .c-lp-meta')!.textContent).toMatch(/^(Gift auction|Upgraded gift)$/);
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
