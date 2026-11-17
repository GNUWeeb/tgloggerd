// Direction C - "Data desk": Grafana-like, dense, query-first, master-detail.
import { useEffect, useState } from 'react';
import {
	Terminal, ChevronDown, ChevronUp, X, ArrowUp, ArrowDown, Copy, MessageCircle,
	ExternalLink, Maximize2, Menu, Play, CircleHelp,
} from 'lucide-react';
import { Link, useRouter } from '../router.jsx';
import { useUserList, Avatar, GroupAvatar, Sparkline, userFlags, pageList, parseSub } from '../shared.jsx';
import { getUser, getHistory, getGroupsOf, getUsernames, getActivity, fmt, compact, ago } from '../data.js';

const NAV = [['', 'Overview'], ['/users', 'Users'], ['/groups', 'Groups'], ['/group_messages', 'Group msgs'],
	['/private_messages', 'Private msgs'], ['/files', 'Files'], ['/platform-fwd', 'Bridges'], ['/mcp-admin', 'MCP'], ['/settings', 'Settings']];

// Query language <-> filter conditions.
const KEYS = { premium: 'is_premium', verified: 'is_verified', scam: 'is_scam', fake: 'is_fake', type: 'type',
	msgs: 'msg_count', lang: 'language', id: 'id', bio: 'bio', phone: 'phone', user: 'username', seen: 'created_at' };
const BACK = Object.fromEntries(Object.entries(KEYS).map(([k, v]) => [v, k]));

function parse(text) {
	const conds = [];
	const free = [];
	for (const tok of text.trim().split(/\s+/).filter(Boolean)) {
		const m = tok.match(/^(\w+):(>|<|!)?(.*)$/);
		if (m && KEYS[m[1]]) {
			const field = KEYS[m[1]];
			if (['is_premium', 'is_verified', 'is_scam', 'is_fake'].includes(field)) conds.push({ field, op: 'is', value: m[3] || 'yes' });
			else if (m[2] === '>' || m[2] === '<') conds.push({ field, op: m[2], value: m[3] });
			else if (m[2] === '!') conds.push({ field, op: '!=', value: m[3] });
			else conds.push({ field, op: ['bio', 'username', 'phone'].includes(field) ? 'contains' : '=', value: m[3] });
		} else free.push(tok);
	}
	return { conds, q: free.join(' ') };
}

function unparse(conds, q) {
	const parts = conds.map((c) => {
		const k = BACK[c.field] || c.field;
		if (c.op === 'is') return `${k}:${c.value}`;
		if (c.op === '>' || c.op === '<') return `${k}:${c.op}${c.value}`;
		if (c.op === '!=') return `${k}:!${c.value}`;
		return `${k}:${c.value}`;
	});
	if (q) parts.push(q);
	return parts.join(' ');
}

function Highlight({ text }) {
	return text.split(/(\s+)/).map((t, i) => {
		const m = t.match(/^(\w+):(.*)$/);
		if (m && KEYS[m[1]]) return <span key={i}><span className="c-qk">{m[1]}:</span><span className="c-qv">{m[2]}</span></span>;
		return <span key={i}>{t}</span>;
	});
}

export default function DataApp({ base, sub }) {
	const r = parseSub(sub);
	const user = r.id ? getUser(r.id) : null;
	const [menu, setMenu] = useState(false);
	return (
		<div className="c-app">
			<header className="c-top">
				<span className="c-brand"><Terminal size={16} />tgd</span>
				<button className="c-iconbtn c-menu" onClick={() => setMenu((m) => !m)} aria-label="Menu"><Menu size={18} /></button>
				<nav className={menu ? 'open' : ''} onClick={() => setMenu(false)}>
					{NAV.map(([h, t]) => <Link key={t} to={base + h} className={h && sub.startsWith(h) ? 'on' : ''}>{t}</Link>)}
				</nav>
				<span className="c-live"><span className="c-pulse" />live</span>
				<span className="c-who">admin</span>
			</header>
			{r.screen === 'stub'
				? <div className="c-emptypage">Not part of the first design round.</div>
				: <Users base={base} user={user} tab={r.screen} />}
		</div>
	);
}

