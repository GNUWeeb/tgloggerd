// @vitest-environment happy-dom
// A chat opened before its newest messages (the oldest, a message, a
// date): scrolling down loads the messages after the window, page by
// page, until the newest, and then the view follows the live tail.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup, type Call } from '../test/page.tsx';
import { FORUM_GROUP, TOPICS } from '../../standin/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
afterEach(() => vi.restoreAllMocks());

const gid = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const windowCalls = (calls: Call[]) => callsTo(calls, 'GET', /^\/api\/chats\/group\/-?\d+\/messages$/);
const after = (calls: Call[]) => windowCalls(calls).filter((c) => c.query.get('after'));

// The scroller as a browser lays it out: 5000 px of messages in an
// 800 px view, scrolled where a test puts it.
function layout() {
	let top = 0;
	vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? 5000 : 0; });
	vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? 800 : 0; });
	vi.spyOn(HTMLElement.prototype, 'scrollTop', 'get').mockImplementation(function (this: HTMLElement) { return this.classList.contains('c-scroll') ? top : 0; });
	vi.spyOn(HTMLElement.prototype, 'scrollTop', 'set').mockImplementation(function (this: HTMLElement, v: number) { if (this.classList.contains('c-scroll')) top = v; });
	const el = () => document.querySelector<HTMLElement>('.c-scroll')!;
	return { scrollTo: (y: number) => { top = y; fireEvent.scroll(el()); }, top: () => top };
}

const later = (last: ChatMessage, n: number, from = 1): ChatMessage[] => Array.from({ length: n }, (_, i) => ({ ...last, id: 50_000_000 - (from + i) * 7, message_id: last.message_id + from + i, text: `later message ${from + i}`, content_type: 'text', media: null, album: [], detail: null, service: null }));

describe('loading newer messages', () => {
	it('appends the messages after the window when the reader nears the bottom, and keeps its place', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const page = fixture<ChatProps>(url);
		expect(page.props.has_newer).toBe(true);
		const last = page.props.messages.at(-1)!;
		// Row ids out of order, as later backfills leave them: the cursor
		// is the message id.
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': (c) => ({ messages: c.query.get('after') === String(last.message_id) ? later(last, 2) : [], has_older: true, has_newer: true }) });
		const s = layout();
		renderPage(url);
		s.scrollTo(2000);
		expect(after(calls)).toHaveLength(0);
		s.scrollTo(4000);
		await screen.findByText('later message 2');
		expect(after(calls)[0]!.query.get('after')).toBe(String(last.message_id));
		expect(s.top()).toBe(4000);
		expect(screen.getByText('Loading newer messages...')).toBeTruthy();
	});

	it('stops at the newest messages and then follows the live tail', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const last = fixture<ChatProps>(url).props.messages.at(-1)!;
		let n = 0;
		const calls = fakeApi('superadmin', {
			'GET /api/chats/group/:id/messages': (c) => {
				const a = Number(c.query.get('after'));
				// One page up to the newest; then the live tail finds one more.
				if (a === last.message_id) return { messages: later(last, 3), has_older: true, has_newer: false };
				if (a === last.message_id + 3 && n++ === 0) return { messages: later(last, 1, 4), has_older: true, has_newer: false };
				return { messages: [], has_older: true, has_newer: false };
			},
		});
		const s = layout();
		renderPage(url);
		s.scrollTo(4100);
		await screen.findByText('later message 3');
		expect(screen.queryByText('Loading newer messages...')).toBeNull();
		// Without a stream the tail polls every second from the newest.
		await screen.findByText('later message 4', {}, { timeout: 2500 });
		expect(after(calls).some((c) => c.query.get('after') === String(last.message_id + 3))).toBe(true);
	});

	it('keeps to the forum topic it shows', async () => {
		const t = TOPICS[1]!;
		const url = `/groups/${FORUM_GROUP.id}/chat?topic=${t.id}&after=0`;
		const page = fixture<ChatProps>(url);
		expect(page.props.has_newer).toBe(true);
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: true }) });
		const s = layout();
		renderPage(url);
		s.scrollTo(4200);
		await waitFor(() => expect(after(calls)).toHaveLength(1));
		expect(after(calls)[0]!.query.get('topic')).toBe(String(t.id));
	});

	it('waits for a click when loading while scrolling is off', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const page = fixture<ChatProps>(url);
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: true }) });
		const s = layout();
		renderPage(url, 'superadmin', { viewer: { ...page.viewer, chat: { ...page.viewer.chat, auto_load: false } } });
		s.scrollTo(4200);
		expect(after(calls)).toHaveLength(0);
		fireEvent.click(screen.getByRole('button', { name: 'Load newer messages' }));
		await waitFor(() => expect(after(calls)).toHaveLength(1));
	});

	it('loads on by itself when the window is too short to scroll', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => ({ messages: [], has_older: true, has_newer: false }) });
		renderPage(url);
		await waitFor(() => expect(after(calls)).toHaveLength(1));
	});

	it('keeps the window as it is when a load fails, and tries again on the next scroll', async () => {
		const url = `/groups/${gid()}/chat?after=0`;
		const calls = fakeApi('superadmin', { 'GET /api/chats/group/:id/messages': () => json(503, { error: 'unavailable' }) });
		const s = layout();
		renderPage(url);
		s.scrollTo(4100);
		await waitFor(() => expect(after(calls)).toHaveLength(1));
		await new Promise((r) => setTimeout(r, 20));
		expect(screen.getByText('Loading newer messages...')).toBeTruthy();
		s.scrollTo(4150);
		await waitFor(() => expect(after(calls)).toHaveLength(2));
	});
});
