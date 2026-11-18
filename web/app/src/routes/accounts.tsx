// Administration > Accounts (web-accounts.md sections 2-5 and 8): who can
// sign in, each account's role, an admin's federations and restrictions,
// a member's grants per federation and abilities, and its audit trail.
import { useState } from 'react';
import { Ban, Check, Copy, Crown, History, Info, KeyRound, LogOut, MailCheck, MailWarning, Network, Pencil, Plus, Send, ShieldCheck, TriangleAlert, User as UserIcon } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { useNav } from '../nav.tsx';
import { Avatar, Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { Field, MultiPicker, Seg, Switch, type Finder, type PickItem } from '../ui/forms.tsx';
import { federationGroups } from '../api/federations.ts';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { ago } from '../format.ts';
import type { PageData, Role } from '../page.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type Fed = S['Fed'];

export type AccountRow = S['AccountRow'];

export type AccountsProps = S['AccountsProps'];

const ROLE_ICON = { superadmin: Crown, admin: ShieldCheck, member: UserIcon } as const;
const ROLES: [Role, string, string][] = [
	['superadmin', 'Superadmin', 'Everything: all accounts, roles, federations and settings.'],
	['admin', 'Admin', 'Everything except what a superadmin restricts. Manages members of its federations only.'],
	['member', 'Member', 'Only the groups granted in its federations, and the abilities given.'],
];
const ABILITIES: [S['Ability'], string, string][] = [
	['forward', 'Set up forwarding', 'Own Telegram and Discord forwarding for its granted groups.'],
	['fetch_files', 'Fetch files from Telegram', 'Download media the archive did not store.'],
];

function RolePill({ role }: { role: Role }) {
	const I = ROLE_ICON[role];
	return <Pill tone={role === 'superadmin' ? 'accent' : role === 'admin' ? 'info' : ''}><I size={11} />{role}</Pill>;
}

function accessSummary(a: AccountRow): string {
	if (a.role === 'superadmin') return 'Everything';
	if (a.role === 'admin') {
		const r = a.restrict;
		const parts = [];
		if (r?.hide_private) parts.push('no private messages');
		if (r?.hide_groups.length) parts.push(`${r.hide_groups.length} hidden group${r.hide_groups.length > 1 ? 's' : ''}`);
		if (r?.join_chat) parts.push('can join chats');
		return parts.length ? parts.join(', ') : 'No restrictions';
	}
	const n = a.grants.reduce((x, g) => x + g.groups.length, 0);
	return [`${n} group${n === 1 ? '' : 's'}`, ...a.abilities.map((k) => (k === 'forward' ? 'forwarding' : 'fetch files'))].join(', ');
}

function EmailLine({ a }: { a: AccountRow }) {
	if (!a.email) return <span className="a-emailline a-warn"><MailWarning size={14} />No address yet; asked for one at the next sign-in</span>;
	return (
		<span className="a-emailline">
			{a.email_verified ? <MailCheck size={14} className="a-ok" /> : <MailWarning size={14} className="a-warnicon" />}
			<span className="a-break">{a.email}</span>
			<span className="a-muted sm">{a.email_verified ? 'verified' : a.invited ? 'has not chosen its own password yet' : 'not verified'}</span>
		</span>
	);
}

// A temporary password, shown once. It lives only in this component's
// state while the dialog is open; nothing stores or logs it.
function TemporaryPassword({ username, password }: { username: string; password: string }) {
	const [copied, setCopied] = useState(false);
	const copy = () => void navigator.clipboard?.writeText(password).then(() => setCopied(true), () => setCopied(false));
	return (
		<div className="a-form">
			<p>The temporary password for <b>{username}</b>:</p>
			<div className="a-code">
				<pre aria-label="Temporary password">{password}</pre>
				<Btn ghost icon={copied ? Check : Copy} onClick={copy}>{copied ? 'Copied' : 'Copy'}</Btn>
			</div>
			<div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>Shown only now: it is not stored and cannot be shown again. Give it to {username} through a channel you trust; they must choose their own password at the first sign-in. If it is lost, reset the password again.</span></div>
		</div>
	);
}

// What the access editor cannot change yet, in one sentence; nothing
// when everything can be.
function NotYet({ editable: e }: { editable: AccountsProps['editable'] }) {
	// The restrictions built one by one: name only those still missing.
	const restrictions = e.restrict_private && e.restrict_groups ? 'Join chat' : e.restrict_private ? 'hidden groups and Join chat' : 'restrictions';
	const off = [!e.federations && 'federations', !e.group_grants && 'group grants', !e.restrictions && restrictions, !e.abilities && 'abilities'].filter((x): x is string => !!x);
	if (!off.length) return null;
	const list = off.length === 1 ? off[0]! : `${off.slice(0, -1).join(', ')} and ${off.at(-1)}`;
	const plural = off.length > 1 || off[0] === 'hidden groups and Join chat';
	return <div className="a-banner" role="note"><Info size={15} /><span>{list[0]!.toUpperCase() + list.slice(1)} {plural ? 'are' : 'is'} not available yet.</span></div>;
}

// A federation's groups to choose from: its own group list, searched on
// the server, so nothing outside the federation can be granted.
const fedFinder = (fed: number): Finder => ({
	key: `fed:${fed}`,
	find: (q, signal) => federationGroups(fed, { q, limit: 20 }, signal).then((p) => p.items.map((g): PickItem => ({ id: g.id, title: g.title, kind: 'group', photo: g.photo, ...(g.username ? { sub: `@${g.username}` } : {}) }))),
});

type Draft = AccountRow & { isNew?: boolean };

function Editor({ init, props, viewerRole, onClose, onSaved }: { init: Draft; props: AccountsProps; viewerRole: Role; onClose: () => void; onSaved: () => void }) {
	const isNew = !!init.isNew;
	const isSuper = viewerRole === 'superadmin';
	const [a, setA] = useState<Draft>(init);
	const [tab, setTab] = useState<'account' | 'access' | 'activity'>('account');
	const [editEmail, setEditEmail] = useState(false);
	const [err, setErr] = useState('');
	const [note, setNote] = useState('');
	// A temporary password just issued, held only while this dialog is
	// open: never stored, never shown again.
	const [temp, setTemp] = useState<{ username: string; password: string; reset: boolean } | null>(null);
	// A new account's first password (empty: the server generates one), or
	// a set-password link instead. Held only while this dialog is open.
	const [firstPw, setFirstPw] = useState('');
	const [invite, setInvite] = useState(false);
	const [done, setDone] = useState<string | null>(null);
	const email = (a.email ?? '').trim();
	const set = (patch: Partial<Draft>) => setA({ ...a, ...patch });
	const roles = isSuper ? ROLES : ROLES.filter((r) => r[0] === 'member');
	const grantOf = (fid: number) => a.grants.find((g) => g.fed === fid);
	// What the server lets this page change yet (AccountsProps.editable).
	const E = props.editable;
	// Whether set-password links can be mailed at all (AccountsProps.mail).
	const mail = props.mail !== false;
	const feds = E.federations ? props.federations : props.federations.filter((f) => f.default);
	const mine = (fed: number) => props.federations.some((f) => f.id === fed);
	// The member's federations another admin manages: shown, not changed.
	const others = a.grants.filter((g) => !mine(g.fed));
	const fedName = (id: number) => props.all_federations.find((f) => f.id === id)?.name ?? `Federation ${id}`;
	const call = async (fn: () => Promise<unknown>, after: 'close' | string) => {
		setErr('');
		try {
			await fn();
			if (after === 'close') { onSaved(); onClose(); } else { setNote(after); onSaved(); }
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const create = async () => {
		setErr('');
		if (!invite && firstPw && new TextEncoder().encode(firstPw).length < 8) { setErr('The first password needs at least 8 characters; or leave it empty to have one generated.'); return; }
		try {
			const r = await ok(api.POST('/api/accounts', { body: {
				username: a.username, name: a.name, role: a.role, email: email || null,
				...(invite ? { invite: true } : { password: firstPw || null }),
				...(a.role === 'member' && a.grants.length ? { grants: a.grants.map((g) => ({ fed: g.fed, group_ids: g.groups.map((x) => x.id) })) } : {}),
			} }));
			if (r.temporary_password) setTemp({ username: a.username, password: r.temporary_password, reset: false });
			else setDone(r.invited ? `An email with a link to choose a password went to ${email}.` : `${a.username} has the password you typed, and must change it at the first sign-in.`);
			setFirstPw('');
			onSaved();
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const reset = async () => {
		if (!confirm(`Reset ${a.username}'s password? Every session of the account ends. A set-password link goes to its verified address when mail is set up; otherwise you get a temporary password, shown once.`)) return;
		setErr(''); setNote('');
		try {
			const r = await ok(api.POST('/api/accounts/{id}/password-reset', { params: { path: { id: a.id } } }));
			if (r.temporary_password) setTemp({ username: a.username, password: r.temporary_password, reset: true });
			else setNote(r.link_sent ? 'A link to choose a new password went to the account\'s address.' : 'The password was reset.');
			onSaved();
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const save = () => (isNew ? create() : call(() => ok(api.PATCH('/api/accounts/{id}', { params: { path: { id: a.id } }, body: {
			name: a.name, role: a.role,
			...(a.email !== init.email ? { email: a.email ?? '' } : {}),
			...(a.role === 'admin' && E.federations ? { admin_of: a.admin_of } : {}),
			...(a.role === 'admin' && E.restrictions && a.restrict ? { restrict: { ...a.restrict, hide_groups: a.restrict.hide_groups.map((g) => g.id) } } : {}),
			// The restrictions built so far (hidden private messages, hidden
			// groups; not Join chat): a superadmin's save sends them for an
			// admin, including one this save makes admin.
			...(a.role === 'admin' && partial ? { restrict: { hide_private: E.restrict_private ? restrict.hide_private : false, hide_groups: E.restrict_groups ? restrict.hide_groups.map((g) => g.id) : [], join_chat: false } } : {}),
			// Only the federations the viewer administers: naming another
			// is refused, and the member's others stay as they are.
			...(a.role === 'member' ? { grants: a.grants.filter((g) => mine(g.fed)).map((g) => ({ fed: g.fed, group_ids: E.group_grants ? g.groups.map((x) => x.id) : [] })) } : {}),
			...(a.role === 'member' && E.abilities ? { abilities: a.abilities } : {}),
		} })), 'close'));
	const restrict = a.restrict ?? { hide_private: false, hide_groups: [], join_chat: false };
	// Only a superadmin may change them; to anyone else the PATCH is 403.
	const partial = !E.restrictions && isSuper && (E.restrict_private || E.restrict_groups);
	// A member's groups per federation: on the Access tab, and when
	// creating a member (an admin must grant at least one federation).
	const grantsBlock = (
		<>
			<h3 className="a-sec">{E.group_grants ? 'Groups, per federation' : 'Federations'}</h3>
			{E.group_grants && <p className="a-muted sm">A member sees only the groups granted here, and people only as they appear in those groups. Members never see private messages.</p>}
			{feds.map((f) => {
				const g = grantOf(f.id);
				return (
					<section key={f.id} className={`a-fedgrant ${g ? 'on' : ''}`}>
						<header>
							<Switch checked={!!g} onChange={() => set({ grants: g ? a.grants.filter((x) => x.fed !== f.id) : [...a.grants, { fed: f.id, groups: [] }] })} label={<><Network size={14} /> {f.name}</>} />
							{g && E.group_grants && <span className="a-muted sm">{g.groups.length} of {f.group_count} groups</span>}
						</header>
						{g && E.group_grants && <MultiPicker value={g.groups} onChange={(gs) => set({ grants: a.grants.map((x) => (x.fed !== f.id ? x : { ...x, groups: gs })) })} search={fedFinder(f.id)} placeholder={`Add a group of ${f.name}...`} />}
						{g && E.group_grants && !g.groups.length && <p className="a-muted sm">No group granted yet: the member sees nothing of {f.name}.</p>}
					</section>
				);
			})}
			{others.length > 0 && (
				// Their groups are not shown to an admin (the server leaves them
				// out), so only the federations are named.
				<p className="a-muted sm" role="note"><Info size={13} /> Also filed in {others.map((g, i) => <span key={g.fed}>{i ? ', ' : ''}<b>{fedName(g.fed)}</b></span>)}, which you do not administer; its grants there stay as they are.</p>
			)}
		</>
	);
	const grantRequired = isNew && !isSuper && a.role === 'member' && a.grants.length === 0;
	const notYet = <NotYet editable={E} />;
	if (done) {
		return (
			<Modal title={`${a.username} is created`} icon={<UserIcon size={16} />} onClose={onClose}
				footer={<><span className="a-grow" /><Btn primary onClick={onClose}>Done</Btn></>}>
				<div className="a-banner" role="status"><MailCheck size={15} /><span>{done}</span></div>
			</Modal>
		);
	}
	if (temp) {
		return (
			<Modal title={temp.reset ? `New password for ${temp.username}` : `${temp.username} is created`} icon={<KeyRound size={16} />} onClose={onClose}
				footer={<><span className="a-grow" /><Btn primary onClick={onClose}>Done</Btn></>}>
				<TemporaryPassword username={temp.username} password={temp.password} />
			</Modal>
		);
	}
	return (
		<Modal wide title={isNew ? 'New account' : init.name} icon={!isNew ? <Avatar id={init.id} name={init.name} size={24} /> : undefined} onClose={onClose}
			footer={<>
				{!isNew && !init.last_superadmin && <Btn ghost danger icon={Ban} onClick={() => void call(() => ok(a.active ? api.POST('/api/accounts/{id}/disable', { params: { path: { id: a.id } } }) : api.POST('/api/accounts/{id}/enable', { params: { path: { id: a.id } } })), 'close')}>{a.active ? 'Disable account' : 'Enable account'}</Btn>}
				{init.last_superadmin && <span className="a-muted sm a-note"><Info size={13} />The last active superadmin cannot be disabled or demoted.</span>}
				<span className="a-grow" />
				<Btn ghost onClick={onClose}>Cancel</Btn>
				{grantRequired && <span className="a-muted sm a-note"><Info size={13} />Grant at least one federation.</span>}
				<Btn primary disabled={grantRequired} onClick={() => void save()}>{isNew ? 'Create account' : 'Save changes'}</Btn>
			</>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			{note && <div className="a-banner" role="status"><MailCheck size={15} /><span>{note}</span></div>}
			{!isNew && <Seg value={tab} onChange={setTab} items={[['account', 'Account'], ['access', 'Access'], ['activity', 'Activity']]} />}

			{(isNew || tab === 'account') && (
				<div className="a-acctgrid">
					<section className="a-form">
						<h3 className="a-sec">Sign-in</h3>
						<Field label="Username" hint={isNew ? 'Used to sign in. Letters, digits, underscores.' : 'Usernames do not change.'}>
							<input value={a.username} onChange={(e) => set({ username: e.target.value })} disabled={!isNew} autoComplete="off" />
						</Field>
						<Field label="Display name"><input value={a.name} onChange={(e) => set({ name: e.target.value })} /></Field>
						{isNew || a.invited ? (
							<Field label="Email address" hint={isNew ? 'Optional. For password resets and security notices; must not be used by another account.' : 'Not signed in with its own password yet, so the address can still be corrected.'}>
								<input type="email" value={a.email ?? ''} onChange={(e) => set({ email: e.target.value })} autoComplete="off" />
							</Field>
						) : editEmail ? (
							<Field label="New email address" hint="Superadmins only, for someone who lost their mailbox: verified by link, the old address is told, audited.">
								<input type="email" value={a.email ?? ''} onChange={(e) => set({ email: e.target.value })} autoComplete="off" autoFocus />
							</Field>
						) : (
							<div className="a-field">
								<span className="a-field-label">Email address</span>
								<span className="a-row"><EmailLine a={init} />{isSuper && <Btn ghost onClick={() => setEditEmail(true)}>Change address</Btn>}</span>
								<span className="a-field-hint">Accounts change their own address in Settings; the old address is told.</span>
							</div>
						)}
						{isNew ? (
							<>
								{mail && <Switch checked={invite && !!email} disabled={!email} onChange={setInvite} label="Email a set-password link instead" />}
								{mail && invite && email
									? <p className="a-muted sm">The account chooses its password from the link. Needs mail set up on the server.</p>
									: <Field label="First password" hint="Optional: leave it empty to have one generated and shown to you once. At least 8 characters. Either way, it must be changed at the first sign-in.">
										<input type="password" value={firstPw} onChange={(e) => setFirstPw(e.target.value)} autoComplete="new-password" />
									</Field>}
							</>
						) : (
							<div className="a-btnrow">
								<Btn ghost icon={KeyRound} onClick={() => void reset()}>Reset password</Btn>
								{mail && a.invited && a.email && <Btn ghost icon={Send} onClick={() => void call(() => ok(api.POST('/api/accounts/{id}/resend-invite', { params: { path: { id: a.id } } })), 'A new set-password link is on its way; earlier ones stop working.')}>Resend set-password link</Btn>}
								<Btn ghost icon={LogOut} onClick={() => void call(() => ok(api.POST('/api/accounts/{id}/sign-out-everywhere', { params: { path: { id: a.id } } })), 'Every session of this account has been signed out.')}>Sign out everywhere</Btn>
							</div>
						)}
					</section>
					<section className="a-form">
						<h3 className="a-sec">Role</h3>
						<div className="a-roles">{roles.map(([k, t, d]) => {
							const I = ROLE_ICON[k];
							const locked = init.last_superadmin && k !== 'superadmin';
							return (
								<label key={k} className={`a-role ${a.role === k ? 'on' : ''} ${locked ? 'is-off' : ''}`}>
									<input type="radio" name="role" checked={a.role === k} disabled={locked} onChange={() => set({ role: k })} />
									<I size={16} /><span><b>{t}</b><span className="a-muted sm">{d}</span></span>
								</label>
							);
						})}</div>
						{isNew && a.role === 'admin' && <p className="a-muted sm">Federations and restrictions are set on the Access tab once the account exists.</p>}
						{isNew && a.role === 'member' && <p className="a-muted sm">Abilities are set on the Access tab once the account exists.</p>}
					</section>
				</div>
			)}

			{!isNew && tab === 'access' && a.role === 'superadmin' && <p className="a-muted">Superadmins see and manage everything; nothing here applies.</p>}

			{!isNew && tab === 'access' && a.role === 'admin' && (
				<div className="a-acctgrid">
					{!(E.federations && E.restrictions) && notYet}
					{E.federations && <section className="a-form">
						<h3 className="a-sec">Administers</h3>
						<p className="a-muted sm">Federations limit what this admin manages, not what it sees.</p>
						<div className="a-checkgrid">{props.all_federations.map((f) => {
							const on = a.admin_of.includes(f.id);
							return (
								<label key={f.id} className={`a-checkchip ${on ? 'on' : ''}`}>
									<input type="checkbox" checked={on} onChange={() => set({ admin_of: on ? a.admin_of.filter((x) => x !== f.id) : [...a.admin_of, f.id] })} />
									<Network size={13} />{f.name}
								</label>
							);
						})}</div>
					</section>}
					{partial && <section className="a-form">
						<h3 className="a-sec">Restrictions</h3>
						<ul className="a-perms">
							{E.restrict_private && <li><label><input type="checkbox" checked={restrict.hide_private} onChange={() => set({ restrict: { ...restrict, hide_private: !restrict.hide_private } })} /><span><b>Hide private messages</b><span className="a-muted sm">No private-message menu, pages or API; their URLs answer 404.</span></span></label></li>}
							{E.restrict_groups && <li><div className="a-restrict"><span><b>Hide groups</b><span className="a-muted sm">The groups, their messages, members, files, forwarding and MCP exposure become invisible to this admin. A public group stays public.</span></span>
								<MultiPicker value={restrict.hide_groups} onChange={(gs) => set({ restrict: { ...restrict, hide_groups: gs } })} search={{ kind: 'group' }} placeholder="Add a group to hide..." /></div></li>}
						</ul>
					</section>}
					{E.restrictions && <section className="a-form">
						<h3 className="a-sec">Restrictions</h3>
						<ul className="a-perms">
							<li><label><input type="checkbox" checked={restrict.hide_private} onChange={() => set({ restrict: { ...restrict, hide_private: !restrict.hide_private } })} /><span><b>Hide private messages</b><span className="a-muted sm">No private-message menu, pages or API; their URLs answer 404.</span></span></label></li>
							<li><div className="a-restrict"><span><b>Hide groups</b><span className="a-muted sm">The groups, their messages, members, files, forwarding and MCP exposure become invisible.</span></span>
								<MultiPicker value={restrict.hide_groups} onChange={(gs) => set({ restrict: { ...restrict, hide_groups: gs } })} search={{ kind: 'group' }} placeholder="Add a group to hide..." /></div></li>
							<li><label><input type="checkbox" checked={restrict.join_chat} onChange={() => set({ restrict: { ...restrict, join_chat: !restrict.join_chat } })} /><span><b>Allow Join chat</b><span className="a-muted sm">Make the logging account join groups, and add them to its federations.</span></span></label></li>
						</ul>
					</section>}
				</div>
			)}

			{isNew && a.role === 'member' && <div className="a-form">{!(E.federations && E.group_grants) && notYet}{grantsBlock}</div>}

			{!isNew && tab === 'access' && a.role === 'member' && (
				<div className="a-form">
					{!(E.federations && E.group_grants && E.abilities) && notYet}
					{grantsBlock}
					{E.abilities && <>
						<h3 className="a-sec">Abilities</h3>
						<ul className="a-perms">{ABILITIES.map(([k, t, d]) => (
							<li key={k}><label><input type="checkbox" checked={a.abilities.includes(k)} onChange={() => set({ abilities: a.abilities.includes(k) ? a.abilities.filter((x) => x !== k) : [...a.abilities, k] })} /><span><b>{t}</b><span className="a-muted sm">{d}</span></span></label></li>
						))}</ul>
					</>}
				</div>
			)}

			{!isNew && tab === 'activity' && (
				<ol className="a-audit">{init.audit.map((r, i) => (
					<li key={i}><History size={14} /><span className="a-grow">{r.what}<span className="a-muted"> by {r.by}</span></span><time className="a-muted sm" dateTime={r.at}>{r.at.slice(0, 10)}</time></li>
				))}</ol>
			)}
		</Modal>
	);
}

function Accounts({ page }: { page: PageData<AccountsProps> }) {
	const { navigate } = useNav();
	const v = page.viewer;
	const role = v.account?.role ?? 'member';
	const isSuper = role === 'superadmin';
	const [edit, setEdit] = useState<Draft | null>(null);
	const [filter, setFilter] = useState<'all' | Role>('all');
	const rows = filter === 'all' ? page.props.accounts : page.props.accounts.filter((a) => a.role === filter);
	const fedName = (id: number) => page.props.all_federations.find((f) => f.id === id)?.name ?? String(id);
	const feds = (a: AccountRow) => (a.role === 'admin' ? a.admin_of : a.grants.map((g) => g.fed));
	const blank: Draft = { isNew: true, id: 0, username: '', name: '', email: '', email_verified: false, invited: false, role: 'member', active: true, last_signin: null, last_superadmin: false, admin_of: [], restrict: null, grants: [], abilities: [], audit: [] };
	const refresh = () => navigate(page.url, { replace: true, scroll: false });
	const last = (a: AccountRow) => (a.invited ? <Pill tone="info"><KeyRound size={11} />invited</Pill> : !a.email ? <Pill tone="danger"><MailWarning size={11} />no email</Pill> : a.last_signin ? ago(a.last_signin, v) : <span className="a-muted">never</span>);
	return (
		<Shell crumbs={[['Accounts', null]]}>
			<PageHead title="Accounts" sub={isSuper ? 'Who can sign in, their role, and what each one may see and manage.' : 'Members of the federations you administer.'}
				actions={<Btn primary icon={Plus} onClick={() => setEdit(blank)}>{isSuper ? 'New account' : 'New member'}</Btn>} />
			{isSuper && <div className="a-toolbar"><Seg value={filter} onChange={setFilter} items={[['all', 'All'], ['superadmin', 'Superadmins'], ['admin', 'Admins'], ['member', 'Members']]} /></div>}
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Account</th><th>Role</th><th>Federations</th><th>Access</th><th>Last sign-in</th><th /></tr></thead>
						<tbody>{rows.map((a) => (
							<tr key={a.id} className={a.active ? '' : 'a-off'}>
								<td><button type="button" className="a-usercell a-plain" onClick={() => setEdit(a)}>
									<Avatar id={a.id} name={a.name} size={32} />
									<span><span className="a-uname">{a.name}</span><span className="a-handle">{a.username}{a.email ? ` . ${a.email}` : ''}{!a.active && ' . disabled'}</span></span>
								</button></td>
								<td><RolePill role={a.role} /></td>
								<td className="a-wrapcell">{a.role === 'superadmin' ? <span className="a-muted">All</span> : feds(a).map((f) => <Pill key={f}><Network size={11} />{fedName(f)}</Pill>)}</td>
								<td className="a-muted">{accessSummary(a)}</td>
								<td>{last(a)}</td>
								<td className="a-rowactions"><button type="button" className="a-icon-btn" aria-label={`Edit ${a.username}`} onClick={() => setEdit(a)}><Pencil size={15} /></button></td>
							</tr>
						))}</tbody>
					</table>
				</div>
				<ul className="a-list">{rows.map((a) => (
					<li key={a.id}><button type="button" className="a-list-row a-plain" onClick={() => setEdit(a)}>
						<Avatar id={a.id} name={a.name} size={36} />
						<span className="a-list-main"><span className="a-uname">{a.name}</span><span className="a-handle">{a.role} . {accessSummary(a)}</span></span>
						<span className="a-list-side"><span className="a-muted sm">{a.active ? last(a) : 'disabled'}</span></span>
					</button></li>
				))}</ul>
			</div>
			<p className="a-muted a-foot">Accounts are disabled, never deleted, so the audit trail and ownership keep pointing at them.</p>
			{edit && <Editor init={edit} props={page.props} viewerRole={role} onClose={() => setEdit(null)} onSaved={refresh} />}
		</Shell>
	);
}

export const accountsRoute: RouteDef<AccountsProps> = { Page: Accounts, head: () => ({ title: 'Accounts - tgloggerd', robots: 'noindex' }) };
