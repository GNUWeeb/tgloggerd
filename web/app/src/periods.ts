// The dashboard chart's periods (minute to year) on the viewer's
// clock: where a period starts and ends in a timezone, the longest range
// each may span, and how a period's start reads. The stand-in buckets
// with these too, so the chart and its sample answers agree.

export const PERIODS = ['minute', 'hour', '4h', 'day', 'week', 'month', 'quarter', 'year'] as const;
export type Period = typeof PERIODS[number];
export const isPeriod = (v: unknown): v is Period => PERIODS.includes(v as Period);

export const PERIOD_LABEL: Record<Period, string> = {
	minute: 'Minute', hour: 'Hour', '4h': '4 hours', day: 'Day', week: 'Week', month: 'Month', quarter: '3 months', year: 'Year',
};
// "Messages per <noun>", the chart's title.
export const PERIOD_NOUN: Record<Period, string> = {
	minute: 'minute', hour: 'hour', '4h': '4 hours', day: 'day', week: 'week', month: 'month', quarter: 'quarter', year: 'year',
};

const DAY = 86400;
// The longest range per period, in seconds (null: all time), so a chart
// has at most about a thousand points. Three years is 1096 days and
// fifteen 5479, so a range back to the same date always fits whatever
// leap days it spans.
export const LIMIT: Record<Period, number | null> = {
	minute: DAY, hour: 45 * DAY, '4h': 180 * DAY, day: 1096 * DAY, week: 5479 * DAY, month: null, quarter: null, year: null,
};

// About one period, in seconds: the shortest range the pickers leave.
export const ROUGH: Record<Period, number> = {
	minute: 60, hour: 3600, '4h': 4 * 3600, day: DAY, week: 7 * DAY, month: 31 * DAY, quarter: 92 * DAY, year: 366 * DAY,
};

// A moment's wall clock in a timezone (month 1-12, weekday 0 = Monday).
export interface Wall { y: number; mo: number; d: number; h: number; mi: number; s: number; wd: number }

