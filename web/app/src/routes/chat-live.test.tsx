// @vitest-environment happy-dom
// The chat's live events for messages inside the window: edits and
// deletions update the message in place.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, configure, screen, waitFor } from '@testing-library/react';
import { callsTo, fakeApi, fixture, renderPage, setup, type Call } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
configure({ asyncUtilTimeout: 5000 });

// A stand-in for the browser's EventSource: the page's live stream.
class FakeSource {
	static last: FakeSource | null = null;
	readyState = 0;
	onopen: ((ev: Event) => unknown) | null = null;
	onerror: ((ev: Event) => unknown) | null = null;
	private ls = new Map<string, ((ev: { data: string }) => void)[]>();
	constructor(public url: string) { FakeSource.last = this; }
	addEventListener(kind: string, fn: (ev: { data: string }) => void) { this.ls.set(kind, [...(this.ls.get(kind) ?? []), fn]); }
	removeEventListener() {}
	close() { this.readyState = 2; }
	open() { this.readyState = 1; this.onopen?.(new Event('open')); }
	emit(e: Record<string, unknown>) { for (const fn of this.ls.get(String(e.kind)) ?? []) fn({ data: JSON.stringify(e) }); }
}

beforeEach(() => {
	FakeSource.last = null;
	vi.stubGlobal('EventSource', FakeSource);
});
afterEach(() => vi.restoreAllMocks());

const gid = () => fixture<GroupsProps>('/groups').props.rows[0]!.id;
const url = () => `/groups/${gid()}/chat`;
const windowCalls = (calls: Call[]) => callsTo(calls, 'GET', /^\/api\/chats\/group\/-?\d+\/messages$/);
const around = (calls: Call[]) => windowCalls(calls).filter((c) => c.query.get('around'));

// The chat's whole window, as the server has it now.
const full = () => fixture<ChatProps>(url()).props.messages;
// The ids of the window's rows, in DOM order.
const order = () => [...document.querySelectorAll('.c-feed > [id^="m"]')].map((x) => Number(x.id.slice(1)));

// Answers "around" from the whole window as the server would; the tail
// has nothing newer.
function server(msgs: () => ChatMessage[]) {
	return fakeApi('superadmin', {
		'GET /api/chats/group/:id/messages': (c) => {
			const all = msgs();
			const a = c.query.get('around');
			if (a) {
				const i = all.findIndex((m) => m.message_id >= Number(a));
				const lim = Number(c.query.get('limit'));
				const from = Math.max(0, i - Math.floor(lim / 2));
				return { messages: all.slice(from, from + lim), has_older: true, has_newer: true };
			}
			return { messages: [], has_older: true, has_newer: false };
		},
	});
}

// The page opened, without some of its messages if given, and its
// live stream open.
async function open(missing: number[] = []) {
	const all = full();
	const shown = all.filter((m) => !missing.includes(m.message_id));
	const page = fixture<ChatProps>(url());
	renderPage(url(), 'superadmin', { props: { ...page.props, messages: shown } });
	await waitFor(() => expect(FakeSource.last?.url).toMatch(/chat%3A|chat:/));
	act(() => FakeSource.last!.open());
	return FakeSource.last!;
}

describe('edits and deletions in the window', () => {
	it('updates an edited message in place', async () => {
		const all = full();
		const m = all[15]!;
		const edited = () => full().map((x) => (x.id === m.id ? { ...x, content_type: 'text' as const, text: 'changed on recheck', media: null, album: [], detail: null, service: null, edited_at: x.date } : x));
		const calls = server(edited);
		const s = await open();
		act(() => s.emit({ seq: 1, kind: 'message.edited', chat_id: gid(), message_id: m.message_id }));
		await screen.findByText('changed on recheck');
		const o = order();
		expect(o.indexOf(m.message_id)).toBe(all.findIndex((x) => x.message_id === m.message_id));
		expect(around(calls)[0]!.query.get('around')).toBe(String(m.message_id));
	});

	it('marks a deleted message in place', async () => {
		const all = full();
		const m = all.find((x, i) => i > 10 && !x.deleted_at && x.content_type !== 'service')!;
		const gone = () => full().map((x) => (x.id === m.id ? { ...x, deleted_at: x.date } : x));
		server(gone);
		const s = await open();
		const row = () => document.getElementById('m' + m.message_id)!;
		expect(row().querySelector('.c-deltag')).toBeNull();
		act(() => s.emit({ seq: 1, kind: 'message.deleted', chat_id: gid(), message_id: m.message_id }));
		await waitFor(() => expect(row().querySelector('.c-deltag')).toBeTruthy());
		expect(order()).toHaveLength(all.length);
	});
});
