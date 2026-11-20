// A vertical change timeline, optionally grouped by month.
import { monthLabel } from '../data/seed.js';

// items: [{ icon, tone?, title, at, body }]
export function Timeline({ items, compact = false }) {
	return (
		<ol className={`a-timeline ${compact ? 'compact' : ''}`}>
			{items.map((e, i) => {
				const Icon = e.icon;
				return (
					<li key={i} className={`a-tl ${e.tone ? 'a-tl-' + e.tone : ''}`}>
						<span className="a-tl-icon"><Icon size={14} /></span>
						<div className="a-tl-body">
							<div className="a-tl-title">
								<span>{e.title}</span>
								<time className="a-muted a-tl-time" dateTime={e.at.replace(' ', 'T') + 'Z'} title={e.at}>{e.at.slice(0, 16)}</time>
							</div>
							{e.body && <div className="a-tl-val">{e.body}</div>}
						</div>
					</li>
				);
			})}
		</ol>
	);
}

export function MonthTimeline({ items }) {
	const months = [];
	for (const e of items) {
		const m = e.at.slice(0, 7);
		if (!months.length || months[months.length - 1][0] !== m) months.push([m, []]);
		months[months.length - 1][1].push(e);
	}
	return months.map(([m, list]) => (
		<section key={m} className="a-month">
			<h3>{monthLabel(m)}</h3>
			<Timeline items={list} />
		</section>
	));
}

export function PhotoSwatch({ hue, size = 72 }) {
	return <span className="a-tl-photo" style={{ width: size, height: size, background: `linear-gradient(135deg, hsl(${hue} 70% 62%), hsl(${(hue + 40) % 360} 65% 48%))` }} />;
}
