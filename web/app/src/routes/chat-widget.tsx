// A widget message: a bot's HTML page that Telegram draws in a box.
// The archive never renders that HTML (decision
// 2026-11-20-msg-widgets-html): a bot's page on the web's origin would
// run with the viewer's session. So the box is drawn empty at the
// widget's size, and the message page shows the HTML as React text in
// a code block, which the browser never parses as markup. Nothing in
// this file may hand the HTML to the DOM any other way (a test checks).
import { AppWindow } from 'lucide-react';
import type { CSSProperties } from 'react';
import { Link } from '../nav.tsx';
import type { S } from '../api/schema.ts';

// The box's height when the widget's is not fixed (0).
export const WIDGET_DEFAULT_HEIGHT = 160;

// The box at the widget's size, kept within the bubble: width 0 is the
// whole bubble; a fixed width and height keep their aspect when the
// bubble is narrower.
export function widgetBox(width: number, height: number): CSSProperties {
	const w = width > 0 ? { width: `${width}px`, maxWidth: '100%' } : { width: '100%' };
	if (width > 0 && height > 0) return { ...w, aspectRatio: `${width} / ${height}` };
	return { ...w, height: `${height > 0 ? height : WIDGET_DEFAULT_HEIGHT}px` };
}

// page: the message page's link, given in the chat; full: the message
// page itself, which shows the source.
export function Widget({ d, page, full = false }: { d: S['WidgetDetail']; page?: string; full?: boolean }) {
	return (
		<div className="k-card k-widget">
			<div className="k-widgetbox" style={widgetBox(d.width, d.height)}>
				<AppWindow size={22} aria-hidden="true" />
				<span>Widget{d.width > 0 && d.height > 0 ? ` . ${d.width} x ${d.height}` : ''}</span>
				<span className="c-sub">This archive does not run widgets.</span>
				{!full && page && <Link to={page} className="a-link">Its source is on the message page</Link>}
			</div>
			{full && d.widget_id && <div className="c-sub">Shared widget <span className="mono">{d.widget_id}</span></div>}
			{full && (d.html !== null
				? (
					<details className="k-widgetsrc">
						<summary>Show source</summary>
						<pre className="rt-pre"><code>{d.html}</code></pre>
					</details>
				)
				: <div className="c-sub">Its HTML was not kept.</div>)}
		</div>
	);
}
