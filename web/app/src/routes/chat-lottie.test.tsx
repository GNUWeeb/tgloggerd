// @vitest-environment happy-dom
// An animated (.tgs) sticker in the chat, with the player stubbed.
import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';

let done: { ready: () => void; failed: () => void } | null = null;
const stop = vi.fn();
vi.mock('../ui/lottie.ts', () => ({ playTgs: vi.fn((_el: HTMLElement, _url: string, ready: () => void, failed: () => void) => { done = { ready, failed }; return stop; }) }));
const { MessageMedia } = await import('./chat-media.tsx');
const { playTgs } = await import('../ui/lottie.ts');
import type { ChatMessage } from '../types.ts';

afterEach(() => { cleanup(); done = null; vi.clearAllMocks(); });
const msg = (url: string | null): ChatMessage => ({
	id: 1, message_id: 1, date: '2026-11-17T10:00:00Z', scope: 'group', sender: null, content_type: 'sticker', text: '',
	media: { stored: !!url, thumb: null, url, format: 'tgs', render: 'lottie', emoji: '🙏' }, album: [], reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null,
});

describe('animated stickers', () => {
	it('shows the emoji until the animation is drawn, then only the animation', () => {
		const { container, unmount } = render(<MessageMedia msg={msg('/files/tgs-1')} onClick={false} />);
		expect(playTgs).toHaveBeenCalledWith(container.querySelector('.c-lottie-box'), '/files/tgs-1', expect.any(Function), expect.any(Function));
		expect(screen.getByRole('img', { name: 'sticker 🙏' }).textContent).toBe('🙏');
		act(() => done!.ready());
		expect(screen.getByRole('img', { name: 'sticker 🙏' }).textContent).toBe('');
		unmount();
		expect(stop).toHaveBeenCalled();
	});

	it('keeps the emoji when the animation cannot be played', () => {
		render(<MessageMedia msg={msg('/files/tgs-1')} onClick={false} />);
		act(() => done!.failed());
		const s = screen.getByRole('img', { name: 'sticker 🙏' });
		expect(s.textContent).toBe('🙏');
		expect(s.getAttribute('title')).toMatch(/could not be played/);
	});

	it('does not try to play a sticker whose file is not stored', () => {
		render(<MessageMedia msg={msg(null)} onClick={false} />);
		expect(playTgs).not.toHaveBeenCalled();
		expect(screen.getByRole('img', { name: 'sticker 🙏' }).textContent).toBe('🙏');
	});
});
