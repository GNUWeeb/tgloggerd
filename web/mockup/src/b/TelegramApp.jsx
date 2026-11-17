// Direction B - "Messenger": looks and feels like Telegram's own apps.
import { useEffect, useRef, useState } from 'react';
import {
	Menu, Search, SlidersHorizontal, ArrowLeft, MessageCircle, History, Copy,
	AtSign, Info, Phone, Languages, CalendarDays, Hash, Users, UsersRound,
	FileText, MessagesSquare, LayoutDashboard, ArrowLeftRight, Bot, Settings,
	ChevronRight, X, Check, BadgeCheck, Star, Image, Type, Quote,
} from 'lucide-react';
import { Link } from '../router.jsx';
import { useUserList, Avatar, GroupAvatar, Sparkline, userFlags, parseSub } from '../shared.jsx';
import { queryUsers, getUser, getHistory, getGroupsOf, getUsernames, getActivity, fmt, compact, ago } from '../data.js';

const NAV = [
	['', 'Dashboard', LayoutDashboard],
	['/users', 'Users', Users],
	['/groups', 'Groups', UsersRound],
	['/group_messages', 'Group messages', MessagesSquare],
	['/private_messages', 'Private messages', MessageCircle],
	['/files', 'Files', FileText],
	['/platform-fwd', 'Platform forwarding', ArrowLeftRight],
	['/mcp-admin', 'MCP', Bot],
	['/settings', 'Settings', Settings],
];

export default function TelegramApp({ base, sub }) {
	const r = parseSub(sub);
	const [drawer, setDrawer] = useState(false);
	const user = r.id ? getUser(r.id) : null;
	return (
		<div className={`b-app ${r.screen !== 'users' ? 'b-has-detail' : ''}`}>
			<div className={`b-drawer ${drawer ? 'open' : ''}`} onClick={() => setDrawer(false)}>
				<nav onClick={(e) => e.stopPropagation()}>
					<div className="b-drawer-head">
						<span className="b-drawer-av">A</span>
						<div><b>admin</b><div className="b-muted">tgloggerd</div></div>
					</div>
					{NAV.map(([h, t, Icon]) => (
						<Link key={t} to={base + h} className={h && sub.startsWith(h) ? 'on' : ''} onClick={() => setDrawer(false)}>
							<Icon size={20} strokeWidth={1.8} />{t}
						</Link>
					))}
				</nav>
			</div>
			<UsersPane base={base} onMenu={() => setDrawer(true)} activeId={user?.id} />
			<section className="b-detail">
				{user && r.screen === 'user' && <Profile base={base} u={user} />}
				{user && r.screen === 'history' && <HistoryView base={base} u={user} />}
				{r.screen === 'users' && (
					<div className="b-placeholder">
						<span className="b-ph-pill">Select a user to see their profile</span>
					</div>
				)}
				{r.screen === 'stub' && <div className="b-placeholder"><span className="b-ph-pill">Not part of the first design round</span></div>}
			</section>
		</div>
	);
}

const QUICK = [
	['All', []],
	['Premium', [{ field: 'is_premium', op: 'is', value: 'yes' }]],
	['Bots', [{ field: 'type', op: '=', value: 'bot' }]],
	['Active', [{ field: 'msg_count', op: '>', value: '1000' }]],
	['Deleted', [{ field: 'type', op: '=', value: 'deleted' }]],
	['Scam / fake', [{ field: 'is_scam', op: 'is', value: 'yes' }]],
];
const SORTS = [['msg_count', 'Most messages'], ['created_at', 'Recently seen first'], ['updated_at', 'Recently updated'], ['name', 'Name']];

