// Settings: the signed-in account (email, password, security notices),
// its preferences, and its sessions. Every account has these pages.
import { useState, type FormEvent, type ReactNode } from 'react';
import { Bell, Bookmark, CircleCheck, LogOut, Mail, MailCheck, MailWarning, Monitor, PinOff, ShieldCheck, Smartphone, Trash2, TriangleAlert } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { Avatar, Btn, Card, PageHead, Pill } from '../ui/primitives.tsx';
import { Field, Switch } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, mutate } from '../api/mutate.ts';
import { ago, SITE_DEFAULTS } from '../format.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';

export interface AccountProps {
	account: { username: string; name: string; role: string; email: string | null; email_verified: boolean };
	notices: string[];
}

export interface Prefs {
	timezone?: string | null;
	date_format?: '24h' | '12h' | 'iso' | null;
	lang?: 'en' | 'id' | null;
	theme?: 'system' | 'light' | 'dark' | null;
	rows_per_page?: number | null;
	landing?: string | null;
	chat_auto_load?: boolean | null;
	chat_live?: boolean | null;
	chat_media?: 'auto' | 'click' | null;
	chat_show_deleted?: boolean | null;
	chat_show_edited?: boolean | null;
	chat_show_service?: boolean | null;
}

export interface PreferencesProps {
	prefs: Prefs;
	timezones: string[];
	landing_options: [string, string][];
	pinned: { id: number; title: string; photo: string | null }[];
	saved_searches: { id: number; title: string; url: string }[];
}

export interface SessionsProps {
	sessions: { id: string; device: string; ip: string; place: string; started: string; last: string; current: boolean }[];
}

function SettingsNav({ on }: { on: 'account' | 'prefs' | 'sessions' }) {
	return (
		<nav className="a-settingsnav" aria-label="Settings sections">
			<Link to="/settings" className={on === 'account' ? 'on' : ''}>Your account</Link>
			<Link to="/settings/preferences" className={on === 'prefs' ? 'on' : ''}>Preferences</Link>
			<Link to="/settings/sessions" className={on === 'sessions' ? 'on' : ''}>Sessions</Link>
		</nav>
	);
}

function Frame({ on, title, sub, actions, children }: { on: 'account' | 'prefs' | 'sessions'; title: string; sub?: string; actions?: ReactNode; children: ReactNode }) {
	const crumbs: [string, string | null][] = on === 'account' ? [['Settings', null]] : [['Settings', '/settings'], [title, null]];
	return (
		<Shell crumbs={crumbs}>
			<PageHead title={title} sub={sub} actions={actions} />
			<div className="a-settings"><SettingsNav on={on} /><div className="a-stack">{children}</div></div>
		</Shell>
	);
}

function errText(e: unknown): string {
	return e instanceof ApiError ? e.message : 'The server could not be reached.';
}

function ChangeEmail({ onClose }: { onClose: () => void }) {
	const [sent, setSent] = useState('');
	const [err, setErr] = useState('');
	const submit = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const f = new FormData(e.currentTarget);
		const email = String(f.get('email') ?? '');
		try {
			await mutate('POST', '/api/account/email', { email, password: String(f.get('password') ?? '') });
			setSent(email);
		} catch (x) { setErr(errText(x)); }
	};
	return (
		<Modal title="Change email address" icon={<Mail size={16} />} onClose={onClose}
			footer={sent ? <><span className="a-grow" /><Btn primary onClick={onClose}>Done</Btn></>
				: <><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><button className="a-btn a-btn-primary" type="submit" form="chg-email">Send verification link</button></>}>
			{sent ? (
				<div className="a-banner"><MailCheck size={15} /><span>A verification link is on its way to <b>{sent}</b>. The address changes once you open it; your current address is told about the change.</span></div>
			) : (
				<form id="chg-email" className="a-form" onSubmit={submit}>
					{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
					<Field label="New email address"><input name="email" type="email" autoComplete="email" required /></Field>
					<Field label="Current password" hint="To confirm it is you."><input name="password" type="password" autoComplete="current-password" required /></Field>
				</form>
			)}
		</Modal>
	);
}

