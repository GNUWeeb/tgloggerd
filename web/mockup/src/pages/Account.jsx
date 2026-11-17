// Settings and the sign-in page.
import { useState } from 'react';
import { CircleCheck, LogIn, ShieldCheck, LogOut, Monitor, Smartphone, PinOff, Bookmark, Trash2 } from 'lucide-react';
import { useRouter, Link } from '../router.jsx';
import Shell, { ThemeButton } from '../ui/Shell.jsx';
import { PageHead, Btn, Card, Pill, Avatar } from '../ui/primitives.jsx';
import { Field, Switch } from '../ui/forms.jsx';
import { useViewer } from '../viewer.jsx';
import { mySessions } from '../data/accounts.js';
import { groups } from '../data/groups.js';
import { ago } from '../data/seed.js';

export function SettingsNav({ on }) {
	return (
		<nav className="a-settingsnav" aria-label="Settings sections">
			<Link to="/settings" className={on === 'account' ? 'on' : ''}>Your account</Link>
			<Link to="/settings/preferences" className={on === 'prefs' ? 'on' : ''}>Preferences</Link>
			<Link to="/settings/sessions" className={on === 'sessions' ? 'on' : ''}>Sessions</Link>
		</nav>
	);
}

export function Settings() {
	const v = useViewer();
	const [done, setDone] = useState(false);
	return (
		<Shell crumbs={[['Settings', null]]}>
			<PageHead title="Settings" />
			<div className="a-settings">
				<SettingsNav on="account" />
				<div className="a-stack">
					<Card title="Account" className="a-narrow">
						<p className="a-muted">Signed in as <b>{v.acct.username}</b> ({v.acct.name}) . {v.role}</p>
					</Card>
					<Card title="Change password" className="a-narrow">
						{done && <div className="a-banner"><CircleCheck size={15} /><span>Password changed. Every other session has been signed out.</span></div>}
						<form className="a-form" onSubmit={(e) => { e.preventDefault(); setDone(true); }}>
							<Field label="Current password"><input type="password" autoComplete="current-password" required /></Field>
							<Field label="New password" hint="At least 8 characters."><input type="password" autoComplete="new-password" minLength={8} required /></Field>
							<Field label="Confirm new password"><input type="password" autoComplete="new-password" minLength={8} required /></Field>
							<p className="a-muted sm"><ShieldCheck size={13} /> Changing the password signs out every other browser and device.</p>
							<div><button className="a-btn a-btn-primary">Change password</button></div>
						</form>
					</Card>
				</div>
			</div>
		</Shell>
	);
}

// Every preference has a site default; an account that never set one
// (or a server that does not send it yet) simply gets the default.
const TZS = ['Asia/Jakarta', 'Asia/Makassar', 'Asia/Jayapura', 'Asia/Singapore', 'Europe/Berlin', 'UTC'];

function Pref({ label, hint, children }) {
	return (
		<div className="a-pref">
			<div><b>{label}</b>{hint && <span className="a-muted sm">{hint}</span>}</div>
			<div className="a-pref-ctl">{children}</div>
		</div>
	);
}

function Sel({ value, onChange, options, def }) {
	return (
		<select className="a-select" value={value} onChange={(e) => onChange(e.target.value)}>
			<option value="">Site default ({def})</option>
			{options.map(([k, t]) => <option key={k} value={k}>{t}</option>)}
		</select>
	);
}

