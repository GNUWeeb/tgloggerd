// @vitest-environment happy-dom
// A service message's stored detail on its line in the chat and on its
// message page: the new photo, the pinned message quoted, a video
// chat's length and start, a poll option, the topic and community
// linked, the old title, a background set for the actor alone.
import { beforeEach, describe, expect, it } from 'vitest';
import { fireEvent, screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { ALL, FORUM_GROUP, KINDS_GROUP, PUBLIC_POLLS_GROUP, TOPICS, sampleOf } from '../../standin/kinds.ts';
import { chatStream } from '../../../mockup/src/data/messages.js';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';
import type { ServiceDetail } from '../ui/servicedetail.tsx';

beforeEach(setup);
const chatUrl = `/groups/${FORUM_GROUP.id}/chat`;
const ref = (id: number, title: string) => ({ id, title, kind: 'group' as const, photo: null });
const PHOTO = { stored: true, fetchable: false, url: '/files/p1', thumb: '/files/p1t', render: 'image', ext: 'jpg', w: 640, h: 640, name: '', size: 1000 } as unknown as ChatMessage['media'] & object;
// A background logged before the background itself was kept.
const OLD_BG = { kind: 'background', only_for_self: false, type: null, dark: false, image: null, fill: null, intensity: null, is_inverted: false, is_moving: false, is_blurred: false, theme_name: null } as const;
const SENDER = { id: 5, name: 'Sample Sender' } as ChatMessage['sender'];

// The chat with one service message, built on a plain text one;
// undefined leaves the detail out, as the API does when there is none.
function line(type: string, text: string, detail: ServiceDetail | null | undefined, o: Partial<ChatMessage> = {}, viewer: object = {}) {
	const page = fixture<ChatProps>(chatUrl);
	const t = page.props.messages.find((m) => m.content_type === 'text')!;
	const service = { type, refs: { users: [], chats: [], message_id: null }, ...(detail !== undefined ? { detail } : {}) } as ChatMessage['service'];
	const m = { ...t, id: 1e9, message_id: 9e5, content_type: 'service', text, forward: null, reply_to: null, link_preview: null, deleted_at: null, edits: [], sender: SENDER, service, ...o } as ChatMessage;
	fakeApi('superadmin');
	renderPage(chatUrl, 'superadmin', { props: { ...page.props, messages: [m], at: null }, viewer: { ...page.viewer, ...viewer } });
	return document.getElementById('m900000')!.firstElementChild as HTMLElement;
}
// The line's words, without its time and deleted tag.
const words = (el: Element) => [...el.childNodes].filter((n) => !(n instanceof Element && n.matches('.c-svctime, .c-deltag'))).map((n) => n.textContent).join('').trim();

describe('a service line in the chat', () => {
	it('shows the new group photo, which opens the viewer', () => {
		const l = line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO, animation: null });
		const a = within(l).getByRole('link', { name: 'The new photo' });
		expect(a.querySelector('img')!.getAttribute('src')).toBe('/files/p1t');
		fireEvent.click(a);
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/p1');
	});

	it('says a new photo was not stored, and waits to be asked when media load on click', () => {
		const lost = line('suggest_profile_photo', 'suggested a profile photo', { kind: 'photo', photo: { ...PHOTO, stored: false, url: null, thumb: null }, animation: null });
		expect(within(lost).getByRole('img', { name: 'Photo not stored' })).toBeTruthy();
		document.body.innerHTML = '';
		const l = line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO, animation: null }, {}, { chat: { media: 'click' } });
		expect(l.querySelector('img')).toBeNull();
		fireEvent.click(within(l).getByRole('button', { name: 'Show the photo' }));
		expect(within(l).getByRole('link', { name: 'The new photo' })).toBeTruthy();
	});

	it('plays an animated photo in the viewer, its still as the poster', () => {
		const anim = { ...PHOTO, url: '/files/a1', thumb: null, render: 'animation' as const, ext: 'mp4', poster: '/files/a1p' };
		const l = line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO, animation: anim });
		const a = within(l).getByRole('link', { name: 'The new animated photo' });
		expect(a.querySelector('img')!.getAttribute('src')).toBe('/files/p1t');
		expect(a.getAttribute('href')).toBe('/files/a1');
		fireEvent.click(a);
		const v = screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('video')!;
		expect(v.getAttribute('src')).toBe('/files/a1');
		expect(v.getAttribute('poster')).toBe('/files/a1p');
		document.body.innerHTML = '';
		// Without a poster of its own, the still photo is the poster.
		fireEvent.click(within(line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO, animation: { ...anim, poster: undefined } })).getByRole('link', { name: 'The new animated photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('video')!.getAttribute('poster')).toBe('/files/p1');
		document.body.innerHTML = '';
		// A video not stored leaves the still photo.
		fireEvent.click(within(line('chat_change_photo', 'changed the group photo', { kind: 'photo', photo: PHOTO, animation: { ...anim, stored: false, url: null } })).getByRole('link', { name: 'The new photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/p1');
	});

	it('quotes the pinned message', () => {
		const l = line('pin_message', 'pinned a message', { kind: 'pin', snippet: 'Meeting moved to Friday' });
		expect(l.querySelector('q')!.textContent).toBe('Meeting moved to Friday');
		expect(words(l)).toBe('Sample Sender pinned a message Meeting moved to Friday');
	});

	it('says how long a video chat ran, and when one is planned', () => {
		expect(words(line('video_chat_ended', 'ended the video chat', { kind: 'video_chat', duration: 3725, start_date: null }))).toBe('Sample Sender ended the video chat after 1 h 2 min');
		document.body.innerHTML = '';
		const l = line('video_chat_scheduled', 'scheduled a video chat', { kind: 'video_chat', duration: null, start_date: '2026-12-01T12:00:00Z' });
		expect(l.querySelector('time')!.getAttribute('datetime')).toBe('2026-12-01T12:00:00Z');
		expect(words(l)).toMatch(/^Sample Sender scheduled a video chat for .*2026/);
	});

	it('quotes a poll option added or removed', () => {
		expect(words(line('poll_option_added', 'added an option to a poll', { kind: 'poll_option', text: 'Rust' }))).toBe('Sample Sender added an option to a poll: Rust');
		expect(document.querySelector('.c-service q')!.textContent).toBe('Rust');
	});

	it('links the topic a text names, with its colour, and names one it does not', () => {
		const t = TOPICS[1]!;
		const topic = { id: t.id, name: t.name, icon_color: t.icon_color, icon_custom_emoji_id: null, icon_custom_emoji: null };
		const l = line('forum_topic_created', `created topic "${t.name}"`, { kind: 'topic', topic });
		const a = within(l).getByRole('link', { name: t.name });
		expect(a.getAttribute('href')).toContain(`topic=${t.id}`);
		expect((a.querySelector('.k-topicdot') as HTMLElement).style.background).toBeTruthy();
		expect(words(l)).toBe(`Sample Sender created topic "${t.name}"`);
		document.body.innerHTML = '';
		const e = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic });
		expect(words(e)).toBe(`Sample Sender changed the topic icon: ${t.name}`);
		expect(within(e).getByRole('link', { name: t.name }).getAttribute('href')).toContain(`topic=${t.id}`);
	});

	it('names a topic whose id, name or colour is not known', () => {
		const l = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic: { id: null, name: 'Help', icon_color: null, icon_custom_emoji_id: null, icon_custom_emoji: null } });
		expect(words(l)).toBe('Sample Sender changed the topic icon: Help');
		expect(within(l).queryByRole('link', { name: 'Help' })).toBeNull();
		expect((l.querySelector('.c-svctopic .k-topicdot') as HTMLElement).style.background).toBeTruthy();
		document.body.innerHTML = '';
		const n = line('forum_topic_edited', 'changed the topic icon', { kind: 'topic', topic: { id: 12, name: null, icon_color: 0xffd67e, icon_custom_emoji_id: null, icon_custom_emoji: null } });
		expect(words(n)).toBe('Sample Sender changed the topic icon: a topic');
		expect(within(n).getByRole('link', { name: 'a topic' }).getAttribute('href')).toContain('topic=12');
	});

	it('names the community with its photo, and links nothing, as it has no page', () => {
		const l = line('chat_added_to_community', 'added the group to a community', { kind: 'community', community: { id: 4200001, name: 'Linux Users', photo: '/files/tok-c' } });
		expect(words(l)).toContain('added the group to a community: Linux Users');
		expect(within(l).queryByRole('link', { name: 'Linux Users' })).toBeNull();
		expect(l.querySelector('.c-svccommunity img')!.getAttribute('src')).toBe('/files/tok-c');
	});

	it('names a community without a photo by its initials', () => {
		const noPhoto = line('chat_join_from_community', 'joined the group from a community', { kind: 'community', community: { id: 4200002, name: 'Rust Folks', photo: null } });
		expect(noPhoto.querySelector('.c-svccommunity img')).toBeNull();
		expect(noPhoto.querySelector('.c-svccommunity')!.textContent).toContain('Rust Folks');
	});

	it('gives a created group its title, and links the basic group a supergroup came from', () => {
		expect(words(line('chat_create', 'created the group', { kind: 'title', title: 'Study Club', basic_group: null }))).toBe('Sample Sender created the group Study Club');
		document.body.innerHTML = '';
		const l = line('chat_upgrade_from', 'the supergroup was created from a group', { kind: 'title', title: 'Study Club', basic_group: ref(-42, 'Study Club') });
		expect(words(l)).toBe('The supergroup was created from a group Study Club');
		expect(within(l).getByRole('link', { name: 'Study Club' }).getAttribute('href')).toBe('/groups/-42');
	});

	it('says a background was set for the actor alone', () => {
		expect(words(line('chat_set_background', 'changed the chat background', { ...OLD_BG, only_for_self: true }))).toBe('Sample Sender changed the chat background for themselves');
		document.body.innerHTML = '';
		expect(words(line('chat_set_background', 'changed the chat background', { ...OLD_BG, only_for_self: true }, { sender: null, sender_chat: undefined }))).toBe('You changed the chat background for yourself');
		document.body.innerHTML = '';
		expect(words(line('chat_set_background', 'changed the chat background', { ...OLD_BG, only_for_self: false }))).toBe('Sample Sender changed the chat background');
	});

	it('shows a wallpaper, blurred, which opens the viewer', () => {
		const image = { ...PHOTO, url: '/files/w1', thumb: '/files/w1t', w: 1440, h: 2560 };
		const l = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'wallpaper', image, is_blurred: true });
		const a = within(l).getByRole('link', { name: 'Wallpaper, blurred' });
		expect(a.classList.contains('c-bgblur')).toBe(true);
		expect(a.style.width).toBe('36px');
		expect(a.style.height).toBe('48px');
		expect(a.querySelector('img')!.getAttribute('src')).toBe('/files/w1t');
		fireEvent.click(a);
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/w1');
		document.body.innerHTML = '';
		const ask = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'wallpaper', image }, {}, { chat: { media: 'click' } });
		expect(ask.querySelector('img')).toBeNull();
		fireEvent.click(within(ask).getByRole('button', { name: 'Show the background' }));
		expect(within(ask).getByRole('link', { name: 'Wallpaper' }).querySelector('img')).toBeTruthy();
	});

	it('draws a pattern over its fill at its intensity, and an inverted one on a dark ground', () => {
		const image = { ...PHOTO, url: '/files/pt', thumb: null, ext: 'png' };
		const fill = { type: 'gradient' as const, colors: [0xdbddbb, 0x6ba587], rotation_angle: 45 };
		const l = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'pattern', image, fill, intensity: 40, is_moving: true });
		const a = within(l).getByRole('link', { name: 'Pattern over a gradient, 40% intensity, moving' });
		expect(a.getAttribute('style')).toContain('linear-gradient(225deg, #dbddbb, #6ba587)');
		expect(a.querySelector('img')!.style.opacity).toBe('0.4');
		expect(a.querySelector('.c-bgpat')).toBeNull();
		document.body.innerHTML = '';
		const inv = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'pattern', image, fill: { type: 'freeform_gradient', colors: [0x111111, 0x222222, 0x333333, 0x444444], rotation_angle: 0 }, intensity: 55, is_inverted: true, dark: true });
		const b = within(inv).getByRole('link', { name: 'Pattern over a freeform gradient, 55% intensity, inverted, for a dark theme' });
		expect(b.getAttribute('style')).toContain('radial-gradient(');
		const pat = b.querySelector('.c-bgpat') as HTMLElement;
		expect(pat.style.opacity).toBe('0.55');
		expect(pat.querySelector('img')!.getAttribute('src')).toBe('/files/pt');
	});

	it('shows a pattern it cannot draw as its fill with a hint', () => {
		const image = { ...PHOTO, url: '/files/tgv', thumb: null, render: 'file' as const, ext: 'tgv' };
		const l = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'pattern', image, fill: { type: 'solid', colors: [0x2b5278], rotation_angle: 0 }, intensity: 50 });
		const s = within(l).getByRole('img', { name: 'Pattern over a solid colour, 50% intensity (the pattern is not drawn)' });
		expect(s.classList.contains('c-bghint')).toBe(true);
		expect(s.querySelector('img')).toBeNull();
		expect(s.getAttribute('style')).toMatch(/#2b5278|rgb\(43, 82, 120\)/);
	});

	it('shows a fill\'s colours, and names a chat theme', () => {
		const l = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'fill', fill: { type: 'gradient', colors: [0xffd67e, 0xe17076], rotation_angle: 90 }, only_for_self: true });
		const s = within(l).getByRole('img', { name: 'Fill of a gradient' });
		expect(s.getAttribute('style')).toContain('linear-gradient(270deg, #ffd67e, #e17076)');
		expect(words(l).replace(/\s+/g, ' ')).toBe('Sample Sender changed the chat background for themselves');
		document.body.innerHTML = '';
		const t = line('chat_set_background', 'changed the chat background', { ...OLD_BG, type: 'chat_theme', theme_name: 'Snow' });
		expect(words(t)).toBe('Sample Sender changed the chat background to the chat theme Snow');
		expect(t.querySelector('.c-bgswatch')).toBeNull();
	});

	it('stays as it was without a detail', () => {
		expect(words(line('pin_message', 'pinned a message', null))).toBe('Sample Sender pinned a message');
		document.body.innerHTML = '';
		const pin = line('pin_message', 'pinned a message', undefined);
		expect(words(pin)).toBe('Sample Sender pinned a message');
		expect(pin.querySelector('q')).toBeNull();
		document.body.innerHTML = '';
		const c = line('chat_added_to_community', 'added the group to a community', undefined);
		expect(words(c)).toBe('Sample Sender added the group to a community');
		expect(c.querySelector('a[href^="/groups/"]')).toBeNull();
	});
});

