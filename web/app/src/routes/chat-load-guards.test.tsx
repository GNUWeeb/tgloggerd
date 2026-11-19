// @vitest-environment happy-dom
// The chat's window loads belong to the window that asked for them: an
// answer that comes back after the page moved to another window (a
// jump, the oldest, another chat) is dropped, and it does not hold up
// the new window's own loads. A window too short to scroll loads on by
// itself, but not again after a page that added nothing.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup, type Call } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
afterEach(() => vi.restoreAllMocks());

const gid = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const windowCalls = (calls: Call[]) => callsTo(calls, 'GET', /^\/api\/chats\/group\/-?\d+\/messages$/);
const after = (calls: Call[]) => windowCalls(calls).filter((c) => c.query.get('after'));
const before = (calls: Call[]) => windowCalls(calls).filter((c) => c.query.get('before'));
const pages = (calls: Call[]) => callsTo(calls, 'GET', /^\/api\/page\//);
const tick = (ms = 30) => new Promise((r) => setTimeout(r, ms));

// The scroller as a browser lays it out: 5000 px of messages in an
// 800 px view.
function layout() {
	let top = 0;
	vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? 5000 : 0; });
	vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? 800 : 0; });
	vi.spyOn(HTMLElement.prototype, 'scrollTop', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? top : 0; });
	vi.spyOn(HTMLElement.prototype, 'scrollTop', 'set').mockImplementation(function (this: HTMLElement, v: number) { if (this.classList.contains('c-scroll')) top = v; });
	const el = () => document.querySelector<HTMLElement>('.c-scroll')!;
	return { scrollTo: (y: number) => { top = y; fireEvent.scroll(el()); } };
}

const later = (last: ChatMessage, n: number): ChatMessage[] => Array.from({ length: n }, (_, i) => ({ ...last, id: 50_000_000 - (i + 1) * 7, message_id: last.message_id + 1 + i, text: `stale message ${i + 1}`, content_type: 'text', media: null, album: [], detail: null, service: null }));

// A promise the test settles when it chooses.
function held<T>() {
	let release!: (v: T) => void;
	const p = new Promise<T>((r) => { release = r; });
	return { p, release };
}

describe('window loads', () => {
	it('drops a newer page that answers after the window was replaced, and lets the new window load', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const last = fixture<ChatProps>(url).props.messages.at(-1)!;
		const latest = fixture<ChatProps>(`/groups/${gid()}/chat`).props;
		expect(latest.has_older).toBe(true);
		expect(latest.has_newer).toBe(false);
		const h = held<void>();
		const calls = fakeApi('superadmin', {
			'GET /api/chats/group/:id/messages': async (c) => {
				if (c.query.get('after') === String(last.message_id)) {
					await h.p;
					return { messages: later(last, 2), has_older: true, has_newer: true };
				}
				return { messages: [], has_older: true, has_newer: false };
			},
		});
		const s = layout();
		renderPage(url);
		s.scrollTo(4100);
		await waitFor(() => expect(after(calls)).toHaveLength(1));
		// Jump to the newest while that page is still on its way.
		fireEvent.click(screen.getByRole('button', { name: 'Jump to latest' }));
		await waitFor(() => expect(pages(calls)).toHaveLength(1));
		await waitFor(() => expect(document.getElementById(`m${latest.messages.at(-1)!.message_id}`)).toBeTruthy());
		expect(screen.queryByText('Loading newer messages...')).toBeNull();
		// The new window's own load is not held up by the old one.
		s.scrollTo(100);
		await waitFor(() => expect(before(calls)).toHaveLength(1));
		expect(before(calls)[0]!.query.get('before')).toBe(String(latest.messages[0]!.message_id));
		h.release();
		await tick();
		expect(screen.queryByText('stale message 1')).toBeNull();
		expect(screen.queryByText('Loading newer messages...')).toBeNull();
	});

	it('drops an older page that answers after the window was replaced', async () => {
		const url = `/groups/${gid()}/chat`;
		const first = fixture<ChatProps>(url).props.messages[0]!;
		const h = held<void>();
		const calls = fakeApi('superadmin', {
			'GET /api/chats/group/:id/messages': async (c) => {
				if (c.query.get('before') === String(first.message_id)) {
					await h.p;
					return { messages: later({ ...first, message_id: first.message_id - 10 }, 2), has_older: true, has_newer: true };
				}
				return { messages: [], has_older: true, has_newer: false };
			},
		});
		const s = layout();
		renderPage(url);
		s.scrollTo(100);
		await waitFor(() => expect(before(calls)).toHaveLength(1));
		fireEvent.click(screen.getByRole('button', { name: 'Jump to the oldest messages' }));
		await waitFor(() => expect(pages(calls)).toHaveLength(1));
		await waitFor(() => expect(screen.getByText('Beginning of the archived history')).toBeTruthy());
		h.release();
		await tick();
		expect(screen.queryByText('stale message 1')).toBeNull();
		// The stale answer said there is more; the oldest window still ends here.
		expect(screen.getByText('Beginning of the archived history')).toBeTruthy();
	});

	it('drops a live tail answer that comes back after the window was replaced', async () => {
		const url = `/groups/${gid()}/chat`;
		const last = fixture<ChatProps>(url).props.messages.at(-1)!;
		const h = held<void>();
		const calls = fakeApi('superadmin', {
			'GET /api/chats/group/:id/messages': async (c) => {
				if (c.query.get('after') === String(last.message_id)) {
					await h.p;
					return { messages: later(last, 2), has_older: true, has_newer: false };
				}
				return { messages: [], has_older: true, has_newer: true };
			},
		});
		layout();
		renderPage(url);
		// Without a stream the tail polls every second.
		await waitFor(() => expect(after(calls).length).toBeGreaterThan(0), { timeout: 2500 });
		fireEvent.click(screen.getByRole('button', { name: 'Jump to the oldest messages' }));
		await waitFor(() => expect(pages(calls)).toHaveLength(1));
		await waitFor(() => expect(screen.getByText('Beginning of the archived history')).toBeTruthy());
		h.release();
		await tick();
		expect(screen.queryByText('stale message 1')).toBeNull();
	});

	it('does not ask again by itself after a short window got an empty page', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		// Answered after a moment, so that a load that keeps asking runs at
		// a pace the test can count instead of starving it.
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': async () => { await tick(10); return { messages: [], has_older: true, has_newer: true }; } });
		renderPage(url);
		await waitFor(() => expect(after(calls)).toHaveLength(1));
		await tick(150);
		expect(after(calls)).toHaveLength(1);
	});
});
