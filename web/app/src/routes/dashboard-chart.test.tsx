// @vitest-environment happy-dom
// The dashboard's message chart: its period and range controls, the
// clamp to the period's limit, the account's remembered choice, the
// first series from the page data, and the bars with their tooltip.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { callsTo, fakeApi, fixture, json, renderPage, setup, type Call } from '../test/page.tsx';
import { clampRange } from './dashboard-chart.tsx';
import type { DashboardProps } from './dashboard.tsx';
import { LIMIT, type Period } from '../periods.ts';

const NOW = Date.parse('2026-11-17T16:00:00Z') / 1000;

beforeEach(() => {
	setup();
	// The server's now and this browser's agree, to the second.
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(NOW * 1000);
});
afterEach(() => vi.useRealTimers());

const series = (calls: Call[]) => callsTo(calls, 'GET', '/api/dashboard/series');
const lastQuery = (calls: Call[]) => Object.fromEntries(series(calls).at(-1)!.query);
const patches = (calls: Call[]) => callsTo(calls, 'PATCH', '/api/account/preferences').map((c) => c.body as Record<string, unknown>);
const control = (name: string) => screen.getByLabelText(name) as HTMLInputElement;
const chart = () => screen.findByRole('img', { name: /^Messages per/ });
// The page data with other chart preferences in the viewer, and the
// first series the server would send with them, if given.
const viewer = (extra: Record<string, unknown>, first?: unknown) => {
	const page = fixture<DashboardProps>('/');
	return { viewer: { ...page.viewer, ...extra }, props: { ...page.props, ...(first ? { series: first } : {}) } };
};
const firstOf = (points: number[], period = 'hour', d = 3600) => ({
	period, start: NOW - (points.length - 1) * d, end: NOW + d, live: true, tz: 'UTC', limit: LIMIT[period as Period],
	points: points.map((n, i) => ({ at: NOW - (points.length - 1 - i) * d, n })),
});

