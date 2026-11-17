// Settings > Accounts: who can sign in, what they may do, what they see.
import { useState } from 'react';
import { Plus, Pencil, KeyRound, LogOut, Ban, ShieldCheck, Shuffle, Crown, CircleCheck } from 'lucide-react';
import Shell from '../ui/Shell.jsx';
import { Pill, PageHead, Btn } from '../ui/primitives.jsx';
import { Modal, Field, Switch, MultiPicker, ScopeField } from '../ui/forms.jsx';
import { SettingsNav } from './Account.jsx';
import { accounts, PERMISSIONS, PAGES, scopeSummary } from '../data/accounts.js';
import { groups } from '../data/groups.js';
import { users } from '../data/users.js';
import { ago } from '../data/seed.js';

const PERM_SHORT = { view_files: 'files', fetch_files: 'fetch', view_private: 'private', manage_integrations: 'integrations', manage_accounts: 'accounts' };

function Letter({ a, size = 32 }) {
	return <span className="a-letter" style={{ width: size, height: size, background: `hsl(${a.hue} 55% 52%)`, fontSize: size * 0.42 }}>{a.username[0].toUpperCase()}</span>;
}

const BLANK = { username: '', name: '', role: 'member', perms: ['view_files'], pages: null, groups: null, users: null, active: true, mustChange: true };

function Editor({ init, onClose }) {
	const [a, setA] = useState(init);
	const [pw, setPw] = useState('');
	const isNew = !init.id;
	const admin = a.role === 'admin';
	const set = (patch) => setA({ ...a, ...patch });
	const togglePerm = (k) => set({ perms: a.perms.includes(k) ? a.perms.filter((x) => x !== k) : [...a.perms, k] });
	const togglePage = (k) => {
		const cur = a.pages || PAGES.map((p) => p[0]);
		set({ pages: cur.includes(k) ? cur.filter((x) => x !== k) : [...cur, k] });
	};
	return (
		<Modal wide title={isNew ? 'New account' : `Edit ${init.username}`} onClose={onClose}
			footer={<>
				{!isNew && <Btn ghost danger icon={Ban}>{a.active ? 'Disable' : 'Enable'}</Btn>}
				<span className="a-grow" />
				<Btn ghost onClick={onClose}>Cancel</Btn>
				<Btn primary onClick={onClose}>{isNew ? 'Create account' : 'Save changes'}</Btn>
			</>}>
			<div className="a-acctgrid">
				<section className="a-form">
					<h3 className="a-sec">Sign-in</h3>
					<Field label="Username" hint="Used to sign in. Letters, digits, underscores.">
						<input value={a.username} onChange={(e) => set({ username: e.target.value })} autoComplete="off" disabled={!isNew} />
					</Field>
					<Field label="Display name">
						<input value={a.name} onChange={(e) => set({ name: e.target.value })} />
					</Field>
					{isNew ? (
						<Field label="Temporary password" hint="Hand it over privately. It is not shown again after the account is created.">
							<span className="a-inputwrap">
								<input value={pw} onChange={(e) => setPw(e.target.value)} type="text" autoComplete="off" className="mono" />
								<button type="button" className="a-icon-btn sm" title="Generate" aria-label="Generate a password" onClick={() => setPw('tgd-' + Math.random().toString(36).slice(2, 8) + '-' + Math.random().toString(36).slice(2, 8))}><Shuffle size={14} /></button>
							</span>
						</Field>
					) : (
						<div className="a-btnrow">
							<Btn ghost icon={KeyRound}>Reset password</Btn>
							<Btn ghost icon={LogOut}>Sign out everywhere</Btn>
						</div>
					)}
					{isNew && <Switch checked={a.mustChange} onChange={(v) => set({ mustChange: v })} label="Must choose a new password at first sign-in" />}

					<h3 className="a-sec">Role</h3>
					<div className="a-roles">
						{[['admin', 'Administrator', 'Everything, on every page. Permissions and access below do not apply.', Crown],
							['member', 'Member', 'Only the permissions and the access chosen below.', ShieldCheck]].map(([k, t, d, I]) => (
							<label key={k} className={`a-role ${a.role === k ? 'on' : ''}`}>
								<input type="radio" name="role" checked={a.role === k} onChange={() => set({ role: k })} />
								<I size={16} /><span><b>{t}</b><span className="a-muted sm">{d}</span></span>
							</label>
						))}
					</div>

					<h3 className="a-sec">Permissions</h3>
					<ul className={`a-perms ${admin ? 'is-off' : ''}`}>
						{PERMISSIONS.map(([k, t, d]) => (
							<li key={k}>
								<label>
									<input type="checkbox" checked={admin || a.perms.includes(k)} disabled={admin} onChange={() => togglePerm(k)} />
									<span><b>{t}</b><span className="a-muted sm">{d}</span></span>
								</label>
							</li>
						))}
					</ul>
				</section>

				<section className={`a-form ${admin ? 'is-off' : ''}`}>
					<h3 className="a-sec">Access</h3>
					{admin && <p className="a-muted sm">Administrators see everything.</p>}
					<ScopeField label="Pages" all={admin || a.pages === null} onAll={(v) => set({ pages: v ? null : PAGES.map((p) => p[0]) })} allText="All pages">
						<div className="a-checkgrid">
							{PAGES.map(([k, t]) => {
								const on = (a.pages || []).includes(k);
								return (
									<label key={k} className={`a-checkchip ${on ? 'on' : ''}`}>
										<input type="checkbox" checked={on} onChange={() => togglePage(k)} />{t}
									</label>
								);
							})}
						</div>
						<span className="a-field-hint">Pages left out are hidden from the menu and refused if opened by URL.</span>
					</ScopeField>
					<ScopeField label="Groups" all={admin || a.groups === null} onAll={(v) => set({ groups: v ? null : [] })} allText="All groups">
						<MultiPicker value={a.groups || []} onChange={(g) => set({ groups: g })} options={groups.slice(0, 60)} placeholder="Add a group..." />
						<span className="a-field-hint">Messages, members and files of other groups are not shown anywhere, search included.</span>
					</ScopeField>
					<ScopeField label="Users" all={admin || a.users === null} onAll={(v) => set({ users: v ? null : [] })} allText="All users">
						<MultiPicker value={a.users || []} onChange={(u) => set({ users: u })} options={users.filter((u) => u.type !== 'deleted').slice(0, 80)} placeholder="Add a user..." />
						<span className="a-field-hint">Limits user profiles, their history and private chats to these people.</span>
					</ScopeField>
				</section>
			</div>
		</Modal>
	);
}

