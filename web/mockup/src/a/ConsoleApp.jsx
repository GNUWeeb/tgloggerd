// Direction A - "Console": a Linear/Vercel-style admin console.
import { useState } from 'react';
import {
	LayoutDashboard, Users, UsersRound, MessagesSquare, MessageCircle, FileText,
	ArrowLeftRight, Bot, Settings, Search, Plus, X, ChevronDown, ChevronUp,
	ChevronLeft, ChevronRight, Menu, Columns3, Copy, History, ExternalLink,
	BadgeCheck, Star, AtSign, Phone, Languages, CalendarDays, Image, Type,
	Quote, LogOut, Download,
} from 'lucide-react';
import { Link } from '../router.jsx';
import { useUserList, Avatar, GroupAvatar, Sparkline, userFlags, pageList, parseSub } from '../shared.jsx';
import { FIELDS, getUser, getHistory, getGroupsOf, getUsernames, getActivity, fmt, compact, ago, totals } from '../data.js';

const NAV = [
	{ label: null, items: [['', 'Dashboard', LayoutDashboard]] },
	{ label: 'Data', items: [
		['/users', 'Users', Users, totals.users],
		['/groups', 'Groups', UsersRound, totals.groups],
		['/group_messages', 'Group messages', MessagesSquare],
		['/private_messages', 'Private messages', MessageCircle],
		['/files', 'Files', FileText],
	] },
	{ label: 'Integrations', items: [
		['/platform-fwd', 'Platform forwarding', ArrowLeftRight],
		['/mcp-admin', 'MCP', Bot],
	] },
	{ label: null, items: [['/settings', 'Settings', Settings]] },
];

export default function ConsoleApp({ base, sub }) {
	const r = parseSub(sub);
	const [open, setOpen] = useState(false);
	const user = r.id ? getUser(r.id) : null;
	const crumbs = [['Users', base + '/users']];
	if (user) crumbs.push([user.name, `${base}/users/${user.id}`]);
	if (r.screen === 'history') crumbs.push(['History', null]);
	return (
		<div className={`a-shell ${open ? 'nav-open' : ''}`}>
			<aside className="a-side" onClick={(e) => e.target.closest('a') && setOpen(false)}>
				<div className="a-brand">
					<span className="a-logo">tg</span>
					<span>tgloggerd</span>
				</div>
				<nav>
					{NAV.map((g, i) => (
						<div key={i} className="a-navgroup">
							{g.label && <div className="a-navlabel">{g.label}</div>}
							{g.items.map(([href, label, Icon, n]) => (
								<Link key={label} to={base + href} className={`a-navitem ${sub.startsWith(href || '/~') ? 'on' : ''}`}>
									<Icon size={16} strokeWidth={1.8} />
									<span>{label}</span>
									{n && <span className="a-navcount">{compact(n)}</span>}
								</Link>
							))}
						</div>
					))}
				</nav>
				<div className="a-me">
					<span className="a-me-av">A</span>
					<span>admin</span>
					<button className="a-icon-btn" title="Sign out"><LogOut size={15} /></button>
				</div>
			</aside>
			<div className="a-scrim" onClick={() => setOpen(false)} />
			<div className="a-main">
				<header className="a-top">
					<button className="a-icon-btn a-menu" onClick={() => setOpen(true)} aria-label="Menu"><Menu size={18} /></button>
					<nav className="a-crumbs" aria-label="Breadcrumb">
						{crumbs.map(([t, h], i) => (
							<span key={i}>
								{i > 0 && <span className="a-sep">/</span>}
								{h && i < crumbs.length - 1 ? <Link to={h}>{t}</Link> : <span className="a-cur">{t}</span>}
							</span>
						))}
					</nav>
					<button className="a-cmdk">
						<Search size={14} /><span>Search or jump to...</span><kbd>Ctrl K</kbd>
					</button>
				</header>
				<main className="a-content">
					{r.screen === 'users' && <UsersScreen base={base} />}
					{(r.screen === 'user' || r.screen === 'history') && user && <UserScreen base={base} u={user} tab={r.screen} />}
					{r.screen === 'stub' && <div className="a-empty">This page is not part of the first design round.</div>}
				</main>
			</div>
		</div>
	);
}

function Flag({ f }) {
	return <span className={`a-pill a-pill-${f[1]}`}>{f[0]}</span>;
}

const OPS = { number: ['=', '>', '<', '!='], text: ['contains', '=', 'empty'], enum: ['=', '!='], bool: ['is'], date: ['>', '<'] };

