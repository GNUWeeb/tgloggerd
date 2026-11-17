// Administration > Accounts: roles, admin restrictions, member grants
// per federation, and each account's audit trail (web-accounts.md).
import { useState } from 'react';
import { Plus, Pencil, KeyRound, LogOut, Ban, Crown, ShieldCheck, User as UserIcon, Network, History, Info, Mail, MailCheck, MailWarning, Send, X } from 'lucide-react';
import Shell from '../ui/Shell.jsx';
import { Pill, PageHead, Btn, Seg, Avatar } from '../ui/primitives.jsx';
import { Modal, Field, Switch, MultiPicker } from '../ui/forms.jsx';
import { useViewer } from '../viewer.jsx';
import { accounts, ROLES, RESTRICTIONS, ABILITIES, allFeds, fedById, fedGroups, activeSuperadmins, auditFor } from '../data/accounts.js';
import { groups } from '../data/groups.js';
import { ago } from '../data/seed.js';

const ROLE_ICON = { superadmin: Crown, admin: ShieldCheck, member: UserIcon };

export function Letter({ a, size = 32 }) {
	return <span className="a-letter" style={{ width: size, height: size, background: `hsl(${a.hue} 55% 52%)`, fontSize: size * 0.42 }}>{a.username[0].toUpperCase()}</span>;
}

function RolePill({ role }) {
	const I = ROLE_ICON[role];
	return <Pill tone={role === 'superadmin' ? 'accent' : role === 'admin' ? 'info' : ''}><I size={11} />{role}</Pill>;
}

function feds(a) {
	if (a.role === 'admin') return a.adminOf.map(fedById);
	if (a.role === 'member') return a.grants.map((g) => fedById(g.fed));
	return [];
}

function accessSummary(a) {
	if (a.role === 'superadmin') return 'Everything';
	if (a.role === 'admin') {
		const r = [];
		if (a.restrict.hide_private) r.push('no private messages');
		if (a.restrict.hide_groups.length) r.push(`${a.restrict.hide_groups.length} hidden group${a.restrict.hide_groups.length > 1 ? 's' : ''}`);
		if (a.restrict.join_chat) r.push('can join chats');
		return r.length ? r.join(', ') : 'No restrictions';
	}
	const n = a.grants.reduce((x, g) => x + g.groupIds.length, 0);
	return [`${n} group${n === 1 ? '' : 's'}`, ...a.abilities.map((k) => (k === 'forward' ? 'forwarding' : 'fetch files'))].join(', ');
}

export function EmailLine({ a }) {
	if (!a.email) return <span className="a-emailline a-warn"><MailWarning size={14} />No address yet; asked for one at the next sign-in</span>;
	return (
		<span className="a-emailline">
			{a.verified ? <MailCheck size={14} className="a-ok" /> : <MailWarning size={14} className="a-warnicon" />}
			<span className="a-break">{a.email}</span>
			<span className="a-muted sm">{a.verified ? 'verified' : a.invited ? 'invitation sent, password not set yet' : 'not verified'}</span>
		</span>
	);
}

