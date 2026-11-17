import { Users, UsersRound, MessagesSquare, MessageCircle, FileText, Activity, HardDrive, ArrowLeftRight, Trash2 } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, Card, PageHead } from '../ui/primitives.tsx';
import { Bar, Sparkline } from '../ui/bits.tsx';
import { ago, compact, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { GroupSummary, UserSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

// Counts cover only what the viewer may see; the server leaves out
// what does not apply (logger status for members, private messages
// when hidden).
export type DashboardProps = S['DashboardProps'];

const KPI = {
	users: ['Users', '/users', Users], groups: ['Groups', '/groups', UsersRound],
	group_messages: ['Group messages', '/group_messages', MessagesSquare],
	private_messages: ['Private messages', '/private_messages', MessageCircle],
	files: ['Files', '/files', FileText],
} as const;

function Dashboard({ page }: { page: PageData<DashboardProps> }) {
	const p = page.props;
	const v = page.viewer;
	const member = v.account?.role === 'member';
	const maxBusy = Math.max(1, ...p.busiest.map((g) => g.msg_count));
	return (
		<Shell crumbs={[['Dashboard', null]]}>
			<PageHead title="Dashboard" sub={member ? 'Activity in the groups granted to you.' : 'What the logger has archived, and what it is doing now.'} />
			<div className="a-kpis">
				{p.kpis.map((k) => {
					const [label, href, I] = KPI[k.key];
					return (
						<Link key={k.key} to={href} className="a-card a-kpi">
							<span className="a-kpi-top"><I size={15} />{label}</span>
							<b className="num">{compact(k.total)}</b>
							<span className="a-kpi-delta">+{fmt(k.today)} today</span>
						</Link>
					);
				})}
			</div>
			<div className={`a-dash ${p.status ? '' : 'member'}`}>
				<Card title="Messages logged per day" action={<span className="a-muted sm">last 30 days</span>} className="a-dash-wide">
					<div className="a-bigchart"><Sparkline data={p.per_day} bars height={140} /></div>
				</Card>
				{p.status && (
					<Card title="Logger status" className="a-status">
						<ul className="a-statuslist">
							<li><Activity size={15} /><span className="a-grow">Telegram session</span><span className={p.status.session === 'connected' ? 'a-ok' : 'a-warn'}>{p.status.session}</span></li>
							<li><MessagesSquare size={15} /><span className="a-grow">Last message logged</span><span>{ago(p.status.last_message_at, v)}</span></li>
							<li><ArrowLeftRight size={15} /><span className="a-grow">Forwarding routes</span><span>{p.status.forwarding_active} active</span></li>
							<li><HardDrive size={15} /><span className="a-grow">Media store</span><span>{p.status.store_used} of {p.status.store_total}</span></li>
						</ul>
						<div className="a-diskbar"><span style={{ width: `${Math.round(p.status.store_ratio * 100)}%` }} /></div>
					</Card>
				)}
				<Card className="a-d-busy" title="Busiest groups" action={<Link to="/groups" className="a-link">All groups</Link>}>
					<ul className="a-groups">{p.busiest.map((g) => (
						<li key={g.id}>
							<Avatar id={g.id} name={g.title} photo={g.photo} size={28} square />
							<Link to={`/groups/${g.id}`} className="a-grow a-rowlink a-ellipsis">{g.title}</Link>
							<Bar value={g.msg_count} max={maxBusy} />
							<span className="num a-muted a-numcell">{compact(g.msg_count)}</span>
						</li>
					))}</ul>
				</Card>
				<Card className="a-d-deleted" title="Recently deleted">
					<ul className="a-dellist">{p.deleted.map((m) => (
						<li key={m.id}>
							<Trash2 size={14} className="a-delicon" />
							<div className="a-grow a-minw">
								<div className="a-ellipsis"><b>{m.sender.name}</b> <span className="a-muted">in {m.chat.title}</span></div>
								<Link to={`/messages/${m.scope}/${m.id}`} className="a-ellipsis a-delsnip">{m.snippet}</Link>
							</div>
							<span className="a-muted sm">{ago(m.deleted_at, v)}</span>
						</li>
					))}</ul>
				</Card>
			</div>
		</Shell>
	);
}

export const dashboardRoute: RouteDef<DashboardProps> = {
	Page: Dashboard,
	head: () => ({ title: 'Dashboard - tgloggerd', robots: 'noindex' }),
};
