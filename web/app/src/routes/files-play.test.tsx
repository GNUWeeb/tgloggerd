// @vitest-environment happy-dom
// The files preview plays a file with the chat's own players: a video
// on a click, an animation and a .webm sticker inline, audio and voice
// with their players, a .tgs sticker with the Lottie player.
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';

vi.mock('../ui/lottie.ts', () => ({ playTgs: vi.fn(() => () => {}) }));
const { playTgs } = await import('../ui/lottie.ts');
import { fixture, renderPage, setup } from '../test/page.tsx';
import type { FilesProps } from './files.tsx';
import type { FileRow } from '../types.ts';

beforeEach(() => { setup(); vi.clearAllMocks(); });

const MINI = 'data:image/jpeg;base64,AA';
const base = () => fixture<FilesProps>('/files?view=list').props;
const row = (o: Partial<FileRow>): FileRow => ({ ...base().rows[0]!, stored: true, fetchable: false, duration: 30, thumb: null, minithumb: undefined, ...o });
// The preview of f, opened directly; "media on click" from the viewer's
// chat preference when asked.
const open = async (f: FileRow, onClick = false) => {
	const page = fixture<FilesProps>(`/files?view=list&file=${f.id}`, 'superadmin');
	const viewer = onClick ? { ...page.viewer, chat: { ...page.viewer.chat!, media: 'click' as const } } : page.viewer;
	renderPage(`/files?view=list&file=${f.id}`, 'superadmin', { props: { ...base(), rows: [f], preview: f }, viewer });
	const d = await screen.findByRole('dialog', { name: f.name });
	return d.querySelector('.a-preview') as HTMLElement;
};
const mediaViewer = () => screen.queryByRole('dialog', { name: 'Media viewer' });