function AccountPage({ page }: { page: PageData<AccountProps> }) {
	const { account: a, notices } = page.props;
	const [chg, setChg] = useState(false);
	const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
	const changePassword = async (e: FormEvent<HTMLFormElement>) => {
		e.preventDefault();
		const form = e.currentTarget;
		const f = new FormData(form);
		if (f.get('new_password') !== f.get('confirm_password')) { setMsg({ ok: false, text: 'The new passwords do not match.' }); return; }
		try {
			await mutate('POST', '/api/account/password', { current_password: f.get('current_password'), new_password: f.get('new_password') });
			form.reset();
			setMsg({ ok: true, text: 'Password changed. Every other session has been signed out.' });
		} catch (x) { setMsg({ ok: false, text: errText(x) }); }
	};
	return (
		<Frame on="account" title="Settings">
			<Card title="Account" className="a-narrow">
				<p className="a-muted">Signed in as <b>{a.username}</b> ({a.name}) . {a.role}</p>
				<div className="a-pref">
					<div><b>Email address</b><span className="a-muted sm">For password resets and security notices.</span></div>
					<div className="a-pref-ctl a-row">
						{a.email ? <span className="a-emailline">{a.email_verified ? <MailCheck size={14} className="a-ok" /> : <MailWarning size={14} className="a-warnicon" />}<span className="a-break">{a.email}</span><span className="a-muted sm">{a.email_verified ? 'verified' : 'not verified'}</span></span>
							: <span className="a-emailline a-warn"><MailWarning size={14} />No address yet</span>}
						<Btn ghost onClick={() => setChg(true)}>{a.email ? 'Change' : 'Add'}</Btn>
					</div>
				</div>
			</Card>
			<Card title="Change password" className="a-narrow">
				{msg && <div className={`a-banner ${msg.ok ? '' : 'warn'}`} role={msg.ok ? 'status' : 'alert'}>{msg.ok ? <CircleCheck size={15} /> : <TriangleAlert size={15} />}<span>{msg.text}</span></div>}
				<form className="a-form" onSubmit={changePassword}>
					<Field label="Current password"><input name="current_password" type="password" autoComplete="current-password" required /></Field>
					<Field label="New password" hint="At least 8 characters."><input name="new_password" type="password" autoComplete="new-password" minLength={8} required /></Field>
					<Field label="Confirm new password"><input name="confirm_password" type="password" autoComplete="new-password" minLength={8} required /></Field>
					<p className="a-muted sm"><ShieldCheck size={13} /> Changing the password signs out every other browser and device.</p>
					<div><button className="a-btn a-btn-primary">Change password</button></div>
				</form>
			</Card>
			<Card title="Security notices" className="a-narrow">
				<p className="a-muted sm"><Bell size={13} /> Always emailed to your verified address; they cannot be turned off.</p>
				<ul className="a-notices">{notices.map((n) => <li key={n}>{n}</li>)}</ul>
			</Card>
			{chg && <ChangeEmail onClose={() => setChg(false)} />}
		</Frame>
	);
}

// A preference left unset follows the site default, so "Site default" is
// always the first choice.
function Sel<T extends string | number>({ value, onChange, options, def }: { value: T | null | undefined; onChange: (v: T | null) => void; options: [T, string][]; def: string }) {
	return (
		<select className="a-select" value={value ?? ''} onChange={(e) => {
			const raw = e.target.value;
			const hit = options.find(([k]) => String(k) === raw);
			onChange(hit ? hit[0] : null);
		}}>
			<option value="">Site default ({def})</option>
			{options.map(([k, t]) => <option key={String(k)} value={String(k)}>{t}</option>)}
		</select>
	);
}

