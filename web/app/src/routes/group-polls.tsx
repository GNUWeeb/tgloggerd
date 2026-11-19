// A group's Polls tab (route group_polls, /groups/{id}/polls): its
// polls, newest message first, each as the chat draws a poll (question,
// type, results, the quiz's correct option) with its sender, the date,
// and links to its message in the chat and to its message page. Older
// polls come a page at a time from GET /api/groups/{id}/polls.
import { useCallback } from 'react';
import { useHistoryPages } from '../ui/history.tsx';
import { Empty } from '../ui/primitives.tsx';
import { KindDetail } from './chat-kinds.tsx';
import { LoadMore, MessageLinks, SentBy, cursorOf } from './group-files.tsx';
import { dateTime } from '../format.ts';
import type { Viewer } from '../page.ts';
import { fetchGroupPolls, type GroupPollsPage } from '../api/groupfiles.ts';

// pub: a public tab of an exposed group, as GroupFiles takes it.
export function GroupPolls({ groupId, p, v, pub = false }: { groupId: number; p: GroupPollsPage; v: Viewer; pub?: boolean }) {
	const load = useCallback(async (before: string) => {
		const r = await fetchGroupPolls(groupId, Number(before));
		return { entries: r.rows, next: cursorOf(r.next_before) };
	}, [groupId]);
	const h = useHistoryPages(p.rows, cursorOf(p.next_before), load);
	return (
		<div className="a-grouppolls">
			{h.entries.length > 0 && <ul className="a-pollcards">{h.entries.map((r) => (
				<li key={r.message_id} className="a-card a-pollcard">
					<div className="a-pollcard-head">
						<SentBy s={r.sender} plain={pub} />
						<time className="a-muted sm" dateTime={r.date}>{dateTime(r.date, v)}</time>
					</div>
					{r.poll ? <KindDetail d={r.poll} text="" v={v} /> : <p className="a-muted">This poll was not stored.</p>}
					<MessageLinks r={r} pub={pub} />
				</li>
			))}</ul>}
			{!h.entries.length && <Empty>No polls in this group.</Empty>}
			<LoadMore h={h} noun="polls" />
		</div>
	);
}
