// Settings and the sign-in page.
import { useState } from 'react';
import { CircleCheck, LogIn, ShieldCheck, LogOut, Monitor, Smartphone, PinOff, Bookmark, Trash2, Mail, MailCheck, Bell, KeyRound, TriangleAlert, ArrowLeft } from 'lucide-react';
import { useRouter, Link, useQuery } from '../router.jsx';
import Shell, { ThemeButton } from '../ui/Shell.jsx';
import { PageHead, Btn, Card, Pill, Avatar } from '../ui/primitives.jsx';
import { Field, Switch, Modal } from '../ui/forms.jsx';
import { useViewer } from '../viewer.jsx';
import { mySessions, SECURITY_NOTICES } from '../data/accounts.js';
import { EmailLine } from './Accounts.jsx';
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

function ChangeEmail({ onClose }) {
	const [sent, setSent] = useState('');
	return (
		<Modal title="Change email address" icon={<Mail size={16} />} onClose={onClose}
			footer={sent
				? <><span className="a-grow" /><Btn primary onClick={onClose}>Done</Btn></>
				: <><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn primary type="submit" form="chg-email">Send verification link</Btn></>}>
			{sent ? (
				<div className="a-banner"><MailCheck size={15} /><span>A verification link is on its way to <b>{sent}</b>. The address changes once you open it; your current address is told about the change.</span></div>
			) : (
				<form id="chg-email" className="a-form" onSubmit={(e) => { e.preventDefault(); setSent(e.target.email.value || 'new@example.org'); }}>
					<Field label="New email address"><input name="email" type="email" autoComplete="email" placeholder="name@example.org" /></Field>
					<Field label="Current password" hint="To confirm it is you."><input type="password" autoComplete="current-password" /></Field>
				</form>
			)}
		</Modal>
	);
}

export function Settings() {
	const v = useViewer();
	const [done, setDone] = useState(false);
	const [chg, setChg] = useState(false);
	return (
		<Shell crumbs={[['Settings', null]]}>
			<PageHead title="Settings" />
			<div className="a-settings">
				<SettingsNav on="account" />
				<div className="a-stack">
					<Card title="Account" className="a-narrow">
						<p className="a-muted">Signed in as <b>{v.acct.username}</b> ({v.acct.name}) . {v.role}</p>
						<div className="a-pref">
							<div><b>Email address</b><span className="a-muted sm">For password resets and security notices.</span></div>
							<div className="a-pref-ctl a-row"><EmailLine a={v.acct} /><Btn ghost onClick={() => setChg(true)}>Change</Btn></div>
						</div>
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
					<Card title="Security notices" className="a-narrow">
						<p className="a-muted sm"><Bell size={13} /> Always emailed to your verified address; they cannot be turned off.</p>
						<ul className="a-notices">{SECURITY_NOTICES.map((n) => <li key={n}>{n}</li>)}</ul>
					</Card>
				</div>
			</div>
			{chg && <ChangeEmail onClose={() => setChg(false)} />}
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
			<form className="a-card a-login" onSubmit={(e) => { e.preventDefault(); const u = e.target.user.value; if (!u) setErr(true); else navigate(u === 'ops' ? '/welcome/email' : '/'); /* ops has no address yet */ }}>
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>Sign in</h1>
				<p className="a-muted">Telegram archive. Sign in to browse it.</p>
				{err && <div className="a-banner warn"><span>Wrong username or password.</span></div>}
				<Field label="Username"><input name="user" autoComplete="username" autoFocus /></Field>
				<Field label="Password"><input name="pass" type="password" autoComplete="current-password" /></Field>
				<button className="a-btn a-btn-primary a-wide"><LogIn size={15} />Sign in</button>
				<Link to="/forgot-password" className="a-link a-center">Forgot your password?</Link>
				<div className="a-login-foot"><ThemeButton /></div>
			</form>
		</div>
	);
}

// ---- Signed-out account flows (web-accounts.md section 8) ----

function AuthCard({ title, lead, children }) {
	return (
		<div className="app a-loginpage">
			<div className="a-card a-login">
				<div className="a-brand a-login-brand"><span className="a-logo">tg</span><span>tgloggerd</span></div>
				<h1>{title}</h1>
				{lead && <p className="a-muted">{lead}</p>}
				{children}
				<div className="a-login-foot"><ThemeButton /></div>
			</div>
		</div>
	);
}

