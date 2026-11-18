// Change timelines of users and groups, page by page: the page carries
// the newest page and the cursor of the next (history_next); older
// pages come from GET /api/{users|groups}/{id}/history?before=.
import { useCallback, useEffect, useState } from 'react';
import { track } from './busy.ts';
import { Gem, History as HistoryIcon, LoaderCircle, TriangleAlert } from 'lucide-react';
import { ApiError } from '../api/client.ts';
import { Pill } from './primitives.tsx';
import type { S } from '../api/schema.ts';

export type HistoryPage<T> = { entries: T[]; next: string | null };

export function useHistoryPages<T>(first: T[], next: string | null, load: (before: string) => Promise<HistoryPage<T>>) {
	const [entries, setEntries] = useState(first);
	const [cursor, setCursor] = useState<string | null>(next);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	// New page data (a live refresh, another user) starts over.
	useEffect(() => { setEntries(first); setCursor(next); setErr(''); }, [first, next]);
	const more = useCallback(async () => {
		if (!cursor || busy) return;
		setBusy(true); setErr('');
		try {
			const p = await track(load(cursor));
			setEntries((cur) => [...cur, ...p.entries]);
			setCursor(p.next);
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
		setBusy(false);
	}, [cursor, busy, load]);
	return { entries, hasMore: cursor !== null, busy, err, more };
}

// The end of a timeline: a button for the next page, or its start.
export function OlderChanges({ h }: { h: ReturnType<typeof useHistoryPages<unknown>> }) {
	return (
		<div className="a-loadmore">
			{h.err && <span className="a-banner warn" role="alert"><TriangleAlert size={14} /><span>{h.err}</span></span>}
			{h.hasMore
				? <button type="button" className="a-btn a-btn-ghost" disabled={h.busy} onClick={() => void h.more()}>{h.busy ? <LoaderCircle size={14} className="a-spin" /> : <HistoryIcon size={14} />}{h.busy ? 'Loading...' : 'Load older changes'}</button>
				: <span className="a-muted"><HistoryIcon size={14} />Start of the recorded history</span>}
		</div>
	);
}

type Username = S['Username'];

// A username as a pill: active ones stand out, disabled ones are
// muted, bought (collectible) ones carry a gem.
export function UsernamePill({ n }: { n: Username }) {
	const off = n.kind === 'disabled';
	return (
		<Pill tone={off ? 'muted' : 'accent'}>
			{n.collectible && <Gem size={11} aria-label="collectible" />}@{n.username}{off ? ' (off)' : ''}
		</Pill>
	);
}

// A username event's title (HistoryEntry and GroupHistoryEntry).
export function usernameTitle(e: { action?: string; username_kind?: string }): string {
	switch (e.action) {
		case 'reordered': return 'Usernames reordered';
		case 'kind_changed': return e.username_kind === 'disabled' ? 'Username turned off' : 'Username turned on';
		case 'collectible_changed': return 'Username collectible status changed';
		default: return `Username ${e.action ?? ''}`.trim();
	}
}
