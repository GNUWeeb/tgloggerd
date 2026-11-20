// @vitest-environment happy-dom
// The dashboard's counts: exact or compact as the account chooses, and
// current while the page is open.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup } from '../test/page.tsx';
import { compact, fmt } from '../format.ts';
import { KPI_REFRESH_MS, type DashboardProps } from './dashboard.tsx';

beforeEach(setup);

const kpi = (label: string) => within(within(document.querySelector<HTMLElement>('.a-kpis')!).getByRole('link', { name: new RegExp(`^${label}`) })).getByText((_, el) => el?.tagName === 'B');
const total = (key: string) => fixture<DashboardProps>('/').props.kpis.find((k) => k.key === key)!.total;

describe('dashboard numbers', () => {
	it('shows every digit by default', () => {
		fakeApi('superadmin');
		renderPage('/');
		expect(kpi('Group messages').textContent).toBe(fmt(total('group_messages')));
		expect(fmt(total('group_messages'))).toMatch(/,/);
		expect(screen.getByRole('button', { name: 'Exact' }).getAttribute('aria-pressed')).toBe('true');
	});

	it('renders compact numbers when the account chose them', () => {
		fakeApi('superadmin');
		const page = fixture('/');
		renderPage('/', 'superadmin', { viewer: { ...page.viewer, dashboard_numbers: 'compact' } });
		expect(kpi('Users').textContent).toBe(compact(total('users')));
		expect(kpi('Users').getAttribute('title')).toBe(`${fmt(total('users'))} users`);
	});

	it('switches at once and saves the choice', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/account/preferences': () => ({ prefs: { dashboard_numbers: 'compact' } }) });
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
		expect(kpi('Users').textContent).toBe(compact(total('users')));
		await waitFor(() => expect(callsTo(calls, 'PATCH', '/api/account/preferences')).toHaveLength(1));
		expect(callsTo(calls, 'PATCH', '/api/account/preferences')[0]!.body).toEqual({ dashboard_numbers: 'compact' });
		expect(screen.queryByText(/Not saved/)).toBeNull();
	});

	it('keeps the choice on this page when saving fails', async () => {
		fakeApi('superadmin', { 'PATCH /api/account/preferences': () => json(500, { error: 'internal' }) });
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Compact' }));
		await screen.findByText(/Not saved; shown on this page only/);
		expect(kpi('Users').textContent).toBe(compact(total('users')));
	});

	it('does not save when the choice does not change', () => {
		const calls = fakeApi('superadmin');
		renderPage('/');
		fireEvent.click(screen.getByRole('button', { name: 'Exact' }));
		expect(callsTo(calls, 'PATCH', '/api/account/preferences')).toHaveLength(0);
	});
});

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

