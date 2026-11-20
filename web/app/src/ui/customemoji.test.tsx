// @vitest-environment happy-dom
// Custom emoji at text size, with the Lottie player and the page's
// IntersectionObserver stubbed.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';

const anim = { play: vi.fn(), pause: vi.fn(), destroy: vi.fn(), goToAndStop: vi.fn() };
const loadAnimation = vi.fn((_o: { container: HTMLElement }) => anim);
vi.mock('./lottie.ts', () => ({ loadPlayer: vi.fn(async () => ({ loadAnimation })), loadTgs: vi.fn(async () => ({ v: '5.5.2', layers: [] })) }));
const { loadTgs } = await import('./lottie.ts');

// One observer for the page, as the module makes; fire() says an
// element came into or went out of view.
let notify: ((e: { target: Element; isIntersecting: boolean }[]) => void) | null = null;
let made = 0;
vi.stubGlobal('IntersectionObserver', class {
	constructor(cb: typeof notify) { notify = cb; made++; }
	observe() {}
	unobserve() {}
	disconnect() {}
});
const fire = (el: Element, isIntersecting: boolean) => act(() => notify!([{ target: el, isIntersecting }]));
const settle = () => act(() => new Promise((r) => setTimeout(r, 0)));
const { CustomEmoji } = await import('./customemoji.tsx');

afterEach(() => { cleanup(); vi.clearAllMocks(); });
const emoji = (format: 'webp' | 'webm' | 'tgs' | null, url: string | null, needs_repainting = false) => ({ emoji: '🎉', format, url, needs_repainting });

describe('CustomEmoji', () => {
	it('draws the fallback emoji without a file or a known format, or while unknown', () => {
		const { container, rerender } = render(<p><CustomEmoji e={emoji('webp', null)} /></p>);
		expect(container.textContent).toBe('🎉');
		expect(container.querySelector('img, video, [role="img"]')).toBeNull();
		rerender(<p><CustomEmoji e={emoji(null, '/files/ce-x')} /></p>);
		expect(container.textContent).toBe('🎉');
		rerender(<p><CustomEmoji e={null} alt="😀"><b>😀</b></CustomEmoji></p>);
		expect(container.querySelector('b')!.textContent).toBe('😀');
	});

	it('draws a WebP emoji as an image with the emoji as its alt text, and the emoji if it fails', () => {
		const { container } = render(<p><CustomEmoji e={emoji('webp', '/files/ce-1')} alt="🙂" /></p>);
		const img = screen.getByRole('img', { name: '🙂' }) as HTMLImageElement;
		expect(img.tagName).toBe('IMG');
		expect(img.getAttribute('src')).toBe('/files/ce-1');
		expect(img.className).toContain('ce');
		fireEvent.error(img);
		expect(container.querySelector('img')).toBeNull();
		expect(container.textContent).toBe('🙂');
	});

	it('paints a WebP emoji that needs repainting in the text colour, through a mask', () => {
		render(<CustomEmoji e={emoji('webp', '/files/ce-2"x', true)} />);
		const el = screen.getByRole('img', { name: '🎉' });
		expect(el.tagName).toBe('SPAN');
		expect(el.className).toContain('ce-mask');
		expect(el.className).toContain('ce-repaint');
		expect(el.style.maskImage || el.style.getPropertyValue('-webkit-mask-image')).toContain('/files/ce-2\\"x');
	});

	it('draws a WebM emoji as a muted looping video that loads nothing until in view', () => {
		const play = vi.spyOn(HTMLMediaElement.prototype, 'play').mockImplementation(async () => {});
		const pause = vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {});
		const { container } = render(<CustomEmoji e={emoji('webm', '/files/ce-3', true)} />);
		const v = container.querySelector('video')!;
		expect(v.muted).toBe(true);
		expect(v.loop).toBe(true);
		expect(v.hasAttribute('playsinline')).toBe(true);
		expect(v.getAttribute('aria-label')).toBe('🎉');
		expect(v.className).toContain('ce-repaint');
		expect(v.preload).toBe('none');
		expect(play).not.toHaveBeenCalled();
		fire(v, true);
		expect(v.preload).toBe('auto');
		expect(play).toHaveBeenCalled();
		fire(v, false);
		expect(pause).toHaveBeenCalled();
		play.mockRestore();
		pause.mockRestore();
	});

	it('keeps a TGS emoji still, as its fallback, until it is in view, then plays it there', async () => {
		const { container } = render(<CustomEmoji e={emoji('tgs', '/files/ce-4.tgs')} />);
		const el = screen.getByRole('img', { name: '🎉' });
		expect(el.querySelector('.ce-alt')!.textContent).toBe('🎉');
		await settle();
		expect(loadTgs).not.toHaveBeenCalled();
		expect(loadAnimation).not.toHaveBeenCalled();
		fire(container.querySelector('.ce-box')!, true);
		await settle();
		expect(loadTgs).toHaveBeenCalledWith('/files/ce-4.tgs');
		expect(loadAnimation).toHaveBeenCalledWith(expect.objectContaining({ container: container.querySelector('.ce-box'), renderer: 'svg', loop: true, autoplay: false }));
		expect(anim.play).toHaveBeenCalled();
		expect(el.querySelector('.ce-alt')).toBeNull();
		expect(el.className).toContain('ready');
		fire(container.querySelector('.ce-box')!, false);
		expect(anim.pause).toHaveBeenCalled();
	});

	it('starts a TGS emoji under the pointer though it was never seen in view', async () => {
		render(<CustomEmoji e={emoji('tgs', '/files/ce-5.tgs')} />);
		const el = screen.getByRole('img', { name: '🎉' });
		fireEvent.mouseEnter(el);
		await settle();
		expect(loadAnimation).toHaveBeenCalled();
		expect(anim.play).toHaveBeenCalled();
		fireEvent.mouseLeave(el);
		expect(anim.pause).toHaveBeenCalled();
	});

	it('shares one observer and one download among many copies, and frees each', async () => {
		const many = Array.from({ length: 30 }, (_, i) => <CustomEmoji key={i} e={emoji('tgs', '/files/ce-6.tgs')} />);
		const { container, unmount } = render(<p>{many}</p>);
		for (const b of container.querySelectorAll('.ce-box')) fire(b, true);
		await settle();
		expect(made).toBe(1);
		expect(loadTgs).toHaveBeenCalledTimes(1);
		expect(loadAnimation).toHaveBeenCalledTimes(30);
		unmount();
		expect(anim.destroy).toHaveBeenCalledTimes(30);
	});

	it('keeps the fallback when the animation cannot be loaded', async () => {
		vi.mocked(loadTgs).mockRejectedValueOnce(new Error('sticker: HTTP 404'));
		const { container } = render(<CustomEmoji e={emoji('tgs', '/files/ce-7.tgs')} />);
		fire(container.querySelector('.ce-box')!, true);
		await settle();
		const el = screen.getByRole('img', { name: '🎉' });
		expect(el.querySelector('.ce-alt')!.textContent).toBe('🎉');
		expect(el.getAttribute('title')).toMatch(/could not be played/);
	});
});
