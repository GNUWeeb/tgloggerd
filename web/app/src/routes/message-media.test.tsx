// @vitest-environment happy-dom
// A message's own page draws its media as the chat does: each kind with
// the chat's component, the placeholder for unstored media, and the
// full-size viewer on a click.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen } from '@testing-library/react';

vi.mock('../ui/lottie.ts', () => ({ playTgs: vi.fn(() => () => {}) }));
const { playTgs } = await import('../ui/lottie.ts');
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import { groupMessages } from '../../../mockup/src/data/messages.js';
import type { MessageProps } from './message.tsx';

beforeEach(() => { setup(); vi.clearAllMocks(); });

type Full = MessageProps['message'];
const urlOf = (id: number) => `/messages/group/${id}`;
// The first sample message whose page shows media the pick wants.
const find = (pick: (m: Full) => boolean) => {
	for (const x of groupMessages() as { id: number; media?: unknown }[]) {
		if (!x.media) continue;
		const m = fixture<MessageProps>(urlOf(x.id)).props.message;
		if (m.media && pick(m)) return m;
	}
	throw new Error('no such sample');
};
const open = (m: Full, patch: Partial<Full> = {}) => {
	fakeApi('superadmin');
	return renderPage(urlOf(m.id), 'superadmin', { props: { message: { ...m, ...patch } } });
};
const box = () => document.querySelector('.a-msgmedia') as HTMLElement;
const viewer = () => screen.queryByRole('dialog', { name: 'Media viewer' });
const stored = (m: Full) => m.media!.stored;

describe('the message page\'s media', () => {
	it('shows a photo, which opens in the viewer and closes with Esc', () => {
		const m = find((x) => x.content_type === 'photo' && stored(x));
		open(m);
		const a = box().querySelector('a.c-media[aria-label="Photo"]')!;
		expect(a.querySelector('img')!.getAttribute('src')).toBe(m.media!.thumb ?? m.media!.url);
		fireEvent.click(a);
		expect(viewer()!.querySelector('img')!.getAttribute('src')).toBe(m.media!.url);
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(viewer()).toBeNull();
	});

	it('shows a video, which plays in the viewer', () => {
		const m = find((x) => x.content_type === 'video' && x.media!.render === 'video' && stored(x));
		open(m);
		const a = box().querySelector('a.c-media[aria-label="Video"]')!;
		expect(a.querySelector('video')).toBeTruthy();
		fireEvent.click(a);
		expect(viewer()!.querySelector('video')!.getAttribute('src')).toBe(m.media!.url);
	});

	it('shows a video\'s poster and blurred preview, loading none of the video until it is played', () => {
		const m = find((x) => x.content_type === 'video' && x.media!.render === 'video' && stored(x));
		open(m, { media: { ...m.media!, poster: '/files/tok-poster', minithumb: 'data:image/jpeg;base64,AA', thumb: '/files/tok-poster' } });
		const a = box().querySelector('a.c-media[aria-label="Video"]')!;
		const v = a.querySelector('video')!;
		expect(v.getAttribute('poster')).toBe('/files/tok-poster');
		expect(v.getAttribute('preload')).toBe('none');
		expect(v.getAttribute('src')).toBeNull();
		expect(a.querySelector('img.c-mini')!.getAttribute('src')).toBe('data:image/jpeg;base64,AA');
		expect(a.querySelector('.c-playbig')).toBeTruthy();
	});

	it('plays an animation muted and looping, and opens it in the viewer', () => {
		const m = find((x) => x.media!.render === 'animation' && x.content_type !== 'sticker' && stored(x));
		open(m);
		const v = box().querySelector('a.c-media[aria-label="Animation"] video') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe(m.media!.url);
		expect(v.loop).toBe(true);
		expect(v.muted).toBe(true);
		fireEvent.click(v.closest('a')!);
		expect(viewer()!.querySelector('video')).toBeTruthy();
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(viewer()).toBeNull();
	});

	it('plays a GIF whose thumb is its minithumb', () => {
		const m = find((x) => x.media!.render === 'animation' && x.content_type !== 'sticker' && stored(x));
		open(m, { content_type: 'animation', media: { ...m.media!, thumb: 'data:image/jpeg;base64,AA' } });
		const v = box().querySelector('video') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe(m.media!.url);
		expect(v.loop).toBe(true);
	});

	it('plays a .webm sticker as a looping muted video', () => {
		const m = find((x) => x.content_type === 'sticker' && x.media!.format === 'webm');
		open(m);
		const v = box().querySelector('video.c-sticker') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe(m.media!.url);
		expect(v.loop).toBe(true);
		expect(v.muted).toBe(true);
	});

	it('plays a .tgs sticker with the Lottie player', () => {
		const m = find((x) => x.content_type === 'sticker' && x.media!.format === 'tgs');
		open(m);
		expect(box().querySelector('.c-lottie')).toBeTruthy();
		expect(playTgs).toHaveBeenCalledWith(box().querySelector('.c-lottie-box'), m.media!.url, expect.any(Function), expect.any(Function));
	});

	it('shows a .webp sticker as a picture', () => {
		const m = find((x) => x.content_type === 'sticker' && x.media!.format === 'webp');
		open(m);
		expect(box().querySelector('.c-sticker-btn img')!.getAttribute('src')).toBe(m.media!.thumb ?? m.media!.url);
	});

	it('gives a voice note its player', () => {
		const m = find((x) => x.content_type === 'voice');
		open(m);
		expect(box().querySelector('.c-voice button[aria-label="Play"]')).toBeTruthy();
		expect(box().querySelector('.c-voice audio')!.getAttribute('src')).toBe(m.media!.url);
	});

	it('gives an audio file a link and controls', () => {
		const m = find((x) => x.content_type === 'audio' && stored(x));
		open(m);
		expect(box().querySelector('a.c-doc')!.getAttribute('href')).toBe(m.media!.url);
		expect(box().querySelector('audio.c-audio')!.hasAttribute('controls')).toBe(true);
	});

	it('links a document to its file, and keeps the File row', () => {
		const m = find((x) => x.content_type === 'document' && stored(x));
		open(m);
		const a = box().querySelector('a.c-doc')!;
		expect(a.getAttribute('href')).toBe(m.media!.url);
		expect(a.getAttribute('download')).toBe(m.media!.name);
		expect(screen.getByRole('link', { name: `file #${m.media!.file_id.toLocaleString('en-US')}` })).toBeTruthy();
	});

	it('shows the placeholder for media not stored, and offers to fetch it', () => {
		const m = find((x) => x.content_type === 'video' && x.media!.render === 'video');
		const r = open(m, { media: { ...m.media!, stored: false, url: null, thumb: null } });
		expect(box().querySelector('.c-missing')!.textContent).toContain('Not stored');
		r.unmount();
		open(m, { media: { ...m.media!, stored: false, url: '/files/fetch-me', thumb: null } });
		expect(box().querySelector('.c-fetch')!.textContent).toBe('Load from Telegram');
	});
});
