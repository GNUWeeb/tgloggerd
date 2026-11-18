// @vitest-environment happy-dom
// The same message, shown to accounts in different timezones and
// formats: each sees it at its own local time, on the chat and on the
// message's own page. The server sends UTC; only the page converts it.
import { beforeEach, describe, expect, it } from 'vitest';
import { fakeApi, fixture, renderPage, setup } from '../test/page.tsx';
import type { ChatProps } from './chat.tsx';
import type { GroupsProps } from './groups.tsx';
import type { MessagesProps } from './messages.tsx';
import type { Viewer } from '../page.ts';

beforeEach(setup);

// 23:30 UTC: already the next morning in Jakarta, the evening before in
// New York (UTC-5 in November).
const AT = '2026-11-17T23:30:00Z';
const ZONES: [Partial<Viewer>, string, string][] = [
	[{ timezone: 'Asia/Jakarta', date_format: '24h' }, '06:30', '18 Nov 2026, 6:30'],
	[{ timezone: 'America/New_York', date_format: '24h' }, '18:30', '17 Nov 2026, 18:30'],
	[{ timezone: 'America/New_York', date_format: '12h' }, '06:30 pm', '17 Nov 2026, 6:30 pm'],
	[{ timezone: 'UTC', date_format: 'iso' }, '23:30', '2026-11-17 23:30'],
];

describe('local times', () => {
	it.each(ZONES)('the chat shows a message at the viewer\'s time: %o', (zone, clock) => {
		const url = `/groups/${fixture<GroupsProps>('/groups').props.rows[0]!.id}/chat`;
		const page = fixture<ChatProps>(url);
		const last = page.props.messages.findLast((m) => m.content_type !== 'service')!;
		const messages = page.props.messages.map((m) => (m.id === last.id ? { ...m, date: AT } : m));
		fakeApi('superadmin');
		renderPage(url, 'superadmin', { viewer: { ...page.viewer, ...zone }, props: { ...page.props, messages } });
		expect(document.querySelector(`#m${last.message_id} .c-time`)!.textContent!.toLowerCase()).toBe(clock);
	});

	it.each(ZONES)('the message page shows it at the viewer\'s date and time: %o', (zone, _clock, full) => {
		const id = fixture<MessagesProps>('/group_messages').props.rows[0]!.id;
		const page = fixture<{ message: { date: string } }>(`/messages/group/${id}`);
		fakeApi('superadmin');
		renderPage(`/messages/group/${id}`, 'superadmin', { viewer: { ...page.viewer, ...zone }, props: { ...page.props, message: { ...page.props.message, date: AT } } });
		expect(document.querySelector('.a-pagehead .a-sub')!.textContent!.toLowerCase()).toContain(full.toLowerCase());
	});
});