describe('dashboard chart', () => {
	it('shows the last 48 hours per hour, ending now, as bars, from the page data', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/');
		const img = await chart();
		// Drawn from the page data's first series: nothing to fetch.
		await new Promise((r) => setTimeout(r, 20));
		expect(series(calls)).toHaveLength(0);
		expect(screen.getByRole('heading', { name: 'Messages per hour' })).toBeTruthy();
		expect((control('Period') as unknown as HTMLSelectElement).value).toBe('hour');
		expect(control('Now').checked).toBe(true);
		expect(control('End').disabled).toBe(true);
		// The stand-in's first series: 49 hours, a bar for each with messages.
		const body = fixture<DashboardProps>('/').props.series;
		expect(body.points).toHaveLength(49);
		const total = body.points.reduce((s, p) => s + p.n, 0);
		expect(img.getAttribute('aria-label')).toMatch(new RegExp(`to now: ${total.toLocaleString('en-US')} messages$`));
		expect(img.querySelectorAll('.a-sc-bar')).toHaveLength(body.points.filter((p) => p.n > 0).length);
		// No legend: the title names the one series.
		expect(document.querySelector('.a-sc-legend')).toBeNull();
	});

	it('picks in the precision the period needs', async () => {
		fakeApi('superadmin');
		renderPage('/');
		await chart();
		const want: [string, string, string | null][] = [['minute', 'datetime-local', '60'], ['hour', 'datetime-local', '3600'], ['4h', 'datetime-local', '3600'], ['day', 'date', null], ['week', 'date', null], ['month', 'month', null], ['quarter', 'month', null], ['year', 'number', '1']];
		for (const [p, type, step] of want) {
			fireEvent.change(control('Period'), { target: { value: p } });
			expect(control('Start').type, p).toBe(type);
			expect(control('Start').getAttribute('step'), p).toBe(step);
			expect(control('End').type, p).toBe(type);
		}
		// Jakarta is UTC+7: 17 Nov 16:00 UTC is 23:00 there.
		expect(control('Start').value).toBe('2026');
		fireEvent.change(control('Period'), { target: { value: 'month' } });
		expect(control('End').value).toBe('2026-11');
		fireEvent.change(control('Period'), { target: { value: 'hour' } });
		expect(control('End').value).toBe('2026-11-17T23:00');
	});

	it('moves the start when a period change takes the range over its limit', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/account/preferences': () => ({ prefs: {} }) });
		renderPage('/');
		await chart();
		fireEvent.change(control('Period'), { target: { value: 'minute' } });
		await waitFor(() => expect(lastQuery(calls)).toEqual({ period: 'minute', start: String(NOW - 86400), end: 'now' }));
		expect(patches(calls).at(-1)).toEqual({ dashboard_period: 'minute', dashboard_last: 86400, dashboard_start: null, dashboard_end: null });
		expect(screen.getByRole('heading', { name: 'Messages per minute' })).toBeTruthy();
	});

	it('moves the start when a picked start is further back than the limit', async () => {
		const calls = fakeApi('superadmin');
		renderPage('/', 'superadmin', viewer({ dashboard_period: 'minute', dashboard_last: 3600 }, firstOf([1, 2], 'minute', 60)));
		await chart();
		expect(control('Start').value).toBe('2026-11-17T22:00');
		// Two days back, in Jakarta time: more than a minute chart's 24 hours.
		fireEvent.change(control('Start'), { target: { value: '2026-11-15T23:00' } });
		await waitFor(() => expect(lastQuery(calls).start).toBe(String(NOW - 86400)));
		// A start after the end moves to a period before it.
		fireEvent.change(control('Start'), { target: { value: '2026-11-18T05:00' } });
		await waitFor(() => expect(lastQuery(calls).start).toBe(String(NOW - 60)));
	});

	it('fixes the end when Now is unticked, and takes a picked end', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/account/preferences': () => ({ prefs: {} }) });
		renderPage('/');
		await chart();
		fireEvent.click(control('Now'));
		expect(control('End').disabled).toBe(false);
		// The end of the hour holding now: 17 Nov 23:00-24:00 in Jakarta.
		await waitFor(() => expect(lastQuery(calls)).toEqual({ period: 'hour', start: String(NOW - 48 * 3600), end: String(NOW + 3600) }));
		expect(patches(calls).at(-1)).toEqual({ dashboard_period: 'hour', dashboard_start: NOW - 48 * 3600, dashboard_end: NOW + 3600, dashboard_last: null });
		fireEvent.change(control('End'), { target: { value: '2026-11-17T10:00' } });
		await waitFor(() => expect(lastQuery(calls).end).toBe(String(Date.parse('2026-11-17T04:00:00Z') / 1000)));
		fireEvent.click(control('Now'));
		await waitFor(() => expect(lastQuery(calls).end).toBe('now'));
	});

	it('starts from the account\'s remembered choice', async () => {
		const calls = fakeApi('superadmin');
		// 28 Oct to 16 Nov 23:00, Jakarta time.
		const start = NOW - 20 * 86400, end = NOW - 86400;
		renderPage('/', 'superadmin', viewer({ dashboard_period: 'day', dashboard_start: start, dashboard_end: end, dashboard_last: undefined }, { ...firstOf([4, 5, 6], 'day', 86400), live: false }));
		await chart();
		expect((control('Period') as unknown as HTMLSelectElement).value).toBe('day');
		expect(control('Now').checked).toBe(false);
		expect(control('Start').value).toBe('2026-10-28');
		expect(control('End').value).toBe('2026-11-16');
		expect(screen.getByRole('heading', { name: 'Messages per day' })).toBeTruthy();
		expect(series(calls)).toHaveLength(0);
		// A change asks for the new range, from the remembered one.
		fireEvent.change(control('Period'), { target: { value: 'week' } });
		await waitFor(() => expect(lastQuery(calls)).toEqual({ period: 'week', start: String(start), end: String(end) }));
	});

	it('says so when the choice cannot be saved, and keeps it', async () => {
		const calls = fakeApi('superadmin', { 'PATCH /api/account/preferences': () => json(500, { error: 'internal' }) });
		renderPage('/');
		await chart();
		fireEvent.change(control('Period'), { target: { value: 'day' } });
		await screen.findByText(/Not saved; shown on this page only/);
		expect(lastQuery(calls).period).toBe('day');
	});

	it('shows the server\'s refusal', async () => {
		fakeApi('superadmin', { 'GET /api/dashboard/series': () => json(400, { error: 'bad_request', message: 'The range is longer than this period allows.' }) });
		renderPage('/');
		await chart();
		fireEvent.change(control('Period'), { target: { value: 'day' } });
		expect((await screen.findByRole('alert')).textContent).toBe('The range is longer than this period allows.');
	});

	it('gives a period\'s start and count on hover, anywhere over its column', async () => {
		fakeApi('superadmin');
		renderPage('/', 'superadmin', viewer({}, firstOf([0, 3, 12, 0, 5])));
		const img = await chart();
		expect(img.querySelectorAll('.a-sc-bar')).toHaveLength(3);
		const hit = img.querySelector('.a-sc-hit')!;
		const w = Number(hit.getAttribute('width'));
		// High over the third column, well above its bar.
		fireEvent.pointerMove(hit, { clientX: w * 0.5, clientY: 2 });
		const tip = screen.getByRole('status');
		expect(tip.textContent).toContain('12 messages');
		expect(tip.textContent).toMatch(/Tue,? 17 Nov,? 14:00/);
		expect(img.querySelector('.a-sc-cross')).not.toBeNull();
		fireEvent.pointerLeave(hit);
		expect(screen.queryByRole('status')).toBeNull();
		// And from the keyboard.
		fireEvent.keyDown(img, { key: 'End' });
		expect(screen.getByRole('status').textContent).toContain('5 messages');
		fireEvent.keyDown(img, { key: 'ArrowLeft' });
		expect(screen.getByRole('status').textContent).toContain('0 messages');
	});
});

