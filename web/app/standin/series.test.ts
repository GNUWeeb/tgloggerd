// The stand-in's dashboard series, as the dashboard-stats contract has
// the Go service answer it: the 400s, every period zero-filled and
// snapped in the viewer's timezone, the limits, and a live end.
import { describe, expect, it } from 'vitest';
import { seriesApi } from './series.ts';
import { dashboardSeries } from './fixtures.ts';

type Body = { error?: string; field?: string; period: string; start: number; end: number; live: boolean; tz: string; limit: number | null; points: { at: number; n: number }[] };
const q = (s: string) => new URLSearchParams(s);
const at = (iso: string) => Date.parse(iso) / 1000;
const NOW = at('2026-11-17T16:00:00Z');
const ask = (s: string, times: number[] = [], tz = 'Asia/Jakarta') => seriesApi(q(s), tz, times, NOW) as [number, Body];

describe('stand-in dashboard series', () => {
	it('refuses an unknown period, a missing or reversed range, and one over the limit', () => {
		expect(ask(`period=fortnight&start=${NOW - 3600}&end=now`)).toEqual([400, expect.objectContaining({ error: 'bad_request', field: 'period' })]);
		expect(ask('period=hour&end=now')[0]).toBe(400);
		expect(ask(`period=hour&start=${NOW - 3600}&end=soon`)[1].field).toBe('end');
		expect(ask(`period=hour&start=${NOW}&end=${NOW}`)[1].field).toBe('start');
		expect(ask(`period=hour&start=${NOW}&end=${NOW - 60}`)[0]).toBe(400);
		expect(ask(`period=year&start=${NOW}&end=253402300799`)[0]).toBe(200);
		expect(ask(`period=year&start=${NOW}&end=253402300800`)[1].field).toBe('end');
		expect(ask(`period=year&start=253402300800&end=now`)[1].field).toBe('start');
		expect(ask(`period=minute&start=${NOW - 86400}&end=now`)[0]).toBe(200);
		expect(ask(`period=minute&start=${NOW - 86401}&end=now`)[0]).toBe(400);
		expect(ask(`period=hour&start=${NOW - 45 * 86400 - 1}&end=${NOW}`)[0]).toBe(400);
		expect(ask(`period=4h&start=${NOW - 181 * 86400}&end=${NOW}`)[0]).toBe(400);
		expect(ask(`period=day&start=${at('2023-11-17T00:00:00Z')}&end=${at('2026-11-17T00:00:00Z')}`)[0]).toBe(200);
		expect(ask(`period=day&start=${at('2023-11-16T00:00:00Z')}&end=${at('2026-11-17T00:00:00Z')}`)[0]).toBe(400);
		expect(ask(`period=week&start=${at('2011-11-01T00:00:00Z')}&end=${NOW}`)[0]).toBe(400);
		// Month and longer have no limit.
		expect(ask(`period=month&start=0&end=now`)[1].limit).toBeNull();
		expect(ask(`period=month&start=0&end=now`)[0]).toBe(200);
	});

	it('counts per hour, zero-filled, the last point the current hour when live', () => {
		const times = [NOW - 7200 + 5, NOW - 7200 + 60, NOW - 30, NOW + 10];
		const [st, b] = ask(`period=hour&start=${NOW - 3 * 3600 + 120}&end=now`, times);
		expect(st).toBe(200);
		expect(b.live).toBe(true);
		expect(b.tz).toBe('Asia/Jakarta');
		expect(b.limit).toBe(45 * 86400);
		expect(b.start).toBe(NOW - 3 * 3600);
		expect(b.end).toBe(NOW + 3600);
		expect(b.points).toEqual([
			{ at: NOW - 3 * 3600, n: 0 }, { at: NOW - 7200, n: 2 }, { at: NOW - 3600, n: 1 }, { at: NOW, n: 1 },
		]);
	});

	it('ends a fixed range with the period holding its last second, not live', () => {
		const [, b] = ask(`period=hour&start=${NOW - 7200}&end=${NOW}`);
		expect(b.live).toBe(false);
		expect(b.points.map((p) => p.at)).toEqual([NOW - 7200, NOW - 3600]);
		const [, c] = ask(`period=hour&start=${NOW - 7200}&end=${NOW + 1}`);
		expect(c.end).toBe(NOW + 3600);
	});

	it('snaps days, weeks, months, quarters and years in the viewer\'s timezone', () => {
		// 17 Nov 16:00 UTC is 23:00 in Jakarta (UTC+7), 05:00 on 18 Nov in Auckland (UTC+13).
		const [, d] = ask(`period=day&start=${NOW - 86400}&end=now`);
		expect(d.points.map((p) => p.at)).toEqual([at('2026-11-15T17:00:00Z'), at('2026-11-16T17:00:00Z')]);
		const [, a] = ask(`period=day&start=${NOW - 3600}&end=now`, [], 'Pacific/Auckland');
		expect(a.points.map((p) => p.at)).toEqual([at('2026-11-17T11:00:00Z')]);
		// Weeks start on Monday: 16 Nov 2026 is one.
		const [, w] = ask(`period=week&start=${NOW - 3600}&end=now`, [], 'UTC');
		expect(w.start).toBe(at('2026-11-16T00:00:00Z'));
		expect(w.end).toBe(at('2026-11-23T00:00:00Z'));
		const [, m] = ask(`period=month&start=${at('2026-01-15T00:00:00Z')}&end=now`, [], 'UTC');
		expect(m.points).toHaveLength(11);
		expect(m.points[1]!.at).toBe(at('2026-02-01T00:00:00Z'));
		const [, qq] = ask(`period=quarter&start=${at('2026-02-15T00:00:00Z')}&end=now`, [], 'UTC');
		expect(qq.points.map((p) => p.at)).toEqual(['2026-01-01', '2026-04-01', '2026-07-01', '2026-10-01'].map((x) => at(x + 'T00:00:00Z')));
		const [, y] = ask(`period=year&start=${at('2024-06-01T00:00:00Z')}&end=now`, [], 'UTC');
		expect(y.points.map((p) => p.at)).toEqual([2024, 2025, 2026].map((x) => at(`${x}-01-01T00:00:00Z`)));
	});

	it('keeps 4-hour periods on the wall clock across a daylight saving change', () => {
		// Berlin falls back on 25 Oct 2026, 03:00 CEST to 02:00 CET.
		const [, b] = ask(`period=4h&start=${at('2026-10-24T22:00:00Z')}&end=${at('2026-10-25T23:00:00Z')}`, [], 'Europe/Berlin');
		const hours = b.points.map((p) => new Intl.DateTimeFormat('en-GB', { timeZone: 'Europe/Berlin', hour: '2-digit', hourCycle: 'h23' }).format(new Date(p.at * 1000)));
		expect(hours).toEqual(['00', '04', '08', '12', '16', '20']);
		expect(b.points[1]!.at - b.points[0]!.at).toBe(5 * 3600);
	});

	it('answers from the fixtures\' messages, in the viewer\'s scope', () => {
		const [st, b] = dashboardSeries('superadmin', q(`period=hour&start=${NOW - 48 * 3600}&end=now`)) as [number, Body];
		expect(st).toBe(200);
		expect(b.points).toHaveLength(49);
		const total = b.points.reduce((s, p) => s + p.n, 0);
		expect(total).toBeGreaterThan(100);
		const [, m] = dashboardSeries('member', q(`period=hour&start=${NOW - 48 * 3600}&end=now`)) as [number, Body];
		expect(m.points.reduce((s, p) => s + p.n, 0)).toBeLessThan(total);
	});
});