function Editor({ init, viewer, onClose }) {
	const isNew = !init.id;
	const [a, setA] = useState(init);
	const [tab, setTab] = useState('account');
	const set = (patch) => setA({ ...a, ...patch });
	const [editEmail, setEditEmail] = useState(false);
	const isSuper = viewer.role === 'superadmin';
	const lastSuper = !isNew && init.role === 'superadmin' && init.active && activeSuperadmins().length === 1;
	// An admin can never grant what it cannot see, and manages only its federations.
	const manageable = isSuper ? allFeds() : viewer.manageableFeds;
	const roleChoices = isSuper ? ROLES : ROLES.filter((r) => r[0] === 'member');
	const changeRole = (role) => set({
		role,
		adminOf: a.adminOf || [], restrict: a.restrict || { hide_private: false, hide_groups: [], join_chat: false },
		grants: a.grants || [], abilities: a.abilities || [],
	});
	const grantOf = (fid) => (a.grants || []).find((g) => g.fed === fid);
	const toggleFedMember = (fid) => set({ grants: grantOf(fid) ? a.grants.filter((g) => g.fed !== fid) : [...(a.grants || []), { fed: fid, groupIds: [] }] });
	const toggleGrant = (fid, gid) => set({ grants: a.grants.map((g) => (g.fed !== fid ? g : { ...g, groupIds: g.groupIds.includes(gid) ? g.groupIds.filter((x) => x !== gid) : [...g.groupIds, gid] })) });
	const toggleAbility = (k) => set({ abilities: a.abilities.includes(k) ? a.abilities.filter((x) => x !== k) : [...a.abilities, k] });

	return (
		<Modal wide title={isNew ? 'New account' : init.name} icon={!isNew && <Letter a={init} size={24} />} onClose={onClose}
			footer={<>
				{!isNew && !lastSuper && <Btn ghost danger icon={Ban}>{a.active ? 'Disable account' : 'Enable account'}</Btn>}
				{lastSuper && <span className="a-muted sm a-note"><Info size={13} />The last active superadmin cannot be disabled or demoted.</span>}
				<span className="a-grow" />
				<Btn ghost onClick={onClose}>Cancel</Btn>
				<Btn primary onClick={onClose}>{isNew ? 'Create and email link' : 'Save changes'}</Btn>
			</>}>
			{!isNew && <Seg value={tab} onChange={setTab} items={[['account', 'Account'], ['access', 'Access'], ['activity', 'Activity']]} />}

			{(isNew || tab === 'account') && (
				<div className="a-acctgrid">
					<section className="a-form">
						<h3 className="a-sec">Sign-in</h3>
						<Field label="Username" hint={isNew ? 'Used to sign in. Letters, digits, underscores.' : 'Usernames do not change.'}>
							<input value={a.username} onChange={(e) => set({ username: e.target.value })} autoComplete="off" disabled={!isNew} />
						</Field>
						<Field label="Display name"><input value={a.name} onChange={(e) => set({ name: e.target.value })} /></Field>
						{isNew ? (
							<>
								<Field label="Email address" hint="Must not be used by another account.">
									<input type="email" value={a.email} onChange={(e) => set({ email: e.target.value })} placeholder="name@example.org" autoComplete="off" />
								</Field>
								<div className="a-banner"><Mail size={15} /><span>The account gets an email with a one-time link to choose its own password. The link expires; no password is ever shown here or sent by email.</span></div>
							</>
						) : (
							<>
								{init.invited ? (
									// Still invited (never signed in): fix a typo made at creation.
									<Field label="Email address" hint="The account has not signed in yet, so its address can still be corrected. Saving sends a new set-password link there; the old link stops working.">
										<input type="email" value={a.email} onChange={(e) => set({ email: e.target.value })} autoComplete="off" />
									</Field>
								) : editEmail ? (
									<Field label="New email address" hint="Superadmins only, for someone who lost their mailbox. The new address is verified by link, the old address is told, and the change is audited.">
										<span className="a-inputwrap">
											<input type="email" value={a.email} onChange={(e) => set({ email: e.target.value })} autoComplete="off" autoFocus />
											<button type="button" className="a-icon-btn sm" aria-label="Cancel" onClick={() => { set({ email: init.email }); setEditEmail(false); }}><X size={14} /></button>
										</span>
									</Field>
								) : (
									<div className="a-field">
										<span className="a-field-label">Email address</span>
										<span className="a-row"><EmailLine a={init} />{isSuper && <Btn ghost onClick={() => setEditEmail(true)}>Change address</Btn>}</span>
										<span className="a-field-hint">Accounts change their own address in Settings; the old address is told.</span>
									</div>
								)}
								<div className="a-btnrow">
									{init.invited
										? <Btn ghost icon={Send}>Resend set-password link</Btn>
										: <Btn ghost icon={KeyRound} disabled={!init.verified}>Send password reset link</Btn>}
									<Btn ghost icon={LogOut}>Sign out everywhere</Btn>
								</div>
							</>
						)}
					</section>
					<section className="a-form">
						<h3 className="a-sec">Role</h3>
						<div className="a-roles">
							{roleChoices.map(([k, t, d]) => {
								const I = ROLE_ICON[k];
								const locked = lastSuper && k !== 'superadmin';
								return (
									<label key={k} className={`a-role ${a.role === k ? 'on' : ''} ${locked ? 'is-off' : ''}`}>
										<input type="radio" name="role" checked={a.role === k} disabled={locked} onChange={() => changeRole(k)} />
										<I size={16} /><span><b>{t}</b><span className="a-muted sm">{d}</span></span>
									</label>
								);
							})}
						</div>
						{isNew && a.role !== 'superadmin' && <p className="a-muted sm">Federations, restrictions and grants are set on the Access tab once the account exists.</p>}
					</section>
				</div>
			)}

			{!isNew && tab === 'access' && a.role === 'superadmin' && (
				<p className="a-muted">Superadmins see and manage everything; nothing here applies.</p>
			)}

			{!isNew && tab === 'access' && a.role === 'admin' && (
				<div className="a-acctgrid">
					<section className="a-form">
						<h3 className="a-sec">Administers</h3>
						<p className="a-muted sm">Federations limit what this admin manages (members, their grants, forwarding), not what it sees.</p>
						<div className="a-checkgrid">
							{allFeds().map((f) => {
								const on = a.adminOf.includes(f.id);
								return (
									<label key={f.id} className={`a-checkchip ${on ? 'on' : ''}`}>
										<input type="checkbox" checked={on} onChange={() => set({ adminOf: on ? a.adminOf.filter((x) => x !== f.id) : [...a.adminOf, f.id] })} />
										<Network size={13} />{f.name}
									</label>
								);
							})}
						</div>
					</section>
					<section className="a-form">
						<h3 className="a-sec">Restrictions</h3>
						<ul className="a-perms">
							{RESTRICTIONS.map(([k, t, d]) => (
								<li key={k}>
									{k === 'hide_groups' ? (
										<div className="a-restrict">
											<span><b>{t}</b><span className="a-muted sm">{d}</span></span>
											<MultiPicker value={a.restrict.hide_groups.map((id) => groups.find((g) => g.id === id))} onChange={(gs) => set({ restrict: { ...a.restrict, hide_groups: gs.map((g) => g.id) } })} options={groups.slice(0, 60)} placeholder="Add a group to hide..." />
										</div>
									) : (
										<label>
											<input type="checkbox" checked={!!a.restrict[k]} onChange={() => set({ restrict: { ...a.restrict, [k]: !a.restrict[k] } })} />
											<span><b>{t}</b><span className="a-muted sm">{d}</span></span>
										</label>
									)}
								</li>
							))}
						</ul>
					</section>
				</div>
			)}

			{!isNew && tab === 'access' && a.role === 'member' && (
				<div className="a-form">
					<h3 className="a-sec">Groups, per federation</h3>
					<p className="a-muted sm">A member sees only the groups granted here, and people only as they appear in those groups. Private messages are never visible to members.</p>
					{manageable.map((f) => {
						const g = grantOf(f.id);
						const list = fedGroups(f).filter(viewer.group).slice(0, 10);
						return (
							<section key={f.id} className={`a-fedgrant ${g ? 'on' : ''}`}>
								<header>
									<Switch checked={!!g} onChange={() => toggleFedMember(f.id)} label={<><Network size={14} /> {f.name}</>} />
									{g && <span className="a-muted sm">{g.groupIds.length} of {list.length} groups</span>}
								</header>
								{g && (
									<div className="a-checkgrid">
										{list.map((grp) => {
											const on = g.groupIds.includes(grp.id);
											return (
												<label key={grp.id} className={`a-checkchip ${on ? 'on' : ''}`}>
													<input type="checkbox" checked={on} onChange={() => toggleGrant(f.id, grp.id)} />
													<Avatar e={grp} size={16} square />{grp.title}
												</label>
											);
										})}
									</div>
								)}
							</section>
						);
					})}
					<h3 className="a-sec">Abilities</h3>
					<ul className="a-perms">
						{ABILITIES.map(([k, t, d]) => (
							<li key={k}>
								<label>
									<input type="checkbox" checked={a.abilities.includes(k)} onChange={() => toggleAbility(k)} />
									<span><b>{t}</b><span className="a-muted sm">{d}</span></span>
								</label>
							</li>
						))}
					</ul>
				</div>
			)}

			{!isNew && tab === 'activity' && (
				<ol className="a-audit">
					{auditFor(init).map((r, i) => (
						<li key={i}><History size={14} /><span className="a-grow">{r.what}<span className="a-muted"> by {r.by}</span></span><time className="a-muted sm" title={r.at}>{ago(r.at)}</time></li>
					))}
				</ol>
			)}
		</Modal>
	);
}

