import { useEffect, useRef, useState } from 'react';
import { Users, UsersRound, MessagesSquare, MessageCircle, FileText, Activity, HardDrive, ArrowLeftRight, Trash2 } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, Card, PageHead } from '../ui/primitives.tsx';
import { Bar } from '../ui/bits.tsx';
import { MessagesChart } from './dashboard-chart.tsx';
import { ago, compact, fmt } from '../format.ts';
import { savePrefs } from '../api/prefs.ts';
import { dashboardKpis, type KPI } from '../api/dashboard.ts';
import { useLive } from '../live/react.tsx';
import type { LiveEvent } from '../live/events.ts';
import type { PageData } from '../page.ts';
import type { GroupSummary, UserSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

// Counts cover only what the viewer may see; the server leaves out
// what does not apply (logger status for members, private messages
// when hidden).
export type DashboardProps = S['DashboardProps'];
type NumberFormat = NonNullable<S['Viewer']['dashboard_numbers']>;

const KPI = {
	users: ['Users', '/users', Users], groups: ['Groups', '/groups', UsersRound],
	group_messages: ['Group messages', '/group_messages', MessagesSquare],
	private_messages: ['Private messages', '/private_messages', MessageCircle],
	files: ['Files', '/files', FileText],
} as const;

// Exact or compact counts: the account's choice, which the server
// renders with, saved on change. A failed save keeps the choice for this
// page and says so.
function NumbersToggle({ value, onChange }: { value: NumberFormat; onChange: (f: NumberFormat) => void }) {
	const [err, setErr] = useState(false);
	const pick = (f: NumberFormat) => {
		if (f === value) return;
		onChange(f);
		setErr(false);
		savePrefs({ dashboard_numbers: f }).catch(() => setErr(true));
	};
	return (
		<>
			{err && <span className="a-muted sm" role="status">Not saved; shown on this page only.</span>}
			<div className="a-seg a-seg-text" role="group" aria-label="Numbers">
				<button type="button" className={value === 'exact' ? 'on' : ''} aria-pressed={value === 'exact'} onClick={() => pick('exact')} title="Every digit, as 466,123">Exact</button>
				<button type="button" className={value === 'compact' ? 'on' : ''} aria-pressed={value === 'compact'} onClick={() => pick('compact')} title="Rounded, as 466k">Compact</button>
			</div>
		</>
	);
}

// How often the counts are fetched again while the page is visible: the
// server caches them for as long, so sooner brings nothing new.
export const KPI_REFRESH_MS = 60_000;

// The counts as they stand: from the page data, one up for each new
// message the stream announces (a group's when its chat id is negative,
// a private chat's when positive), and replaced by the server's counts
// every minute while the page is visible and after a resync. Users,
// groups and files change with those refreshes only. A refresh of the
// page in place leaves them alone: its counts are the same cached ones,
// and would undo the messages counted since.
function useKpis(first: KPI[]): KPI[] {
	const [kpis, setKpis] = useState(first);
	const last = useRef(Date.now());
	const seq = useRef(0);
	const refetch = useRef<() => void>(() => {});
	refetch.current = () => {
		const n = ++seq.current;
		last.current = Date.now();
		dashboardKpis().then((k) => { if (n === seq.current) setKpis(k); }, () => { /* keep what is shown */ });
	};
	useLive(['messages'], (e: LiveEvent) => {
		if (e.kind === 'resync') { refetch.current(); return; }
		if (e.kind !== 'message.new' || !e.chat_id) return;
		const key = e.chat_id < 0 ? 'group_messages' : 'private_messages';
		setKpis((ks) => ks.map((k) => (k.key === key ? { ...k, total: k.total + 1, today: k.today + 1 } : k)));
	});
	useEffect(() => {
		const tick = () => { if (!document.hidden) refetch.current(); };
		const t = setInterval(tick, KPI_REFRESH_MS);
		// Back from a hidden tab: catch up if a refresh was missed.
		const shown = () => { if (!document.hidden && Date.now() - last.current >= KPI_REFRESH_MS) refetch.current(); };
		document.addEventListener('visibilitychange', shown);
		return () => { clearInterval(t); document.removeEventListener('visibilitychange', shown); seq.current++; };
	}, []);
	return kpis;
}

function Dashboard({ page }: { page: PageData<DashboardProps> }) {
	const p = page.props;
	const v = page.viewer;
	const member = v.account?.role === 'member';
	const [numbers, setNumbers] = useState<NumberFormat>(v.dashboard_numbers ?? 'exact');
	const show = numbers === 'exact' ? fmt : compact;
	const kpis = useKpis(p.kpis);
	const maxBusy = Math.max(1, ...p.busiest.map((g) => g.msg_count));
	return (
		<Shell crumbs={[['Dashboard', null]]}>
			<PageHead title="Dashboard" sub={member ? 'Activity in the groups granted to you.' : 'What the logger has archived, and what it is doing now.'} actions={<NumbersToggle value={numbers} onChange={setNumbers} />} />
			<div className="a-kpis">
				{kpis.map((k) => {
					const [label, href, I] = KPI[k.key];
					return (
						<Link key={k.key} to={href} className="a-card a-kpi">
							<span className="a-kpi-top"><I size={15} />{label}</span>
							<b className="num" title={`${fmt(k.total)} ${label.toLowerCase()}`}>{show(k.total)}</b>
							<span className="a-kpi-delta">+{fmt(k.today)} today</span>
						</Link>
					);
				})}
			</div>
			<div className={`a-dash ${p.status ? '' : 'member'}`}>
				<MessagesChart viewer={v} first={p.series} />
				{p.status && (
					<Card title="Logger status" className="a-status">
						<ul className="a-statuslist">
							<li><Activity size={15} /><span className="a-grow">Telegram session</span><span className={p.status.session === 'connected' ? 'a-ok' : 'a-warn'}>{p.status.session}</span></li>
							<li><MessagesSquare size={15} /><span className="a-grow">Last message logged</span><span>{p.status.last_message_at ? ago(p.status.last_message_at, v) : 'none yet'}</span></li>
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
