// @vitest-environment happy-dom
// A deletion the daemon found by rechecking the chat against Telegram,
// rather than one it saw live, reads as found on recheck wherever a
// deletion shows, and its time as when it was found; a live deletion
// keeps its old words.
import { beforeEach, describe, expect, it } from 'vitest';
import { cleanup, render, screen, within } from '@testing-library/react';
import { fixture, renderPage, setup } from '../test/page.tsx';
import { MessageMedia, type Media } from './chat-media.tsx';
import { PLAIN_KINDS } from '../test/kinds.ts';
import type { ChatProps } from './chat.tsx';
import type { DashboardProps } from './dashboard.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessageProps } from './message.tsx';
import type { MessagesProps } from './messages.tsx';
import type { ChatMessage } from '../types.ts';

beforeEach(setup);
const AT = '2026-11-16T10:00:00Z';
const variants = [
	{ name: 'live', flag: {}, tag: 'deleted', title: `Deleted ${AT}` },
	{ name: 'found on recheck', flag: { deleted_on_recheck: true }, tag: 'deleted (found on recheck)', title: `Found deleted on recheck at ${AT}` },
];

describe.each(variants)('a $name deletion', ({ flag, tag, title }) => {
	const recheck = 'deleted_on_recheck' in flag;

	it('in the chat: the bubble and a service line', () => {
		const url = `/groups/${fixture<GroupsProps>('/groups').props.rows[0]!.id}/chat`;
		const p = fixture<ChatProps>(url).props;
		const t = p.messages.find((m) => m.content_type === 'text')!;
		const s = p.messages.find((m) => m.content_type === 'service')!;
		const messages = [
			{ ...t, id: 1e9, message_id: 9e5, reply_to: null, link_preview: null, edits: [], deleted_at: AT, ...flag },
			{ ...s, id: 1e9 + 1, message_id: 9e5 + 1, deleted_at: AT, ...flag },
		] as ChatMessage[];
		renderPage(url, 'superadmin', { props: { ...p, messages, at: null } });
		for (const id of [9e5, 9e5 + 1]) {
			const mark = document.getElementById(`m${id}`)!.querySelector('.c-deltag')!;
			expect(mark.textContent).toBe(tag);
			expect(mark.getAttribute('title')).toBe(title);
		}
		cleanup();
	});

	it('on an album item', () => {
		const item = (o: Partial<Media>): Media => ({ stored: true, thumb: null, url: '/files/tok-1', kind: 'photo', ...o });
		const items = [item({}), item({ url: '/files/tok-2', deleted_at: AT, ...flag })];
		const msg = {
			id: 1, message_id: 1, date: AT, scope: 'group', sender: null, content_type: 'photo', text: '', media: items[0]!, album: items.slice(1),
			reply_to: null, forward: null, edits: [], edited_at: null, deleted_at: null, link_preview: null, ...PLAIN_KINDS,
		} as ChatMessage;
		const r = render(<div className="app"><MessageMedia msg={msg} onClick={false} /></div>);
		const mark = r.container.querySelector('.c-itemdel')!;
		expect(mark.textContent).toBe(tag);
		expect(mark.getAttribute('title')).toBe(title);
		cleanup();
	});

	it('on the message page: the flag, the banner and the time', () => {
		const url = `/messages/group/${fixture<MessagesProps>('/group_messages').props.rows[0]!.id}`;
		const p = fixture<MessageProps>(url).props;
		renderPage(url, 'superadmin', { props: { message: { ...p.message, deleted_at: AT, ...flag } } });
		expect(screen.getByText(tag)).toBeTruthy();
		const banner = document.querySelector('.a-banner.warn')!.textContent!;
		expect(banner.startsWith(recheck ? 'Found deleted on Telegram by a recheck at ' : 'Deleted on Telegram at ')).toBe(true);
		expect(screen.getByText(recheck ? 'Found deleted' : 'Deleted')).toBeTruthy();
		expect(screen.queryByText(recheck ? 'Deleted' : 'Found deleted')).toBeNull();
		cleanup();
	});

	it('in the message lists', () => {
		const p = fixture<MessagesProps>('/group_messages').props;
		const rows = p.rows.map((m, i) => ({ ...m, deleted_at: i === 0 ? AT : null, deleted_on_recheck: i === 0 && recheck ? true : undefined }));
		renderPage('/group_messages', 'superadmin', { props: { ...p, rows } });
		const marks = [...document.querySelectorAll('.a-del')];
		expect(marks.length).toBeGreaterThan(0);
		for (const m of marks) expect(m.getAttribute('title')).toBe(recheck ? 'Deleted (found on recheck)' : 'Deleted');
		cleanup();
	});

	it('in the dashboard\'s recently deleted list', () => {
		const p = fixture<DashboardProps>('/').props;
		const d = p.deleted[0]!;
		const deleted = [{ ...d, deleted_at: AT, deleted_on_recheck: recheck ? true : undefined }];
		renderPage('/', 'superadmin', { props: { ...p, deleted } });
		const li = document.querySelector('.a-dellist li') as HTMLElement;
		const when = within(li).getByText(/ago$/);
		expect(when.textContent!.startsWith('found on recheck ')).toBe(recheck);
		expect(when.getAttribute('title')?.startsWith('Found deleted on recheck at ') ?? false).toBe(recheck);
		cleanup();
	});
});
