// Search, filters, a sortable table (a card list on phones) and a pager
// for the advanced-search pages. All list state lives in the URL; every
// change navigates, and the server answers with the filtered page.
import { useState, type FormEvent, type ReactNode } from 'react';
import { Search, Plus, X, ChevronUp, ChevronDown, ChevronLeft, ChevronRight, Bookmark, BookmarkCheck, TriangleAlert } from 'lucide-react';
import { Modal } from './Modal.tsx';
import { Btn } from './primitives.tsx';
import { ApiError, api, ok } from '../api/client.ts';
import { Link, setQuery, useNav } from '../nav.tsx';
import { fmt } from '../format.ts';
import type { Cond, FieldDef, ListProps, Op } from '../types.ts';

const OPS: Record<FieldDef['type'], Op[]> = {
	number: ['=', '>', '<', '!='], text: ['contains', '=', 'empty'], enum: ['=', '!='], bool: ['is'], date: ['>', '<'],
};
// A field's operators: its own, or its type's.
const opsOf = (f: FieldDef): Op[] => f.ops ?? OPS[f.type];

export function useListNav<T>(L: ListProps<T>) {
	const { page, navigate } = useNav();
	const go = (patch: Record<string, string | number | undefined>, resetPage = true) =>
		navigate(setQuery(page.url, { ...(resetPage ? { page: undefined } : {}), ...patch }), { scroll: false });
	return {
		...L,
		setQ: (q: string) => go({ q }),
		setConds: (c: Cond[]) => go({ f: c.length ? JSON.stringify(c) : undefined }),
		setPage: (n: number) => go({ page: n > 1 ? n : undefined }, false),
		setPer: (n: number) => go({ per: n }),
		sortBy: (key: string) => go({ sort: key, dir: L.sort === key && L.dir === 'desc' ? 'asc' : 'desc' }),
	};
}
type ListNav<T> = ReturnType<typeof useListNav<T>>;

function FilterAdd({ fields, onAdd }: { fields: FieldDef[]; onAdd: (c: Cond) => void }) {
	const first = fields[0] as FieldDef;
	const [open, setOpen] = useState(false);
	const [field, setField] = useState(first.key);
	const f = fields.find((x) => x.key === field) ?? first;
	const [op, setOp] = useState<Op>(opsOf(f)[0] as Op);
	const [value, setValue] = useState(f.type === 'bool' ? 'yes' : '');
	const change = (k: string) => {
		const nf = fields.find((x) => x.key === k) ?? first;
		setField(k); setOp(opsOf(nf)[0] as Op);
		setValue(nf.type === 'bool' ? 'yes' : nf.type === 'enum' ? (nf.values?.[0] ?? '') : '');
	};
	return (
		<div className="a-pop-wrap">
			<button type="button" className="a-btn a-btn-ghost" onClick={() => setOpen((o) => !o)} aria-expanded={open}><Plus size={14} />Filter</button>
			{open && (
				<form className="a-pop" onSubmit={(e) => { e.preventDefault(); onAdd({ field, op, value }); setOpen(false); }}>
					<label>Field<select value={field} onChange={(e) => change(e.target.value)}>{fields.map((x) => <option key={x.key} value={x.key}>{x.label}</option>)}</select></label>
					<label>Condition<select value={op} onChange={(e) => setOp(e.target.value as Op)}>{opsOf(f).map((o) => <option key={o}>{o}</option>)}</select></label>
					{op !== 'empty' && (
						<label>Value
							{f.type === 'bool' ? <select value={value} onChange={(e) => setValue(e.target.value)}><option>yes</option><option>no</option></select>
								: f.type === 'enum' ? <select value={value} onChange={(e) => setValue(e.target.value)}>{f.values?.map((v) => <option key={v}>{v}</option>)}</select>
									: <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={f.type === 'date' ? 'YYYY-MM-DD' : ''} />}
						</label>
					)}
					<div className="a-pop-actions">
						<button type="button" className="a-btn a-btn-ghost" onClick={() => setOpen(false)}>Cancel</button>
						<button className="a-btn a-btn-primary">Apply</button>
					</div>
				</form>
			)}
		</div>
	);
}

