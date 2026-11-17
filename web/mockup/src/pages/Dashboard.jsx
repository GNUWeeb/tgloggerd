import { Users, UsersRound, MessagesSquare, MessageCircle, FileText, Activity, HardDrive, ArrowLeftRight, Bot, Trash2, ArrowRight } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Card, PageHead, Sparkline } from '../ui/primitives.jsx';
import { Timeline } from '../ui/Timeline.jsx';
import { Bar } from './Users.jsx';
import { histItem } from './User.jsx';
import { snippet, chatHref } from './Messages.jsx';
import { TOTALS } from '../data/totals.js';
import { groups } from '../data/groups.js';
import { users, getUserHistory, getActivity } from '../data/users.js';
import { groupMessages, senderOf } from '../data/messages.js';
import { t2d, d2t, mcpExposed, mcpTokens } from '../data/admin.js';
import { fmt, compact, ago } from '../data/seed.js';
import { useViewer, allowed } from '../viewer.jsx';
import { files } from '../data/files.js';

// [label, href, icon, total, today, page key]. Counts cover only what
// the viewer may see; a member gets counts over its own groups.
function kpis(v) {
	if (v.role === 'member') {
		const gs = groups.filter(v.group);
		const msgs = groupMessages().filter((m) => v.group(m.chat));
		return [
			['People', '/users', Users, users.filter(v.user).length, 2, 'users'],
			['Groups', '/groups', UsersRound, gs.length, 0, 'groups'],
			['Messages', '/group_messages', MessagesSquare, gs.reduce((a, g) => a + g.msg_count, 0), msgs.length, 'group_messages'],
			['Files', '/files', FileText, files().filter((f) => f.msg.scope === 'group' && v.group(f.msg.chat)).length * 1400, 96, 'files'],
		];
	}
	return [
		['Users', '/users', Users, TOTALS.users, 412, 'users'],
		['Groups', '/groups', UsersRound, v.role === 'superadmin' ? TOTALS.groups : TOTALS.groups - 1, 3, 'groups'],
		['Group messages', '/group_messages', MessagesSquare, TOTALS.group_messages, 38211, 'group_messages'],
		['Private messages', '/private_messages', MessageCircle, TOTALS.private_messages, 1290, 'private_messages'],
		['Files', '/files', FileText, TOTALS.files, 2406, 'files'],
	].filter((k) => allowed(v, k[5]));
}

export default function Dashboard() {
	const v = useViewer();
	const KPIS = kpis(v);
	const perDay = getActivity(31337, 38000 * 900 / 30);
	const busiest = groups.slice(0, 12).filter(v.group).sort((a, b) => b.msg_count - a.msg_count).slice(0, 6);
	const changes = users.slice(0, 40).flatMap((u) => getUserHistory(u).slice(0, 1).map((e) => ({ ...histItem(e), u })))
		.sort((a, b) => (a.at < b.at ? 1 : -1)).slice(0, 6);
	const deleted = groupMessages().filter((m) => m.deleted_at && v.group(m.chat)).slice(0, 5);
	return (
		<Shell crumbs={[['Dashboard', null]]}>
			<PageHead title="Dashboard" sub={v.role === 'member' ? 'Activity in the groups granted to you.' : 'What the logger has archived, and what it is doing now.'} />
			<div className="a-kpis">
				{KPIS.map(([label, href, Icon, n, today]) => (
					<Link key={label} to={href} className="a-card a-kpi">
						<span className="a-kpi-top"><Icon size={15} />{label}</span>
						<b className="num">{compact(n)}</b>
						<span className="a-kpi-delta">+{fmt(today)} today</span>
					</Link>
				))}
			</div>
			<div className={`a-dash ${v.role === 'member' ? 'member' : ''}`}>
				<Card title="Messages logged per day" action={<span className="a-muted sm">last 30 days</span>} className="a-dash-wide">
					<div className="a-bigchart"><Sparkline data={perDay} bars height={140} /></div>
					<div className="a-axis"><span>Oct 19</span><span>Nov 2</span><span>today</span></div>
				</Card>
				{v.role !== 'member' && <Card title="Logger status" className="a-status">
					<ul className="a-statuslist">
						<li><Activity size={15} /><span className="a-grow">Telegram session</span><span className="a-ok"><span className="a-livedot" />connected</span></li>
						<li><MessagesSquare size={15} /><span className="a-grow">Last message logged</span><span>4s ago</span></li>
						<li><ArrowLeftRight size={15} /><span className="a-grow">Forwarding routes</span><Link to="/platform-fwd" className="a-rowlink">{t2d.filter((r) => r.enabled).length + d2t.filter((r) => r.enabled).length} active</Link></li>
						<li><Bot size={15} /><span className="a-grow">MCP</span><Link to="/mcp-admin" className="a-rowlink">{mcpExposed.length} groups . {mcpTokens.filter((t) => !t.revoked).length} tokens</Link></li>
						<li className="a-disk"><HardDrive size={15} /><span className="a-grow">Media store</span><span>5.3 of 8 TB</span></li>
					</ul>
					<div className="a-diskbar"><span style={{ width: '66%' }} /></div>
				</Card>}
				<Card className="a-d-busy" title="Busiest groups" action={<Link to="/groups" className="a-link">All groups</Link>}>
					<ul className="a-groups">
						{busiest.map((g) => (
							<li key={g.id}>
								<Avatar e={g} size={28} square />
								<Link to={`/groups/${g.id}`} className="a-grow a-rowlink a-ellipsis">{g.title}</Link>
								<Bar value={g.msg_count} max={busiest[0].msg_count} />
								<span className="num a-muted a-numcell">{compact(g.msg_count)}</span>
							</li>
						))}
					</ul>
				</Card>
				{v.fullUser && <Card className="a-d-changes" title="Recent profile changes" action={<Link to="/users?sort=updated_at" className="a-link">All users</Link>}>
					<Timeline compact items={changes.map((c) => ({ ...c, title: <><Link to={`/users/${c.u.id}`} className="a-rowlink">{c.u.name}</Link> <span className="a-muted">{String(c.title).toLowerCase()}</span></> }))} />
				</Card>}
				<Card className="a-d-deleted" title="Recently deleted" action={<Link to="/group_messages?f=%5B%7B%22field%22%3A%22is_deleted%22%2C%22op%22%3A%22is%22%2C%22value%22%3A%22yes%22%7D%5D" className="a-link">All deleted</Link>}>
					<ul className="a-dellist">
						{deleted.map((m) => {
							const s = senderOf(m);
							return (
								<li key={m.id}>
									<Trash2 size={14} className="a-delicon" />
									<div className="a-grow a-minw">
										<div className="a-ellipsis"><b>{s.name}</b> <span className="a-muted">in {m.chat.title}</span></div>
										<Link to={chatHref(m)} className="a-ellipsis a-delsnip">{snippet(m)}</Link>
									</div>
									<span className="a-muted sm">{ago(m.deleted_at)}</span>
								</li>
							);
						})}
					</ul>
					<Link to="/group_messages" className="a-link a-more">Search messages <ArrowRight size={13} /></Link>
				</Card>
			</div>
		</Shell>
	);
}