describe('dashboard counts, live', () => {
	const kpis = () => fixture<DashboardProps>('/').props.kpis;
	const bumped = (key: string, by: number) => kpis().map((k) => (k.key === key ? { ...k, total: k.total + by, today: k.today + by } : k));

	beforeEach(() => {
		FakeSource.last = null;
		vi.stubGlobal('EventSource', FakeSource);
	});
	afterEach(() => vi.useRealTimers());

	const stream = async () => {
		await waitFor(() => expect(FakeSource.last?.url).toMatch(/topics=[^&]*messages/));
		act(() => FakeSource.last!.open());
		return FakeSource.last!;
	};
	const todayOf = (label: string) => within(within(document.querySelector<HTMLElement>('.a-kpis')!).getByRole('link', { name: new RegExp(`^${label}`) })).getByText(/today$/).textContent;

	it('counts each new message into its kind, total and today', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/');
		const s = await stream();
		act(() => { s.emit({ seq: 1, kind: 'message.new', chat_id: -100123, message_id: 1 }); s.emit({ seq: 2, kind: 'message.new', chat_id: -100124, message_id: 1 }); });
		act(() => s.emit({ seq: 3, kind: 'message.new', chat_id: 4567, message_id: 1 }));
		act(() => s.emit({ seq: 4, kind: 'message.edited', chat_id: -100123, message_id: 1 }));
		expect(kpi('Group messages').textContent).toBe(fmt(total('group_messages') + 2));
		expect(kpi('Private messages').textContent).toBe(fmt(total('private_messages') + 1));
		expect(kpi('Users').textContent).toBe(fmt(total('users')));
		const today = kpis().find((k) => k.key === 'group_messages')!.today;
		expect(todayOf('Group messages')).toBe(`+${fmt(today + 2)} today`);
		// The counts follow by themselves: no page refetch, no count call.
		await act(() => new Promise((r) => setTimeout(r, 50)));
		expect(callsTo(calls, 'GET', /^\/api\/(page|dashboard\/kpis)/)).toHaveLength(0);
	});

	it('takes the server\'s counts every minute, over the ticked ones', async () => {
		vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
		const calls = fakeApi('superadmin', { 'GET /api/dashboard/kpis': () => ({ kpis: bumped('users', 5) }) });
		renderPage('/');
		const s = await stream();
		act(() => s.emit({ seq: 1, kind: 'message.new', chat_id: -100123, message_id: 1 }));
		expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(0);
		act(() => { vi.advanceTimersByTime(KPI_REFRESH_MS); });
		await waitFor(() => expect(kpi('Users').textContent).toBe(fmt(total('users') + 5)));
		expect(kpi('Group messages').textContent).toBe(fmt(total('group_messages')));
		expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(1);
	});

	it('does not refresh a hidden page, and catches up when it is shown', async () => {
		vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval', 'Date'] });
		let hidden = true;
		vi.spyOn(document, 'hidden', 'get').mockImplementation(() => hidden);
		const calls = fakeApi('superadmin', { 'GET /api/dashboard/kpis': () => ({ kpis: kpis() }) });
		renderPage('/');
		act(() => { vi.advanceTimersByTime(3 * KPI_REFRESH_MS); });
		expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(0);
		hidden = false;
		act(() => { document.dispatchEvent(new Event('visibilitychange')); });
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(1));
		// Shown again soon after a refresh: nothing to catch up.
		hidden = true; act(() => { document.dispatchEvent(new Event('visibilitychange')); });
		hidden = false; act(() => { document.dispatchEvent(new Event('visibilitychange')); });
		expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(1);
	});

	it('refreshes the counts right after a resync', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/dashboard/kpis': () => ({ kpis: bumped('files', 3) }) });
		renderPage('/');
		const s = await stream();
		act(() => s.emit({ seq: 9, kind: 'resync' }));
		await waitFor(() => expect(kpi('Files').textContent).toBe(fmt(total('files') + 3)));
		expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(1);
	});

	it('keeps the counts it shows when a refresh fails', async () => {
		const calls = fakeApi('superadmin', { 'GET /api/dashboard/kpis': () => json(503, { error: 'unavailable' }) });
		renderPage('/');
		const s = await stream();
		act(() => s.emit({ seq: 1, kind: 'message.new', chat_id: -100123, message_id: 1 }));
		act(() => s.emit({ seq: 2, kind: 'resync' }));
		await waitFor(() => expect(callsTo(calls, 'GET', '/api/dashboard/kpis')).toHaveLength(1));
		await Promise.resolve();
		expect(kpi('Group messages').textContent).toBe(fmt(total('group_messages') + 1));
	});
});

describe('dashboard status', () => {
	it('says no message is logged yet on an empty archive', () => {
		fakeApi('superadmin');
		const page = fixture<DashboardProps>('/');
		renderPage('/', 'superadmin', { props: { ...page.props, status: { ...page.props.status!, last_message_at: '' } } });
		const row = screen.getByText('Last message logged').parentElement!;
		expect(row.textContent).toContain('none yet');
		expect(row.textContent).not.toMatch(/NaN/);
	});
});
