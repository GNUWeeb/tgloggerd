// The dashboard's message chart: messages per period (minute to year)
// over a start and an end, picked in the viewer's timezone. The pickers
// show the precision the period needs, and a range longer than the
// period allows moves the start rather than being refused. The choice is
// the account's dashboard_* preferences, saved as it changes and sent
// back in the viewer: a period, and either a fixed range
// (dashboard_start and dashboard_end) or the last dashboard_last seconds
// to now. The first series comes with the page data, so the server
// render draws it; the browser fetches only when the choice changes.
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { Card } from '../ui/primitives.tsx';
import { SeriesChart, VARIANTS, isVariant, type Variant } from '../ui/serieschart.tsx';
import { fmt, SITE_DEFAULTS } from '../format.ts';
import { savePrefs } from '../api/prefs.ts';
import { dashboardSeries, type Series } from '../api/dashboard.ts';
import { ApiError } from '../api/client.ts';
import { LIMIT, PERIODS, PERIOD_LABEL, PERIOD_NOUN, PRECISION, ROUGH, fromInput, isPeriod, periodLabel, toInput, unitEnd, type Period, type Precision } from '../periods.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

// The end is now (live), or a fixed time; unix seconds, end exclusive.
export type Range = { live: true; start: number } | { live: false; start: number; end: number };

export const DEFAULT_LAST = 48 * 3600;

// The range kept within what the period allows: a start at or after the
// end, or one further back than the limit, moves to fit.
export function clampRange(r: Range, p: Period, limit: number | null, now: number): Range {
	const end = r.live ? now : r.end;
	let start = r.start;
	if (start >= end) start = end - ROUGH[p];
	if (limit !== null && end - start > limit) start = end - limit;
	start = Math.max(0, Math.floor(start));
	return r.live ? { live: true, start } : { live: false, start, end };
}

function initial(v: Viewer, now: number): { period: Period; range: Range } {
	const period = isPeriod(v.dashboard_period) ? v.dashboard_period : 'hour';
	const s = v.dashboard_start, e = v.dashboard_end;
	const range: Range = typeof e === 'number' && typeof s === 'number' && s < e
		? { live: false, start: s, end: e }
		: { live: true, start: now - (typeof v.dashboard_last === 'number' && v.dashboard_last > 0 ? v.dashboard_last : DEFAULT_LAST) };
	return { period, range: clampRange(range, period, LIMIT[period], now) };
}

const INPUT: Record<Precision, { type: string; step?: number }> = {
	second: { type: 'datetime-local', step: 1 }, minute: { type: 'datetime-local', step: 60 }, hour: { type: 'datetime-local', step: 3600 },
	day: { type: 'date' }, month: { type: 'month' }, year: { type: 'number', step: 1 },
};

// A picker that keeps what is typed until it reads as a time, so a
// half-typed year is not put back under the cursor.
function TimeInput({ label, value, pr, tz, disabled, onPick }: { label: string; value: number; pr: Precision; tz: string; disabled?: boolean; onPick: (sec: number) => void }) {
	const [draft, setDraft] = useState<string | null>(null);
	const k = INPUT[pr];
	return (
		<label className="a-sc-ctl">
			<span>{label}</span>
			<input type={k.type} step={k.step} min={pr === 'year' ? 1970 : undefined} max={pr === 'year' ? 9999 : undefined}
				className="a-select" disabled={disabled} value={draft ?? toInput(value, pr, tz)}
				onChange={(e) => {
					setDraft(e.target.value);
					const t = fromInput(e.target.value, pr, tz);
					if (t !== null) onPick(t);
				}}
				onBlur={() => setDraft(null)} />
		</label>
	);
}

function Select<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: readonly (readonly [T, string])[]; onChange: (v: T) => void }) {
	return (
		<label className="a-sc-ctl">
			<span>{label}</span>
			<select className="a-select" value={value} onChange={(e) => onChange(e.target.value as T)}>
				{options.map(([v, l]) => <option key={v} value={v}>{l}</option>)}
			</select>
		</label>
	);
}

type Load = { state: 'loading' | 'ok'; data: Series | null; error: string | null };
type Prefs = Partial<S['Prefs']>;

// What a view fetches: the period and the range.
const keyOf = (p: Period, r: Range) => `${p} ${r.start} ${r.live ? 'now' : r.end}`;

