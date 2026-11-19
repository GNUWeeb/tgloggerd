// The group page's Members tab: Telegram's member list, a page at a
// time, or why it is not available.
import { useEffect, useRef, useState } from 'react';
import { Crown, Eye, EyeOff, Hash, LoaderCircle, Search, ShieldCheck, TriangleAlert, Ban, Users as UsersIcon, X } from 'lucide-react';
import { Link, setQuery, useNav } from '../nav.tsx';
import { ApiError } from '../api/client.ts';
import { MEMBERS_PAGE, MEMBERS_Q_MAX, fetchMembers, reasonText, setHideMemberList, type GroupMember, type MembersPage } from '../api/members.ts';
import { Avatar, Btn, Empty, Pill } from '../ui/primitives.tsx';
import { UserCell } from '../ui/bits.tsx';
import { track } from '../ui/busy.ts';
import { ago, fmt } from '../format.ts';
import type { PageData } from '../page.ts';

// The member page's own props: the list's page at ?offset=.
export type MembersProps = MembersPage & { offset?: number; limit?: number };

// A member the archive has not stored yet comes with its id alone.
const named = (m: GroupMember) => (m.user.name ? m.user : { ...m.user, name: `User ${m.user.id}` });

function StatusChip({ s }: { s: GroupMember['status'] }) {
	if (s === 'creator') return <Pill tone="accent"><Crown size={11} />owner</Pill>;
	if (s === 'administrator') return <Pill tone="info"><ShieldCheck size={11} />admin</Pill>;
	if (s === 'restricted') return <Pill tone="danger"><Ban size={11} />restricted</Pill>;
	return null;
}

// A member a search found by its exact id (a q of digits).
// The members of a page that count toward its offset: all but a
// search's id match, which the first page holds in addition to limit.
const paged = (ms: GroupMember[]) => ms.filter((m) => m.matched_by !== 'id').length;
const ById = ({ m }: { m: GroupMember }) => (m.matched_by === 'id' ? <Pill tone="info"><Hash size={11} />matched by ID</Pill> : null);

// The search box's text, kept in the page's URL as ?q=: typing waits
// for a pause, then fetches the page data for the new search in place
// of the current entry, so Back does not step through keystrokes. A
// change of the URL from outside (Back, a link) puts its q in the box.
function useMemberSearch() {
	const { page, navigate } = useNav();
	const q = new URL(page.url, 'http://x').searchParams.get('q') ?? '';
	const [text, setText] = useState(q);
	const sent = useRef(q);
	const url = useRef(page.url);
	url.current = page.url;
	useEffect(() => {
		if (q !== sent.current) { sent.current = q; setText(q); }
	}, [q]);
	const go = (t: string) => {
		sent.current = t;
		navigate(setQuery(url.current, { q: t.trim() ? t : undefined, offset: undefined }), { replace: true, scroll: false });
	};
	useEffect(() => {
		if (text === sent.current) return;
		const t = setTimeout(() => go(text), 300);
		return () => clearTimeout(t);
	}, [text]);
	const clear = () => {
		setText('');
		if (sent.current.trim() || q.trim()) go('');
		else sent.current = '';
	};
	return { q, text, setText, clear };
}

// A superadmin's switch for a channel's list: hidden, only superadmins
// see it. The page data is fetched again after, for the new state.
export function HideMemberList({ groupId, hidden }: { groupId: number; hidden: boolean }) {
	const { refresh } = useNav();
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const toggle = async () => {
		setBusy(true); setErr('');
		try {
			await setHideMemberList(groupId, !hidden);
			await refresh();
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); } finally { setBusy(false); }
	};
	return (
		<div className="a-membershide">
			<Btn ghost icon={hidden ? EyeOff : Eye} disabled={busy} aria-pressed={hidden} onClick={() => void toggle()}>Hide member list</Btn>
			<span className="a-muted sm">{hidden ? 'Hidden: only superadmins see this channel\'s members.' : 'Admins see this channel\'s members.'}</span>
			{err && <span className="a-banner warn" role="alert"><TriangleAlert size={14} /><span>{err}</span></span>}
		</div>
	);
}

