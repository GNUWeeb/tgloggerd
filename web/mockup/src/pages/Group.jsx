import { useState } from 'react';
import {
	MessageCircle, ExternalLink, Type, AtSign, Quote, Image, ShieldCheck, Hash,
	CalendarDays, Users as UsersIcon, Globe, History as HistoryIcon, Crown, EyeOff,
} from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, Card, Stat, Tabs, Seg, Empty, KV, Btn, Sparkline } from '../ui/primitives.jsx';
import { Timeline, MonthTimeline, PhotoSwatch } from '../ui/Timeline.jsx';
import { UserCell, Bar } from './Users.jsx';
import { CopyId } from './User.jsx';
import { TYPE_LABEL } from './Groups.jsx';
import { getGroup, getAdmins, getGroupHistory, getTopPosters } from '../data/groups.js';
import { getActivity } from '../data/users.js';
import { fmt, ago, compact } from '../data/seed.js';
import NotFound from './NotFound.jsx';
import { useViewer } from '../viewer.jsx';

const KIND = {
	title: [Type, 'Title changed'],
	description: [Quote, 'Description changed'],
	username: [AtSign, 'Username'],
	photo: [Image, 'Group photo'],
	admin: [ShieldCheck, 'Admin'],
};

function histItem(e) {
	const [icon, title] = KIND[e.kind];
	let body;
	if (e.kind === 'photo') body = e.removed ? <span className="a-muted">Photo removed</span> : <PhotoSwatch hue={e.hue} />;
	else if (e.kind === 'username') body = <span className={e.action === 'removed' ? 'strike' : ''}>@{e.detail}</span>;
	else if (e.kind === 'admin') body = <><Link to={`/users/${e.user.id}`} className="a-rowlink">{e.user.name}</Link> <span className="a-muted">{e.action}</span></>;
	else body = e.detail || <span className="a-muted">(cleared)</span>;
	return {
		icon, at: e.at, body,
		tone: e.kind === 'admin' ? 'accent' : e.kind === 'photo' ? 'info' : '',
		title: e.kind === 'username' ? `Username ${e.action}` : e.kind === 'admin' ? `Admin ${e.action}` : title,
	};
}

function Admins({ g }) {
	const admins = getAdmins(g);
	return (
		<div className="a-card a-tablecard">
			<div className="a-tablescroll">
				<table className="a-table">
					<thead><tr><th>Admin</th><th>Role</th><th>Title</th><th>Permissions</th><th className="a-hide-md">Since</th></tr></thead>
					<tbody>
						{admins.map((a) => (
							<tr key={a.user.id}>
								<td><UserCell u={a.user} /></td>
								<td>
									<Pill tone={a.status === 'creator' ? 'accent' : ''}>{a.status === 'creator' && <Crown size={11} />}{a.status}</Pill>
									{a.is_anonymous && <Pill tone="muted"><EyeOff size={11} />anonymous</Pill>}
								</td>
								<td>{a.custom_title || <span className="a-muted">-</span>}</td>
								<td className="a-wrapcell">
									{a.status === 'creator' ? <span className="a-muted">All permissions</span>
										: a.perms.length ? a.perms.map((p) => <Pill key={p}>{p}</Pill>) : <span className="a-muted">None</span>}
								</td>
								<td className="a-hide-md" title={a.since}>{ago(a.since)}</td>
							</tr>
						))}
					</tbody>
				</table>
			</div>
			<ul className="a-list">
				{admins.map((a) => (
					<li key={a.user.id}>
						<Link to={`/users/${a.user.id}`}>
							<Avatar e={a.user} size={36} />
							<span className="a-list-main">
								<span className="a-uname">{a.user.name}{a.custom_title && <span className="a-muted"> . {a.custom_title}</span>}</span>
								<span className="a-handle">{a.status === 'creator' ? 'creator, all permissions' : `${a.perms.length} permissions`}</span>
							</span>
						</Link>
					</li>
				))}
			</ul>
		</div>
	);
}