export function MessagesChart({ viewer: v, first }: { viewer: Viewer; first: Series }) {
	const tz = v.timezone || SITE_DEFAULTS.timezone;
	const h12 = v.date_format === '12h';
	// The server's clock: its now when the page was made, and the time
	// passed in this browser since.
	const skew = useRef<number | null>(null);
	if (skew.current === null) skew.current = Date.parse(v.now) - Date.now();
	const now = () => Math.floor((Date.now() + skew.current!) / 1000);
	const [{ period, range }, setView] = useState(() => initial(v, now()));
	const [variant, setVariant] = useState<Variant>(isVariant(v.dashboard_variant) ? v.dashboard_variant : 'bars');
	const [load, setLoad] = useState<Load>({ state: 'ok', data: first, error: null });
	const [saveErr, setSaveErr] = useState(false);
	// The limits the server answered with, over the built-in ones.
	const limits = useRef<Partial<Record<Period, number | null>>>({ [first.period]: first.limit });
	const limitOf = (p: Period) => (p in limits.current ? limits.current[p]! : LIMIT[p]);

	const seq = useRef(0);
	// The view the series shown answers: the page data's first.
	const shown = useRef(keyOf(period, range));
	useEffect(() => {
		const key = keyOf(period, range);
		if (key === shown.current) return;
		const n = ++seq.current;
		const ac = new AbortController();
		setLoad((l) => ({ ...l, state: 'loading', error: null }));
		dashboardSeries({ period, start: range.start, end: range.live ? 'now' : range.end }, ac.signal).then((d) => {
			if (n !== seq.current) return;
			shown.current = key;
			limits.current[d.period] = d.limit;
			setLoad({ state: 'ok', data: d, error: null });
		}, (e: unknown) => {
			if (n !== seq.current || ac.signal.aborted) return;
			setLoad((l) => ({ ...l, state: 'ok', error: e instanceof ApiError ? e.message : 'The chart could not be loaded.' }));
		});
		return () => ac.abort();
	}, [period, range.live, range.start, range.live ? 0 : range.end]);

	const save = (patch: Prefs) => {
		setSaveErr(false);
		savePrefs(patch).catch(() => setSaveErr(true));
	};
	// The drawing changes nothing fetched.
	const draw = (x: Variant) => {
		if (x === variant) return;
		setVariant(x);
		save({ dashboard_variant: x });
	};
	const change = (p: Period, r: Range) => {
		// One reading of the clock, so the saved length is the one clamped.
		const t = now();
		const next = clampRange(r, p, limitOf(p), t);
		setView({ period: p, range: next });
		const patch: Prefs = next.live
			? { dashboard_period: p, dashboard_last: t - next.start, dashboard_start: null, dashboard_end: null }
			: { dashboard_period: p, dashboard_start: next.start, dashboard_end: next.end, dashboard_last: null };
		save(patch);
	};

	const pr = PRECISION[period];
	// A fixed end shows the last unit it covers; now shows itself.
	const end = range.live ? now() : range.end;
	const data = load.data;
	const total = data ? data.points.reduce((s, p) => s + p.n, 0) : 0;
	const title = `Messages per ${PERIOD_NOUN[period]}${variant === 'total' ? ', running total' : ''}`;
	const summary = data && data.points.length
		? `${title}, ${periodLabel(data.points[0]!.at, data.period, data.tz, h12)} to ${data.live ? 'now' : periodLabel(data.points.at(-1)!.at, data.period, data.tz, h12)}: ${fmt(total)} messages`
		: title;
	let body: ReactNode;
	if (data) body = <SeriesChart points={data.points} period={data.period} variant={variant} tz={data.tz} h12={h12} label={summary} />;
	else body = <div className="a-sc-empty a-muted">{load.error ? '' : 'Loading the chart...'}</div>;
	return (
		<Card title={title} className="a-dash-wide">
			<div className="a-sc-ctls">
				<Select label="Period" value={period} options={PERIODS.map((p) => [p, PERIOD_LABEL[p]] as const)} onChange={(p) => change(p, range)} />
				<TimeInput label="Start" value={range.start} pr={pr} tz={tz} onPick={(t) => change(period, range.live ? { live: true, start: t } : { live: false, start: t, end: range.end })} />
				<TimeInput label="End" value={range.live ? end : Math.max(range.start, end - 1)} pr={pr} tz={tz} disabled={range.live}
					onPick={(t) => change(period, { live: false, start: range.start, end: unitEnd(t, pr, tz) })} />
				<label className="a-sc-now">
					<input type="checkbox" checked={range.live} onChange={(e) => change(period, e.target.checked
						? { live: true, start: range.start }
						: { live: false, start: range.start, end: unitEnd(fromInput(toInput(now(), pr, tz), pr, tz)!, pr, tz) })} />
					Now
				</label>
				<Select label="Show as" value={variant} options={VARIANTS} onChange={draw} />
				{saveErr && <span className="a-muted sm" role="status">Not saved; shown on this page only.</span>}
			</div>
			{load.error && <p className="a-sc-err" role="alert">{load.error}</p>}
			<div className={`a-bigchart ${load.state === 'loading' && data ? 'a-sc-busy' : ''}`}>{body}</div>
		</Card>
	);
}