function FilterAdd({ onAdd }) {
	const [open, setOpen] = useState(false);
	const [field, setField] = useState('is_premium');
	const f = FIELDS.find((x) => x.key === field);
	const [op, setOp] = useState('is');
	const [value, setValue] = useState('yes');
	const changeField = (k) => {
		const nf = FIELDS.find((x) => x.key === k);
		setField(k); setOp(OPS[nf.type][0]);
		setValue(nf.type === 'bool' ? 'yes' : nf.type === 'enum' ? nf.values[0] : '');
	};
	return (
		<div className="a-pop-wrap">
			<button className="a-btn a-btn-ghost" onClick={() => setOpen((o) => !o)}><Plus size={14} />Filter</button>
			{open && (
				<form className="a-pop" onSubmit={(e) => { e.preventDefault(); onAdd({ field, op, value }); setOpen(false); }}>
					<label>Field
						<select value={field} onChange={(e) => changeField(e.target.value)}>
							{FIELDS.map((x) => <option key={x.key} value={x.key}>{x.label}</option>)}
						</select>
					</label>
					<label>Condition
						<select value={op} onChange={(e) => setOp(e.target.value)}>
							{OPS[f.type].map((o) => <option key={o}>{o}</option>)}
						</select>
					</label>
					{op !== 'empty' && (
						<label>Value
							{f.type === 'bool' ? (
								<select value={value} onChange={(e) => setValue(e.target.value)}><option>yes</option><option>no</option></select>
							) : f.type === 'enum' ? (
								<select value={value} onChange={(e) => setValue(e.target.value)}>{f.values.map((v) => <option key={v}>{v}</option>)}</select>
							) : (
								<input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={f.type === 'date' ? 'YYYY-MM-DD' : ''} />
							)}
						</label>
					)}
					<div className="a-pop-actions">
						<button type="button" className="a-btn a-btn-ghost" onClick={() => setOpen(false)}>Cancel</button>
						<button className="a-btn a-btn-primary">Apply</button>
					</div>
				</form>
			)}
		</div>
	);
}

const COLS = [
	['name', 'User'], ['id', 'ID'], ['type', 'Type'], ['msg_count', 'Messages'],
	['flags', 'Flags'], ['language', 'Lang'], ['created_at', 'First seen'], ['updated_at', 'Updated'],
];

function UsersScreen({ base }) {
	const L = useUserList();
	const [q, setQ] = useState(L.q);
	const maxMsgs = Math.max(1, ...L.rows.map((u) => u.msg_count));
	const label = (c) => FIELDS.find((x) => x.key === c.field)?.label;
	const SortIcon = ({ k }) => (L.sort === k ? (L.dir === 'asc' ? <ChevronUp size={13} /> : <ChevronDown size={13} />) : null);
	const from = (L.page - 1) * L.per + 1;
	return (
		<>
			<div className="a-pagehead">
				<div>
					<h1>Users</h1>
					<p className="a-sub">{fmt(totals.users)} users seen across all logged chats</p>
				</div>
				<div className="a-head-actions">
					<button className="a-btn a-btn-ghost"><Download size={14} />Export</button>
				</div>
			</div>

			<div className="a-toolbar">
				<form className="a-search" onSubmit={(e) => { e.preventDefault(); L.update({ q }); }}>
					<Search size={15} />
					<input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Name, @username or ID" />
				</form>
				{L.conds.map((c, i) => (
					<span key={i} className="a-chip">
						<b>{label(c)}</b> {c.op} {c.op !== 'empty' && <b>{String(c.value)}</b>}
						<button aria-label="Remove filter" onClick={() => L.update({ conds: L.conds.filter((_, j) => j !== i) })}><X size={12} /></button>
					</span>
				))}
				<FilterAdd onAdd={(c) => L.update({ conds: [...L.conds, c] })} />
				{L.conds.length > 0 && <button className="a-link" onClick={() => L.update({ conds: [], q: '' })}>Clear all</button>}
				<span className="a-grow" />
				<button className="a-btn a-btn-ghost"><Columns3 size={14} />Columns</button>
			</div>

			<div className="a-card a-tablecard">
				<table className="a-table">
					<thead>
						<tr>
							{COLS.map(([k, t]) => (
								<th key={k} className={`c-${k}`}>
									{k === 'flags' ? t : <button onClick={() => L.toggleSort(k)}>{t}<SortIcon k={k} /></button>}
								</th>
							))}
						</tr>
					</thead>
					<tbody>
						{L.rows.map((u) => (
							<tr key={u.id}>
								<td className="c-name">
									<Link to={`${base}/users/${u.id}`} className="a-usercell">
										<Avatar u={u} size={28} />
										<span>
											<span className="a-uname">{u.name}</span>
											{u.username && <span className="a-handle">@{u.username}</span>}
										</span>
									</Link>
								</td>
								<td className="c-id mono">{u.id}</td>
								<td className="c-type"><span className={`a-dot a-dot-${u.type}`} />{u.type}</td>
								<td className="c-msg_count">
									<span className="a-bar"><span style={{ width: `${(u.msg_count / maxMsgs) * 100}%` }} /></span>
									<span className="num">{fmt(u.msg_count)}</span>
								</td>
								<td className="c-flags">{userFlags(u).filter((f) => f[0] !== 'deleted' && f[0] !== 'bot').map((f) => <Flag key={f[0]} f={f} />)}</td>
								<td className="c-language">{u.language || <span className="a-muted">-</span>}</td>
								<td className="c-created_at" title={u.created_at}>{ago(u.created_at)}</td>
								<td className="c-updated_at" title={u.updated_at}>{ago(u.updated_at)}</td>
							</tr>
						))}
					</tbody>
				</table>
				{L.rows.length === 0 && <div className="a-empty">No users match these filters.</div>}

				<ul className="a-list">
					{L.rows.map((u) => (
						<li key={u.id}>
							<Link to={`${base}/users/${u.id}`}>
								<Avatar u={u} size={36} />
								<span className="a-list-main">
									<span className="a-uname">{u.name}</span>
									<span className="a-handle">{u.username ? '@' + u.username : u.id}</span>
									<span className="a-list-flags">{userFlags(u).map((f) => <Flag key={f[0]} f={f} />)}</span>
								</span>
								<span className="a-list-side">
									<span className="num">{compact(u.msg_count)}</span>
									<span className="a-muted">msgs</span>
								</span>
							</Link>
						</li>
					))}
				</ul>

				<div className="a-pager">
					<span className="a-muted">{L.total ? `${fmt(from)}-${fmt(from + L.rows.length - 1)} of ${fmt(L.total)}` : '0 results'}</span>
					<span className="a-grow" />
					<label className="a-per">Rows
						<select value={L.per} onChange={(e) => L.update({ per: e.target.value })}>
							{[25, 50, 100].map((n) => <option key={n}>{n}</option>)}
						</select>
					</label>
					<button className="a-icon-btn" disabled={L.page <= 1} onClick={() => L.update({ page: L.page - 1 })} aria-label="Previous page"><ChevronLeft size={16} /></button>
					<span className="a-pages">
						{pageList(L.page, L.pages).map((n, i) => n === '...'
							? <span key={'e' + i} className="a-muted">...</span>
							: <button key={n} className={n === L.page ? 'on' : ''} onClick={() => L.update({ page: n })}>{n}</button>)}
					</span>
					<button className="a-icon-btn" disabled={L.page >= L.pages} onClick={() => L.update({ page: L.page + 1 })} aria-label="Next page"><ChevronRight size={16} /></button>
				</div>
			</div>
		</>
	);
}