// The stand-in's forum service messages with their detail.
type Raw = { id: number; message_id: number; chat: { id: number; kind: string } };
const forumSample = (type: string, kind?: string) => (chatStream(FORUM_GROUP) as Raw[]).find((m) => {
	const s = sampleOf(m)?.service?.(ALL);
	return s?.type === type && (kind === undefined || (s.detail as { kind?: string } | null)?.kind === kind);
})!;
const open = (raw: Raw, persona: 'superadmin' | 'member' = 'superadmin') => {
	fakeApi(persona);
	renderPage(`/messages/group/${raw.id}`, persona);
	return fixture<MessageProps>(`/messages/group/${raw.id}`, persona).props.message;
};
const text = () => document.querySelector('.a-msgtext')!;

describe('a service message\'s page', () => {
	it('shows the new photo, which opens the viewer', () => {
		open((chatStream(FORUM_GROUP) as Raw[]).find((x) => (sampleOf(x)?.service?.(ALL)?.detail as { photo?: { url: string | null }; animation?: unknown } | null)?.photo?.url && !(sampleOf(x)!.service!(ALL).detail as { animation: unknown }).animation)!);
		fireEvent.click(within(text() as HTMLElement).getByRole('link', { name: 'The new photo' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/standin-986010');
	});

	it('plays the new animated photo', () => {
		open((chatStream(FORUM_GROUP) as Raw[]).find((x) => (sampleOf(x)?.service?.(ALL)?.detail as { animation?: unknown } | null)?.animation)!);
		const a = within(text() as HTMLElement).getByRole('link', { name: 'The new animated photo' });
		expect((a as HTMLElement).style.width).toBe('96px');
		fireEvent.click(a);
		const v = screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('video')!;
		expect(v.getAttribute('src')).toBe('/files/standin-986012');
		expect(v.getAttribute('poster')).toBe('/files/standin-986011');
	});

	it('shows each of the stand-in\'s backgrounds, larger, and a wallpaper opens the viewer', () => {
		const bgs = (chatStream(FORUM_GROUP) as Raw[]).filter((x) => (sampleOf(x)?.service?.(ALL)?.detail as { type?: string } | null)?.type);
		expect(bgs.map((x) => (sampleOf(x)!.service!(ALL).detail as { type: string }).type)).toEqual(['wallpaper', 'pattern', 'pattern', 'pattern', 'fill', 'fill', 'chat_theme']);
		for (const x of bgs.slice(0, 6)) {
			open(x);
			const s = text().querySelector('.c-bgswatch') as HTMLElement;
			expect(s.style.width).toBe('72px');
			expect(s.style.height).toBe('96px');
			document.body.innerHTML = '';
		}
		open(bgs[0]!);
		fireEvent.click(within(text() as HTMLElement).getByRole('link', { name: 'Wallpaper, blurred' }));
		expect(screen.getByRole('dialog', { name: 'Media viewer' }).querySelector('img')!.getAttribute('src')).toBe('/files/standin-986020');
		document.body.innerHTML = '';
		open(bgs[6]!);
		expect(text().textContent).toContain('to the chat theme \u{1F384}');
	});

	it('quotes the pinned message, and leaves a pin logged without it as it was', () => {
		const m = open(forumSample('pin_message', 'pin'));
		expect(text().querySelector('q')!.textContent).toBe((m.service?.detail as { snippet: string }).snippet);
		expect(screen.getByRole('link', { name: 'The message this is about' })).toBeTruthy();
		document.body.innerHTML = '';
		const old = open((chatStream(FORUM_GROUP) as Raw[]).find((x) => { const s = sampleOf(x)?.service?.(ALL); return s?.type === 'pin_message' && s.detail == null; })!);
		expect('detail' in old.service!).toBe(false);
		expect(document.querySelector('q')).toBeNull();
	});

	it('says how long the video chat ran', () => {
		open(forumSample('video_chat_ended'));
		expect(text().textContent).toContain('after 1 h 2 min');
	});

	it('links its topic', () => {
		open(forumSample('forum_topic_created'));
		expect(within(text() as HTMLElement).getByRole('link', { name: TOPICS[1]!.name }).getAttribute('href')).toBe(`${chatUrl}?topic=${TOPICS[1]!.id}`);
	});

	it('names the community and links the basic group the supergroup came from', () => {
		open(forumSample('chat_added_to_community'));
		expect(text()!.querySelector('.c-svccommunity')!.textContent).toContain('Open Source Hub');
		expect(screen.queryByRole('link', { name: 'Open Source Hub' })).toBeNull();
		document.body.innerHTML = '';
		open(forumSample('chat_upgrade_from'));
		expect(within(text() as HTMLElement).getByRole('link', { name: 'Linux Study Group' }).getAttribute('href')).toBe(`/groups/${KINDS_GROUP.id}`);
	});
});