export function Members({ groupId, count, p, v }: { groupId: number; count: number | null; p: MembersProps; v: PageData['viewer'] }) {
	const offset = p.offset ?? 0;
	const limit = p.limit ?? MEMBERS_PAGE;
	const [list, setList] = useState(p.members);
	// The size of the last page fetched, without a search's id match
	// (which comes in addition to limit): a short one is the end.
	const [last, setLast] = useState(paged(p.members));
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	useEffect(() => { setList(p.members); setLast(paged(p.members)); setErr(''); }, [p]);
	const total = p.total ?? count;
	const search = useMemberSearch();
	// The search the page data answers (the box may be ahead of it).
	const q = search.q.trim();

	if (!p.available) {
		return (
			<div className="a-card a-membersoff">
				<UsersIcon size={18} />
				<div>
					<b>{total != null ? `${fmt(total)} members` : 'Member list not available'}</b>
					<p className="a-muted">{total != null && 'Member list not available. '}{reasonText(p.reason)}.</p>
				</div>
			</div>
		);
	}

	// The next page starts after the members paged so far; total counts
	// the id match too.
	const next = offset + paged(list);
	const more = last >= limit && (p.total == null || next + (list.length - paged(list)) < p.total);
	const loadMore = async () => {
		setBusy(true); setErr('');
		try {
			const page = await track(fetchMembers(groupId, next, limit, q));
			setList((cur) => { const seen = new Set(cur.map((m) => m.user.id)); return [...cur, ...page.members.filter((m) => !seen.has(m.user.id))]; });
			setLast(paged(page.members));
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
		setBusy(false);
	};
	const rows = list.map((m) => ({ ...m, user: named(m) }));
	return (
		<>
			<div className="a-search a-membersearch" role="search">
				<Search size={15} />
				<input
					type="search" name="member-search" value={search.text} maxLength={MEMBERS_Q_MAX}
					placeholder="Search members by name, @username or ID" aria-label="Search members by name, @username or ID"
					onChange={(e) => search.setText(e.target.value)}
					onKeyDown={(e) => { if (e.key === 'Escape' && search.text) { e.preventDefault(); search.clear(); } }}
					autoComplete="off" autoCorrect="off" autoCapitalize="off" spellCheck={false}
				/>
				{search.text && <button type="button" className="a-search-x" aria-label="Clear search" onClick={search.clear}><X size={13} /></button>}
			</div>
			{offset > 0 && <p className="a-muted sm">From {q ? 'match' : 'member'} {fmt(offset + 1)}. <Link to={setQuery(`/groups/${groupId}/members`, { q: q || undefined })} className="a-link">Back to the first</Link></p>}
			{rows.length ? (
				<div className="a-card a-tablecard">
					<div className="a-tablescroll">
						<table className="a-table">
							<thead><tr><th>Member</th><th>Status</th><th className="a-hide-md">Joined</th></tr></thead>
							<tbody>{rows.map((m) => (
								<tr key={m.user.id}>
									<td><UserCell u={m.user} /></td>
									<td><span className="a-chips"><StatusChip s={m.status} /><ById m={m} /></span></td>
									<td className="a-hide-md" title={m.joined_at ?? undefined}>{m.joined_at ? ago(m.joined_at, v) : <span className="a-muted">-</span>}</td>
								</tr>
							))}</tbody>
						</table>
					</div>
					<ul className="a-list">{rows.map((m) => (
						<li key={m.user.id}>
							<Link to={`/users/${m.user.id}`}>
								<Avatar id={m.user.id} name={m.user.name} photo={m.user.photo} size={36} />
								<span className="a-list-main">
									<span className="a-uname">{m.user.name}</span>
									<span className="a-handle">{m.user.username ? `@${m.user.username}` : m.joined_at ? `joined ${ago(m.joined_at, v)}` : ''}</span>
								</span>
								<StatusChip s={m.status} />
								<ById m={m} />
							</Link>
						</li>
					))}</ul>
				</div>
			) : q && offset === 0 ? (
				<Empty>
					<p>No member matches <b>{q}</b>.</p>
					<button type="button" className="a-btn a-btn-ghost" onClick={search.clear}>Clear search</button>
				</Empty>
			) : <Empty>{offset > 0 ? `No ${q ? 'matches' : 'members'} this far into the list.` : 'Telegram gave no members for this group.'}</Empty>}
			<div className="a-loadmore">
				{err && <span className="a-banner warn" role="alert"><TriangleAlert size={14} /><span>{err}</span></span>}
				{more
					? <button type="button" className="a-btn a-btn-ghost" disabled={busy} onClick={() => void loadMore()}>{busy && <LoaderCircle size={14} className="a-spin" />}{busy ? 'Loading...' : 'Load more'}</button>
					: rows.length > 0 && <span className="a-muted">{q ? `${fmt(offset + rows.length)} ${offset + rows.length === 1 ? 'match' : 'matches'}` : total != null && total > offset + rows.length ? `${fmt(offset + rows.length)} of ${fmt(total)} members: Telegram lists no more` : 'End of the list'}</span>}
			</div>
		</>
	);
}