const KIND = {
	name: [Type, 'Name changed'],
	username: [AtSign, 'Username'],
	bio: [Quote, 'Bio changed'],
	phone: [Phone, 'Phone changed'],
	photo: [Image, 'Profile photo'],
};

function HistItem({ e }) {
	const [Icon, title] = KIND[e.kind];
	return (
		<li className={`a-tl a-tl-${e.kind}`}>
			<span className="a-tl-icon"><Icon size={14} /></span>
			<div className="a-tl-body">
				<div className="a-tl-title">
					{e.kind === 'username' ? `Username ${e.action}` : title}
					<span className="a-muted a-tl-time" title={e.at}>{e.at.slice(0, 16)}</span>
				</div>
				{e.kind === 'photo' ? (
					e.removed ? <span className="a-muted">Photo removed</span>
						: <span className="a-tl-photo" style={{ background: `linear-gradient(135deg, hsl(${e.hue} 70% 62%), hsl(${(e.hue + 40) % 360} 65% 48%))` }} />
				) : (
					<div className={`a-tl-val ${e.kind === 'username' && e.action === 'removed' ? 'strike' : ''}`}>
						{e.detail ? (e.kind === 'username' ? '@' : '') + e.detail : <span className="a-muted">(cleared)</span>}
					</div>
				)}
			</div>
		</li>
	);
}

