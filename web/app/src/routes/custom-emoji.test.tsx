// @vitest-environment happy-dom
// Custom emoji where the pages show them: in a message's text, in its
// reactions, as forum topic icons (strip, tag, service line, message
// page) and a message's effect, from the stand-in's samples
// (standin/kinds.ts CUSTOM_EMOJI, EMOJI_SAMPLE, TOPICS).
import { beforeEach, describe, expect, it } from 'vitest';
import { screen, within } from '@testing-library/react';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { CUSTOM_EMOJI, FORUM_GROUP, KINDS_GROUP, TOPICS } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { MessageProps } from './message.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);

const url = `/groups/${KINDS_GROUP.id}/chat`;
const msgs = (u = url) => fixture<ChatProps>(u).props.messages;
const emojiMsg = () => msgs().find((m) => m.text.startsWith('Custom emoji:'))!;
const bubble = (m: ChatMessage) => document.getElementById(`m${m.message_id}`)!;
const openChat = (u = url) => { fakeApi('superadmin'); renderPage(u); };
const openMessage = (m: ChatMessage) => { fakeApi('superadmin'); renderPage(`/messages/group/${m.id}`); };
const E = CUSTOM_EMOJI;

// The custom emoji drawn in a text: a picture, an animation (its
// fallback until in view), a video, a mask in the text colour, and the
// emoji itself without a file or while not known.
function expectEmojiText(t: HTMLElement) {
	const w = within(t);
	const img = w.getByRole('img', { name: E.webp.e.emoji });
	expect(img.tagName).toBe('IMG');
	expect(img.getAttribute('src')).toBe(E.webp.e.url);
	const tgs = w.getByRole('img', { name: E.tgs.e.emoji });
	expect(tgs.className).toContain('ce-tgs');
	expect(tgs.querySelector('.ce-alt')!.textContent).toBe(E.tgs.e.emoji);
	const video = t.querySelector('video.ce')!;
	expect(video.getAttribute('src')).toBe(E.webm.e.url);
	expect(video.getAttribute('aria-label')).toBe(E.webm.e.emoji);
	const mask = w.getByRole('img', { name: E.repaint.e.emoji });
	expect(mask.className).toContain('ce-mask');
	expect(t.textContent).toContain(`${E.nofile.e.emoji} without a file`);
	expect([...t.querySelectorAll('.rt-emoji')].map((e) => e.textContent)).toEqual([E.nofile.e.emoji, '❓']);
	expect(t.textContent).toContain('❓ not known yet');
}

// Its reactions: the custom emoji's stickers.
function expectReactions(root: HTMLElement) {
	const r = within(root).getByLabelText('Reactions');
	const [anim, pic, thumbs] = r.querySelectorAll('.k-reaction');
	expect(anim!.getAttribute('title')).toBe(`Custom emoji ${E.tgs.e.emoji}`);
	expect(anim!.querySelector('.ce-tgs')).not.toBeNull();
	expect(anim!.textContent).toContain('7');
	expect(pic!.querySelector('img.ce')!.getAttribute('src')).toBe(E.webp.e.url);
	expect(thumbs!.textContent).toBe('👍2');
}

describe('custom emoji in the chat', () => {
	it('draws them in a message\'s text', () => {
		openChat();
		expectEmojiText(bubble(emojiMsg()).querySelector('.c-text') as HTMLElement);
	});

	it('draws a custom emoji reaction as its sticker, and one not known as a diamond', () => {
		openChat();
		expectReactions(bubble(emojiMsg()));
		const poll = msgs().find((m) => m.reactions.some((r) => r.custom_emoji_id && !r.custom_emoji))!;
		const unknown = [...bubble(poll).querySelectorAll('.k-reaction')].find((r) => r.getAttribute('title') === 'Custom emoji')!;
		expect(unknown.querySelector('.ce')).toBeNull();
		expect(unknown.textContent).toBe('◆3');
	});

	it('marks a message\'s effect on the bubble, and one not known yet', () => {
		openChat();
		const mark = bubble(emojiMsg()).querySelector('.c-meta .k-effect')!;
		expect(mark.getAttribute('title')).toBe('Message effect 🔥');
		expect(mark.querySelector('img')!.getAttribute('src')).toBe('/files/standin-987010');
		const loc = msgs().find((m) => m.content_type === 'location' && m.effect_id !== null)!;
		const unknown = bubble(loc).querySelector('.c-meta .k-effect')!;
		expect(unknown.getAttribute('title')).toBe('Message effect');
		expect(within(unknown as HTMLElement).getByLabelText('Message effect')).toBeTruthy();
		const plain = msgs().find((m) => m.effect_id === null)!;
		expect(bubble(plain).querySelector('.k-effect')).toBeNull();
	});
});

describe('custom emoji topic icons', () => {
	const furl = `/groups/${FORUM_GROUP.id}/chat`;
	const iconic = TOPICS.find((t) => t.icon_custom_emoji)!;
	const unknown = TOPICS.find((t) => t.icon_custom_emoji_id && !t.icon_custom_emoji)!;

	it('draws a topic\'s custom icon in the strip, and its colour while the icon is not known', () => {
		openChat(furl);
		const strip = screen.getByRole('navigation', { name: 'Forum topics' });
		const a = within(strip).getByRole('link', { name: iconic.name });
		expect(a.querySelector('.k-topicicon .ce-tgs')).not.toBeNull();
		expect(a.querySelector('.k-topicdot')).toBeNull();
		const b = within(strip).getByRole('link', { name: new RegExp(unknown.name) });
		expect(b.querySelector('.k-topicicon')).toBeNull();
		expect((b.querySelector('.k-topicdot') as HTMLElement).style.background).toBeTruthy();
	});

	it('draws it on a message\'s topic tag and on a service line naming the topic', () => {
		openChat(furl);
		const m = msgs(furl).find((x) => x.content_type !== 'service' && x.topic_id === iconic.id)!;
		const tag = bubble(m).querySelector('.k-topictag')!;
		expect(tag.querySelector('.k-topicicon .ce')).not.toBeNull();
		const svc = msgs(furl).find((x) => x.service?.detail?.kind === 'topic')!;
		const link = bubble(svc).querySelector('.c-svctopic')!;
		expect(link.querySelector('.k-topicicon .ce')).not.toBeNull();
	});

	it('draws it on the message page', () => {
		const m = msgs(furl).find((x) => x.content_type !== 'service' && x.topic_id === iconic.id)!;
		fakeApi('superadmin');
		renderPage(`/messages/group/${m.id}`);
		const link = screen.getByRole('link', { name: iconic.name });
		expect(link.querySelector('.k-topicicon .ce')).not.toBeNull();
	});
});

describe('custom emoji on the message page', () => {
	it('draws them in the text and the reactions, and marks the effect', () => {
		const m = emojiMsg();
		openMessage(m);
		expect(fixture<MessageProps>(`/messages/group/${m.id}`).props.message.effect_id).not.toBeNull();
		expectEmojiText(document.querySelector('.a-msgtext') as HTMLElement);
		const inter = document.querySelector('.a-msginteract') as HTMLElement;
		expectReactions(inter);
		expect(inter.querySelector('.k-effect img')!.getAttribute('src')).toBe('/files/standin-987010');
	});
});
