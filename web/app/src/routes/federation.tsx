// Administration > Federations > one federation: its description, every
// group it holds (busiest first, searchable, a page at a time), its
// admins and its members. A superadmin sees every federation, an admin
// the ones it administers; either may rename it and edit its
// description where the page data allows (can_edit).
import { useEffect, useRef, useState } from 'react';
import { LoaderCircle, Network, Pencil, Search, Trash2, TriangleAlert, Users, UserCog } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, useNav } from '../nav.tsx';
import { Avatar, Btn, Card, Pill } from '../ui/primitives.tsx';
import { Field } from '../ui/forms.tsx';
import { Modal } from '../ui/Modal.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { federationGroups, updateFederation, type FederationDetailProps, type FedGroup, type Person } from '../api/federations.ts';
import { ago, compact, dateTime, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { RouteDef } from './index.tsx';

export type { FederationDetailProps };

const MAX_DESC = 2000;
const errText = (x: unknown) => (x instanceof ApiError ? x.message : 'The server could not be reached.');

function EditDetails({ props, onClose, onSaved }: { props: FederationDetailProps; onClose: () => void; onSaved: () => void }) {
	const f = props.federation;
	const ce = props.can_edit;
	const [name, setName] = useState(f.name);
	const [desc, setDesc] = useState(f.description);
	const [err, setErr] = useState('');
	const [busy, setBusy] = useState(false);
	const save = async () => {
		// Only what changed, and only what the viewer may change.
		const body = {
			...(ce.name && name.trim() !== f.name ? { name: name.trim() } : {}),
			...(ce.description && desc.trim() !== f.description ? { description: desc.trim() } : {}),
		};
		if (!Object.keys(body).length) { onClose(); return; }
		setBusy(true); setErr('');
		try { await updateFederation(f.id, body); onSaved(); onClose(); } catch (x) { setErr(errText(x)); }
		setBusy(false);
	};
	return (
		<Modal title={`Edit ${f.name}`} icon={<Pencil size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn primary disabled={busy || (ce.name && !name.trim()) || desc.length > MAX_DESC} onClick={() => void save()}>Save changes</Btn></>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<Field label="Name" hint={ce.name ? undefined : f.default ? 'The Default federation keeps its name.' : 'You cannot rename this federation.'}>
				<input value={name} onChange={(e) => setName(e.target.value)} disabled={!ce.name} maxLength={120} />
			</Field>
			{ce.description && (
				<Field label="Description" hint={`${desc.length > MAX_DESC ? 'Too long: ' : ''}${fmt(desc.length)} of ${fmt(MAX_DESC)} characters. Plain text; new lines are kept.`}>
					<textarea value={desc} onChange={(e) => setDesc(e.target.value)} rows={6} aria-invalid={desc.length > MAX_DESC} />
				</Field>
			)}
		</Modal>
	);
}

// Deleting a federation, confirmed first: what happens to its groups,
// admins and members, as the server does it.
function ConfirmDelete({ props, onClose }: { props: FederationDetailProps; onClose: () => void }) {
	const { navigate } = useNav();
	const f = props.federation;
	const [err, setErr] = useState('');
	const [busy, setBusy] = useState(false);
	const go = async () => {
		setBusy(true); setErr('');
		try {
			await ok(api.DELETE('/api/federations/{id}', { params: { path: { id: f.id } } }));
			onClose();
			navigate('/settings/federations');
		} catch (x) { setErr(errText(x)); setBusy(false); }
	};
	return (
		<Modal title={`Delete ${f.name}?`} icon={<Trash2 size={16} />} onClose={onClose}
			footer={<><span className="a-grow" /><Btn ghost onClick={onClose}>Cancel</Btn><Btn danger icon={Trash2} disabled={busy} onClick={() => void go()}>Delete federation</Btn></>}>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			<p>This cannot be undone. Deleting <b>{f.name}</b>:</p>
			<ul className="a-bullets">
				<li>returns its {fmt(f.group_count)} groups to Default, unless another federation holds them;</li>
				<li>ends what its {props.admins.length} admins administer through it;</li>
				<li>files its {props.members.length} members in Default if it was their only federation.</li>
			</ul>
		</Modal>
	);
}

// The federation's groups: the first page from the page data, more on
// request, and a search that asks the server again.
function Groups({ id, first }: { id: number; first: FederationDetailProps['groups'] }) {
	const [q, setQ] = useState('');
	const [shown, setShown] = useState(first);
	const [busy, setBusy] = useState(false);
	const [err, setErr] = useState('');
	const seq = useRef(0);
	// New page data (another federation, a refresh) starts over.
	useEffect(() => { setShown(first); setQ(''); setErr(''); }, [first]);
	useEffect(() => {
		if (!q.trim()) { setShown(first); return; }
		const ac = new AbortController();
		const n = ++seq.current;
		const t = setTimeout(() => {
			setBusy(true);
			federationGroups(id, { q: q.trim() }, ac.signal)
				.then((p) => { if (n === seq.current) { setShown(p); setErr(''); } }, (x: Error) => { if (x.name !== 'AbortError') setErr(errText(x)); })
				.finally(() => { if (n === seq.current) setBusy(false); });
		}, 200);
		return () => { clearTimeout(t); ac.abort(); };
	}, [q, id, first]);
	const more = async () => {
		if (!shown.next_cursor) return;
		setBusy(true); setErr('');
		try {
			const p = await federationGroups(id, { q: q.trim() || undefined, cursor: shown.next_cursor });
			setShown((cur) => ({ ...p, items: [...cur.items, ...p.items] }));
		} catch (x) { setErr(errText(x)); }
		setBusy(false);
	};
	return (
		<Card title={`Groups (${fmt(shown.total)}${q.trim() ? ` matching` : ''})`}>
			<label className="a-search a-fedsearch">
				<Search size={15} />
				<input value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search this federation's groups" aria-label="Search this federation's groups" />
			</label>
			{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
			{shown.items.length ? (
				<ul className="a-groups a-fedlist">{shown.items.map((g: FedGroup) => (
					<li key={g.id}>
						<Avatar id={g.id} name={g.title} photo={g.photo} size={28} square />
						<Link to={`/groups/${g.id}`} className="a-grow a-rowlink a-ellipsis">{g.title}</Link>
						{g.username && <span className="a-muted sm a-hide-sm">@{g.username}</span>}
						<span className="num a-muted" title={`${fmt(g.msg_count)} messages`}>{compact(g.msg_count)}</span>
					</li>
				))}</ul>
			) : <p className="a-muted">{q.trim() ? 'No group of this federation matches.' : 'This federation holds no group you can see.'}</p>}
			{shown.next_cursor && (
				<div className="a-loadmore">
					<button type="button" className="a-btn a-btn-ghost" disabled={busy} onClick={() => void more()}>
						{busy ? <LoaderCircle size={14} className="a-spin" /> : null}{busy ? 'Loading...' : `Load more (${fmt(shown.total - shown.items.length)} left)`}
					</button>
				</div>
			)}
		</Card>
	);
}

function People({ title, icon: I, people, empty }: { title: string; icon: typeof Users; people: Person[]; empty: string }) {
	return (
		<Card title={`${title} (${people.length})`}>
			{people.length ? (
				<ul className="a-groups">{people.map((p) => (
					<li key={p.id}><Avatar id={p.id} name={p.name} size={26} /><span className="a-grow"><b>{p.name}</b> <span className="a-muted sm">@{p.username}</span></span></li>
				))}</ul>
			) : <p className="a-muted"><I size={14} /> {empty}</p>}
		</Card>
	);
}

function FederationPage({ page }: { page: PageData<FederationDetailProps> }) {
	const { refresh } = useNav();
	const p = page.props;
	const f = p.federation;
	const v = page.viewer;
	const [editing, setEditing] = useState(false);
	const [deleting, setDeleting] = useState(false);
	const canEdit = p.can_edit.name || p.can_edit.description;
	return (
		<Shell crumbs={[['Federations', '/settings/federations'], [f.name, null]]}>
			<div className="a-profilehead">
				<span className="a-fedicon a-fedicon-big"><Network size={26} /></span>
				<div className="a-ph-main">
					<h1>{f.name}</h1>
					<div className="a-ph-meta">
						{f.default && <Pill tone="info">Default</Pill>}
						<span>{fmt(f.group_count)} groups</span>
						{f.created && <span title={dateTime(f.created, v)}>created {ago(f.created, v)}{f.created_by ? ` by ${f.created_by.name}` : ''}</span>}
					</div>
				</div>
				{(canEdit || p.can_edit.delete) && (
					<div className="a-head-actions">
						{canEdit && <Btn icon={Pencil} onClick={() => setEditing(true)}>Edit</Btn>}
						{p.can_edit.delete && <Btn ghost danger icon={Trash2} onClick={() => setDeleting(true)}>Delete</Btn>}
					</div>
				)}
			</div>
			{f.description
				? <p className="a-feddesc">{f.description}</p>
				: <p className="a-muted a-feddesc">{f.default ? 'Every group that is in no other federation.' : 'No description yet.'}</p>}
			{(p.can_edit.groups || p.can_edit.admins) && <p className="a-muted sm">Change its {p.can_edit.groups && p.can_edit.admins ? 'groups and admins' : p.can_edit.groups ? 'groups' : 'admins'} from the edit button on its card on the <Link to="/settings/federations" className="a-link">Federations</Link> page.</p>}
			<div className="a-cols">
				<Groups id={f.id} first={p.groups} />
				<div className="a-stack">
					<People title="Admins" icon={UserCog} people={p.admins} empty="No admin administers it; superadmins administer every federation." />
					<People title="Members" icon={Users} people={p.members} empty="No member holds a grant in it." />
				</div>
			</div>
			{editing && <EditDetails props={p} onClose={() => setEditing(false)} onSaved={() => void refresh()} />}
			{deleting && <ConfirmDelete props={p} onClose={() => setDeleting(false)} />}
		</Shell>
	);
}

export const federationRoute: RouteDef<FederationDetailProps> = {
	Page: FederationPage,
	head: (p) => ({ title: `${p.props.federation.name} - Federations - tgloggerd`, robots: 'noindex' }),
};
