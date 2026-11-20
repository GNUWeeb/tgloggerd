// The .tgs loader and player, with lottie-web replaced by a stub: the
// real one needs a browser's SVG, which the browser suite covers.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { gzipSync } from 'node:zlib';

const anim = { play: vi.fn(), pause: vi.fn(), destroy: vi.fn(), goToAndStop: vi.fn() };
const loadAnimation = vi.fn(() => anim);
vi.mock('lottie-web/build/player/esm/lottie_light.min.js', () => ({ default: { loadAnimation } }));
const { loadTgs, playTgs } = await import('./lottie.ts');

const DATA = { v: '5.5.2', fr: 60, w: 512, h: 512, layers: [] };
const serve = (body: Uint8Array | string, status = 200) => vi.stubGlobal('fetch', vi.fn(async () => new Response(body as BodyInit, { status })));
const el = {} as HTMLElement;
const settle = () => new Promise((r) => setTimeout(r, 0));

afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks(); });

describe('loadTgs', () => {
	it('unpacks a gzipped sticker, as Telegram stores them', async () => {
		serve(gzipSync(JSON.stringify(DATA)));
		expect(await loadTgs('/files/t')).toEqual(DATA);
	});

	it('reads one already unpacked', async () => {
		serve(JSON.stringify(DATA));
		expect(await loadTgs('/files/t')).toEqual(DATA);
	});

	it('fails on a refused file', async () => {
		serve('{}', 404);
		await expect(loadTgs('/files/t')).rejects.toThrow('404');
	});
});

describe('playTgs', () => {
	it('draws the sticker as SVG, looping, and frees it when stopped', async () => {
		serve(gzipSync(JSON.stringify(DATA)));
		const ready = vi.fn();
		const stop = playTgs(el, '/files/t', ready, () => {});
		await vi.waitFor(() => expect(ready).toHaveBeenCalled());
		expect(loadAnimation).toHaveBeenCalledWith({ container: el, renderer: 'svg', loop: true, autoplay: true, animationData: DATA });
		stop();
		expect(anim.destroy).toHaveBeenCalled();
	});

	it('waits until the sticker is in view, and pauses it out of view', async () => {
		serve(gzipSync(JSON.stringify(DATA)));
		let seen: (e: { isIntersecting: boolean }[]) => void = () => {};
		vi.stubGlobal('IntersectionObserver', class { constructor(cb: typeof seen) { seen = cb; } observe() {} disconnect() {} });
		const ready = vi.fn();
		playTgs(el, '/files/t', ready, () => {});
		await settle();
		expect(fetch).not.toHaveBeenCalled();
		seen([{ isIntersecting: true }]);
		await vi.waitFor(() => expect(ready).toHaveBeenCalled());
		seen([{ isIntersecting: false }]);
		expect(anim.pause).toHaveBeenCalled();
		seen([{ isIntersecting: true }]);
		expect(anim.play).toHaveBeenCalled();
		expect(fetch).toHaveBeenCalledTimes(1);
	});

	it('keeps to the first frame for a viewer who asked for reduced motion', async () => {
		serve(gzipSync(JSON.stringify(DATA)));
		vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce') }));
		const ready = vi.fn();
		playTgs(el, '/files/t', ready, () => {});
		await vi.waitFor(() => expect(ready).toHaveBeenCalled());
		expect(loadAnimation).toHaveBeenCalledWith(expect.objectContaining({ autoplay: false }));
		expect(anim.goToAndStop).toHaveBeenCalledWith(0, true);
	});

	it('reports a sticker it cannot read', async () => {
		serve('not json');
		const failed = vi.fn();
		playTgs(el, '/files/t', () => {}, failed);
		await vi.waitFor(() => expect(failed).toHaveBeenCalled());
		expect(loadAnimation).not.toHaveBeenCalled();
	});
});