describe('dashboard chart drawings', () => {
	const first = firstOf([2, 0, 5, 1]);
	const answer = { 'PATCH /api/account/preferences': () => ({ prefs: {} }) };
	// The y of each vertex of the line, top down.
	const ys = (d: string) => [...d.matchAll(/[ML][\d.]+,([\d.]+)/g)].map((m) => Number(m[1]));

	it('draws bars by default, then a line, an area and a running total', async () => {
		const calls = fakeApi('superadmin', answer);
		renderPage('/', 'superadmin', viewer({}, first));
		const img = await chart();
		expect((control('Show as') as unknown as HTMLSelectElement).value).toBe('bars');
		expect(img.querySelectorAll('.a-sc-bar')).toHaveLength(3);
		expect(img.querySelector('.a-sc-line')).toBeNull();

		fireEvent.change(control('Show as'), { target: { value: 'line' } });
		expect(img.querySelectorAll('.a-sc-bar')).toHaveLength(0);
		const line = img.querySelector('.a-sc-line')!.getAttribute('d')!;
		const ly = ys(line);
		expect(ly).toHaveLength(4);
		// 5 is the highest point, 0 on the baseline.
		expect(Math.min(...ly)).toBe(ly[2]);
		expect(Math.max(...ly)).toBe(ly[1]);
		expect(img.querySelector('.a-sc-area')).toBeNull();

		fireEvent.change(control('Show as'), { target: { value: 'area' } });
		expect(img.querySelector('.a-sc-line')!.getAttribute('d')).toBe(line);
		expect(img.querySelector('.a-sc-area')!.getAttribute('d')).toMatch(/Z$/);

		fireEvent.change(control('Show as'), { target: { value: 'total' } });
		expect(screen.getByRole('heading', { name: 'Messages per hour, running total' })).toBeTruthy();
		const ty = ys(img.querySelector('.a-sc-line')!.getAttribute('d')!);
		// 2, 2, 7, 8: never falling, flat where nothing came.
		expect(ty[1]).toBe(ty[0]);
		expect(ty[2]).toBeLessThan(ty[1]!);
		expect(ty[3]).toBeLessThan(ty[2]!);
		fireEvent.keyDown(img, { key: 'End' });
		expect(screen.getByRole('status').textContent).toContain('8 messages so far');
		expect(screen.getByRole('status').textContent).toContain('+1 in this period');
		// Drawing again fetches nothing; each choice is saved.
		expect(series(calls)).toHaveLength(0);
		await waitFor(() => expect(patches(calls)).toHaveLength(3));
		expect(patches(calls)).toEqual([{ dashboard_variant: 'line' }, { dashboard_variant: 'area' }, { dashboard_variant: 'total' }]);
	});

	it('draws as the account chose', async () => {
		fakeApi('superadmin', answer);
		renderPage('/', 'superadmin', viewer({ dashboard_variant: 'area' }, first));
		const img = await chart();
		expect(img.querySelector('.a-sc-area')).not.toBeNull();
		expect((control('Show as') as unknown as HTMLSelectElement).value).toBe('area');
	});
});

