// A vertical change timeline, grouped by month on full history pages.
import type { ComponentType, ReactNode } from 'react';
import type { Viewer } from '../page.ts';
import { dateTime } from '../format.ts';

export interface TlItem { icon: ComponentType<{ size?: number }>; tone?: 'accent' | 'info' | ''; title: ReactNode; at: string; body?: ReactNode }

export function Timeline({ items, v, compact = false }: { items: TlItem[]; v: Viewer; compact?: boolean }) {
	return (
		<ol className={`a-timeline ${compact ? 'compact' : ''}`}>
			{items.map((e, i) => {
				const I = e.icon;
				return (
					<li key={i} className={`a-tl ${e.tone ? 'a-tl-' + e.tone : ''}`}>
						<span className="a-tl-icon"><I size={14} /></span>
						<div className="a-tl-body">
							<div className="a-tl-title"><span>{e.title}</span><time className="a-muted a-tl-time" dateTime={e.at}>{dateTime(e.at, v)}</time></div>
							{e.body && <div className="a-tl-val">{e.body}</div>}
						</div>
					</li>
				);
			})}
		</ol>
	);
}

export function MonthTimeline({ items, v }: { items: TlItem[]; v: Viewer }) {
	const tz = v.timezone || 'Asia/Jakarta';
	const label = (iso: string) => new Intl.DateTimeFormat('en-US', { month: 'long', year: 'numeric', timeZone: tz }).format(new Date(iso));
	const months: [string, TlItem[]][] = [];
	for (const e of items) {
		const m = label(e.at);
		const last = months[months.length - 1];
		if (!last || last[0] !== m) months.push([m, [e]]);
		else last[1].push(e);
	}
	return <>{months.map(([m, list]) => <section key={m} className="a-month"><h3>{m}</h3><Timeline items={list} v={v} /></section>)}</>;
}