export function Preferences() {
	const v = useViewer();
	const [p, setP] = useState({ tz: '', fmt: '', lang: '', rows: '', landing: '', media: '', autoload: true, live: true, deleted: true, edited: true, service: true });
	const set = (k) => (val) => setP({ ...p, [k]: val });
	const pinned = groups.slice(0, 12).filter(v.group).slice(0, 3);
	const saved = [
		['Deleted messages this week', '/group_messages?f=%5B%7B%22field%22%3A%22is_deleted%22%2C%22op%22%3A%22is%22%2C%22value%22%3A%22yes%22%7D%5D'],
		['Premium users, most active', '/users?f=%5B%7B%22field%22%3A%22is_premium%22%2C%22op%22%3A%22is%22%2C%22value%22%3A%22yes%22%7D%5D'],
	];
	return (
		<Shell crumbs={[['Settings', '/settings'], ['Preferences', null]]}>
			<PageHead title="Preferences" sub="How dates, lists and chats look for you. Changes apply to every device you sign in on." />
			<div className="a-settings">
				<SettingsNav on="prefs" />
				<div className="a-stack">
					<Card title="Region and display" className="a-narrow">
						<Pref label="Timezone" hint="Dates are stored in UTC and shown in this zone."><Sel value={p.tz} onChange={set('tz')} def="Asia/Jakarta" options={TZS.map((z) => [z, z])} /></Pref>
						<Pref label="Date and time format"><Sel value={p.fmt} onChange={set('fmt')} def="24-hour" options={[['24h', '24-hour, 17 Nov 2026 16:05'], ['12h', '12-hour, 17 Nov 2026 4:05 PM'], ['iso', 'ISO, 2026-11-17 16:05']]} /></Pref>
						<Pref label="Language"><Sel value={p.lang} onChange={set('lang')} def="English" options={[['en', 'English'], ['id', 'Bahasa Indonesia']]} /></Pref>
						<Pref label="Theme"><span className="a-row"><span className="a-muted sm">System, light or dark</span><ThemeButton /></span></Pref>
						<Pref label="Rows per page"><Sel value={p.rows} onChange={set('rows')} def="25" options={[['25', '25'], ['50', '50'], ['100', '100']]} /></Pref>
						<Pref label="Landing page" hint="Where signing in takes you."><Sel value={p.landing} onChange={set('landing')} def="Dashboard" options={[['/', 'Dashboard'], ['/groups', 'Groups'], ['/group_messages', 'Group messages'], ...pinned.map((g) => [`/groups/${g.id}/chat`, `${g.title} chat`])]} /></Pref>
					</Card>
					<Card title="Chat view" className="a-narrow">
						<Pref label="Load older messages while scrolling"><Switch checked={p.autoload} onChange={set('autoload')} /></Pref>
						<Pref label="Live updates" hint="New messages appear as they are logged."><Switch checked={p.live} onChange={set('live')} /></Pref>
						<Pref label="Media"><Sel value={p.media} onChange={set('media')} def="load automatically" options={[['auto', 'Load automatically'], ['click', 'Load on click']]} /></Pref>
						<Pref label="Show deleted messages"><Switch checked={p.deleted} onChange={set('deleted')} /></Pref>
						<Pref label="Show edit markers"><Switch checked={p.edited} onChange={set('edited')} /></Pref>
						<Pref label="Show service messages" hint="Joins, leaves, pins, photo changes."><Switch checked={p.service} onChange={set('service')} /></Pref>
					</Card>
					<Card title="Pinned groups" className="a-narrow">
						<p className="a-muted sm">Pinned groups appear at the top of the sidebar. Pin a group from its page.</p>
						<ul className="a-audit">
							{pinned.map((g) => (
								<li key={g.id}><Avatar e={g} size={22} square /><Link to={`/groups/${g.id}`} className="a-grow a-rowlink">{g.title}</Link><button type="button" className="a-icon-btn sm" aria-label={`Unpin ${g.title}`}><PinOff size={13} /></button></li>
							))}
						</ul>
					</Card>
					<Card title="Saved searches" className="a-narrow">
						<p className="a-muted sm">Save a search from any list's filter bar.</p>
						<ul className="a-audit">
							{saved.map(([t, href]) => (
								<li key={t}><Bookmark size={14} /><Link to={href} className="a-grow a-rowlink">{t}</Link><button type="button" className="a-icon-btn sm" aria-label={`Delete ${t}`}><Trash2 size={13} /></button></li>
							))}
						</ul>
					</Card>
					<div><Btn primary>Save preferences</Btn></div>
				</div>
			</div>
		</Shell>
	);
}

export function Sessions() {
	const [list, setList] = useState(mySessions);
	return (
		<Shell crumbs={[['Settings', '/settings'], ['Sessions', null]]}>
			<PageHead title="Sessions" sub="Where your account is signed in." actions={<Btn ghost danger icon={LogOut} onClick={() => setList(list.filter((x) => x.current))}>Sign out all others</Btn>} />
			<div className="a-settings">
				<SettingsNav on="sessions" />
				<div className="a-card a-tablecard a-narrow">
					<ul className="a-sessions">
						{list.map((s) => (
							<li key={s.id}>
								<span className="a-sessicon">{/Android|iPhone/.test(s.device) ? <Smartphone size={18} /> : <Monitor size={18} />}</span>
								<span className="a-grow a-minw">
									<b>{s.device}{s.current && <Pill tone="ok">this browser</Pill>}</b>
									<span className="a-muted sm">{s.place} . {s.ip} . signed in {ago(s.started)}</span>
								</span>
								<span className="a-muted sm" title={s.last}>{s.current ? 'active now' : `active ${ago(s.last)}`}</span>
								{!s.current && <Btn ghost onClick={() => setList(list.filter((x) => x.id !== s.id))}>Sign out</Btn>}
							</li>
						))}
					</ul>
				</div>
			</div>
		</Shell>
	);
}

export function Login() {
	const { navigate } = useRouter();
	const [err, setErr] = useState(false);
	return (
		<div className="app a-loginpage">
			<form className="a-card a-login" onSubmit={(e) => { e.preventDefault(); if (e.target.user.value) navigate('/'); else setErr(true); }}>
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>Sign in</h1>
				<p className="a-muted">Telegram archive. Sign in to browse it.</p>
				{err && <div className="a-banner warn"><span>Wrong username or password.</span></div>}
				<Field label="Username"><input name="user" autoComplete="username" autoFocus /></Field>
				<Field label="Password"><input name="pass" type="password" autoComplete="current-password" /></Field>
				<button className="a-btn a-btn-primary a-wide"><LogIn size={15} />Sign in</button>
				<div className="a-login-foot"><ThemeButton /></div>
			</form>
		</div>
	);
}