const fmts = new Map<string, Intl.DateTimeFormat>();
const WD: Record<string, number> = { Mon: 0, Tue: 1, Wed: 2, Thu: 3, Fri: 4, Sat: 5, Sun: 6 };
export function wall(sec: number, tz: string): Wall {
	let f = fmts.get(tz);
	if (!f) {
		f = new Intl.DateTimeFormat('en-US', { timeZone: tz, year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric', second: 'numeric', weekday: 'short', hourCycle: 'h23' });
		fmts.set(tz, f);
	}
	const p: Record<string, string> = {};
	for (const x of f.formatToParts(new Date(sec * 1000))) p[x.type] = x.value;
	return { y: +p.year!, mo: +p.month!, d: +p.day!, h: +p.hour! % 24, mi: +p.minute!, s: +p.second!, wd: WD[p.weekday!] ?? 0 };
}

// The moment a wall clock time names in a timezone. Fields may run over
// (day 0, month 13), as with Date.UTC. A time a clock change skips reads
// as the moment after the gap; one it repeats, as the first.
export function fromWall(tz: string, y: number, mo: number, d = 1, h = 0, mi = 0, s = 0): number {
	const want = Date.UTC(y, mo - 1, d, h, mi, s) / 1000;
	const off = (t: number) => {
		const w = wall(t, tz);
		return Date.UTC(w.y, w.mo - 1, w.d, w.h, w.mi, w.s) / 1000 - t;
	};
	const o1 = off(want);
	let t = want - o1;
	const o2 = off(t);
	if (o2 !== o1) t = want - o2;
	return t;
}

// The start of the period holding `sec`.
export function floorTo(sec: number, p: Period, tz: string): number {
	sec = Math.floor(sec);
	if (p === 'minute') return sec - (sec % 60 + 60) % 60;
	const w = wall(sec, tz);
	// Back by the wall clock's minutes and seconds: right in any zone, and
	// through a repeated hour, where a wall time names two moments.
	if (p === 'hour') return sec - w.mi * 60 - w.s;
	if (p === '4h') return fromWall(tz, w.y, w.mo, w.d, w.h - (w.h % 4));
	if (p === 'day') return fromWall(tz, w.y, w.mo, w.d);
	if (p === 'week') return fromWall(tz, w.y, w.mo, w.d - w.wd);
	if (p === 'month') return fromWall(tz, w.y, w.mo, 1);
	if (p === 'quarter') return fromWall(tz, w.y, w.mo - ((w.mo - 1) % 3), 1);
	return fromWall(tz, w.y, 1, 1);
}

// The start of the period after the one starting at `start`.
export function nextOf(start: number, p: Period, tz: string): number {
	if (p === 'minute') return start + 60;
	if (p === 'hour') return start + 3600;
	const w = wall(start, tz);
	let t: number;
	if (p === '4h') t = fromWall(tz, w.y, w.mo, w.d, w.h + 4);
	else if (p === 'day') t = fromWall(tz, w.y, w.mo, w.d + 1);
	else if (p === 'week') t = fromWall(tz, w.y, w.mo, w.d + 7);
	else if (p === 'month') t = fromWall(tz, w.y, w.mo + 1, 1);
	else if (p === 'quarter') t = fromWall(tz, w.y, w.mo + 3, 1);
	else t = fromWall(tz, w.y + 1, 1, 1);
	return t > start ? floorTo(t, p, tz) : start + ROUGH[p];
}

// A period's start, as the tooltip and the table give it, and the
// shorter form the axis labels use.
export function periodLabel(sec: number, p: Period, tz: string, h12 = false): string {
	const f = (o: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat('en-GB', { timeZone: tz, hour12: h12, ...o }).format(new Date(sec * 1000));
	const w = wall(sec, tz);
	switch (p) {
		case 'minute': case 'hour': case '4h': return f({ weekday: 'short', day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
		case 'day': return f({ weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' });
		case 'week': return `Week of ${f({ day: 'numeric', month: 'short', year: 'numeric' })}`;
		case 'month': return f({ month: 'long', year: 'numeric' });
		case 'quarter': return `Q${Math.floor((w.mo - 1) / 3) + 1} ${w.y}`;
		default: return String(w.y);
	}
}
export function axisLabel(sec: number, p: Period, tz: string, h12 = false): string {
	const f = (o: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat('en-GB', { timeZone: tz, hour12: h12, ...o }).format(new Date(sec * 1000));
	const w = wall(sec, tz);
	switch (p) {
		case 'minute': case 'hour': case '4h':
			// Midnight names its day instead of 00:00.
			return w.h === 0 && w.mi === 0 ? f({ day: 'numeric', month: 'short' }) : f({ hour: '2-digit', minute: '2-digit' });
		case 'day': case 'week': return f({ day: 'numeric', month: 'short' });
		case 'month': return f({ month: 'short', year: 'numeric' });
		case 'quarter': return `Q${Math.floor((w.mo - 1) / 3) + 1} ${w.y}`;
		default: return String(w.y);
	}
}

// The pickers: the precision a period's range is picked in, and the
// value a native input takes for it, both ways. An end picked names the
// last unit it covers, so it is read as the end of that unit.
export type Precision = 'second' | 'minute' | 'hour' | 'day' | 'month' | 'year';
export const PRECISION: Record<Period, Precision> = {
	minute: 'minute', hour: 'hour', '4h': 'hour', day: 'day', week: 'day', month: 'month', quarter: 'month', year: 'year',
};
const pad = (n: number, w = 2) => String(n).padStart(w, '0');

export function toInput(sec: number, pr: Precision, tz: string): string {
	const w = wall(sec, tz);
	const date = `${pad(w.y, 4)}-${pad(w.mo)}-${pad(w.d)}`;
	switch (pr) {
		case 'second': return `${date}T${pad(w.h)}:${pad(w.mi)}:${pad(w.s)}`;
		case 'minute': return `${date}T${pad(w.h)}:${pad(w.mi)}`;
		case 'hour': return `${date}T${pad(w.h)}:00`;
		case 'day': return date;
		case 'month': return `${pad(w.y, 4)}-${pad(w.mo)}`;
		default: return String(w.y);
	}
}

// The start of the unit an input's value names, or null for no value.
export function fromInput(v: string, pr: Precision, tz: string): number | null {
	const m = v.trim().match(/^(\d{4})(?:-(\d{2})(?:-(\d{2})(?:T(\d{2}):(\d{2})(?::(\d{2}))?)?)?)?$/);
	if (!m) return null;
	const [y, mo = 1, d = 1, h = 0, mi = 0, s = 0] = m.slice(1).map((x) => (x === undefined ? undefined : +x)) as number[];
	if (pr === 'year') return fromWall(tz, y!, 1, 1);
	if (pr === 'month') return fromWall(tz, y!, mo, 1);
	if (pr === 'day') return fromWall(tz, y!, mo, d);
	if (pr === 'hour') return fromWall(tz, y!, mo, d, h);
	if (pr === 'minute') return fromWall(tz, y!, mo, d, h, mi);
	return fromWall(tz, y!, mo, d, h, mi, s);
}

// The end of the unit starting at `sec`.
export function unitEnd(sec: number, pr: Precision, tz: string): number {
	const w = wall(sec, tz);
	if (pr === 'second') return sec + 1;
	if (pr === 'minute') return sec + 60;
	if (pr === 'hour') return sec + 3600;
	if (pr === 'day') return fromWall(tz, w.y, w.mo, w.d + 1);
	if (pr === 'month') return fromWall(tz, w.y, w.mo + 1, 1);
	return fromWall(tz, w.y + 1, 1, 1);
}