// Save the current search (query and filters) under a title; it then
// shows in Settings > Preferences. The title defaults to what the
// search is: the text, the filters and the list's name.
function SaveSearch({ name, title: initial }: { name: string; title: string }) {
	const { page } = useNav();
	const [open, setOpen] = useState(false);
	const [title, setTitle] = useState(initial);
	const [err, setErr] = useState('');
	const [saved, setSaved] = useState(false);
	const save = async (e: FormEvent) => {
		e.preventDefault();
		setErr('');
		try {
			await ok(api.POST('/api/account/saved-searches', { body: { title: title.trim() || initial, url: setQuery(page.url, { page: undefined }) } }));
			setSaved(true);
		} catch (x) { setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); }
	};
	const close = () => { setOpen(false); setSaved(false); setErr(''); };
	return (
		<>
			<button type="button" className="a-link a-savesearch" onClick={() => { setTitle(initial); setOpen(true); }}><Bookmark size={13} />Save search</button>
			{open && (
				<Modal title="Save this search" icon={<Bookmark size={16} />} onClose={close}
					footer={saved ? <><span className="a-grow" /><Btn primary onClick={close}>Done</Btn></>
						: <><span className="a-grow" /><Btn ghost onClick={close}>Cancel</Btn><button type="submit" form="save-search" className="a-btn a-btn-primary">Save</button></>}>
					{err && <div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>{err}</span></div>}
					{saved
						? <div className="a-banner" role="status"><BookmarkCheck size={15} /><span>Saved. Your saved searches are listed in <Link to="/settings/preferences" className="a-link" onClick={close}>Settings, Preferences</Link>.</span></div>
						: (
							<form id="save-search" className="a-form" onSubmit={(e) => void save(e)}>
								<label className="a-field"><span className="a-field-label">Title</span><input value={title} onChange={(e) => setTitle(e.target.value)} autoFocus maxLength={120} /></label>
								<span className="a-muted sm">{name}: the search text, filters and sort as they are now.</span>
							</form>
						)}
				</Modal>
			)}
		</>
	);
}

export function FilterBar<T>({ L, fields, placeholder, quick, name }: { L: ListNav<T>; fields: FieldDef[]; placeholder: string; quick?: [string, Cond[]][]; name: string }) {
	const [q, setQ] = useState(L.q);
	const label = (c: Cond) => fields.find((x) => x.key === c.field)?.label ?? c.field;
	return (
		<div className="a-toolbar">
			<form className="a-search" role="search" onSubmit={(e) => { e.preventDefault(); L.setQ(q); }}>
				<Search size={15} />
				<input value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} aria-label="Search" />
				{q && <button type="button" className="a-search-x" aria-label="Clear search" onClick={() => { setQ(''); L.setQ(''); }}><X size={13} /></button>}
			</form>
			{quick?.map(([t, c]) => {
				const on = JSON.stringify(c) === JSON.stringify(L.conds);
				return <button key={t} type="button" className={`a-quick ${on ? 'on' : ''}`} onClick={() => L.setConds(on ? [] : c)}>{t}</button>;
			})}
			{L.conds.map((c, i) => (
				<span key={i} className="a-chip">
					<b>{label(c)}</b> {c.op} {c.op !== 'empty' && <b>{c.value}</b>}
					<button type="button" aria-label="Remove filter" onClick={() => L.setConds(L.conds.filter((_, j) => j !== i))}><X size={12} /></button>
				</span>
			))}
			<FilterAdd fields={fields} onAdd={(c) => L.setConds([...L.conds, c])} />
			{(L.conds.length > 0 || L.q) && <button type="button" className="a-link" onClick={() => { setQ(''); L.setConds([]); }}>Clear all</button>}
			{(L.conds.length > 0 || L.q) && <SaveSearch name={name} title={[L.q && `"${L.q}"`, ...L.conds.map((c) => `${label(c)} ${c.op}${c.op === 'empty' ? '' : ' ' + c.value}`)].filter(Boolean).join(', ') + ` in ${name}`} />}
		</div>
	);
}

