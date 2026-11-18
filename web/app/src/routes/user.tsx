import { BadgeCheck, Cake, CircleAlert, Gem, Megaphone, ScanSearch, ShieldAlert, Smile, Star, MessageCircle, Type, AtSign, Quote, Phone, Image, Languages, Hash, CalendarDays, Contact } from 'lucide-react';
import { useCallback, useState } from 'react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, Btn, Card, KV, Pill, Stat, Tabs, Empty } from '../ui/primitives.tsx';
import { Flags, Sparkline, userFlags } from '../ui/bits.tsx';
import { LookupName } from './lookupname.tsx';
import { OlderChanges, UsernamePill, useHistoryPages, usernameTitle } from '../ui/history.tsx';
import { api, ok } from '../api/client.ts';
import { ago, dateTime, fmt } from '../format.ts';
import type { PageData, Viewer } from '../page.ts';
import type { GroupSummary, HistoryEntry, UserSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type UserFull = S['UserFull'];

// Full-access viewers get the profile; a member gets only the groups it
// shares with the person (`limited`), web-accounts.md section 5.
export type UserProps = S['UserProps'];

const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const birthday = (b: NonNullable<UserFull['birthday']>) => `${b.day} ${MONTHS[b.month - 1] ?? b.month}${b.year ? ` ${b.year}` : ''}`;

const KIND = {
	name: [Type, 'Name changed'], username: [AtSign, 'Username'], bio: [Quote, 'Bio changed'],
	phone: [Phone, 'Phone changed'], photo: [Image, 'Profile photo'],
} as const;

function HistItem({ e, v }: { e: HistoryEntry; v: Viewer }) {
	const [I, title] = KIND[e.kind];
	let body;
	if (e.kind === 'photo') body = e.removed || !e.photo ? <span className="a-muted">Photo removed</span> : <img className="a-tl-photo" src={e.photo} alt="" width={72} height={72} />;
	else if (e.kind === 'username') body = <span className={e.action === 'removed' ? 'strike' : ''}>{e.collectible && <Gem size={12} aria-label="collectible" />}@{e.detail}{e.username_kind === 'disabled' ? ' (off)' : ''}</span>;
	else body = e.detail ? <span className="a-multiline">{e.detail}</span> : <span className="a-muted">(cleared)</span>;
	return (
		<li className={`a-tl ${e.kind === 'username' ? 'a-tl-accent' : e.kind === 'photo' ? 'a-tl-info' : ''}`}>
			<span className="a-tl-icon"><I size={14} /></span>
			<div className="a-tl-body">
				<div className="a-tl-title"><span>{e.kind === 'username' ? usernameTitle(e) : title}</span><time className="a-muted a-tl-time" dateTime={e.at}>{dateTime(e.at, v)}</time></div>
				<div className="a-tl-val">{body}</div>
			</div>
		</li>
	);
}

function Limited({ page }: { page: PageData<Extract<UserProps, { limited: true }>> }) {
	const { user: u, shared_groups } = page.props;
	return (
		<Shell crumbs={[['Users', '/users'], [u.name, null]]}>
			<div className="a-profilehead">
				<Avatar id={u.id} name={u.name} photo={u.photo} size={64} />
				<div className="a-ph-main"><h1>{u.name}</h1><div className="a-ph-meta">{u.username && <span>@{u.username}</span>}</div></div>
			</div>
			<Card title="In your groups">
				<ul className="a-groups">
					{shared_groups.map((g) => (
						<li key={g.id}><Avatar id={g.id} name={g.title} photo={g.photo} size={28} square /><Link to={`/groups/${g.id}`} className="a-grow a-rowlink">{g.title}</Link></li>
					))}
				</ul>
			</Card>
		</Shell>
	);
}

function UserPage({ page }: { page: PageData<UserProps> }) {
	const p = page.props;
	if (p.limited) return <Limited page={page as PageData<Extract<UserProps, { limited: true }>>} />;
	return <Full page={page as PageData<Extract<UserProps, { limited?: false }>>} tab={page.route === 'user_history' ? 'history' : 'user'} />;
}

function Full({ page, tab }: { page: PageData<Extract<UserProps, { limited?: false }>>; tab: 'user' | 'history' }) {
	const { user: u, groups, history: first, history_next, activity, can_private, can_lookup_name: canLookup, lookup_pending: pending } = page.props;
	const [lookup, setLookup] = useState(false);
	const v = page.viewer;
	const load = useCallback((before: string) => ok(api.GET('/api/users/{id}/history', { params: { path: { id: u.id }, query: { before } } })), [u.id]);
	const h = useHistoryPages(first, history_next, load);
	const history = h.entries;
	const [kind, setKind] = useState<'all' | HistoryEntry['kind']>('all');
	const shown = kind === 'all' ? history : history.filter((e) => e.kind === kind);
	const base = `/users/${u.id}`;
	const crumbs: [string, string | null][] = [['Users', '/users'], [u.name, tab === 'history' ? base : null]];
	if (tab === 'history') crumbs.push(['History', null]);
	return (
		<Shell crumbs={crumbs}>
			<div className="a-profilehead">
				<Avatar id={u.id} name={u.name} photo={u.photo} size={64} deleted={u.type === 'deleted'} />
				<div className="a-ph-main">
					<h1>{u.name} {u.is_verified && <BadgeCheck size={18} className="a-verified" aria-label="verified" />}</h1>
					<div className="a-ph-meta">
						{u.username && <span>@{u.username}</span>}
						<span className="mono">{u.id}</span>
						{u.contact_name && <span className="a-contactlabel" title="The name the logging account saved this contact under"><Contact size={13} />saved as {u.contact_name}</span>}
						{u.is_contact && <Pill tone="info">{u.is_mutual_contact ? 'mutual contact' : 'contact'}</Pill>}
						<Flags flags={userFlags(u)} />
					</div>
				</div>
				{(can_private || canLookup) && (
					<div className="a-head-actions">
						{canLookup && <Btn ghost icon={ScanSearch} onClick={() => setLookup(true)}>Look up real name</Btn>}
						{can_private && <Btn primary to={`${base}/chat`} icon={MessageCircle}>Private chat</Btn>}
					</div>
				)}
			</div>
			{pending && (
				<div className="a-banner warn" role="status"><CircleAlert size={15} /><span>
					A name lookup{page.props.lookup_pending_since ? ` on ${dateTime(page.props.lookup_pending_since, v)}` : ''} removed this contact and has not added it back yet. The logging account keeps trying.
					{page.props.lookup_pending_error ? <> Last error: <span className="mono">{page.props.lookup_pending_error}</span>.</> : ''}
				</span></div>
			)}
			{u.is_contact && u.own_name_known === false && !pending && <div className="a-banner info" role="note"><CircleAlert size={15} /><span>The name shown is the one the logging account saved this contact under: the user was a contact before logging began, and no name they set has been seen yet.</span></div>}
			{lookup && <LookupName id={u.id} name={u.name} contactName={u.contact_name} onClose={() => setLookup(false)} />}
			<Tabs items={[['Overview', base, tab === 'user'], ['History', `${base}/history`, tab === 'history', h.hasMore ? `${history.length}+` : history.length]]} />
			{tab === 'user' ? (
				<div className="a-overview">
					<div className="a-stats">
						<Stat label="Messages" value={fmt(u.msg_count ?? 0)} />
						<Stat label="Groups" value={groups.length} />
						{u.created_at && <Stat label="First seen" value={ago(u.created_at, v)} sub={dateTime(u.created_at, v)} />}
						<div className="a-card a-stat a-stat-spark"><span className="a-muted">Last 30 days</span><b className="num">{fmt(activity.reduce((a, b) => a + b, 0))}</b><span className="a-spark"><Sparkline data={activity} /></span></div>
					</div>
					<div className="a-cols">
						<Card title="Details">
							<KV rows={[
								[Type, 'First name', u.first_name], [Type, 'Last name', u.last_name],
								[AtSign, 'Usernames', u.usernames.length ? u.usernames.map((n) => <UsernamePill key={n.username} n={n} />) : ''],
								[Quote, 'Bio', u.bio ? <span className="a-multiline">{u.bio}</span> : ''], [Phone, 'Phone', u.phone, true], [Languages, 'Language', u.language ?? ''],
								u.birthday ? [Cake, 'Birthday', birthday(u.birthday)] : null,
								u.personal_chat_id ? [Megaphone, 'Personal channel', u.personal_chat ? <Link to={`/groups/${u.personal_chat.id}`} className="a-rowlink">{u.personal_chat.title}</Link> : <span className="mono">{u.personal_chat_id}</span>] : null,
								u.emoji_status ? [Smile, 'Emoji status', <span>custom emoji <span className="mono">{u.emoji_status.custom_emoji_id}</span>{u.emoji_status.expires_at ? `, until ${dateTime(u.emoji_status.expires_at, v)}` : ''}</span>] : null,
								u.paid_message_star_count ? [Star, 'Messages cost', `${fmt(u.paid_message_star_count)} Stars`] : null,
								u.restricts_new_chats ? [ShieldAlert, 'New chats', 'only from contacts and Premium users'] : null,
								u.restriction_reason ? [ShieldAlert, 'Restricted', u.restriction_reason] : null,
								u.has_sensitive_content ? [ShieldAlert, 'Content', 'marked sensitive'] : null,
								u.photo_file_id ? [Image, 'Profile photo', <Link to={`/files?file=${u.photo_file_id}`} className="a-rowlink">file {u.photo_file_id}</Link>] : null,
								[Hash, 'User ID', u.id, true],
								u.created_at ? [CalendarDays, 'First seen', dateTime(u.created_at, v)] : null,
								u.updated_at ? [CalendarDays, 'Updated', dateTime(u.updated_at, v)] : null,
								u.own_name_checked_at ? [ScanSearch, 'Own name checked', dateTime(u.own_name_checked_at, v)] : null,
							]} />
						</Card>
						<div className="a-stack">
							<Card title="Recent changes" action={<Link to={`${base}/history`} className="a-link">View all</Link>}>
								{history.length ? <ol className="a-timeline compact">{history.slice(0, 4).map((e, i) => <HistItem key={i} e={e} v={v} />)}</ol> : <p className="a-muted">No changes recorded.</p>}
							</Card>
							<Card title="Most active in">
								{groups.length ? (
									<ul className="a-groups">{groups.map((g) => (
										<li key={g.id}><Avatar id={g.id} name={g.title} photo={g.photo} size={28} square /><Link to={`/groups/${g.id}`} className="a-grow a-rowlink">{g.title}</Link><span className="num a-muted">{fmt(g.msgs)}</span></li>
									))}</ul>
								) : <p className="a-muted">No group messages logged.</p>}
							</Card>
						</div>
					</div>
				</div>
			) : (
				<div className="a-history">
					<div className="a-seg" role="tablist">
						{(['all', 'name', 'username', 'bio', 'phone', 'photo'] as const).map((k) => (
							<button key={k} type="button" role="tab" aria-selected={kind === k} className={kind === k ? 'on' : ''} onClick={() => setKind(k)}>{k === 'all' ? 'All' : k[0]!.toUpperCase() + k.slice(1)}</button>
						))}
					</div>
					<ol className="a-timeline">{shown.map((e, i) => <HistItem key={i} e={e} v={v} />)}</ol>
					{!shown.length && <Empty>No changes of this kind recorded{h.hasMore ? ' in these pages' : ''}.</Empty>}
					<OlderChanges h={h} />
				</div>
			)}
		</Shell>
	);
}

export const userRoute: RouteDef<UserProps> = {
	Page: UserPage,
	head: (page) => ({ title: `${page.props.user.name} - tgloggerd`, robots: 'noindex' }),
};