const COLS = [
	['id', 'id', 'num'], ['name', 'name'], ['username', 'username'], ['type', 'type'],
	['msg_count', 'msgs', 'num'], ['flags', 'flags'], ['language', 'lang'], ['created_at', 'first_seen'], ['updated_at', 'updated'],
];

function Users({ base, user, tab }) {
	const L = useUserList({ per: 50 });
	const { navigate, search } = useRouter();
	const [text, setText] = useState(unparse(L.conds, L.q));
	useEffect(() => setText(unparse(L.conds, L.q)), [search]); // eslint-disable-line react-hooks/exhaustive-deps
	const run = () => { const p = parse(text); L.update({ conds: p.conds, q: p.q }); };
	const idx = user ? L.rows.findIndex((u) => u.id === user.id) : -1;
	const go = (d) => {
		const n = L.rows[idx + d];
		if (n) navigate(`${base}/users/${n.id}${tab === 'history' ? '/history' : ''}${search}`, { replace: true });
	};
	useEffect(() => {
		const on = (e) => {
			if (e.target.closest('input, textarea, select')) return;
			if (e.key === 'j') go(1);
			if (e.key === 'k') go(-1);
			if (e.key === 'Escape' && user) navigate(`${base}/users${search}`);
		};
		addEventListener('keydown', on);
		return () => removeEventListener('keydown', on);
	});
	const Sort = ({ k }) => (L.sort === k ? (L.dir === 'asc' ? <ChevronUp size={12} /> : <ChevronDown size={12} />) : null);
	const from = (L.page - 1) * L.per + 1;
	return (
		<div className={`c-body ${user ? 'with-panel' : ''}`}>
			<div className="c-left">
				<div className="c-querybar">
					<span className="c-prompt">users</span>
					<div className="c-qwrap">
						<div className="c-qhl" aria-hidden="true"><Highlight text={text} /></div>
						<input
							value={text}
							onChange={(e) => setText(e.target.value)}
							onKeyDown={(e) => e.key === 'Enter' && run()}
							placeholder="premium:yes msgs:>1000 type:regular lang:id  (free text matches name, @username, id)"
							spellCheck={false}
						/>
					</div>
					<button className="c-run" onClick={run}><Play size={13} />Run</button>
					<button className="c-iconbtn" title="Query syntax"><CircleHelp size={16} /></button>
				</div>
				<div className="c-meta">
					<span><b className="num">{fmt(L.total)}</b> rows</span>
					<span>12 ms</span>
					<span className="c-hint">j / k step through rows, Esc closes the panel</span>
					<span className="c-grow" />
					<span className="c-pagenav">
						<button disabled={L.page <= 1} onClick={() => L.update({ page: L.page - 1 })}>prev</button>
						{pageList(L.page, L.pages).map((n, i) => n === '...' ? <span key={'e' + i}>..</span>
							: <button key={n} className={n === L.page ? 'on' : ''} onClick={() => L.update({ page: n })}>{n}</button>)}
						<button disabled={L.page >= L.pages} onClick={() => L.update({ page: L.page + 1 })}>next</button>
					</span>
				</div>
				<div className="c-gridwrap">
					<table className="c-grid">
						<thead>
							<tr>
								<th className="c-av" />
								{COLS.map(([k, t, cls]) => (
									<th key={k} className={`${cls || ''} col-${k}`}>
										{k === 'flags' ? t : <button onClick={() => L.toggleSort(k)}>{t}<Sort k={k} /></button>}
									</th>
								))}
							</tr>
						</thead>
						<tbody>
							{L.rows.map((u, i) => (
								<tr key={u.id} className={user?.id === u.id ? 'sel' : ''}
									onClick={() => navigate(`${base}/users/${u.id}${tab === 'history' ? '/history' : ''}${search}`, { replace: !!user })}>
									<td className="c-av"><Avatar u={u} size={20} radius="5px" /></td>
									<td className="num col-id">{u.id}</td>
									<td className="col-name">{u.name}</td>
									<td className="col-username">{u.username ? <span className="c-at">@{u.username}</span> : <span className="c-null">null</span>}</td>
									<td className={`col-type c-t-${u.type}`}>{u.type}</td>
									<td className="num col-msg_count"><span className="c-heat" style={{ '--h': Math.min(1, Math.log10(u.msg_count + 1) / 4.7) }}>{fmt(u.msg_count)}</span></td>
									<td className="col-flags">{userFlags(u).filter((f) => !['bot', 'deleted'].includes(f[0])).map((f) => <span key={f[0]} className={`c-flag c-flag-${f[1]}`}>{f[0]}</span>)}</td>
									<td className="col-language">{u.language || <span className="c-null">null</span>}</td>
									<td className="num col-created_at">{u.created_at.slice(0, 10)}</td>
									<td className="num col-updated_at">{u.updated_at.slice(0, 10)}</td>
								</tr>
							))}
						</tbody>
					</table>
					{!L.rows.length && <div className="c-emptypage">0 rows. Try removing a condition.</div>}
				</div>
				<div className="c-foot num">rows {L.total ? fmt(from) : 0}-{fmt(from + L.rows.length - 1)} / {fmt(L.total)} . page {L.page} of {L.pages}</div>
			</div>
			{user && <Panel base={base} u={user} tab={tab} search={search} onStep={go} canPrev={idx > 0} canNext={idx >= 0 && idx < L.rows.length - 1} />}
		</div>
	);
}

