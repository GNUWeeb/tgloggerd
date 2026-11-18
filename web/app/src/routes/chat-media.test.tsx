// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MessageMedia, type Media } from './chat-media.tsx';
import { PLAIN_KINDS } from '../test/kinds.ts';
import type { ChatMessage } from '../types.ts';

afterEach(cleanup);

const URL1 = '/files/tok-1';
const media = (o: Partial<Media> = {}): Media => ({ stored: true, thumb: null, url: URL1, ...o });
const msg = (content_type: ChatMessage['content_type'], m: Media | null, album: Media[] = []): ChatMessage => ({
	id: 1, message_id: 1, date: '2026-11-17T10:00:00Z', scope: 'group', sender: null, content_type, text: '',
	media: m, album, reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null, ...PLAIN_KINDS,
});
// The lightbox portals into the app root, as in the real page.
const show = (el: ReactElement) => render(<div className="app">{el}</div>);
const viewer = () => screen.queryByRole('dialog', { name: 'Media viewer' });

describe('chat media', () => {
	it('draws a photo at its real shape and opens it in the viewer; Esc closes it', () => {
		show(<MessageMedia msg={msg('photo', media({ thumb: URL1, w: 1280, h: 720 }))} onClick={false} />);
		const link = screen.getByRole('link', { name: 'Photo' });
		expect(link.getAttribute('style')).toContain('aspect-ratio: 1280 / 720');
		expect(link.querySelector('img')!.getAttribute('src')).toBe(URL1);
		fireEvent.click(link);
		const box = viewer()!;
		expect(box.querySelector('img')!.getAttribute('src')).toBe(URL1);
		expect(within(box).getByRole('link', { name: 'Open the file' }).getAttribute('href')).toBe(URL1);
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(viewer()).toBeNull();
	});

	it('leaves a new-tab click to the browser', () => {
		show(<MessageMedia msg={msg('photo', media({ thumb: URL1 }))} onClick={false} />);
		// Record whether the app let the click through, then stop the
		// test DOM from following the link.
		let prevented: boolean | null = null;
		const stop = (e: Event) => { prevented = e.defaultPrevented; e.preventDefault(); };
		addEventListener('click', stop);
		fireEvent.click(screen.getByRole('link', { name: 'Photo' }), { ctrlKey: true });
		removeEventListener('click', stop);
		expect(prevented).toBe(false);
		expect(viewer()).toBeNull();
	});

	it('marks a file that is neither stored nor fetchable as metadata only', () => {
		show(<MessageMedia msg={msg('photo', media({ stored: false, url: null, size: 2048 }))} onClick={false} />);
		expect(screen.getByText('Not stored')).toBeTruthy();
		expect(screen.queryByRole('link')).toBeNull();
	});

	it('asks before loading media when the viewer loads it on click', () => {
		show(<MessageMedia msg={msg('photo', media({ thumb: URL1 }))} onClick />);
		expect(document.querySelector('img')).toBeNull();
		fireEvent.click(screen.getByRole('button', { name: 'Show' }));
		expect(document.querySelector('img')!.getAttribute('src')).toBe(URL1);
	});

	it('loads an unstored video from Telegram only when it is clicked, then plays it in the viewer', () => {
		show(<MessageMedia msg={msg('video', media({ stored: false, fetchable: true, duration: 75 }))} onClick={false} />);
		const box = screen.getByRole('link', { name: 'Video' });
		expect(within(box).getByText('Load from Telegram')).toBeTruthy();
		expect(within(box).getByText('1:15')).toBeTruthy();
		const still = box.querySelector('video')!;
		expect(still.getAttribute('src')).toBeNull();
		expect(still.getAttribute('preload')).toBe('none');
		fireEvent.click(box);
		const v = viewer()!.querySelector('video')!;
		expect(v.getAttribute('src')).toBe(URL1);
		expect(v.hasAttribute('controls')).toBe(true);
	});

	it('shows a stored video by its first frame', () => {
		show(<MessageMedia msg={msg('video', media({ w: 16, h: 9 }))} onClick={false} />);
		const v = screen.getByRole('link', { name: 'Video' }).querySelector('video')!;
		expect(v.getAttribute('src')).toBe(URL1);
		expect(v.getAttribute('preload')).toBe('metadata');
	});

	it('plays a stored animation inline, muted and looping', () => {
		show(<MessageMedia msg={msg('animation', media())} onClick={false} />);
		const v = screen.getByRole('link', { name: 'Animation' }).querySelector('video')!;
		expect(v.getAttribute('src')).toBe(URL1);
		expect(v.hasAttribute('loop')).toBe(true);
		expect(v.hasAttribute('autoplay')).toBe(true);
		expect((v as HTMLVideoElement).muted).toBe(true);
	});

	it('draws each sticker format: an image, a looping video, and a Lottie sticker\'s emoji', () => {
		const { unmount } = show(<MessageMedia msg={msg('sticker', media({ format: 'webp', thumb: URL1, emoji: '🔥' }))} onClick={false} />);
		expect(screen.getByRole('button', { name: 'sticker 🔥' }).querySelector('img')!.getAttribute('src')).toBe(URL1);
		unmount();
		const r2 = show(<MessageMedia msg={msg('sticker', media({ format: 'webm', emoji: '🔥' }))} onClick={false} />);
		const v = r2.container.querySelector('video.c-sticker')!;
		expect(v.getAttribute('src')).toBe(URL1);
		expect(v.hasAttribute('loop')).toBe(true);
		r2.unmount();
		show(<MessageMedia msg={msg('sticker', media({ format: 'tgs', emoji: '🔥' }))} onClick={false} />);
		expect(screen.getByRole('img', { name: 'sticker 🔥' }).textContent).toBe('🔥');
	});

	it('tells a sticker\'s format by how it renders or its extension when Go sends none', () => {
		const r = show(<MessageMedia msg={msg('sticker', media({ render: 'animation' }))} onClick={false} />);
		expect(r.container.querySelector('video')).toBeTruthy();
		r.unmount();
		const r2 = show(<MessageMedia msg={msg('sticker', media({ ext: 'tgs', emoji: '🙏' }))} onClick={false} />);
		expect(screen.getByRole('img', { name: 'sticker 🙏' })).toBeTruthy();
		r2.unmount();
		// A blurred preview in thumb no longer passes for an image.
		const r3 = show(<MessageMedia msg={msg('sticker', media({ render: 'image', thumb: 'data:image/jpeg;base64,AA' }))} onClick={false} />);
		expect(r3.container.querySelector('img')!.getAttribute('src')).toBe('data:image/jpeg;base64,AA');
	});

	it('draws a document as what it renders as', () => {
		const r = show(<MessageMedia msg={msg('document', media({ render: 'image', thumb: URL1, name: 'shot.png' }))} onClick={false} />);
		expect(screen.getByRole('link', { name: 'Photo' })).toBeTruthy();
		r.unmount();
		show(<MessageMedia msg={msg('document', media({ render: 'video', name: 'clip.mp4' }))} onClick={false} />);
		expect(screen.getByRole('link', { name: 'Video' })).toBeTruthy();
	});

	it('shows a blurred preview of media that is not stored', () => {
		const r = show(<MessageMedia msg={msg('photo', media({ stored: false, url: null, thumb: 'data:image/jpeg;base64,AA' }))} onClick={false} />);
		const box = r.container.querySelector('.c-missing')!;
		expect(box.className).toContain('c-blurred');
		expect((box as HTMLElement).style.backgroundImage).toContain('data:image/jpeg;base64,AA');
		expect(within(box as HTMLElement).getByText('Not stored')).toBeTruthy();
	});

	it('names music by its performer and title', () => {
		show(<MessageMedia msg={msg('audio', media({ name: 'track01.mp3', performer: 'Ayu', title: 'Kernel Blues', size: 4096 }))} onClick={false} />);
		expect(screen.getByRole('link', { name: /Ayu - Kernel Blues/ })).toBeTruthy();
		expect(screen.getByText('track01.mp3')).toBeTruthy();
		expect(document.querySelector('audio')!.getAttribute('aria-label')).toBe('Play Ayu - Kernel Blues');
	});

	it('plays a voice note and fills its waveform as it plays', () => {
		const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(function (this: HTMLMediaElement) { this.dispatchEvent(new Event('play')); return Promise.resolve(); });
		const r = show(<MessageMedia msg={msg('voice', media({ duration: 10, wave: [0, 31, 15, 7] }))} onClick={false} />);
		const a = r.container.querySelector('audio')!;
		expect(a.getAttribute('preload')).toBe('none');
		const bars = r.container.querySelectorAll('.c-wave span');
		expect(bars).toHaveLength(4);
		expect((bars[1] as HTMLElement).style.height).toBe('22px');
		fireEvent.click(screen.getByRole('button', { name: 'Play' }));
		expect(play).toHaveBeenCalled();
		expect(screen.getByRole('button', { name: 'Pause' })).toBeTruthy();
		Object.defineProperty(a, 'currentTime', { value: 5, configurable: true });
		fireEvent.timeUpdate(a);
		expect([...bars].map((b) => b.className)).toEqual(['on', 'on', '', '']);
		expect(screen.getByText(/0:05 \/ 0:10/)).toBeTruthy();
		play.mockRestore();
	});

	it('cannot play a voice note that is not stored', () => {
		show(<MessageMedia msg={msg('voice', media({ stored: false, url: null, duration: 4 }))} onClick={false} />);
		expect((screen.getByRole('button', { name: 'Play' }) as HTMLButtonElement).disabled).toBe(true);
		expect(screen.getByText(/not stored/)).toBeTruthy();
	});

	it('gives an audio file a player and a download link', () => {
		show(<MessageMedia msg={msg('audio', media({ name: 'ep12.mp3', size: 1 << 20 }))} onClick={false} />);
		const a = document.querySelector('audio')!;
		expect(a.hasAttribute('controls')).toBe(true);
		expect(a.getAttribute('preload')).toBe('none');
		expect(screen.getByRole('link', { name: /ep12\.mp3/ }).getAttribute('href')).toBe(URL1);
	});

	it('links an unstored, fetchable document to Telegram', () => {
		show(<MessageMedia msg={msg('document', media({ stored: false, fetchable: true, name: 'dump.tar.gz', size: 4096 }))} onClick={false} />);
		const link = screen.getByRole('link', { name: /dump\.tar\.gz/ });
		expect(link.getAttribute('href')).toBe(URL1);
		expect(link.textContent).toContain('load from Telegram');
	});

	it('draws a mixed album: photos and videos in the grid, each item with its own deletion', () => {
		const items = [
			media({ kind: 'photo', thumb: URL1 }),
			media({ kind: 'video', url: '/files/tok-2', deleted_at: '2026-11-16T10:00:00Z' }),
			media({ kind: 'document', url: '/files/tok-3', name: 'notes.md' }),
		];
		const r = show(<MessageMedia msg={msg('photo', items[0]!, items.slice(1))} onClick={false} />);
		const grid = r.container.querySelector('.c-album')!;
		expect(grid.className).toContain('n2');
		expect(within(grid as HTMLElement).getByRole('link', { name: 'Photo' })).toBeTruthy();
		const video = within(grid as HTMLElement).getByRole('link', { name: 'Video' });
		expect(within(video).getByText('deleted')).toBeTruthy();
		expect(grid.contains(screen.getByRole('link', { name: /notes\.md/ }))).toBe(false);
		expect(r.container.querySelector('.c-albumwrap')!.getAttribute('aria-label')).toBe('album . 3 items');
	});
});