export default function Group({ id, tab }) {
	const g = getGroup(id);
	const v = useViewer();
	const [kind, setKind] = useState('all');
	if (!g || !v.group(g)) return <NotFound what="group" />;
	const hist = getGroupHistory(g);
	const posters = getTopPosters(g);
	const act = getActivity(-g.id, g.msg_count / 40);
	const shown = kind === 'all' ? hist : hist.filter((e) => e.kind === kind);
	const base = `/groups/${g.id}`;
	const crumbs = [['Groups', '/groups'], [g.title, tab === 'group' ? null : base]];
	if (tab === 'admins') crumbs.push(['Admins', null]);
	if (tab === 'history') crumbs.push(['History', null]);
	return (
		<Shell crumbs={crumbs}>
			<div className="a-profilehead">
				<Avatar e={g} size={64} square />
				<div className="a-ph-main">
					<h1>{g.title}</h1>
					<div className="a-ph-meta">
						{g.username && <span>@{g.username}</span>}
						<CopyId id={g.id} />
						<Pill tone={g.type === 'channel' ? 'info' : ''}>{TYPE_LABEL[g.type]}</Pill>
						{g.exposed && <Pill tone="ok"><Globe size={11} />public</Pill>}
					</div>
				</div>
				<div className="a-head-actions">
					<Btn primary to={`${base}/chat`} icon={MessageCircle}>Open chat</Btn>
					{g.username && <Btn ghost icon={ExternalLink}>Open in Telegram</Btn>}
				</div>
			</div>
			{g.exposed && (
				<div className="a-banner">
					<Globe size={15} />
					<span>This group is exposed through MCP, so its chat and files are also readable on the web without signing in. <Link to="/mcp-admin/groups" className="a-link">Manage exposure</Link></span>
				</div>
			)}

			<Tabs items={[
				['Overview', base, tab === 'group'],
				['Admins', `${base}/admins`, tab === 'admins', g.admin_count],
				['History', `${base}/history`, tab === 'history', hist.length],
				['Messages', `/group_messages?f=${encodeURIComponent(JSON.stringify([{ field: 'chat_id', op: '=', value: String(g.id) }]))}`, false, compact(g.msg_count)],
			]} />

			{tab === 'group' && (
				<div className="a-overview">
					<div className="a-stats">
						<Stat label="Messages" value={fmt(g.msg_count)} />
						<Stat label="Members" value={fmt(g.members)} />
						<Stat label="First seen" value={ago(g.created_at)} sub={g.created_at.slice(0, 10)} />
						<Stat label="Last 30 days" value={fmt(act.reduce((a, b) => a + b, 0))}><Sparkline data={act} className="a-spark" /></Stat>
					</div>
					<div className="a-cols">
						<div className="a-stack">
							<Card title="Details">
								<KV rows={[
									[Type, 'Title', g.title],
									[AtSign, 'Username', g.username ? '@' + g.username : ''],
									[Quote, 'Description', g.description],
									[UsersIcon, 'Type', TYPE_LABEL[g.type]],
									[ShieldCheck, 'Admins', <Link to={`${base}/admins`} className="a-rowlink">{g.admin_count} admins</Link>],
									[Hash, 'Group ID', g.id, true],
									[CalendarDays, 'First seen', g.created_at, true],
									[CalendarDays, 'Updated', g.updated_at, true],
								]} />
							</Card>
							<Card title="Recent changes" action={<Link to={`${base}/history`} className="a-link">View all</Link>}>
								<Timeline compact items={hist.slice(0, 4).map(histItem)} />
							</Card>
						</div>
						<Card title="Most active members">
							<ul className="a-groups">
								{posters.map(({ user, msgs }) => (
									<li key={user.id}>
										<Avatar e={user} size={28} />
										<Link to={`/users/${user.id}`} className="a-grow a-rowlink a-ellipsis">{user.name}</Link>
										<Bar value={msgs} max={posters[0].msgs} />
										<span className="num a-muted a-numcell">{fmt(msgs)}</span>
									</li>
								))}
							</ul>
						</Card>
					</div>
				</div>
			)}
			{tab === 'admins' && <Admins g={g} />}
			{tab === 'history' && (
				<div className="a-history">
					<Seg value={kind} onChange={setKind} items={[['all', 'All'], ['title', 'Title'], ['description', 'Description'], ['username', 'Username'], ['photo', 'Photo'], ['admin', 'Admins']]} />
					<MonthTimeline items={shown.map(histItem)} />
					{!shown.length && <Empty>No changes of this kind recorded.</Empty>}
					<div className="a-loadmore"><HistoryIcon size={14} />Older changes load as you scroll</div>
				</div>
			)}
		</Shell>
	);
}