export default function Accounts() {
	const v = useViewer();
	const [edit, setEdit] = useState(null);
	const [role, setRole] = useState('all');
	// An admin sees and manages only the members of its federations.
	const mine = v.role === 'superadmin' ? accounts
		: accounts.filter((a) => a.role === 'member' && a.grants.some((g) => v.manageableFeds.some((f) => f.id === g.fed)));
	const rows = role === 'all' ? mine : mine.filter((a) => a.role === role);
	const blank = { username: '', name: '', email: '', role: 'member', active: true, hue: 210, grants: [], abilities: [] };
	return (
		<Shell crumbs={[['Accounts', null]]}>
			<PageHead title="Accounts" sub={v.role === 'superadmin' ? 'Who can sign in, their role, and what each one may see and manage.' : 'Members of the federations you administer.'}
				actions={<Btn primary icon={Plus} onClick={() => setEdit(blank)}>{v.role === 'superadmin' ? 'New account' : 'New member'}</Btn>} />
			{v.role === 'superadmin' && (
				<div className="a-toolbar">
					<Seg value={role} onChange={setRole} items={[['all', 'All'], ['superadmin', 'Superadmins'], ['admin', 'Admins'], ['member', 'Members']]} />
				</div>
			)}
			<div className="a-card a-tablecard">
				<div className="a-tablescroll">
					<table className="a-table">
						<thead><tr><th>Account</th><th>Role</th><th>Federations</th><th>Access</th><th>Last sign-in</th><th /></tr></thead>
						<tbody>
							{rows.map((a) => (
								<tr key={a.id} className={a.active ? '' : 'a-off'}>
									<td>
										<button type="button" className="a-usercell a-plain" onClick={() => setEdit(a)}>
											<Letter a={a} />
											<span><span className="a-uname">{a.name}</span><span className="a-handle">{a.username}{a.email ? ` . ${a.email}` : ''}{!a.active && ' . disabled'}</span></span>
										</button>
									</td>
									<td><RolePill role={a.role} /></td>
									<td className="a-wrapcell">{a.role === 'superadmin' ? <span className="a-muted">All</span> : feds(a).map((f) => <Pill key={f.id}><Network size={11} />{f.name}</Pill>)}</td>
									<td className="a-muted">{accessSummary(a)}</td>
									<td title={a.last}>{a.invited ? <Pill tone="info"><Mail size={11} />invited</Pill> : !a.email ? <Pill tone="danger"><MailWarning size={11} />no email</Pill> : a.last ? ago(a.last) : <span className="a-muted">never</span>}</td>
									<td className="a-rowactions"><button type="button" className="a-icon-btn" aria-label={`Edit ${a.username}`} onClick={() => setEdit(a)}><Pencil size={15} /></button></td>
								</tr>
							))}
						</tbody>
					</table>
				</div>
				<ul className="a-list">
					{rows.map((a) => (
						<li key={a.id}>
							<button type="button" className="a-list-row a-plain" onClick={() => setEdit(a)}>
								<Letter a={a} size={36} />
								<span className="a-list-main"><span className="a-uname">{a.name}</span><span className="a-handle">{a.role} . {accessSummary(a)}</span></span>
								<span className="a-list-side"><span className="a-muted sm">{a.active ? (a.last ? ago(a.last) : 'never') : 'disabled'}</span></span>
							</button>
						</li>
					))}
				</ul>
			</div>
			<p className="a-muted a-foot">Accounts are disabled, never deleted, so the audit trail and ownership keep pointing at them. Every change is recorded on the account's Activity tab.</p>
			{edit && <Editor init={edit} viewer={v} onClose={() => setEdit(null)} />}
		</Shell>
	);
}