export default function Accounts() {
	const [edit, setEdit] = useState(null);
	return (
		<Shell crumbs={[['Settings', '/settings'], ['Accounts', null]]}>
			<PageHead title="Accounts" sub="Who can sign in, what each account may do, and which parts of the archive it can see."
				actions={<Btn primary icon={Plus} onClick={() => setEdit(BLANK)}>New account</Btn>} />
			<div className="a-settings">
				<SettingsNav on="accounts" />
				<div className="a-card a-tablecard">
					<div className="a-tablescroll">
						<table className="a-table">
							<thead><tr><th>Account</th><th>Role</th><th>Permissions</th><th>Access</th><th>Last sign-in</th><th /></tr></thead>
							<tbody>
								{accounts.map((a) => (
									<tr key={a.id} className={a.active ? '' : 'a-off'}>
										<td>
											<button type="button" className="a-usercell a-plain" onClick={() => setEdit(a)}>
												<Letter a={a} />
												<span><span className="a-uname">{a.name}</span><span className="a-handle">{a.username}{!a.active && ' . disabled'}</span></span>
											</button>
										</td>
										<td>{a.role === 'admin' ? <Pill tone="accent"><Crown size={11} />admin</Pill> : <Pill>member</Pill>}</td>
										<td className="a-wrapcell">
											{a.role === 'admin' ? <span className="a-muted">All</span>
												: a.perms.length ? a.perms.map((p) => <Pill key={p}>{PERM_SHORT[p]}</Pill>) : <span className="a-muted">Read only</span>}
										</td>
										<td className="a-muted">{scopeSummary(a)}</td>
										<td title={a.last}>{a.last ? ago(a.last) : <span className="a-muted">never</span>}</td>
										<td className="a-rowactions"><button type="button" className="a-icon-btn" aria-label={`Edit ${a.username}`} onClick={() => setEdit(a)}><Pencil size={15} /></button></td>
									</tr>
								))}
							</tbody>
						</table>
					</div>
					<ul className="a-list">
						{accounts.map((a) => (
							<li key={a.id}>
								<button type="button" className="a-list-row a-plain" onClick={() => setEdit(a)}>
									<Letter a={a} size={36} />
									<span className="a-list-main"><span className="a-uname">{a.name}{a.role === 'admin' && <Crown size={12} className="a-crown" />}</span><span className="a-handle">{a.username} . {scopeSummary(a)}</span></span>
									<span className="a-list-side"><span className="a-muted sm">{a.active ? (a.last ? ago(a.last) : 'never') : 'disabled'}</span></span>
								</button>
							</li>
						))}
					</ul>
				</div>
			</div>
			<p className="a-muted a-foot"><CircleCheck size={13} /> A restricted account sees only its pages in the menu, and only its groups and users in every list, search and chat. Finer-grained rules can come later.</p>
			{edit && <Editor init={edit} onClose={() => setEdit(null)} />}
		</Shell>
	);
}
