// What an earlier version of a message held besides its text (its
// MessageDetail), drawn small for the edit lists of the chat and the
// message page: a live location's position, a checklist's tasks, any
// other kind with its card. A poll's versions have none (the poll
// tables keep only the latest results).
import { useState, type ReactNode } from 'react';
import { MapPin, Square, SquareCheck } from 'lucide-react';
import { KindDetail, coords, osm } from '../routes/chat-kinds.tsx';
import { RichText, type RichChat } from './richtext.tsx';
import { fmt } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type Detail = NonNullable<S['MessageDetail']>;

export function EditDetail({ d, text, entities, chat, v }: { d: Detail; text: string; entities?: S['TextEntity'][]; chat?: RichChat; v: Viewer }) {
	if (d.kind === 'location') {
		return (
			<div className="e-detail e-loc">
				<MapPin size={13} aria-hidden="true" />
				<a href={osm(d.lat, d.lon)} target="_blank" rel="noopener noreferrer" className="k-coords mono">{coords(d.lat, d.lon)}</a>
				{d.accuracy !== null && <span className="c-sub">within {fmt(d.accuracy)} m</span>}
			</div>
		);
	}
	if (d.kind === 'checklist') {
		// The title is the version's text, which the list draws itself.
		const more = d.tasks_total - d.tasks.length;
		return (
			<div className="e-detail k-checklist">
				<ul>{d.tasks.map((t) => (
					<li key={t.id} className={t.done ? 'done' : ''}>
						{t.done ? <SquareCheck size={13} aria-label="done" /> : <Square size={13} aria-label="not done" />}
						<span className="k-task"><RichText text={t.text} entities={t.entities} chat={chat} /></span>
					</li>
				))}</ul>
				{more > 0 && <div className="c-sub">and {fmt(more)} more task{more === 1 ? '' : 's'}</div>}
			</div>
		);
	}
	return <div className="e-detail"><KindDetail d={d} text={text} entities={entities} chat={chat} v={v} /></div>;
}

// Versions shown before the list collapses; a live location keeps one
// a minute, so its list can run long.
export const EDITS_SHOWN = 5;

// The latest EDITS_SHOWN versions of a list, oldest first, and a button
// that shows the earlier ones; more is null when nothing is hidden.
export function useEditList<T>(list: T[], className: string): { shown: T[]; more: ReactNode } {
	const [all, setAll] = useState(false);
	const hidden = all ? 0 : Math.max(0, list.length - EDITS_SHOWN);
	return {
		shown: hidden ? list.slice(hidden) : list,
		more: hidden ? <button type="button" className={className} onClick={() => setAll(true)}>Show {fmt(hidden)} more</button> : null,
	};
}