function UsersPane({ base, onMenu, activeId }) {
	const L = useUserList({ per: 40 });
	const [q, setQ] = useState(L.q);
	const [sheet, setSheet] = useState(false);
	// Infinite list: everything up to the current page, like a chat list.
	const rows = queryUsers({ ...L, page: 1, per: L.per * L.page }).rows;
	const quick = QUICK.find(([, c]) => JSON.stringify(c) === JSON.stringify(L.conds))?.[0];
	return (
		<aside className="b-list">
			<header className="b-list-head">
				<button className="b-round" onClick={onMenu} aria-label="Menu"><Menu size={20} /></button>
				<form className="b-search" onSubmit={(e) => { e.preventDefault(); L.update({ q }); }}>
					<Search size={17} />
					<input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search users" />
					{q && <button type="button" onClick={() => { setQ(''); L.update({ q: '' }); }} aria-label="Clear"><X size={15} /></button>}
				</form>
				<button className="b-round" onClick={() => setSheet(true)} aria-label="Filters"><SlidersHorizontal size={19} /></button>
			</header>
			<div className="b-folders">
				{QUICK.map(([t, c]) => (
					<button key={t} className={quick === t || (!quick && t === 'All' && !L.conds.length) ? 'on' : ''} onClick={() => L.update({ conds: c })}>{t}</button>
				))}
				{!quick && L.conds.length > 0 && <button className="on">Custom ({L.conds.length})</button>}
			</div>
			<div className="b-count">{fmt(L.total)} users <span>sorted by {SORTS.find((s) => s[0] === L.sort)?.[1].toLowerCase() || L.sort}</span></div>
			<ul className="b-rows">
				{rows.map((u) => (
					<li key={u.id}>
						<Link to={`${base}/users/${u.id}${location.search}`} className={String(activeId) === String(u.id) ? 'on' : ''}>
							<Avatar u={u} size={48} />
							<span className="b-row-main">
								<span className="b-row-top">
									<b>{u.name}</b>
									{u.is_verified && <BadgeCheck size={15} className="b-verified" />}
									{u.is_premium && <Star size={13} className="b-star" />}
									<span className="b-row-time">{ago(u.updated_at)}</span>
								</span>
								<span className="b-row-bottom">
									<span className="b-row-sub">{u.username ? '@' + u.username : u.bio || 'id ' + u.id}</span>
									{u.msg_count > 0 && <span className="b-badge">{compact(u.msg_count)}</span>}
									{(u.is_scam || u.is_fake) && <span className="b-badge danger">{u.is_scam ? 'SCAM' : 'FAKE'}</span>}
								</span>
							</span>
						</Link>
					</li>
				))}
			</ul>
			{L.page < L.pages && <button className="b-more" onClick={() => L.update({ page: L.page + 1 }, { replace: true })}>Show more</button>}

			{sheet && (
				<div className="b-sheet-wrap" onClick={() => setSheet(false)}>
					<div className="b-sheet" onClick={(e) => e.stopPropagation()}>
						<div className="b-sheet-grip" />
						<h3>Sort by</h3>
						{SORTS.map(([k, t]) => (
							<button key={k} className="b-opt" onClick={() => L.update({ sort: k, dir: k === 'name' ? 'asc' : 'desc' })}>
								{t}{L.sort === k && <Check size={18} />}
							</button>
						))}
						<h3>Show only</h3>
						{[['is_premium', 'Premium'], ['is_verified', 'Verified'], ['is_scam', 'Scam'], ['is_fake', 'Fake']].map(([k, t]) => {
							const on = L.conds.some((c) => c.field === k);
							return (
								<label key={k} className="b-opt">
									{t}
									<input type="checkbox" className="b-switch" checked={on}
										onChange={() => L.update({ conds: on ? L.conds.filter((c) => c.field !== k) : [...L.conds, { field: k, op: 'is', value: 'yes' }] })} />
								</label>
							);
						})}
						<p className="b-muted b-sheet-note">Advanced conditions (any field, AND/OR) open in a full editor.</p>
						<button className="b-primary" onClick={() => setSheet(false)}>Done</button>
					</div>
				</div>
			)}
		</aside>
	);
}

function DetailHead({ base, u, title, sub }) {
	return (
		<header className="b-detail-head">
			<Link to={`${base}/users${location.search}`} className="b-round b-back" aria-label="Back"><ArrowLeft size={20} /></Link>
			<div className="b-dh-title">
				<b>{title}</b>
				<span className="b-muted">{sub}</span>
			</div>
		</header>
	);
}

