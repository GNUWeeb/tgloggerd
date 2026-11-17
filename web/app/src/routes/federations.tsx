// Administration > Federations (web-accounts.md section 3): named sets
// of groups that decide what each admin manages. Superadmins edit them;
// an admin sees the federations it administers, read only.
import { useState, type ReactNode } from 'react';
import { Info, Network, Pencil, Plus, TriangleAlert } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { Avatar, Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { Field, MultiPicker, type PickItem } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, mutate } from '../api/mutate.ts';
import { ago } from '../format.ts';
import type { PageData } from '../page.ts';
import type { GroupSummary } from '../types.ts';
import type { RouteDef } from './index.tsx';

interface Person { id: number; username: string; name: string }

export interface FederationRow {
	id: number;
	name: string;
	default: boolean;
	created: string | null;
	// Only the groups the viewer can see; `group_count` counts them all.
	groups: GroupSummary[];
	group_count: number;
	admins: Person[];
	members: Person[];
}

export interface FederationsProps {
	federations: FederationRow[];
	editable: boolean;
	group_options: GroupSummary[];
	admin_options: Person[];
}

const gPick = (g: GroupSummary): PickItem => ({ id: g.id, title: g.title, kind: 'group', photo: g.photo });
const pPick = (p: Person): PickItem => ({ id: p.id, title: p.name, kind: 'user' });

function Stack<T>({ items, render, max = 3 }: { items: T[]; render: (x: T) => ReactNode; max?: number }) {
	return <span className="a-stackav">{items.slice(0, max).map(render)}{items.length > max && <span className="a-stackmore">+{items.length - max}</span>}</span>;
}

function Editor({ fed, props, onClose, onSaved }: { fed: FederationRow | null; props: FederationsProps; onClose: () => void; onSaved: () => void }) {
	const isDefault = !!fed?.default;
	const [name, setName] = useState(fed?.name ?? '');
	const [groups, setGroups] = useState<PickItem[]>((fed?.groups ?? []).map(gPick));
	const [admins, setAdmins] = useState<PickItem[]>((fed?.admins ?? []).map(pPick));
	const [err, setErr] = useState('');
	const save = async () => {
		const body = { name, group_ids: isDefault ? undefined : groups.map((g) => g.id), admin_ids: admins.map((a) => a.id) };
		try {
			await (fed ? mutate('PATCH', `/api/federations/${fed.id}`, body) : mutate('POST', '/api/federations', body));
			onSaved();
			onClose();
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	return (
		<Modal wide title={fed ? fed.name : 'New federation'} icon={<Network size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn primary onClick={() => void save()}>{fed ? 'Save changes' : 'Create federation'}</Btn></>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			{isDefault
				? <div className="a-banner"><Info size={15} /><span>Default holds every group that is in no other federation, including every group joined from now on. Its groups follow from the other federations; only its admins are set here.</span></div>
				: <Field label="Name"><input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Security community" /></Field>}
			{!isDefault && (
				<Field label="Groups" hint="A group may belong to several federations. Groups taken out of every federation return to Default.">
					<MultiPicker value={groups} onChange={setGroups} options={props.group_options.map(gPick)} placeholder="Add a group..." />
				</Field>
			)}
			<Field label="Admins" hint="They manage this federation's members, their grants and its forwarding.">
				<MultiPicker value={admins} onChange={setAdmins} options={props.admin_options.map(pPick)} placeholder="Add an admin..." />
			</Field>
			{fed && (
				<div className="a-field">
					<span className="a-field-label">Members</span>
					<div className="a-checkgrid">{fed.members.length ? fed.members.map((m) => <Pill key={m.id}>{m.name}</Pill>) : <span className="a-muted sm">None yet.</span>}</div>
					<span className="a-field-hint">Members and their grants are managed on <Link to="/settings/accounts" className="a-link">Accounts</Link>.</span>
				</div>
			)}
		</Modal>
	);
}

function Federations({ page }: { page: PageData<FederationsProps> }) {
	const { navigate } = useNav();
	const v = page.viewer;
	const p = page.props;
	const [edit, setEdit] = useState<FederationRow | null | 'new'>(null);
	return (
		<Shell crumbs={[['Federations', null]]}>
			<PageHead title="Federations"
				sub={p.editable ? 'Named sets of groups. They decide what each admin manages, not what it sees.' : 'The federations you administer.'}
				actions={p.editable && <Btn primary icon={Plus} onClick={() => setEdit('new')}>New federation</Btn>} />
			<div className="a-fedgrid">{p.federations.map((f) => (
				<section key={f.id} className="a-card a-fedcard">
					<header>
						<span className="a-fedicon"><Network size={16} /></span>
						<div className="a-grow"><h2>{f.name}</h2><span className="a-muted sm">{f.default ? 'Every group in no other federation' : f.created ? `created ${ago(f.created, v)}` : ''}</span></div>
						{p.editable && <button type="button" className="a-icon-btn" aria-label={`Edit ${f.name}`} onClick={() => setEdit(f)}><Pencil size={15} /></button>}
					</header>
					<dl className="a-fedstats">
						<div><dt>Groups</dt><dd><b className="num">{f.group_count}</b><Stack items={f.groups} render={(g) => <Avatar key={g.id} id={g.id} name={g.title} photo={g.photo} size={22} square />} /></dd></div>
						<div><dt>Admins</dt><dd><b className="num">{f.admins.length}</b><Stack items={f.admins} render={(a) => <Avatar key={a.id} id={a.id} name={a.name} size={22} />} /></dd></div>
						<div><dt>Members</dt><dd><b className="num">{f.members.length}</b><Stack items={f.members} render={(a) => <Avatar key={a.id} id={a.id} name={a.name} size={22} />} /></dd></div>
					</dl>
					<div className="a-fedgroups">
						{f.groups.slice(0, 6).map((g) => <Link key={g.id} to={`/groups/${g.id}`} className="a-chip a-chip-entity"><Avatar id={g.id} name={g.title} photo={g.photo} size={16} square />{g.title}</Link>)}
						{f.group_count > 6 && <span className="a-muted sm">and {f.group_count - 6} more</span>}
					</div>
				</section>
			))}</div>
			<p className="a-muted a-foot">Only superadmins change which groups a federation holds. An admin allowed to join chats may add a group it joins to its own federations.</p>
			{edit && <Editor fed={edit === 'new' ? null : edit} props={p} onClose={() => setEdit(null)} onSaved={() => navigate(page.url, { replace: true, scroll: false })} />}
		</Shell>
	);
}

export const federationsRoute: RouteDef<FederationsProps> = { Page: Federations, head: () => ({ title: 'Federations - tgloggerd', robots: 'noindex' }) };