describe('dashboard chart as a table', () => {
	const answer = { 'PATCH /api/account/preferences': () => ({ prefs: {} }) };

	it('gives the same numbers as a table, and goes back to the chart', async () => {
		fakeApi('superadmin', answer);
		renderPage('/', 'superadmin', viewer({}, firstOf([2, 0, 5])));
		await chart();
		const toggle = screen.getByRole('button', { name: 'Show as table' });
		expect(toggle.getAttribute('aria-pressed')).toBe('false');
		fireEvent.click(toggle);
		expect(toggle.getAttribute('aria-pressed')).toBe('true');
		expect(screen.queryByRole('img', { name: /^Messages per/ })).toBeNull();
		const table = screen.getByRole('table', { name: /^Messages per hour, .* to now: 7 messages$/ });
		const rows = within(table).getAllByRole('row').map((r) => [...r.querySelectorAll('th, td')].map((c) => c.textContent));
		expect(rows[0]).toEqual(['Hour', 'Messages']);
		expect(rows.slice(1).map((r) => r[1])).toEqual(['2', '0', '5']);
		expect(rows[3]![0]).toMatch(/17 Nov,? 16:00/);
		// A running total adds the sum so far.
		fireEvent.change(control('Show as'), { target: { value: 'total' } });
		const rows2 = within(screen.getByRole('table')).getAllByRole('row').map((r) => [...r.querySelectorAll('th, td')].map((c) => c.textContent));
		expect(rows2[0]).toEqual(['Hour', 'Messages', 'So far']);
		expect(rows2.slice(1).map((r) => r[2])).toEqual(['2', '2', '7']);
		fireEvent.click(toggle);
		expect(screen.queryByRole('table')).toBeNull();
		expect(await chart()).toBeTruthy();
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

describe('dashboard chart, live', () => {
	beforeEach(() => {
		FakeSource.last = null;
		vi.stubGlobal('EventSource', FakeSource);
		vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] });
		vi.setSystemTime(NOW * 1000);
	});
	// Timers are fake here: let promises and due timers run.
	const flush = (ms = 0) => act(() => vi.advanceTimersByTimeAsync(ms));
	// The series as the server counts it at the (fake) time of the call:
	// three messages in every period, the current one included.
	const SECS = { minute: 60, hour: 3600 } as Record<string, number>;
	const serve = (q: URLSearchParams) => {
		const p = q.get('period')!, d = SECS[p]!, start = Number(q.get('start'));
		const now = Math.floor(Date.now() / 1000), live = q.get('end') === 'now';
		const from = start - (start % d), to = (live ? now - (now % d) : Number(q.get('end')) - 1 - ((Number(q.get('end')) - 1) % d)) + d;
		const points = Array.from({ length: (to - from) / d }, (_, i) => ({ at: from + i * d, n: 3 }));
		return { period: p, start: from, end: to, live, tz: 'UTC', limit: p === 'minute' ? 86400 : 45 * 86400, points };
	};
	const server = { 'GET /api/dashboard/series': (c: Call) => serve(c.query), 'PATCH /api/account/preferences': () => ({ prefs: {} }) };
	// The page data for these chart preferences, its first series as served.
	const page = (extra: Record<string, unknown>, q: Record<string, string>) => viewer(extra, serve(new URLSearchParams(q)));
	const hours48 = { period: 'hour', start: String(NOW - 48 * 3600), end: 'now' };
	const open = async () => {
		// The stream opens once its subscriptions settle.
		await flush(1000);
		expect(FakeSource.last?.url).toMatch(/topics=[^&]*messages/);
		act(() => FakeSource.last!.open());
		return FakeSource.last!;
	};
	const lastCount = () => {
		fireEvent.keyDown(screen.getByRole('img', { name: /^Messages per/ }), { key: 'End' });
		return document.querySelector('.a-sc-tip')!.textContent;
	};

	it('counts each new message into the last period', async () => {
		fakeApi('superadmin', server);
		renderPage('/', 'superadmin', page({}, hours48));
		const s = await open();
		expect(lastCount()).toContain('3 messages');
		act(() => { s.emit({ seq: 1, kind: 'message.new', chat_id: -100123, message_id: 1 }); s.emit({ seq: 2, kind: 'message.new', chat_id: 4567, message_id: 1 }); });
		act(() => s.emit({ seq: 3, kind: 'message.edited', chat_id: -100123, message_id: 1 }));
		expect(lastCount()).toContain('5 messages');
	});

	it('starts a new period when the clock passes the last one, keeping the start below the limit', async () => {
		const calls = fakeApi('superadmin', server);
		renderPage('/', 'superadmin', page({}, hours48));
		await open();
		const first = series(calls).length;
		expect(first).toBe(0);
		const before = screen.getByRole('img', { name: /^Messages per/ }).querySelectorAll('.a-sc-bar').length;
		await flush(3599 * 1000);
		expect(series(calls)).toHaveLength(first);
		await flush(2000);
		await flush();
		expect(series(calls)).toHaveLength(first + 1);
		expect(lastQuery(calls)).toEqual({ period: 'hour', start: String(NOW - 48 * 3600), end: 'now' });
		expect(screen.getByRole('img', { name: /^Messages per/ }).querySelectorAll('.a-sc-bar')).toHaveLength(before + 1);
	});

	it('moves the start with the end once the range is at the limit', async () => {
		const calls = fakeApi('superadmin', server);
		renderPage('/', 'superadmin', page({ dashboard_period: 'minute', dashboard_last: 86400 }, { period: 'minute', start: String(NOW - 86400), end: 'now' }));
		await open();
		await flush(61 * 1000);
		await flush();
		expect(Number(lastQuery(calls).start)).toBe(NOW + 60 - 86400);
		await flush(60 * 1000);
		await flush();
		expect(Number(lastQuery(calls).start)).toBe(NOW + 120 - 86400);
		// Each new start is fetched once.
		expect(series(calls).map((c) => c.query.get('start'))).toEqual([NOW + 60 - 86400, NOW + 120 - 86400].map(String));
	});

	it('is not live with a fixed end', async () => {
		const calls = fakeApi('superadmin', server);
		renderPage('/', 'superadmin', page({ dashboard_start: NOW - 7200, dashboard_end: NOW - 3600, dashboard_last: undefined }, { period: 'hour', start: String(NOW - 7200), end: String(NOW - 3600) }));
		const s = await open();
		const before = lastCount();
		act(() => s.emit({ seq: 1, kind: 'message.new', chat_id: -100123, message_id: 1 }));
		expect(lastCount()).toBe(before);
		await flush(3 * 3600 * 1000);
		expect(series(calls)).toHaveLength(0);
	});

	it('fetches again after a resync', async () => {
		const calls = fakeApi('superadmin', server);
		renderPage('/', 'superadmin', page({}, hours48));
		const s = await open();
		act(() => s.emit({ seq: 9, kind: 'resync' }));
		await flush();
		expect(series(calls)).toHaveLength(1);
		expect(lastQuery(calls)).toEqual(hours48);
	});
});

describe('clampRange', () => {
	it('keeps a range within the limit by moving its start', () => {
		expect(clampRange({ live: false, start: 0, end: 100000 }, 'minute', 86400, NOW)).toEqual({ live: false, start: 100000 - 86400, end: 100000 });
		expect(clampRange({ live: true, start: NOW - 3 * 86400 }, 'minute', 86400, NOW)).toEqual({ live: true, start: NOW - 86400 });
		expect(clampRange({ live: true, start: 0 }, 'year', null, NOW)).toEqual({ live: true, start: 0 });
		expect(clampRange({ live: false, start: 500, end: 400 }, 'hour', 45 * 86400, NOW)).toEqual({ live: false, start: 0, end: 400 });
	});
});
