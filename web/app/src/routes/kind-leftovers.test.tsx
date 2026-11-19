// @vitest-environment happy-dom
// The kinds' smaller gaps (docs/todo/msg-kind-leftovers.md): what a card
// says when its detail is thin or missing, and the fields each card
// shows.
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MessageMedia, kindOf, type Media } from './chat-media.tsx';
import { PLAIN_KINDS } from '../test/kinds.ts';
import { TypeIcon } from '../ui/media.tsx';
import type { ChatMessage } from '../types.ts';

afterEach(cleanup);

const show = (el: ReactElement) => render(<div className="app">{el}</div>);
const msg = (content_type: ChatMessage['content_type'], m: Media | null): ChatMessage => ({
	id: 1, message_id: 1, date: '2026-11-17T10:00:00Z', scope: 'group', sender: null, content_type, text: '',
	media: m, album: [], reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null, ...PLAIN_KINDS,
});

describe('a video note', () => {
	it('draws as a round video even when its media has no render', () => {
		const m: Media = { stored: true, thumb: null, url: '/files/tok-note', duration: 9 };
		expect(kindOf('video_note', m)).toBe('video');
		show(<MessageMedia msg={msg('video_note', m)} onClick={false} />);
		const note = screen.getByRole('link', { name: 'Video' });
		expect(note.className).toContain('round');
		expect(note.getAttribute('style')).toContain('aspect-ratio: 1 / 1');
	});
});

describe('the icons', () => {
	it('give an unknown kind its own icon, not the document\'s', () => {
		const cls = (el: ReactElement) => { const r = show(el); const c = r.container.querySelector('svg')!.getAttribute('class'); r.unmount(); return c; };
		const unknown = cls(<TypeIcon type="unknown" />);
		expect(unknown).not.toBe(cls(<TypeIcon type="document" />));
		expect(unknown).not.toBe(cls(<TypeIcon type="unsupported" />));
	});
});