describe('the files preview plays the file', () => {
	it('shows a video\'s poster with the play mark, loads none of it, and plays it in the viewer on a click', async () => {
		const f = row({ id: 4301, name: 'clip.mp4', ext: 'mp4', file_type: 'video', url: '/files/tok-p1', thumb: '/files/tok-p1-still', minithumb: MINI });
		const pane = await open(f);
		const box = within(pane).getByRole('link', { name: 'Video' });
		const v = box.querySelector('video')!;
		expect(v.getAttribute('poster')).toBe('/files/tok-p1-still');
		expect(v.getAttribute('src')).toBeNull();
		expect(v.getAttribute('preload')).toBe('none');
		expect(box.querySelector('.c-playbig')).toBeTruthy();
		// No element anywhere asks for the video's bytes before the click.
		expect(document.querySelector('[src="/files/tok-p1"]')).toBeNull();
		fireEvent.click(box);
		const played = mediaViewer()!.querySelector('video')!;
		expect(played.getAttribute('src')).toBe('/files/tok-p1');
		expect(played.hasAttribute('autoplay')).toBe(true);
		expect(played.hasAttribute('controls')).toBe(true);
		// Esc closes the viewer, and only it; then the preview.
		fireEvent.keyDown(window, { key: 'Escape' });
		expect(mediaViewer()).toBeNull();
		expect(screen.getByRole('dialog', { name: 'clip.mp4' })).toBeTruthy();
		fireEvent.keyDown(window, { key: 'Escape' });
		await waitFor(() => expect(screen.queryByRole('dialog', { name: 'clip.mp4' })).toBeNull());
	});

	it('plays a video note round, on a click', async () => {
		const f = row({ id: 4302, name: 'note.mp4', ext: 'mp4', file_type: 'video_note', url: '/files/tok-p2', thumb: '/files/tok-p2-still' });
		const pane = await open(f);
		const box = within(pane).getByRole('link', { name: 'Video' });
		expect(box.className).toContain('round');
		expect(box.querySelector('video')!.getAttribute('src')).toBeNull();
		fireEvent.click(box);
		expect(mediaViewer()!.querySelector('video')!.getAttribute('src')).toBe('/files/tok-p2');
	});

	it('plays an animation inline, muted and looping, over its poster', async () => {
		const f = row({ id: 4303, name: 'fun.mp4', ext: 'mp4', file_type: 'animation', url: '/files/tok-p3', thumb: '/files/tok-p3-still' });
		const pane = await open(f);
		const v = within(pane).getByRole('link', { name: 'Animation' }).querySelector('video') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe('/files/tok-p3');
		expect(v.hasAttribute('autoplay')).toBe(true);
		expect(v.loop).toBe(true);
		expect(v.muted).toBe(true);
		expect(v.getAttribute('poster')).toBe('/files/tok-p3-still');
	});

	it('keeps an animation still until clicked when media load on click', async () => {
		const f = row({ id: 4304, name: 'fun2.mp4', ext: 'mp4', file_type: 'animation', url: '/files/tok-p4', thumb: '/files/tok-p4-still', minithumb: MINI });
		const pane = await open(f, true);
		const v = within(pane).getByRole('link', { name: 'Animation' }).querySelector('video')!;
		expect(v.getAttribute('src')).toBeNull();
		expect(v.hasAttribute('autoplay')).toBe(false);
	});

	it('plays a .webm sticker inline, muted and looping', async () => {
		const f = row({ id: 4305, name: 'cat.webm', ext: 'webm', file_type: 'sticker', url: '/files/tok-p5', thumb: '/files/tok-p5-still' });
		const pane = await open(f);
		const v = pane.querySelector('video.c-sticker') as HTMLVideoElement;
		expect(v.getAttribute('src')).toBe('/files/tok-p5');
		expect(v.hasAttribute('autoplay')).toBe(true);
		expect(v.loop).toBe(true);
		expect(v.muted).toBe(true);
		expect(within(screen.getByRole('dialog', { name: 'cat.webm' })).queryByRole('button', { name: 'View bigger' })).toBeNull();
	});

	it('plays a .tgs sticker with the Lottie player', async () => {
		const f = row({ id: 4306, name: 'duck.tgs', ext: 'tgs', file_type: 'sticker', url: '/files/tok-p6.tgs' });
		const pane = await open(f);
		expect(pane.querySelector('.c-lottie')).toBeTruthy();
		expect(playTgs).toHaveBeenCalledWith(pane.querySelector('.c-lottie-box'), '/files/tok-p6.tgs', expect.any(Function), expect.any(Function));
	});

	it('plays a voice note with the chat\'s player, fetching nothing before the first play', async () => {
		const f = row({ id: 4307, name: 'voice.ogg', ext: 'ogg', file_type: 'voice', url: '/files/tok-p7', duration: 7 });
		const pane = await open(f);
		const voice = pane.querySelector('.c-voice')!;
		const audio = voice.querySelector('audio')!;
		expect(audio.getAttribute('src')).toBe('/files/tok-p7');
		expect(audio.getAttribute('preload')).toBe('none');
		expect(voice.querySelectorAll('.c-wave span').length).toBeGreaterThan(0);
		const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined);
		fireEvent.click(within(voice as HTMLElement).getByRole('button', { name: 'Play' }));
		expect(play).toHaveBeenCalled();
		play.mockRestore();
	});

	it('plays music with a player beside its download link', async () => {
		const f = row({ id: 4308, name: 'song.mp3', ext: 'mp3', file_type: 'audio', url: '/files/tok-p8' });
		const pane = await open(f);
		const audio = pane.querySelector('audio.c-audio')!;
		expect(audio.getAttribute('src')).toBe('/files/tok-p8');
		expect(audio.hasAttribute('controls')).toBe(true);
		expect(audio.getAttribute('preload')).toBe('none');
	});

	it('offers to load a fetchable video from Telegram on a click, and keeps the not-stored note', async () => {
		const f = row({ id: 4309, name: 'far.mp4', ext: 'mp4', file_type: 'video', stored: false, fetchable: true, url: '/files/tok-p9', minithumb: MINI });
		const pane = await open(f);
		const box = within(pane).getByRole('link', { name: 'Video' });
		expect(within(box).getByText('Load from Telegram')).toBeTruthy();
		expect(box.querySelector('video')!.getAttribute('src')).toBeNull();
		expect(pane.querySelector('.a-preview-missing')).toBeTruthy();
		expect(screen.getByRole('link', { name: /Fetch from Telegram/ }).getAttribute('href')).toBe('/files/tok-p9?download=1');
	});

	it('still shows a photo whole and a document by its icon, with the Download button', async () => {
		const photo = row({ id: 4310, name: 'pic.jpg', ext: 'jpg', file_type: 'photo', url: '/files/tok-p10', thumb: '/files/tok-p10' });
		let pane = await open(photo);
		expect(pane.querySelector('img.a-preview-img')!.getAttribute('src')).toBe('/files/tok-p10');
		expect(pane.querySelector('video, audio')).toBeNull();
		document.body.innerHTML = '';
		const doc = row({ id: 4311, name: 'notes.pdf', ext: 'pdf', file_type: 'document', render: 'pdf', url: '/files/tok-p11' });
		pane = await open(doc);
		expect(pane.querySelector('.a-thumb-document')).toBeTruthy();
		expect(screen.getByRole('link', { name: /Download/ }).getAttribute('href')).toBe('/files/tok-p11?download=1');
	});
});