function Profile({ base, u }) {
	const groups = getGroupsOf(u);
	const act = getActivity(u);
	const hist = getHistory(u);
	const names = getUsernames(u);
	return (
		<div className="b-scroll">
			<DetailHead base={base} u={u} title="User info" sub={u.type === 'bot' ? 'bot' : 'last updated ' + ago(u.updated_at)} />
			<div className="b-sheetcol">
				<div className="b-hero">
					<Avatar u={u} size={112} />
					<h1>{u.name} {u.is_verified && <BadgeCheck size={20} className="b-verified" />}{u.is_premium && <Star size={17} className="b-star" />}</h1>
					<div className="b-muted">{u.type === 'deleted' ? 'deleted account' : 'first seen ' + ago(u.created_at)}</div>
					<div className="b-actions">
						<a href="#" className="b-action"><MessageCircle size={22} /><span>Messages</span></a>
						<Link to={`${base}/users/${u.id}/history${location.search}`} className="b-action"><History size={22} /><span>History</span></Link>
						<button className="b-action"><Copy size={22} /><span>Copy ID</span></button>
					</div>
				</div>

				<div className="b-group">
					{names[0] && <Row icon={AtSign} value={names.map((n) => '@' + n.username).join(', ')} label="Usernames" />}
					{u.bio && <Row icon={Info} value={u.bio} label="Bio" />}
					{u.phone && <Row icon={Phone} value={u.phone} label="Mobile" />}
					<Row icon={Hash} value={<span className="mono">{u.id}</span>} label="User ID" />
					{u.language && <Row icon={Languages} value={u.language} label="Language" />}
					<Row icon={CalendarDays} value={u.created_at} label="First seen" />
				</div>

				{userFlags(u).length > 0 && (
					<div className="b-tags">{userFlags(u).map((f) => <span key={f[0]} className={`b-tag b-tag-${f[1]}`}>{f[0]}</span>)}</div>
				)}

				<div className="b-group b-activity">
					<div className="b-act-top">
						<div><b className="num">{fmt(u.msg_count)}</b><span className="b-muted">messages logged</span></div>
						<div><b className="num">{fmt(act.reduce((a, b) => a + b, 0))}</b><span className="b-muted">last 30 days</span></div>
					</div>
					<Sparkline data={act} bars height={52} className="b-bars" />
				</div>

				<div className="b-group">
					<div className="b-group-title">Groups in common</div>
					{groups.length ? groups.map((g) => (
						<a key={g.id} href="#" className="b-grouprow">
							<GroupAvatar g={g} size={42} />
							<span className="b-row-main"><b>{g.title}</b><span className="b-muted">{fmt(g.msgs)} messages from this user</span></span>
							<ChevronRight size={18} className="b-muted" />
						</a>
					)) : <div className="b-empty-row b-muted">No group messages logged.</div>}
				</div>

				<div className="b-group">
					<Link to={`${base}/users/${u.id}/history${location.search}`} className="b-linkrow">
						<History size={20} /><span>Change history</span><span className="b-muted">{hist.length}</span><ChevronRight size={18} />
					</Link>
					<a href="#" className="b-linkrow"><Image size={20} /><span>Profile photos</span><span className="b-muted">{hist.filter((e) => e.kind === 'photo').length}</span><ChevronRight size={18} /></a>
					<a href="#" className="b-linkrow"><FileText size={20} /><span>Files sent</span><ChevronRight size={18} /></a>
				</div>
			</div>
		</div>
	);
}

function Row({ icon: Icon, value, label }) {
	return (
		<div className="b-inforow">
			<Icon size={21} className="b-muted" />
			<div><div className="b-val">{value}</div><div className="b-muted b-lbl">{label}</div></div>
		</div>
	);
}

const KIND = { name: [Type, 'changed name to'], username: [AtSign, 'username'], bio: [Quote, 'changed bio'], phone: [Phone, 'changed phone to'], photo: [Image, 'changed profile photo'] };

function HistoryView({ base, u }) {
	const hist = getHistory(u);
	const days = [];
	for (const e of hist) {
		const d = e.at.slice(0, 10);
		if (!days.length || days[days.length - 1][0] !== d) days.push([d, []]);
		days[days.length - 1][1].push(e);
	}
	const first = u.first_name || 'User';
	// Like a chat, open at the newest entry; older ones are above.
	const ref = useRef(null);
	useEffect(() => {
		const el = ref.current;
		if (el.scrollHeight > el.clientHeight) el.scrollTop = el.scrollHeight;
		else scrollTo(0, document.documentElement.scrollHeight);
	}, [u.id]);
	return (
		<div className="b-scroll b-chatbg" ref={ref}>
			<header className="b-detail-head">
				<Link to={`${base}/users/${u.id}${location.search}`} className="b-round" aria-label="Back"><ArrowLeft size={20} /></Link>
				<Avatar u={u} size={40} />
				<div className="b-dh-title"><b>{u.name}</b><span className="b-muted">{hist.length} profile changes</span></div>
			</header>
			<div className="b-feed">
				{[...days].reverse().map(([d, items]) => (
					<div key={d}>
						<div className="b-datepill"><span>{new Date(d + 'T00:00:00Z').toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric', timeZone: 'UTC' })}</span></div>
						{[...items].reverse().map((e, i) => {
							const [Icon, verb] = KIND[e.kind];
							return (
								<div key={i} className="b-service">
									<span className="b-svc">
										<Icon size={14} />
										{e.kind === 'username'
											? <>{first} {e.action} {verb} <b>@{e.detail}</b></>
											: e.kind === 'photo'
												? <>{first} {e.removed ? 'removed profile photo' : verb}</>
												: <>{first} {verb} {e.detail ? <b>{e.detail}</b> : <i>(empty)</i>}</>}
										<span className="b-svc-time">{e.at.slice(11, 16)}</span>
									</span>
									{e.kind === 'photo' && !e.removed && (
										<span className="b-svc-photo" style={{ background: `linear-gradient(135deg, hsl(${e.hue} 70% 62%), hsl(${(e.hue + 40) % 360} 65% 48%))` }} />
									)}
								</div>
							);
						})}
					</div>
				))}
			</div>
		</div>
	);
}
