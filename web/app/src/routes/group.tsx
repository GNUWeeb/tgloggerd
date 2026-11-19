import { useCallback, useState } from 'react';
import { MessageCircle, Type, AtSign, Quote, Image, ShieldCheck, Hash, CalendarDays, Users as UsersIcon, Globe, Crown, EyeOff, Gem, Pin, PinOff } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { PublicFrame } from '../ui/PublicFrame.tsx';
import { Link, useNav } from '../nav.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { PhotoCarousel } from '../ui/PhotoCarousel.tsx';
import { OlderChanges, UsernamePill, useHistoryPages, usernameTitle } from '../ui/history.tsx';
import { Avatar, Btn, Card, Empty, KV, Pill, Stat, Tabs } from '../ui/primitives.tsx';
import { Bar, Sparkline, UserCell } from '../ui/bits.tsx';
import { MonthTimeline, Timeline, type TlItem } from '../ui/timeline.tsx';
import { TYPE_LABEL } from './groups.tsx';
import { ago, compact, dateTime, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { GroupAdmin, GroupFull, GroupHistoryEntry, UserSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import type { MemberCount, MemberListFlags } from '../api/members.ts';

// The member count and the list's switch are local until the API
// description has them (api/members.ts).
export type GroupProps = Omit<S['GroupProps'], 'group'> & MemberListFlags & { group: S['GroupProps']['group'] & MemberCount };
// An exposed group to a viewer whose scope does not see it (signed out,
// a member it is not granted to, an admin it is hidden from): the group
// alone, without admins, history, posters, activity or files.
export type GroupPublicProps = Omit<S['GroupPublicProps'], 'group'> & { group: S['GroupPublicProps']['group'] & MemberCount };
export type GroupPageProps = GroupProps | GroupPublicProps;
const isPublic = (p: GroupPageProps): p is GroupPublicProps => 'public' in p && p.public === true;

const KIND = {
	title: [Type, 'Title changed'], description: [Quote, 'Description changed'], username: [AtSign, 'Username'],
	photo: [Image, 'Group photo'], admin: [ShieldCheck, 'Admin'],
} as const;

// Pin or unpin the group in the sidebar. The sidebar follows once the
// page data (and its viewer.pinned) is fetched again.
function PinButton({ id, title }: { id: number; title: string }) {
	const { viewer, refresh } = useNav();
	const server = !!viewer.pinned?.some((p) => p.id === id);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	if (!viewer.signed_in) return null;
	const toggle = async () => {
		setBusy(true); setErr('');
		try {
			const req = { params: { path: { group_id: id } } };
			await ok(server ? api.DELETE('/api/account/pinned/{group_id}', req) : api.PUT('/api/account/pinned/{group_id}', req));
			await refresh();
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); } finally { setBusy(false); }
	};
	return (
		<>
			<Btn ghost icon={server ? PinOff : Pin} disabled={busy} aria-pressed={server} title={server ? `Unpin ${title} from the sidebar` : `Pin ${title} to the sidebar`} onClick={() => void toggle()}>
				{server ? 'Unpin' : 'Pin'}
			</Btn>
			{/* For example the cap on pinned groups (too_many). */}
			{err && <span className="a-pinerr" role="alert">{err}</span>}
		</>
	);
}

function histItem(e: GroupHistoryEntry, onPhoto?: (id: number) => void): TlItem {
	const [icon, title] = KIND[e.kind];
	let body;
	if (e.kind === 'photo') {
		const img = e.photo ? <img className="a-tl-photo" src={e.photo} alt="" width={72} height={72} /> : null;
		// A photo set opens the carousel at it.
		body = e.removed || !img ? <span className="a-muted">Photo removed</span>
			: e.photo_id !== undefined && onPhoto ? <button type="button" className="a-tl-photobtn" onClick={() => onPhoto(e.photo_id!)} aria-label="Show this photo">{img}</button> : img;
	}
	else if (e.kind === 'username') body = <span className={e.action === 'removed' ? 'strike' : ''}>{e.collectible && <Gem size={12} aria-label="collectible" />}@{e.detail}{e.username_kind === 'disabled' ? ' (off)' : ''}</span>;
	else if (e.kind === 'admin' && e.user) body = <><Link to={`/users/${e.user.id}`} className="a-rowlink">{e.user.name}</Link> <span className="a-muted">{e.action}</span></>;
	else body = e.detail ? <span className="a-multiline">{e.detail}</span> : <span className="a-muted">(cleared)</span>;
	return {
		icon, at: e.at, body, tone: e.kind === 'admin' ? 'accent' : e.kind === 'photo' ? 'info' : '',
		title: e.kind === 'username' ? usernameTitle(e) : e.kind === 'admin' ? `Admin ${e.action}${e.status === 'creator' ? ' (owner)' : ''}` : title,
	};
}

function Admins({ admins, v }: { admins: GroupAdmin[]; v: PageData['viewer'] }) {
	return (
		<div className="a-card a-tablecard">
			<div className="a-tablescroll">
				<table className="a-table">
					<thead><tr><th>Admin</th><th>Role</th><th>Title</th><th>Permissions</th><th className="a-hide-md">Since</th></tr></thead>
					<tbody>{admins.map((a) => (
						<tr key={a.user.id}>
							<td><UserCell u={a.user} /></td>
							<td>
								<Pill tone={a.status === 'creator' ? 'accent' : ''}>{a.status === 'creator' && <Crown size={11} />}{a.status}</Pill>
								{a.is_anonymous && <Pill tone="muted"><EyeOff size={11} />anonymous</Pill>}
							</td>
							<td>{a.custom_title || <span className="a-muted">-</span>}</td>
							<td className="a-wrapcell">{a.status === 'creator' ? <span className="a-muted">All permissions</span> : a.perms.length ? a.perms.map((p) => <Pill key={p}>{p}</Pill>) : <span className="a-muted">None</span>}</td>
							<td className="a-hide-md" title={a.since}>{ago(a.since, v)}</td>
						</tr>
					))}</tbody>
				</table>
			</div>
			<ul className="a-list">{admins.map((a) => (
				<li key={a.user.id}>
					<Link to={`/users/${a.user.id}`}>
						<Avatar id={a.user.id} name={a.user.name} photo={a.user.photo} size={36} />
						<span className="a-list-main">
							<span className="a-uname">{a.user.name}{a.custom_title && <span className="a-muted"> . {a.custom_title}</span>}</span>
							<span className="a-handle">{a.status === 'creator' ? 'creator, all permissions' : `${a.perms.length} permissions`}</span>
						</span>
					</Link>
				</li>
			))}</ul>
		</div>
	);
}

// The public page of an exposed group: what it is, what it is about,
// and its chat. Signed out it has the public frame; to a signed-in
// viewer without the group, the usual shell.
function PublicGroup({ page }: { page: PageData<GroupPublicProps> }) {
	const g = page.props.group;
	const v = page.viewer;
	const body = (
		<div className="a-publicgroup">
			<div className="a-profilehead">
				<Avatar id={g.id} name={g.title} photo={g.photo} size={64} square />
				<div className="a-ph-main">
					<h1>{g.title}</h1>
					<div className="a-ph-meta">
						{g.usernames.length ? g.usernames.map((n) => <UsernamePill key={n.username} n={n} />) : g.username && <span>@{g.username}</span>}
						<Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill>
						<Pill tone="ok"><Globe size={11} />public</Pill>
					</div>
				</div>
				<div className="a-head-actions"><Btn primary to={`/groups/${g.id}/chat`} icon={MessageCircle}>Open chat</Btn></div>
			</div>
			<div className="a-stats">
				<Stat label="Messages" value={fmt(g.msg_count)} />
				{g.member_count != null && <Stat label="Members" value={fmt(g.member_count)} />}
				<Stat label="Members seen" value={fmt(g.members)} />
				<Stat label="First seen" value={ago(g.created_at, v)} sub={dateTime(g.created_at, v)} />
			</div>
			<Card title="About">
				{g.description ? <p className="a-multiline">{g.description}</p> : <p className="a-muted">No description.</p>}
			</Card>
			<p className="a-muted sm"><Globe size={13} /> This group's chat is public: anyone can read its archive here, without signing in.</p>
		</div>
	);
	if (!v.signed_in) return <PublicFrame viewer={v} url={page.url}><main className="a-content a-publicmain">{body}</main></PublicFrame>;
	return <Shell crumbs={[['Groups', '/groups'], [g.title, null]]}>{body}</Shell>;
}

function GroupPage({ page }: { page: PageData<GroupPageProps> }) {
	const p = page.props;
	return isPublic(p) ? <PublicGroup page={{ ...page, props: p }} /> : <Group page={{ ...page, props: p }} />;
}

function Group({ page }: { page: PageData<GroupProps> }) {
	const { group: g, admins, history: first, history_next, top_posters, activity } = page.props;
	const v = page.viewer;
	const load = useCallback((before: string) => ok(api.GET('/api/groups/{id}/history', { params: { path: { id: g.id }, query: { before } } })), [g.id]);
	const h = useHistoryPages(first, history_next, load);
	const history = h.entries;
	// The photo carousel: undefined is closed, null opens at the current photo.
	const [photos, setPhotos] = useState<number | null | undefined>(undefined);
	const tab = page.route === 'group_admins' ? 'admins' : page.route === 'group_history' ? 'history' : 'group';
	const [kind, setKind] = useState<'all' | GroupHistoryEntry['kind']>('all');
	const shown = kind === 'all' ? history : history.filter((e) => e.kind === kind);
	const base = `/groups/${g.id}`;
	const crumbs: [string, string | null][] = [['Groups', '/groups'], [g.title, tab === 'group' ? null : base]];
	if (tab !== 'group') crumbs.push([tab === 'admins' ? 'Admins' : 'History', null]);
	const msgsHref = `/group_messages?f=${encodeURIComponent(JSON.stringify([{ field: 'chat_id', op: '=', value: String(g.id) }]))}`;
	return (
		<Shell crumbs={crumbs}>
			<div className="a-profilehead">
				{g.photo
					? <button type="button" className="a-photobtn" onClick={() => setPhotos(null)} aria-label={`Show the photos of ${g.title}`}><Avatar id={g.id} name={g.title} photo={g.photo} size={64} square /></button>
					: <Avatar id={g.id} name={g.title} photo={g.photo} size={64} square />}
				<div className="a-ph-main">
					<h1>{g.title}</h1>
					<div className="a-ph-meta">
						{g.username && <span>@{g.username}</span>}
						<span className="mono">{g.id}</span>
						<Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill>
						{g.member_count != null && <span title={g.member_count_at ? `Counted ${dateTime(g.member_count_at, v)}` : undefined}>{fmt(g.member_count)} members</span>}
						{g.exposed && <Pill tone="ok"><Globe size={11} />public</Pill>}
					</div>
				</div>
				<div className="a-head-actions"><PinButton id={g.id} title={g.title} /><Btn primary to={`${base}/chat`} icon={MessageCircle}>Open chat</Btn></div>
			</div>
			{g.exposed && <div className="a-banner"><Globe size={15} /><span>This group is exposed through MCP, so its chat and files are also readable on the web without signing in.</span></div>}
			<Tabs items={[
				['Overview', base, tab === 'group'],
				['Admins', `${base}/admins`, tab === 'admins', g.admin_count],
				['History', `${base}/history`, tab === 'history', h.hasMore ? `${history.length}+` : history.length],
				['Messages', msgsHref, false, compact(g.msg_count)],
			]} />
			{tab === 'group' && (
				<div className="a-overview">
					<div className={g.member_count != null ? 'a-stats a-stats-5' : 'a-stats'}>
						<Stat label="Messages" value={fmt(g.msg_count)} />
						{/* Telegram's count, when known; and the people the archive
						    has seen there (posters and admins). */}
						{g.member_count != null && <Stat label="Members" value={fmt(g.member_count)} />}
						<Stat label="Members seen" value={fmt(g.members)} />
						<Stat label="First seen" value={ago(g.created_at, v)} sub={dateTime(g.created_at, v)} />
						<div className="a-card a-stat a-stat-spark"><span className="a-muted">Last 30 days</span><b className="num">{fmt(activity.reduce((a, b) => a + b, 0))}</b><span className="a-spark"><Sparkline data={activity} /></span></div>
					</div>
					<div className="a-cols">
						<div className="a-stack">
							<Card title="Details">
								<KV rows={[
									[Type, 'Title', g.title],
									[AtSign, g.usernames.length > 1 ? 'Usernames' : 'Username', g.usernames.length ? g.usernames.map((n) => <UsernamePill key={n.username} n={n} />) : ''],
									[Quote, 'Description', g.description ? <span className="a-multiline">{g.description}</span> : ''],
									[UsersIcon, 'Type', TYPE_LABEL[g.type]],
									[ShieldCheck, 'Admins', <Link to={`${base}/admins`} className="a-rowlink">{g.admin_count} admins</Link>],
									g.photo_file_id ? [Image, 'Photo', <Link to={`/files?file=${g.photo_file_id}`} className="a-rowlink">file {g.photo_file_id}</Link>] : null,
									[Hash, 'Group ID', g.id, true], [CalendarDays, 'First seen', dateTime(g.created_at, v)], [CalendarDays, 'Updated', dateTime(g.updated_at, v)],
								]} />
							</Card>
							<Card title="Recent changes" action={<Link to={`${base}/history`} className="a-link">View all</Link>}>
								{history.length ? <Timeline compact items={history.slice(0, 4).map((e) => histItem(e, setPhotos))} v={v} /> : <p className="a-muted">No changes recorded.</p>}
							</Card>
						</div>
						<Card title="Most active members">
							<ul className="a-groups">{top_posters.map(({ user, msgs }) => (
								<li key={user.id}>
									<Avatar id={user.id} name={user.name} photo={user.photo} size={28} />
									<Link to={`/users/${user.id}`} className="a-grow a-rowlink a-ellipsis">{user.name}</Link>
									<Bar value={msgs} max={top_posters[0]?.msgs ?? 1} />
									<span className="num a-muted a-numcell">{fmt(msgs)}</span>
								</li>
							))}</ul>
						</Card>
					</div>
				</div>
			)}
			{tab === 'admins' && <Admins admins={admins} v={v} />}
			{tab === 'history' && (
				<div className="a-history">
					<div className="a-seg" role="tablist">
						{(['all', 'title', 'description', 'username', 'photo', 'admin'] as const).map((k) => (
							<button key={k} type="button" role="tab" aria-selected={kind === k} className={kind === k ? 'on' : ''} onClick={() => setKind(k)}>{k === 'all' ? 'All' : k === 'admin' ? 'Admins' : k[0]!.toUpperCase() + k.slice(1)}</button>
						))}
					</div>
					<MonthTimeline items={shown.map((e) => histItem(e, setPhotos))} v={v} />
					{!shown.length && <Empty>No changes of this kind recorded{h.hasMore ? ' in these pages' : ''}.</Empty>}
					<OlderChanges h={h} />
				</div>
			)}
			{photos !== undefined && <PhotoCarousel kind="groups" id={g.id} start={photos} title={g.title} viewer={v} onClose={() => setPhotos(undefined)} />}
		</Shell>
	);
}

export const groupRoute: RouteDef<GroupPageProps> = {
	Page: GroupPage,
	head: (p) => ({
		title: `${p.props.group.title} - tgloggerd`,
		description: p.props.group.description || undefined,
		robots: p.props.group.exposed ? undefined : 'noindex',
	}),
};