function UserScreen({ base, u, tab }) {
	const hist = getHistory(u);
	const groups = getGroupsOf(u);
	const act = getActivity(u);
	const [kind, setKind] = useState('all');
	const shown = kind === 'all' ? hist : hist.filter((e) => e.kind === kind);
	const months = [];
	for (const e of shown) {
		const m = e.at.slice(0, 7);
		if (!months.length || months[months.length - 1][0] !== m) months.push([m, []]);
		months[months.length - 1][1].push(e);
	}
	return (
		<>
			<div className="a-profilehead">
				<Avatar u={u} size={64} />
				<div className="a-ph-main">
					<h1>{u.name} {u.is_verified && <BadgeCheck size={18} className="a-verified" />}</h1>
					<div className="a-ph-meta">
						{u.username && <span>@{u.username}</span>}
						<span className="mono">{u.id} <button className="a-icon-btn sm" title="Copy ID"><Copy size={12} /></button></span>
						{userFlags(u).map((f) => <Flag key={f[0]} f={f} />)}
					</div>
				</div>
				<div className="a-head-actions">
					<a className="a-btn a-btn-primary" href="#"><MessageCircle size={14} />Chat history</a>
					<button className="a-btn a-btn-ghost"><ExternalLink size={14} />Open in Telegram</button>
				</div>
			</div>

			<nav className="a-tabs">
				<Link to={`${base}/users/${u.id}`} className={tab === 'user' ? 'on' : ''}>Overview</Link>
				<Link to={`${base}/users/${u.id}/history`} className={tab === 'history' ? 'on' : ''}>History <span className="a-tabcount">{hist.length}</span></Link>
				<a href="#">Groups <span className="a-tabcount">{groups.length}</span></a>
				<a href="#">Files</a>
			</nav>

			{tab === 'user' ? (
				<div className="a-overview">
					<div className="a-stats">
						<div className="a-card a-stat"><span className="a-muted">Messages</span><b className="num">{fmt(u.msg_count)}</b></div>
						<div className="a-card a-stat"><span className="a-muted">Groups</span><b className="num">{groups.length}</b></div>
						<div className="a-card a-stat"><span className="a-muted">First seen</span><b>{ago(u.created_at)}</b><span className="a-muted sm">{u.created_at.slice(0, 10)}</span></div>
						<div className="a-card a-stat a-stat-spark">
							<span className="a-muted">Last 30 days</span>
							<b className="num">{fmt(act.reduce((a, b) => a + b, 0))}</b>
							<Sparkline data={act} className="a-spark" />
						</div>
					</div>
					<div className="a-cols">
						<section className="a-card">
							<h2>Details</h2>
							<dl className="a-kv">
								<dt><Type size={14} />First name</dt><dd>{u.first_name || '-'}</dd>
								<dt><Type size={14} />Last name</dt><dd>{u.last_name || '-'}</dd>
								<dt><AtSign size={14} />Usernames</dt>
								<dd>{getUsernames(u).map((n) => <span key={n.username} className={`a-pill ${n.kind === 'active' ? 'a-pill-accent' : 'a-pill-muted'}`}>@{n.username}</span>)}{!getUsernames(u).length && '-'}</dd>
								<dt><Quote size={14} />Bio</dt><dd>{u.bio || '-'}</dd>
								<dt><Phone size={14} />Phone</dt><dd className="mono">{u.phone || '-'}</dd>
								<dt><Languages size={14} />Language</dt><dd>{u.language || '-'}</dd>
								<dt><Star size={14} />Type</dt><dd>{u.type}</dd>
								<dt><CalendarDays size={14} />First seen</dt><dd className="mono">{u.created_at}</dd>
								<dt><CalendarDays size={14} />Updated</dt><dd className="mono">{u.updated_at}</dd>
							</dl>
						</section>
						<div className="a-stack">
							<section className="a-card">
								<div className="a-cardhead"><h2>Recent changes</h2><Link to={`${base}/users/${u.id}/history`} className="a-link">View all</Link></div>
								<ol className="a-timeline compact">{hist.slice(0, 4).map((e, i) => <HistItem key={i} e={e} />)}</ol>
							</section>
							<section className="a-card">
								<h2>Most active in</h2>
								{groups.length ? (
									<ul className="a-groups">
										{groups.map((g) => (
											<li key={g.id}>
												<GroupAvatar g={g} size={28} radius="8px" />
												<span className="a-grow">{g.title}</span>
												<span className="num a-muted">{fmt(g.msgs)}</span>
											</li>
										))}
									</ul>
								) : <p className="a-muted">No group messages logged.</p>}
							</section>
						</div>
					</div>
				</div>
			) : (
				<div className="a-history">
					<div className="a-seg" role="tablist">
						{['all', 'name', 'username', 'bio', 'phone', 'photo'].map((k) => (
							<button key={k} className={kind === k ? 'on' : ''} onClick={() => setKind(k)}>
								{k === 'all' ? 'All' : KIND[k][1].split(' ')[0] === 'Profile' ? 'Photo' : KIND[k][1].split(' ')[0]}
							</button>
						))}
					</div>
					{months.map(([m, items]) => (
						<section key={m} className="a-month">
							<h3>{new Date(m + '-01T00:00:00Z').toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' })}</h3>
							<ol className="a-timeline">{items.map((e, i) => <HistItem key={i} e={e} />)}</ol>
						</section>
					))}
					{!shown.length && <div className="a-empty">No changes of this kind recorded.</div>}
					<div className="a-loadmore"><History size={14} />Older changes load as you scroll</div>
				</div>
			)}
		</>
	);
}