function Panel({ base, u, tab, search, onStep, canPrev, canNext }) {
	const hist = getHistory(u);
	const groups = getGroupsOf(u);
	const act = getActivity(u);
	const [kind, setKind] = useState('all');
	const rows = kind === 'all' ? hist : hist.filter((e) => e.kind === kind);
	return (
		<aside className="c-panel">
			<div className="c-panel-head">
				<Avatar u={u} size={40} radius="8px" />
				<div className="c-ph-main">
					<div className="c-ph-name">{u.name}</div>
					<div className="c-ph-sub num">{u.username ? '@' + u.username + ' . ' : ''}{u.id}</div>
				</div>
				<button className="c-iconbtn" disabled={!canPrev} onClick={() => onStep(-1)} title="Previous row (k)"><ArrowUp size={16} /></button>
				<button className="c-iconbtn" disabled={!canNext} onClick={() => onStep(1)} title="Next row (j)"><ArrowDown size={16} /></button>
				<button className="c-iconbtn" title="Open full page"><Maximize2 size={15} /></button>
				<Link to={`${base}/users${search}`} className="c-iconbtn" title="Close (Esc)"><X size={16} /></Link>
			</div>
			<div className="c-flags-row">
				<span className={`c-flag c-t-${u.type}`}>{u.type}</span>
				{userFlags(u).filter((f) => !['bot', 'deleted'].includes(f[0])).map((f) => <span key={f[0]} className={`c-flag c-flag-${f[1]}`}>{f[0]}</span>)}
				<span className="c-grow" />
				<a href="#" className="c-btn"><MessageCircle size={13} />chat</a>
				<button className="c-btn"><Copy size={13} />id</button>
				<button className="c-btn"><ExternalLink size={13} />tg</button>
			</div>
			<div className="c-tabs">
				<Link to={`${base}/users/${u.id}${search}`} className={tab === 'user' ? 'on' : ''}>profile</Link>
				<Link to={`${base}/users/${u.id}/history${search}`} className={tab === 'history' ? 'on' : ''}>history <span className="num">{hist.length}</span></Link>
				<a href="#">groups <span className="num">{groups.length}</span></a>
				<a href="#">files</a>
			</div>
			<div className="c-panel-body">
				{tab === 'user' ? (
					<>
						<div className="c-statgrid">
							<Stat t="messages" v={fmt(u.msg_count)} />
							<Stat t="groups" v={groups.length} />
							<Stat t="first_seen" v={ago(u.created_at)} s={u.created_at.slice(0, 10)} />
							<Stat t="changes" v={hist.length} s={'last ' + ago(hist[0]?.at || u.updated_at)} />
						</div>
						<section className="c-box">
							<h4>messages / day <span>30d</span></h4>
							<Sparkline data={act} bars height={64} className="c-chart" />
							<div className="c-axis num"><span>-30d</span><span>-15d</span><span>today</span></div>
						</section>
						<section className="c-box">
							<h4>fields</h4>
							<dl className="c-kv">
								<dt>first_name</dt><dd>{u.first_name || <span className="c-null">''</span>}</dd>
								<dt>last_name</dt><dd>{u.last_name || <span className="c-null">''</span>}</dd>
								<dt>usernames</dt><dd>{getUsernames(u).map((n) => <span key={n.username} className={n.kind === 'active' ? 'c-at' : 'c-null'}>@{n.username} </span>)}{!getUsernames(u).length && <span className="c-null">[]</span>}</dd>
								<dt>bio</dt><dd>{u.bio || <span className="c-null">''</span>}</dd>
								<dt>phone</dt><dd className="num">{u.phone || <span className="c-null">null</span>}</dd>
								<dt>language</dt><dd>{u.language || <span className="c-null">null</span>}</dd>
								<dt>created_at</dt><dd className="num">{u.created_at}</dd>
								<dt>updated_at</dt><dd className="num">{u.updated_at}</dd>
							</dl>
						</section>
						<section className="c-box">
							<h4>top groups</h4>
							{groups.length ? groups.map((g) => (
								<div key={g.id} className="c-grow-row">
									<GroupAvatar g={g} size={18} radius="4px" />
									<span className="c-gname">{g.title}</span>
									<span className="c-gbar"><span style={{ width: `${(g.msgs / groups[0].msgs) * 100}%` }} /></span>
									<span className="num">{compact(g.msgs)}</span>
								</div>
							)) : <span className="c-null">no group messages</span>}
						</section>
					</>
				) : (
					<>
						<div className="c-kinds">
							{['all', 'name', 'username', 'bio', 'phone', 'photo'].map((k) => (
								<button key={k} className={kind === k ? 'on' : ''} onClick={() => setKind(k)}>{k}</button>
							))}
						</div>
						<table className="c-log">
							<tbody>
								{rows.map((e, i) => (
									<tr key={i}>
										<td className="num c-log-t">{e.at.slice(0, 16)}</td>
										<td><span className={`c-k c-k-${e.kind}`}>{e.kind}</span></td>
										<td className="c-log-v">
											{e.kind === 'photo'
												? (e.removed ? <span className="c-del">- removed</span>
													: <span className="c-add"><span className="c-thumb" style={{ background: `hsl(${e.hue} 60% 55%)` }} />+ new photo</span>)
												: e.kind === 'username'
													? <span className={e.action === 'added' ? 'c-add' : 'c-del'}>{e.action === 'added' ? '+' : '-'} @{e.detail}</span>
													: e.detail ? <span className="c-add">= {e.detail}</span> : <span className="c-del">= ''</span>}
										</td>
									</tr>
								))}
							</tbody>
						</table>
						{!rows.length && <div className="c-null">no {kind} changes</div>}
					</>
				)}
			</div>
		</aside>
	);
}

function Stat({ t, v, s }) {
	return (
		<div className="c-stat">
			<span className="c-stat-t">{t}</span>
			<span className="c-stat-v num">{v}</span>
			{s && <span className="c-stat-s num">{s}</span>}
		</div>
	);
}