export function ForgotPassword() {
	const [sent, setSent] = useState(false);
	return (
		<AuthCard title="Reset your password" lead={sent ? null : 'Enter the email address of your account. We will send a link to choose a new password.'}>
			{sent ? (
				<>
					<div className="a-banner"><MailCheck size={15} /><span>If an account uses that address, a reset link is on its way. It works once and expires soon.</span></div>
					<Link to="/login" className="a-link a-center"><ArrowLeft size={13} /> Back to sign in</Link>
				</>
			) : (
				<form className="a-form" onSubmit={(e) => { e.preventDefault(); setSent(true); }}>
					<Field label="Email address"><input type="email" autoComplete="email" autoFocus /></Field>
					<button className="a-btn a-btn-primary a-wide"><Mail size={15} />Send reset link</button>
					<Link to="/login" className="a-link a-center"><ArrowLeft size={13} /> Back to sign in</Link>
				</form>
			)}
		</AuthCard>
	);
}

// /set-password?token=... (first sign-in) and /reset-password?token=...
export function SetPassword({ first }) {
	const [q] = useQuery();
	const [done, setDone] = useState(false);
	if (q.token === 'expired') {
		return (
			<AuthCard title="This link has expired" lead={first ? 'Links to set a first password work once and only for a limited time. Ask the administrator who created your account to send a new one.' : 'Reset links work once and only for a limited time.'}>
				<div className="a-banner warn"><TriangleAlert size={15} /><span>Nothing was changed.</span></div>
				{first ? <Link to="/login" className="a-link a-center">Go to sign in</Link> : <Link to="/forgot-password" className="a-btn a-btn-primary a-wide">Send a new link</Link>}
			</AuthCard>
		);
	}
	if (done) {
		return (
			<AuthCard title={first ? 'You are all set' : 'Password changed'} lead={first ? 'Your password is set and your address is verified.' : 'Every other session has been signed out, and a notice went to your email address.'}>
				<Link to="/login" className="a-btn a-btn-primary a-wide"><LogIn size={15} />Sign in</Link>
			</AuthCard>
		);
	}
	return (
		<AuthCard title={first ? 'Welcome to tgloggerd' : 'Choose a new password'} lead={first ? 'An administrator created an account for you. Choose your password to finish.' : null}>
			<form className="a-form" onSubmit={(e) => { e.preventDefault(); setDone(true); }}>
				<Field label="Username"><input value="mod_kernel" disabled /></Field>
				<Field label="New password" hint="At least 8 characters."><input type="password" autoComplete="new-password" minLength={8} required /></Field>
				<Field label="Confirm new password"><input type="password" autoComplete="new-password" minLength={8} required /></Field>
				<button className="a-btn a-btn-primary a-wide"><KeyRound size={15} />{first ? 'Set password' : 'Change password'}</button>
			</form>
		</AuthCard>
	);
}

// /verify-email?token=...
export function VerifyEmail() {
	const [q] = useQuery();
	const ok = q.token !== 'expired';
	return (
		<AuthCard title={ok ? 'Email address verified' : 'This link has expired'} lead={ok ? 'research@example.com is now the address for password resets and security notices.' : 'Verification links work once and only for a limited time. Request a new one from Settings.'}>
			<Link to={ok ? '/' : '/settings'} className="a-btn a-btn-primary a-wide">{ok ? 'Continue' : 'Open settings'}</Link>
		</AuthCard>
	);
}

// /welcome/email: right after signing in, an account with no address
// must add and verify one before using the dashboard.
export function AddEmail() {
	const [sent, setSent] = useState('');
	return (
		<AuthCard title="Add your email address" lead={sent ? null : 'Accounts now need a verified email address, for password resets and security notices. Add yours to continue.'}>
			{sent ? (
				<>
					<div className="a-banner"><MailCheck size={15} /><span>We sent a verification link to <b>{sent}</b>. Open it to continue; this page moves on by itself once the address is verified.</span></div>
					<div className="a-btnrow a-center-row"><Btn ghost onClick={() => setSent('')}>Use another address</Btn><Btn ghost>Resend link</Btn></div>
				</>
			) : (
				<form className="a-form" onSubmit={(e) => { e.preventDefault(); setSent(e.target.email.value || 'ops@example.org'); }}>
					<Field label="Email address"><input name="email" type="email" autoComplete="email" autoFocus placeholder="name@example.org" /></Field>
					<button className="a-btn a-btn-primary a-wide"><Mail size={15} />Send verification link</button>
				</form>
			)}
			<Link to="/login" className="a-link a-center"><LogOut size={13} /> Sign out</Link>
		</AuthCard>
	);
}