function pageList(page: number, pages: number): (number | '...')[] {
	const s = [...new Set([1, pages, page, page - 1, page + 1])].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b);
	return s.flatMap((n, i) => (i && n - (s[i - 1] as number) > 1 ? ['...' as const, n] : [n]));
}

export function Pager<T>({ L, noun }: { L: ListNav<T>; noun: string }) {
	const from = (L.page - 1) * L.per + 1;
	return (
		<div className="a-pager">
			<span className="a-muted">{L.total ? `${fmt(from)}-${fmt(from + L.rows.length - 1)} of ${fmt(L.total)} ${noun}` : `0 ${noun}`}</span>
			<span className="a-grow" />
			<label className="a-per">Rows
				<select value={L.per} onChange={(e) => L.setPer(Number(e.target.value))}>{[25, 50, 100].map((n) => <option key={n}>{n}</option>)}</select>
			</label>
			<button type="button" className="a-icon-btn" disabled={L.page <= 1} onClick={() => L.setPage(L.page - 1)} aria-label="Previous page"><ChevronLeft size={16} /></button>
			<span className="a-pages">
				{pageList(L.page, L.pages).map((n, i) => n === '...' ? <span key={'e' + i} className="a-muted">...</span>
					: <button type="button" key={n} className={n === L.page ? 'on' : ''} aria-current={n === L.page ? 'page' : undefined} onClick={() => L.setPage(n)}>{n}</button>)}
			</span>
			<button type="button" className="a-icon-btn" disabled={L.page >= L.pages} onClick={() => L.setPage(L.page + 1)} aria-label="Next page"><ChevronRight size={16} /></button>
		</div>
	);
}

export interface Col<T> { key: string; label: string; sort?: boolean; align?: 'right'; cls?: string; render: (row: T) => ReactNode }
export interface MobileRow { lead: ReactNode; title: ReactNode; sub?: ReactNode; extra?: ReactNode; side?: ReactNode }

export function DataTable<T extends { id: number }>({ L, cols, href, mobile, empty, noun }: { L: ListNav<T>; cols: Col<T>[]; href: (row: T) => string; mobile: (row: T) => MobileRow; empty: string; noun: string }) {
	const SortIcon = ({ k }: { k: string }) => (L.sort === k ? (L.dir === 'asc' ? <ChevronUp size={13} /> : <ChevronDown size={13} />) : null);
	return (
		<div className="a-card a-tablecard">
			<div className="a-tablescroll">
				<table className="a-table">
					<thead><tr>{cols.map((c) => (
						<th key={c.key} className={`${c.cls ?? ''} ${c.align === 'right' ? 'r' : ''}`} aria-sort={L.sort === c.key ? (L.dir === 'asc' ? 'ascending' : 'descending') : undefined}>
							{c.sort ? <button type="button" onClick={() => L.sortBy(c.key)}>{c.label}<SortIcon k={c.key} /></button> : c.label}
						</th>
					))}</tr></thead>
					<tbody>{L.rows.map((row) => (
						<tr key={row.id}>{cols.map((c) => <td key={c.key} className={`${c.cls ?? ''} ${c.align === 'right' ? 'r' : ''}`}>{c.render(row)}</td>)}</tr>
					))}</tbody>
				</table>
			</div>
			{L.rows.length === 0 && <div className="a-empty">{empty}</div>}
			<ul className="a-list">
				{L.rows.map((row) => {
					const m = mobile(row);
					return (
						<li key={row.id}>
							<Link to={href(row)}>
								{m.lead}
								<span className="a-list-main">
									<span className="a-uname">{m.title}</span>
									{m.sub && <span className="a-handle">{m.sub}</span>}
									{m.extra && <span className="a-list-flags">{m.extra}</span>}
								</span>
								{m.side && <span className="a-list-side">{m.side}</span>}
							</Link>
						</li>
					);
				})}
			</ul>
			<Pager L={L} noun={noun} />
		</div>
	);
}
