import { useState } from 'react';
import {
	BadgeCheck, Copy, MessageCircle, ExternalLink, Type, AtSign, Quote, Phone,
	Image, Languages, Star, CalendarDays, History as HistoryIcon, Hash,
} from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Flags, Pill, Card, Stat, Tabs, Seg, Empty, KV, Btn, Sparkline } from '../ui/primitives.jsx';
import { Timeline, MonthTimeline, PhotoSwatch } from '../ui/Timeline.jsx';
import { getUser, getUserHistory, getUsernames, userFlags, getActivity } from '../data/users.js';
import { getGroupsOfUser } from '../data/groups.js';
import { fmt, ago } from '../data/seed.js';
import NotFound from './NotFound.jsx';

const KIND = {
	name: [Type, 'Name changed'],
	username: [AtSign, 'Username'],
	bio: [Quote, 'Bio changed'],
	phone: [Phone, 'Phone changed'],
	photo: [Image, 'Profile photo'],
};

export function histItem(e) {
	const [icon, title] = KIND[e.kind];
	let body;
	if (e.kind === 'photo') body = e.removed ? <span className="a-muted">Photo removed</span> : <PhotoSwatch hue={e.hue} />;
	else if (e.kind === 'username') body = <span className={e.action === 'removed' ? 'strike' : ''}>@{e.detail}</span>;
	else body = e.detail || <span className="a-muted">(cleared)</span>;
	return {
		icon, at: e.at, body,
		tone: e.kind === 'username' ? 'accent' : e.kind === 'photo' ? 'info' : '',
		title: e.kind === 'username' ? `Username ${e.action}` : title,
	};
}

export function CopyId({ id }) {
	return <span className="mono">{id} <button type="button" className="a-icon-btn sm" title="Copy ID" aria-label="Copy ID"><Copy size={12} /></button></span>;
}

export default function User({ id, tab }) {
	const u = getUser(id);
	const [kind, setKind] = useState('all');
	if (!u) return <NotFound what="user" />;
	const hist = getUserHistory(u);
	const groups = getGroupsOfUser(u);
	const act = getActivity(u.id, u.msg_count);
	const names = getUsernames(u);
	const shown = kind === 'all' ? hist : hist.filter((e) => e.kind === kind);
	const crumbs = [['Users', '/users'], [u.name, tab === 'history' ? `/users/${u.id}` : null]];
	if (tab === 'history') crumbs.push(['History', null]);
	return (
		<Shell crumbs={crumbs}>
			<div className="a-profilehead">
				<Avatar e={u} size={64} />
				<div className="a-ph-main">
					<h1>{u.name} {u.is_verified && <BadgeCheck size={18} className="a-verified" aria-label="verified" />}</h1>
					<div className="a-ph-meta">
						{u.username && <span>@{u.username}</span>}
						<CopyId id={u.id} />
						<Flags flags={userFlags(u)} />
					</div>
				</div>
				<div className="a-head-actions">
					<Btn primary to={`/users/${u.id}/chat`} icon={MessageCircle}>Private chat</Btn>
					{u.username && <Btn ghost icon={ExternalLink}>Open in Telegram</Btn>}
				</div>
			</div>

			<Tabs items={[
				['Overview', `/users/${u.id}`, tab === 'user'],
				['History', `/users/${u.id}/history`, tab === 'history', hist.length],
				['Messages', `/group_messages?f=${encodeURIComponent(JSON.stringify([{ field: 'sender_id', op: '=', value: String(u.id) }]))}`, false, fmt(u.msg_count)],
				['Files', `/files?f=${encodeURIComponent(JSON.stringify([{ field: 'sender_id', op: '=', value: String(u.id) }]))}`, false],
			]} />

			{tab === 'user' ? (
				<div className="a-overview">
					<div className="a-stats">
						<Stat label="Messages" value={fmt(u.msg_count)} />
						<Stat label="Groups" value={groups.length} />
						<Stat label="First seen" value={ago(u.created_at)} sub={u.created_at.slice(0, 10)} />
						<Stat label="Last 30 days" value={fmt(act.reduce((a, b) => a + b, 0))}><Sparkline data={act} className="a-spark" /></Stat>
					</div>
					<div className="a-cols">
						<Card title="Details">
							<KV rows={[
								[Type, 'First name', u.first_name],
								[Type, 'Last name', u.last_name],
								[AtSign, 'Usernames', names.length ? names.map((n) => <Pill key={n.username} tone={n.kind === 'active' ? 'accent' : 'muted'}>@{n.username}</Pill>) : ''],
								[Quote, 'Bio', u.bio],
								[Phone, 'Phone', u.phone, true],
								[Languages, 'Language', u.language],
								[Star, 'Type', u.type],
								[Hash, 'User ID', u.id, true],
								[CalendarDays, 'First seen', u.created_at, true],
								[CalendarDays, 'Updated', u.updated_at, true],
							]} />
						</Card>
						<div className="a-stack">
							<Card title="Recent changes" action={<Link to={`/users/${u.id}/history`} className="a-link">View all</Link>}>
								{hist.length ? <Timeline compact items={hist.slice(0, 4).map(histItem)} /> : <p className="a-muted">No changes recorded.</p>}
							</Card>
							<Card title="Most active in">
								{groups.length ? (
									<ul className="a-groups">
										{groups.map((g) => (
											<li key={g.id}>
												<Avatar e={g} size={28} square />
												<Link to={`/groups/${g.id}`} className="a-grow a-rowlink">{g.title}</Link>
												<span className="num a-muted">{fmt(g.msgs)}</span>
											</li>
										))}
									</ul>
								) : <p className="a-muted">No group messages logged.</p>}
							</Card>
						</div>
					</div>
				</div>
			) : (
				<div className="a-history">
					<Seg value={kind} onChange={setKind} items={[['all', 'All'], ['name', 'Name'], ['username', 'Username'], ['bio', 'Bio'], ['phone', 'Phone'], ['photo', 'Photo']]} />
					<MonthTimeline items={shown.map(histItem)} />
					{!shown.length && <Empty>No changes of this kind recorded.</Empty>}
					<div className="a-loadmore"><HistoryIcon size={14} />Older changes load as you scroll</div>
				</div>
			)}
		</Shell>
	);
}
