// GET /api/dashboard/series as the Go service answers it: the messages
// counted per period (minute to year) over a range, in the viewer's
// timezone. Start and end are snapped to the periods holding them (an
// end of "now" to the end of the current period), every period of the
// range has a point, zeros included, and a range longer than the
// period's limit, or one that ends before it starts, is refused.
import { LIMIT, floorTo, isPeriod, nextOf } from '../src/periods.ts';

type Answer = [number, Record<string, unknown>];
const bad = (field: string, message: string): Answer => [400, { error: 'bad_request', message, field }];
// A unix time up to the end of the year 9999, else null.
const MAX = 253402300799;
const int = (v: string | null) => (v !== null && /^\d+$/.test(v) && Number(v) <= MAX ? Number(v) : null);

// times: the visible messages' dates, unix seconds; now: the server's.
export function seriesApi(q: URLSearchParams, tz: string, times: number[], now: number): Answer {
	const period = q.get('period');
	if (!isPeriod(period)) return bad('period', 'Unknown period.');
	const start = int(q.get('start'));
	if (start === null) return bad('start', 'A start time is needed, in unix seconds, up to the year 9999.');
	const live = q.get('end') === 'now';
	const end = live ? now : int(q.get('end'));
	if (end === null) return bad('end', 'An end time is needed, in unix seconds up to the year 9999, or now.');
	if (start >= end) return bad('start', 'The start must come before the end.');
	// The range asked for is held to the limit, before snapping: the
	// snapped one may run up to a period over it.
	const limit = LIMIT[period];
	if (limit !== null && end - start > limit) return bad('start', 'The range is longer than this period allows.');
	const from = floorTo(start, period, tz);
	const last = floorTo(live ? now : end - 1, period, tz);
	const to = nextOf(last, period, tz);
	const starts: number[] = [];
	for (let t = from; t < to; t = nextOf(t, period, tz)) starts.push(t);
	const n = starts.map(() => 0);
	for (const t of times) {
		if (t < from || t >= to) continue;
		// The last bucket starting at or before t.
		let lo = 0, hi = starts.length - 1;
		while (lo < hi) {
			const mid = (lo + hi + 1) >> 1;
			if (starts[mid]! <= t) lo = mid; else hi = mid - 1;
		}
		n[lo]!++;
	}
	return [200, { period, start: from, end: to, live, tz, limit, points: starts.map((at, i) => ({ at, n: n[i] })) }];
}
