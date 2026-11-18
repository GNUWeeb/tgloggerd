// A per-day bar chart (the dashboard's messages per day): one column per
// day, oldest first, ending with today. Each column is its own hover and
// focus target, so the pointer need not land on a short bar; the tooltip
// gives the count and the day. Every seventh day is labelled on the
// axis, the peak is marked, and a table gives every value to screen
// readers. The days are the calendar days the server counted: from its
// first day's date (per_day_start, in the viewer's timezone) when it
// says it, else UTC days ending today.
import { useState } from 'react';
import { fmt } from '../format.ts';

const DAY = 86400000;
const day = (t: number, opts: Intl.DateTimeFormatOptions) => new Intl.DateTimeFormat('en-GB', { timeZone: 'UTC', ...opts }).format(new Date(t));

export function DayBars({ data, now, start, height = 140, noun = 'messages' }: { data: number[]; now: string; start?: string; height?: number; noun?: string }) {
	const [at, setAt] = useState<number | null>(null);
	const n = data.length;
	const max = Math.max(1, ...data);
	// A calendar date as UTC midnight, so its label is that date whatever
	// the zone (and a 23- or 25-hour day is still one step).
	const first = start && /^\d{4}-\d{2}-\d{2}$/.test(start) ? Date.parse(start + 'T00:00:00Z') : Date.parse(now.slice(0, 10) + 'T00:00:00Z') - (n - 1) * DAY;
	const dateOf = (i: number) => first + i * DAY;
	const long = (i: number) => (i === n - 1 ? `Today, ${day(dateOf(i), { day: 'numeric', month: 'short' })}` : day(dateOf(i), { weekday: 'short', day: 'numeric', month: 'short' }));
	const label = (i: number) => `${long(i)}: ${fmt(data[i] ?? 0)} ${noun}`;
	// Every seventh day back from today, so today is always labelled.
	const ticks = data.map((_, i) => i).filter((i) => (n - 1 - i) % 7 === 0);
	const peak = data.indexOf(max);
	const tip = at === null ? null : {
		left: `${((at + 0.5) / n) * 100}%`,
		// Near the edges the tooltip opens inward, so it is never cut off.
		shift: at < 3 ? '0%' : at > n - 4 ? '-100%' : '-50%',
		bottom: Math.max(1, ((data[at] ?? 0) / max) * height) + 8,
	};
	return (
		<div className="a-daybars-wrap">
			<div className="a-daybars-peak a-muted sm" aria-hidden="true">{fmt(max)}</div>
			<div className="a-daybars" style={{ height }} onMouseLeave={() => setAt(null)}>
				<span className="a-daybars-grid" aria-hidden="true" />
				{data.map((v, i) => (
					<div key={i} className={`a-daybar ${i === n - 1 ? 'today' : ''} ${i === at ? 'on' : ''} ${i === peak ? 'peak' : ''}`}
						tabIndex={0} role="img" aria-label={label(i)}
						onMouseEnter={() => setAt(i)} onFocus={() => setAt(i)} onBlur={() => setAt((x) => (x === i ? null : x))}>
						<span style={{ height: `${Math.max(1, (v / max) * 100)}%` }} />
					</div>
				))}
				{tip && at !== null && (
					<div className="a-daybars-tip" role="status" style={{ left: tip.left, bottom: tip.bottom, transform: `translateX(${tip.shift})` }}>
						<b className="num">{fmt(data[at] ?? 0)}</b> <span className="a-muted">{noun}</span>
						<div className="a-muted sm">{long(at)}</div>
					</div>
				)}
			</div>
			<div className="a-daybars-axis a-muted sm" aria-hidden="true">
				{ticks.map((i) => (
					<span key={i} style={{ left: `${((i + 0.5) / n) * 100}%` }} className={i === n - 1 ? 'end' : i < 2 ? 'start' : ''}>
						{i === n - 1 ? 'Today' : day(dateOf(i), { day: 'numeric', month: 'short' })}
					</span>
				))}
			</div>
			{/* Hidden by its wrapper: a table will not shrink to the hidden
			    size itself, and its rows would make the page taller. */}
			<div className="a-sr"><table>
				<caption>{noun[0]!.toUpperCase() + noun.slice(1)} per day{start ? '' : ', UTC days'}</caption>
				<thead><tr><th scope="col">Day</th><th scope="col">{noun[0]!.toUpperCase() + noun.slice(1)}</th></tr></thead>
				<tbody>{data.map((v, i) => <tr key={i}><td>{long(i)}</td><td>{fmt(v)}</td></tr>)}</tbody>
			</table></div>
		</div>
	);
}