function Pref({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
	return <div className="a-pref"><div><b>{label}</b>{hint && <span className="a-muted sm">{hint}</span>}</div><div className="a-pref-ctl">{children}</div></div>;
}

function PreferencesPage({ page }: { page: PageData<PreferencesProps> }) {
	const { navigate } = useNav();
	const props = page.props;
	const [p, setP] = useState<Prefs>(props.prefs);
	const [state, setState] = useState<'' | 'saving' | 'saved' | string>('');
	const set = <K extends keyof Prefs>(k: K) => (v: Prefs[K]) => setP({ ...p, [k]: v });
	const flag = (k: keyof Prefs, def: boolean) => <Switch checked={(p[k] as boolean | null | undefined) ?? def} onChange={(v) => setP({ ...p, [k]: v })} />;
	const save = async () => {
		setState('saving');
		try {
			await mutate('PUT', '/api/account/preferences', p);
			setState('saved');
			// The viewer (timezone, theme, menu...) comes from the server.
			navigate(page.url, { replace: true, scroll: false });
		} catch (x) { setState(errText(x)); }
	};
	const remove = async (path: string) => {
		try { await mutate('DELETE', path); navigate(page.url, { replace: true, scroll: false }); } catch (x) { setState(errText(x)); }
	};
	return (
		<Frame on="prefs" title="Preferences" sub="How dates, lists and chats look for you, on every device you sign in on.">
			<Card title="Region and display" className="a-narrow">
				<Pref label="Timezone" hint="Dates are stored in UTC and shown in this zone."><Sel value={p.timezone} onChange={set('timezone')} def={SITE_DEFAULTS.timezone} options={props.timezones.map((z) => [z, z])} /></Pref>
				<Pref label="Date and time format"><Sel value={p.date_format} onChange={set('date_format')} def="24-hour" options={[['24h', '24-hour'], ['12h', '12-hour'], ['iso', 'ISO, 2026-11-17 16:05']]} /></Pref>
				<Pref label="Language"><Sel value={p.lang} onChange={set('lang')} def="English" options={[['en', 'English'], ['id', 'Bahasa Indonesia']]} /></Pref>
				<Pref label="Theme"><Sel value={p.theme} onChange={set('theme')} def="follow the system" options={[['system', 'Follow the system'], ['light', 'Light'], ['dark', 'Dark']]} /></Pref>
				<Pref label="Rows per page"><Sel value={p.rows_per_page} onChange={set('rows_per_page')} def={String(SITE_DEFAULTS.rows_per_page)} options={[[25, '25'], [50, '50'], [100, '100']]} /></Pref>
				<Pref label="Landing page" hint="Where signing in takes you."><Sel value={p.landing} onChange={set('landing')} def="Dashboard" options={props.landing_options} /></Pref>
			</Card>
			<Card title="Chat view" className="a-narrow">
				<Pref label="Load older messages while scrolling">{flag('chat_auto_load', true)}</Pref>
				<Pref label="Live updates" hint="New messages appear as they are logged.">{flag('chat_live', true)}</Pref>
				<Pref label="Media"><Sel value={p.chat_media} onChange={set('chat_media')} def="load automatically" options={[['auto', 'Load automatically'], ['click', 'Load on click']]} /></Pref>
				<Pref label="Show deleted messages">{flag('chat_show_deleted', true)}</Pref>
				<Pref label="Show edit markers">{flag('chat_show_edited', true)}</Pref>
				<Pref label="Show service messages" hint="Joins, leaves, pins, photo changes.">{flag('chat_show_service', true)}</Pref>
			</Card>
			<div className="a-row">
				<Btn primary onClick={() => void save()} disabled={state === 'saving'}>Save preferences</Btn>
				{state === 'saved' && <span className="a-ok"><CircleCheck size={14} />Saved</span>}
				{state && state !== 'saving' && state !== 'saved' && <span className="a-warn">{state}</span>}
			</div>
			<Card title="Pinned groups" className="a-narrow">
				<p className="a-muted sm">Pinned groups appear at the top of the sidebar. Pin a group from its page.</p>
				{props.pinned.length ? <ul className="a-audit">{props.pinned.map((g) => (
					<li key={g.id}><Avatar id={g.id} name={g.title} photo={g.photo} size={22} square /><Link to={`/groups/${g.id}`} className="a-grow a-rowlink">{g.title}</Link>
						<button type="button" className="a-icon-btn sm" aria-label={`Unpin ${g.title}`} onClick={() => void remove(`/api/account/pinned/${g.id}`)}><PinOff size={13} /></button></li>
				))}</ul> : <p className="a-muted">Nothing pinned.</p>}
			</Card>
			<Card title="Saved searches" className="a-narrow">
				<p className="a-muted sm">Save a search from any list's filter bar.</p>
				{props.saved_searches.length ? <ul className="a-audit">{props.saved_searches.map((s) => (
					<li key={s.id}><Bookmark size={14} /><Link to={s.url} className="a-grow a-rowlink">{s.title}</Link>
						<button type="button" className="a-icon-btn sm" aria-label={`Delete ${s.title}`} onClick={() => void remove(`/api/account/saved-searches/${s.id}`)}><Trash2 size={13} /></button></li>
				))}</ul> : <p className="a-muted">No saved searches.</p>}
			</Card>
		</Frame>
	);
}

function SessionsPage({ page }: { page: PageData<SessionsProps> }) {
	const { navigate } = useNav();
	const v = page.viewer;
	const [err, setErr] = useState('');
	const act = async (method: 'POST' | 'DELETE', path: string) => {
		try { await mutate(method, path); navigate(page.url, { replace: true, scroll: false }); } catch (x) { setErr(errText(x)); }
	};
	const others = page.props.sessions.some((s) => !s.current);
	return (
		<Frame on="sessions" title="Sessions" sub="Where your account is signed in."
			actions={others ? <Btn ghost danger icon={LogOut} onClick={() => void act('POST', '/api/account/sessions/sign-out-others')}>Sign out all others</Btn> : undefined}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<div className="a-card a-tablecard a-narrow">
				<ul className="a-sessions">{page.props.sessions.map((s) => (
					<li key={s.id}>
						<span className="a-sessicon">{/Android|iPhone|iPad/.test(s.device) ? <Smartphone size={18} /> : <Monitor size={18} />}</span>
						<span className="a-grow a-minw">
							<b>{s.device}{s.current && <Pill tone="ok">this browser</Pill>}</b>
							<span className="a-muted sm">{s.place} . {s.ip} . signed in {ago(s.started, v)}</span>
						</span>
						<span className="a-muted sm" title={s.last}>{s.current ? 'active now' : `active ${ago(s.last, v)}`}</span>
						{!s.current && <Btn ghost onClick={() => void act('DELETE', `/api/account/sessions/${s.id}`)}>Sign out</Btn>}
					</li>
				))}</ul>
			</div>
		</Frame>
	);
}

export const settingsRoute: RouteDef<AccountProps> = { Page: AccountPage, head: () => ({ title: 'Settings - tgloggerd', robots: 'noindex' }) };
export const preferencesRoute: RouteDef<PreferencesProps> = { Page: PreferencesPage, head: () => ({ title: 'Preferences - tgloggerd', robots: 'noindex' }) };
export const sessionsRoute: RouteDef<SessionsProps> = { Page: SessionsPage, head: () => ({ title: 'Sessions - tgloggerd', robots: 'noindex' }) };
