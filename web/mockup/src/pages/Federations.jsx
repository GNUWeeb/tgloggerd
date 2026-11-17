// Administration > Federations: named sets of groups that decide what
// each admin manages (web-accounts.md section 3).
import { useState } from 'react';
import { Plus, Network, Pencil, Info } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, PageHead, Btn } from '../ui/primitives.jsx';
import { Modal, Field, MultiPicker } from '../ui/forms.jsx';
import { useViewer } from '../viewer.jsx';
import { allFeds, fedGroups, fedAdmins, fedMembers, accounts } from '../data/accounts.js';
import { groups } from '../data/groups.js';
import { ago } from '../data/seed.js';
import { Letter } from './Accounts.jsx';

function Stack({ items, render, max = 3 }) {
	return (
		<span className="a-stackav">
			{items.slice(0, max).map(render)}
			{items.length > max && <span className="a-stackmore">+{items.length - max}</span>}
		</span>
	);
}

function Editor({ fed, onClose }) {
	const isNew = !fed.id && fed.id !== 0;
	const isDefault = fed.id === 0;
	const [name, setName] = useState(fed.name || '');
	const [gs, setGs] = useState(isNew ? [] : fedGroups(fed));
	const [admins, setAdmins] = useState(isNew ? [] : fedAdmins(fed));
	const adminOptions = accounts.filter((a) => a.role === 'admin').map((a) => ({ ...a, title: a.name, kind: 'account' }));
	return (
		<Modal wide title={isNew ? 'New federation' : fed.name} icon={<Network size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn primary onClick={onClose}>{isNew ? 'Create federation' : 'Save changes'}</Btn></>}>
			{isDefault ? (
				<div className="a-banner"><Info size={15} /><span>Default holds every group that is in no other federation, including every group joined from now on. Its groups follow from the other federations; only its admins are set here.</span></div>
			) : (
				<Field label="Name"><input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Security community" /></Field>
			)}
			{!isDefault && (
				<Field label="Groups" hint="A group may belong to several federations. Groups taken out of every federation return to Default.">
					<MultiPicker value={gs} onChange={setGs} options={groups.slice(0, 80)} placeholder="Add a group..." />
				</Field>
			)}
			<Field label="Admins" hint="They manage this federation's members, their grants and its forwarding. They see what their own restrictions allow, whatever the federation.">
				<MultiPicker value={admins.map((a) => ({ ...a, title: a.name, kind: 'account' }))} onChange={setAdmins} options={adminOptions} placeholder="Add an admin..." />
			</Field>
			{!isNew && (
				<div className="a-field">
					<span className="a-field-label">Members</span>
					<div className="a-checkgrid">
						{fedMembers(fed).map((m) => <Pill key={m.id}>{m.name}</Pill>)}
						{!fedMembers(fed).length && <span className="a-muted sm">None yet.</span>}
					</div>
					<span className="a-field-hint">Members and their grants are managed on <Link to="/settings/accounts" className="a-link">Accounts</Link>.</span>
				</div>
			)}
		</Modal>
	);
}

export default function Federations() {
	const v = useViewer();
	const [edit, setEdit] = useState(null);
	const isSuper = v.role === 'superadmin';
	const list = isSuper ? allFeds() : v.manageableFeds;
	return (
		<Shell crumbs={[['Federations', null]]}>
			<PageHead title="Federations"
				sub={isSuper ? 'Named sets of groups. They decide what each admin manages, not what it sees.' : 'The federations you administer.'}
				actions={isSuper && <Btn primary icon={Plus} onClick={() => setEdit({ name: '' })}>New federation</Btn>} />
			<div className="a-fedgrid">
				{list.map((f) => {
					const gs = fedGroups(f).filter(v.group);
					const admins = fedAdmins(f);
					const members = fedMembers(f);
					return (
						<section key={f.id} className="a-card a-fedcard">
							<header>
								<span className="a-fedicon"><Network size={16} /></span>
								<div className="a-grow">
									<h2>{f.name}</h2>
									<span className="a-muted sm">{f.id === 0 ? 'Every group in no other federation' : `created ${ago(f.created)}`}</span>
								</div>
								{isSuper && <button type="button" className="a-icon-btn" aria-label={`Edit ${f.name}`} onClick={() => setEdit(f)}><Pencil size={15} /></button>}
							</header>
							<dl className="a-fedstats">
								<div><dt>Groups</dt><dd><b className="num">{gs.length}</b><Stack items={gs} render={(g) => <Avatar key={g.id} e={g} size={22} square />} /></dd></div>
								<div><dt>Admins</dt><dd><b className="num">{admins.length}</b><Stack items={admins} render={(a) => <Letter key={a.id} a={a} size={22} />} /></dd></div>
								<div><dt>Members</dt><dd><b className="num">{members.length}</b><Stack items={members} render={(a) => <Letter key={a.id} a={a} size={22} />} /></dd></div>
							</dl>
							<div className="a-fedgroups">
								{gs.slice(0, 6).map((g) => <Link key={g.id} to={`/groups/${g.id}`} className="a-chip a-chip-entity"><Avatar e={g} size={16} square />{g.title}</Link>)}
								{gs.length > 6 && <span className="a-muted sm">and {gs.length - 6} more</span>}
							</div>
						</section>
					);
				})}
			</div>
			<p className="a-muted a-foot">Only superadmins change which groups a federation holds. An admin allowed to join chats may add a group it joins to its own federations.</p>
			{edit && <Editor fed={edit} onClose={() => setEdit(null)} />}
		</Shell>
	);
}
