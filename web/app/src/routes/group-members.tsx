// The group page's Members tab: Telegram's member list, a page at a
// time, or why it is not available.
import { useEffect, useState } from 'react';
import { Crown, Eye, EyeOff, LoaderCircle, ShieldCheck, TriangleAlert, Ban, Users as UsersIcon } from 'lucide-react';
import { Link, useNav } from '../nav.tsx';
import { ApiError } from '../api/client.ts';
import { MEMBERS_PAGE, fetchMembers, reasonText, setHideMemberList, type GroupMember, type MembersPage } from '../api/members.ts';
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
	// The size of the last page fetched: a short one is the end.
	const [last, setLast] = useState(p.members.length);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	useEffect(() => { setList(p.members); setLast(p.members.length); setErr(''); }, [p]);
	const total = p.total ?? count;

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

	const more = last >= limit && (p.total == null || offset + list.length < p.total);
	const loadMore = async () => {
		setBusy(true); setErr('');
		try {
			const next = await track(fetchMembers(groupId, offset + list.length, limit));
			setList((cur) => [...cur, ...next.members]);
			setLast(next.members.length);
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
		setBusy(false);
	};
	const rows = list.map((m) => ({ ...m, user: named(m) }));
	return (
		<>
			{offset > 0 && <p className="a-muted sm">From member {fmt(offset + 1)}. <Link to={`/groups/${groupId}/members`} className="a-link">Back to the first</Link></p>}
			{rows.length ? (
				<div className="a-card a-tablecard">
					<div className="a-tablescroll">
						<table className="a-table">
							<thead><tr><th>Member</th><th>Status</th><th className="a-hide-md">Joined</th></tr></thead>
							<tbody>{rows.map((m) => (
								<tr key={m.user.id}>
									<td><UserCell u={m.user} /></td>
									<td><StatusChip s={m.status} /></td>
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
							</Link>
						</li>
					))}</ul>
				</div>
			) : <Empty>{offset > 0 ? 'No members this far into the list.' : 'Telegram gave no members for this group.'}</Empty>}
			<div className="a-loadmore">
				{err && <span className="a-banner warn" role="alert"><TriangleAlert size={14} /><span>{err}</span></span>}
				{more
					? <button type="button" className="a-btn a-btn-ghost" disabled={busy} onClick={() => void loadMore()}>{busy && <LoaderCircle size={14} className="a-spin" />}{busy ? 'Loading...' : 'Load more'}</button>
					: rows.length > 0 && <span className="a-muted">{total != null && total > offset + rows.length ? `${fmt(offset + rows.length)} of ${fmt(total)} members: Telegram lists no more` : 'End of the list'}</span>}
			</div>
		</>
	);
}
