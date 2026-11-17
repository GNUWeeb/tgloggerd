// Administration > Accounts (web-accounts.md sections 2-5 and 8): who can
// sign in, each account's role, an admin's federations and restrictions,
// a member's grants per federation and abilities, and its audit trail.
import { useState } from 'react';
import { Ban, Crown, History, Info, KeyRound, LogOut, Mail, MailCheck, MailWarning, Network, Pencil, Plus, Send, ShieldCheck, TriangleAlert, User as UserIcon } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { useNav } from '../nav.tsx';
import { Avatar, Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { Field, MultiPicker, Seg, Switch, type PickItem } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, mutate } from '../api/mutate.ts';
import { ago } from '../format.ts';
import type { PageData, Role } from '../page.ts';
import type { GroupSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';

export interface Fed { id: number; name: string; default: boolean }

export interface AccountRow {
	id: number;
	username: string;
	name: string;
	email: string | null;
	email_verified: boolean;
	invited: boolean;
	role: Role;
	active: boolean;
	last_signin: string | null;
	last_superadmin: boolean;
	admin_of: number[];
	restrict: { hide_private: boolean; hide_groups: number[]; join_chat: boolean } | null;
	grants: { fed: number; group_ids: number[] }[];
	abilities: string[];
	audit: { what: string; by: string; at: string }[];
}

export interface AccountsProps {
	accounts: AccountRow[];
	// Federations the viewer may manage, with the groups it can see in each.
	federations: (Fed & { groups: GroupSummary[] })[];
	all_federations: Fed[];
	// Groups a superadmin may hide from an admin.
	group_options: GroupSummary[];
}

const ROLE_ICON = { superadmin: Crown, admin: ShieldCheck, member: UserIcon } as const;
const ROLES: [Role, string, string][] = [
	['superadmin', 'Superadmin', 'Everything: all accounts, roles, federations and settings.'],
	['admin', 'Admin', 'Everything except what a superadmin restricts. Manages members of its federations only.'],
	['member', 'Member', 'Only the groups granted in its federations, and the abilities given.'],
];
const ABILITIES: [string, string, string][] = [
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
	const n = a.grants.reduce((x, g) => x + g.group_ids.length, 0);
	return [`${n} group${n === 1 ? '' : 's'}`, ...a.abilities.map((k) => (k === 'forward' ? 'forwarding' : 'fetch files'))].join(', ');
}

const asPick = (g: GroupSummary): PickItem => ({ id: g.id, title: g.title, kind: 'group', photo: g.photo });

function EmailLine({ a }: { a: AccountRow }) {
	if (!a.email) return <span className="a-emailline a-warn"><MailWarning size={14} />No address yet; asked for one at the next sign-in</span>;
	return (
		<span className="a-emailline">
			{a.email_verified ? <MailCheck size={14} className="a-ok" /> : <MailWarning size={14} className="a-warnicon" />}
			<span className="a-break">{a.email}</span>
			<span className="a-muted sm">{a.email_verified ? 'verified' : a.invited ? 'invitation sent, password not set yet' : 'not verified'}</span>
		</span>
	);
}

type Draft = AccountRow & { isNew?: boolean };

function Editor({ init, props, viewerRole, onClose, onSaved }: { init: Draft; props: AccountsProps; viewerRole: Role; onClose: () => void; onSaved: () => void }) {
	const isNew = !!init.isNew;
	const isSuper = viewerRole === 'superadmin';
	const [a, setA] = useState<Draft>(init);
	const [tab, setTab] = useState<'account' | 'access' | 'activity'>('account');
	const [editEmail, setEditEmail] = useState(false);
	const [err, setErr] = useState('');
	const [note, setNote] = useState('');
	const set = (patch: Partial<Draft>) => setA({ ...a, ...patch });
	const roles = isSuper ? ROLES : ROLES.filter((r) => r[0] === 'member');
	const grantOf = (fid: number) => a.grants.find((g) => g.fed === fid);
	const call = async (fn: () => Promise<unknown>, after: 'close' | string) => {
		setErr('');
		try {
			await fn();
			if (after === 'close') { onSaved(); onClose(); } else { setNote(after); onSaved(); }
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const save = () => call(() => (isNew
		? mutate('POST', '/api/accounts', { username: a.username, name: a.name, email: a.email, role: a.role })
		: mutate('PATCH', `/api/accounts/${a.id}`, {
			name: a.name, role: a.role,
			...(a.email !== init.email ? { email: a.email } : {}),
			...(a.role === 'admin' ? { admin_of: a.admin_of, restrict: a.restrict } : {}),
			...(a.role === 'member' ? { grants: a.grants, abilities: a.abilities } : {}),
		})), 'close');
	const restrict = a.restrict ?? { hide_private: false, hide_groups: [], join_chat: false };
	return (
		<Modal wide title={isNew ? 'New account' : init.name} icon={!isNew ? <Avatar id={init.id} name={init.name} size={24} /> : undefined} onClose={onClose}
			footer={<>
				{!isNew && !init.last_superadmin && <Btn ghost danger icon={Ban} onClick={() => void call(() => mutate('POST', `/api/accounts/${a.id}/${a.active ? 'disable' : 'enable'}`), 'close')}>{a.active ? 'Disable account' : 'Enable account'}</Btn>}
				{init.last_superadmin && <span className="a-muted sm a-note"><Info size={13} />The last active superadmin cannot be disabled or demoted.</span>}
				<span className="a-grow" />
				<Btn ghost onClick={onClose}>Cancel</Btn>
				<Btn primary onClick={() => void save()}>{isNew ? 'Create and email link' : 'Save changes'}</Btn>
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
							<Field label="Email address" hint={isNew ? 'Must not be used by another account.' : 'Not signed in yet, so the address can still be corrected; saving sends a new set-password link and the old link stops working.'}>
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
						{isNew ? <div className="a-banner"><Mail size={15} /><span>The account gets an email with a one-time link to choose its own password. No password is ever shown here or sent by email.</span></div> : (
							<div className="a-btnrow">
								{a.invited ? <Btn ghost icon={Send} onClick={() => void call(() => mutate('POST', `/api/accounts/${a.id}/resend-invite`), 'A new set-password link is on its way.')}>Resend set-password link</Btn>
									: <Btn ghost icon={KeyRound} disabled={!a.email_verified} onClick={() => void call(() => mutate('POST', `/api/accounts/${a.id}/password-reset`), 'A password reset link is on its way.')}>Send password reset link</Btn>}
								<Btn ghost icon={LogOut} onClick={() => void call(() => mutate('POST', `/api/accounts/${a.id}/sign-out-everywhere`), 'Every session of this account has been signed out.')}>Sign out everywhere</Btn>
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
						{isNew && a.role !== 'superadmin' && <p className="a-muted sm">Federations, restrictions and grants are set on the Access tab once the account exists.</p>}
					</section>
				</div>
			)}

			{!isNew && tab === 'access' && a.role === 'superadmin' && <p className="a-muted">Superadmins see and manage everything; nothing here applies.</p>}

			{!isNew && tab === 'access' && a.role === 'admin' && (
				<div className="a-acctgrid">
					<section className="a-form">
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
					</section>
					<section className="a-form">
						<h3 className="a-sec">Restrictions</h3>
						<ul className="a-perms">
							<li><label><input type="checkbox" checked={restrict.hide_private} onChange={() => set({ restrict: { ...restrict, hide_private: !restrict.hide_private } })} /><span><b>Hide private messages</b><span className="a-muted sm">No private-message menu, pages or API; their URLs answer 404.</span></span></label></li>
							<li><div className="a-restrict"><span><b>Hide groups</b><span className="a-muted sm">The groups, their messages, members, files, forwarding and MCP exposure become invisible.</span></span>
								<MultiPicker value={props.group_options.filter((g) => restrict.hide_groups.includes(g.id)).map(asPick)} onChange={(gs) => set({ restrict: { ...restrict, hide_groups: gs.map((g) => g.id) } })} options={props.group_options.map(asPick)} placeholder="Add a group to hide..." /></div></li>
							<li><label><input type="checkbox" checked={restrict.join_chat} onChange={() => set({ restrict: { ...restrict, join_chat: !restrict.join_chat } })} /><span><b>Allow Join chat</b><span className="a-muted sm">Make the logging account join groups, and add them to its federations.</span></span></label></li>
						</ul>
					</section>
				</div>
			)}

			{!isNew && tab === 'access' && a.role === 'member' && (
				<div className="a-form">
					<h3 className="a-sec">Groups, per federation</h3>
					<p className="a-muted sm">A member sees only the groups granted here, and people only as they appear in those groups. Members never see private messages.</p>
					{props.federations.map((f) => {
						const g = grantOf(f.id);
						return (
							<section key={f.id} className={`a-fedgrant ${g ? 'on' : ''}`}>
								<header>
									<Switch checked={!!g} onChange={() => set({ grants: g ? a.grants.filter((x) => x.fed !== f.id) : [...a.grants, { fed: f.id, group_ids: [] }] })} label={<><Network size={14} /> {f.name}</>} />
									{g && <span className="a-muted sm">{g.group_ids.length} of {f.groups.length} groups</span>}
								</header>
								{g && <div className="a-checkgrid">{f.groups.map((grp) => {
									const on = g.group_ids.includes(grp.id);
									return (
										<label key={grp.id} className={`a-checkchip ${on ? 'on' : ''}`}>
											<input type="checkbox" checked={on} onChange={() => set({ grants: a.grants.map((x) => (x.fed !== f.id ? x : { ...x, group_ids: on ? x.group_ids.filter((y) => y !== grp.id) : [...x.group_ids, grp.id] })) })} />
											<Avatar id={grp.id} name={grp.title} photo={grp.photo} size={16} square />{grp.title}
										</label>
									);
								})}</div>}
							</section>
						);
					})}
					<h3 className="a-sec">Abilities</h3>
					<ul className="a-perms">{ABILITIES.map(([k, t, d]) => (
						<li key={k}><label><input type="checkbox" checked={a.abilities.includes(k)} onChange={() => set({ abilities: a.abilities.includes(k) ? a.abilities.filter((x) => x !== k) : [...a.abilities, k] })} /><span><b>{t}</b><span className="a-muted sm">{d}</span></span></label></li>
					))}</ul>
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
	const last = (a: AccountRow) => (a.invited ? <Pill tone="info"><Mail size={11} />invited</Pill> : !a.email ? <Pill tone="danger"><MailWarning size={11} />no email</Pill> : a.last_signin ? ago(a.last_signin, v) : <span className="a-muted">never</span>);
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
