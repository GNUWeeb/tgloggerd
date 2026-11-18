import { useState } from 'react';
import { MessageCircle, Type, AtSign, Quote, Image, ShieldCheck, Hash, CalendarDays, Users as UsersIcon, Globe, Crown, EyeOff, History as HistoryIcon, Pin, PinOff } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { Avatar, Btn, Card, Empty, KV, Pill, Stat, Tabs } from '../ui/primitives.tsx';
import { Bar, Sparkline, UserCell } from '../ui/bits.tsx';
import { MonthTimeline, Timeline, type TlItem } from '../ui/timeline.tsx';
import { TYPE_LABEL } from './groups.tsx';
import { ago, compact, dateTime, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { GroupAdmin, GroupFull, GroupHistoryEntry, UserSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type GroupProps = S['GroupProps'];

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

function histItem(e: GroupHistoryEntry): TlItem {
	const [icon, title] = KIND[e.kind];
	let body;
	if (e.kind === 'photo') body = e.removed || !e.photo ? <span className="a-muted">Photo removed</span> : <img className="a-tl-photo" src={e.photo} alt="" width={72} height={72} />;
	else if (e.kind === 'username') body = <span className={e.action === 'removed' ? 'strike' : ''}>@{e.detail}</span>;
	else if (e.kind === 'admin' && e.user) body = <><Link to={`/users/${e.user.id}`} className="a-rowlink">{e.user.name}</Link> <span className="a-muted">{e.action}</span></>;
	else body = e.detail || <span className="a-muted">(cleared)</span>;
	return {
		icon, at: e.at, body, tone: e.kind === 'admin' ? 'accent' : e.kind === 'photo' ? 'info' : '',
		title: e.kind === 'username' ? `Username ${e.action}` : e.kind === 'admin' ? `Admin ${e.action}` : title,
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

function Group({ page }: { page: PageData<GroupProps> }) {
	const { group: g, admins, history, top_posters, activity } = page.props;
	const v = page.viewer;
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
				<Avatar id={g.id} name={g.title} photo={g.photo} size={64} square />
				<div className="a-ph-main">
					<h1>{g.title}</h1>
					<div className="a-ph-meta">
						{g.username && <span>@{g.username}</span>}
						<span className="mono">{g.id}</span>
						<Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill>
						{g.exposed && <Pill tone="ok"><Globe size={11} />public</Pill>}
					</div>
				</div>
				<div className="a-head-actions"><PinButton id={g.id} title={g.title} /><Btn primary to={`${base}/chat`} icon={MessageCircle}>Open chat</Btn></div>
			</div>
			{g.exposed && <div className="a-banner"><Globe size={15} /><span>This group is exposed through MCP, so its chat and files are also readable on the web without signing in.</span></div>}
			<Tabs items={[
				['Overview', base, tab === 'group'],
				['Admins', `${base}/admins`, tab === 'admins', g.admin_count],
				['History', `${base}/history`, tab === 'history', history.length],
				['Messages', msgsHref, false, compact(g.msg_count)],
			]} />
			{tab === 'group' && (
				<div className="a-overview">
					<div className="a-stats">
						<Stat label="Messages" value={fmt(g.msg_count)} />
						<Stat label="Members" value={fmt(g.members)} />
						<Stat label="First seen" value={ago(g.created_at, v)} sub={dateTime(g.created_at, v)} />
						<div className="a-card a-stat a-stat-spark"><span className="a-muted">Last 30 days</span><b className="num">{fmt(activity.reduce((a, b) => a + b, 0))}</b><span className="a-spark"><Sparkline data={activity} /></span></div>
					</div>
					<div className="a-cols">
						<div className="a-stack">
							<Card title="Details">
								<KV rows={[
									[Type, 'Title', g.title], [AtSign, 'Username', g.username ? '@' + g.username : ''], [Quote, 'Description', g.description],
									[UsersIcon, 'Type', TYPE_LABEL[g.type]],
									[ShieldCheck, 'Admins', <Link to={`${base}/admins`} className="a-rowlink">{g.admin_count} admins</Link>],
									[Hash, 'Group ID', g.id, true], [CalendarDays, 'First seen', dateTime(g.created_at, v)], [CalendarDays, 'Updated', dateTime(g.updated_at, v)],
								]} />
							</Card>
							<Card title="Recent changes" action={<Link to={`${base}/history`} className="a-link">View all</Link>}>
								{history.length ? <Timeline compact items={history.slice(0, 4).map(histItem)} v={v} /> : <p className="a-muted">No changes recorded.</p>}
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
					<MonthTimeline items={shown.map(histItem)} v={v} />
					{!shown.length && <Empty>No changes of this kind recorded.</Empty>}
					<div className="a-loadmore"><HistoryIcon size={14} />Older changes load as you scroll</div>
				</div>
			)}
		</Shell>
	);
}

export const groupRoute: RouteDef<GroupProps> = {
	Page: Group,
	head: (p) => ({
		title: `${p.props.group.title} - tgloggerd`,
		description: p.props.group.description || undefined,
		robots: p.props.group.exposed ? undefined : 'noindex',
	}),
};
