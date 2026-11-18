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
import type { Cond, FieldDef, FieldType, ListProps, Op, SearchField } from '../types.ts';

const OPS: Record<FieldDef['type'], Op[]> = {
	number: ['=', '>', '<', '>=', '<=', '!='], text: ['contains', '=', 'empty'], enum: ['=', '!='], bool: ['is'], date: ['>', '<', '>=', '<='],
};
// A field's operators: its own, or its type's.
const opsOf = (f: FieldDef): Op[] => f.ops ?? OPS[f.type];
// Operators that take no value.
const NO_VALUE: readonly string[] = ['empty', 'IS NULL', 'IS NOT NULL'];
const needsValue = (op: string) => !NO_VALUE.includes(op);
// What to type for an operator, where it is not obvious.
const OP_HINT: Partial<Record<Op, string>> = {
	'LIKE': '% matches any text, _ one character',
	'NOT LIKE': '% matches any text, _ one character',
	'matches': 'words; +must -not "a phrase"',
};

// A field of the advanced search, as the filter picker shows it.
export function searchField(f: SearchField): FieldDef {
	const type: FieldType = f.type === 'int' ? 'number' : f.type === 'datetime' ? 'date' : f.type;
	return { key: f.key, label: f.label, type, ops: f.operators as Op[], values: f.enum ? f.enum.split(',') : undefined, search: true, sortable: f.sortable, ...(f.needs_chat ? { needsChat: true } : {}) };
}

// A list's filter fields: its own, then the search's that it lacks.
export function listFields(own: FieldDef[], server?: SearchField[]): FieldDef[] {
	if (!server?.length) return own;
	const have = new Set(own.map((f) => f.key));
	return [...own, ...server.filter((f) => !have.has(f.key)).map(searchField)];
}

// Whether the filters pin the list to one chat: a `chat_id =` filter,
// with every filter joined by AND. Only then does the server take the
// fields that need it (needs_chat), which have no index of their own.
export function chatBound(conds: Cond[]): boolean {
	return conds.some((c) => c.field === 'chat_id' && c.op === '=') && conds.slice(0, -1).every((c) => c.n !== 'OR');
}

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

// What an empty list says. Unfiltered and empty for a member, it has no
// group granted yet, so nothing failed and no filter is to blame.
export function emptyText(L: { q: string; conds: unknown[] }, member: boolean, noun: string, filtered: string): string {
	if (member && !L.q && !L.conds.length) return `No ${noun} to show: no group is granted to you yet. An admin of your federation grants them.`;
	return filtered;
}

// waiting: fields offered once the filters pin one chat (needs_chat).
function FilterAdd({ fields, onAdd, waiting = [], chatLabel = 'chat id' }: { fields: FieldDef[]; onAdd: (c: Cond) => void; waiting?: FieldDef[]; chatLabel?: string }) {
	const first = fields[0] as FieldDef;
	const [open, setOpen] = useState(false);
	const [field, setField] = useState(first.key);
	const f = fields.find((x) => x.key === field) ?? first;
	const [op, setOp] = useState<Op>(opsOf(f)[0] as Op);
	const boolOf = (x: FieldDef) => (x.search ? '1' : 'yes');
	const [value, setValue] = useState(f.type === 'bool' ? boolOf(f) : '');
	const change = (k: string) => {
		const nf = fields.find((x) => x.key === k) ?? first;
		setField(k); setOp(opsOf(nf)[0] as Op);
		setValue(nf.type === 'bool' ? boolOf(nf) : nf.type === 'enum' ? (nf.values?.[0] ?? '') : '');
	};
	const own = fields.filter((x) => !x.search);
	const more = fields.filter((x) => x.search);
	const opt = (x: FieldDef) => <option key={x.key} value={x.key}>{x.label}</option>;
	return (
		<div className="a-pop-wrap">
			<button type="button" className="a-btn a-btn-ghost" onClick={() => setOpen((o) => !o)} aria-expanded={open}><Plus size={14} />Filter</button>
			{open && (
				<form className="a-pop" onSubmit={(e) => { e.preventDefault(); onAdd({ field, op, value: needsValue(op) ? value : '' }); setOpen(false); }}>
					<label>Field<select value={field} onChange={(e) => change(e.target.value)}>
						{more.length ? <><optgroup label="This list">{own.map(opt)}</optgroup><optgroup label="Advanced search">{more.map(opt)}</optgroup></> : fields.map(opt)}
					</select></label>
					{waiting.length > 0 && <span className="a-muted sm">{waiting.map((x) => x.label).join(' and ')} can be added once a {chatLabel} = filter is set, joined by AND.</span>}
					<label>Condition<select value={op} onChange={(e) => setOp(e.target.value as Op)}>{opsOf(f).map((o) => <option key={o}>{o}</option>)}</select></label>
					{needsValue(op) && (
						<label>Value
							{f.type === 'bool' ? <select value={value} onChange={(e) => setValue(e.target.value)}>{f.search ? <><option value="1">yes</option><option value="0">no</option></> : <><option>yes</option><option>no</option></>}</select>
								: f.type === 'enum' ? <select value={value} onChange={(e) => setValue(e.target.value)}>{f.values?.map((v) => <option key={v}>{v}</option>)}</select>
									: <input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={f.type === 'date' ? (f.search ? 'YYYY-MM-DD HH:MM:SS' : 'YYYY-MM-DD') : ''} />}
							{OP_HINT[op] && <span className="a-muted sm">{OP_HINT[op]}</span>}
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

const ERROR_LEAD = { bad_search: 'This search cannot run', search_timeout: 'This search took too long and was stopped' } as const;

export function FilterBar<T>({ L, fields: own, placeholder, quick, name }: { L: ListNav<T>; fields: FieldDef[]; placeholder: string; quick?: [string, Cond[]][]; name: string }) {
	const [q, setQ] = useState(L.q);
	// The lists with the advanced search add its fields, and join filters
	// with AND or OR (AND binds tighter, as in SQL).
	const fields = listFields(own, L.fields);
	const joins = !!L.fields;
	const label = (c: Cond) => fields.find((x) => x.key === c.field)?.label ?? c.field;
	const shown = (c: Cond) => `${label(c)} ${c.op}${needsValue(c.op) ? ' ' + c.value : ''}`;
	const toggle = (i: number) => L.setConds(L.conds.map((c, j) => {
		if (j !== i) return c;
		const { n: _n, ...rest } = c;
		return c.n === 'OR' ? rest : { ...rest, n: 'OR' as const };
	}));
	const sortable = fields.filter((f) => f.sortable);
	return (
		<>
		{L.error && (
			<div className="a-banner warn" role="alert"><TriangleAlert size={15} /><span>
				<b>{ERROR_LEAD[L.error.code]}</b>: {L.error.message} {L.rows.length ? 'Showing the list without it.' : ''}
			</span></div>
		)}
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
				<span key={i} className="a-chipwrap">
					{i > 0 && joins && (
						<button type="button" className={`a-join ${L.conds[i - 1]!.n === 'OR' ? 'or' : ''}`} onClick={() => toggle(i - 1)} title="AND or OR with the filter before; AND binds tighter">
							{L.conds[i - 1]!.n === 'OR' ? 'OR' : 'AND'}
						</button>
					)}
					<span className="a-chip">
						<b>{label(c)}</b> {c.op} {needsValue(c.op) && <b>{c.value}</b>}
						<button type="button" aria-label="Remove filter" onClick={() => L.setConds(L.conds.filter((_, j) => j !== i))}><X size={12} /></button>
					</span>
				</span>
			))}
			{/* Fields that need one chat are offered once the filters pin one. */}
			<FilterAdd fields={chatBound(L.conds) ? fields : fields.filter((f) => !f.needsChat)} waiting={chatBound(L.conds) ? [] : fields.filter((f) => f.needsChat)}
				chatLabel={(fields.find((f) => f.key === 'chat_id')?.label ?? 'chat id').toLowerCase()} onAdd={(c) => L.setConds([...L.conds, c])} />
			{sortable.length > 0 && (
				<label className="a-sortpick">Sort
					<select value={L.sort} onChange={(e) => L.sortBy(e.target.value)} aria-label="Sort by">
						{!sortable.some((f) => f.key === L.sort) && <option value={L.sort}>{fields.find((f) => f.key === L.sort)?.label ?? L.sort}</option>}
						{sortable.map((f) => <option key={f.key} value={f.key}>{f.label}</option>)}
					</select>
				</label>
			)}
			{(L.conds.length > 0 || L.q) && <button type="button" className="a-link" onClick={() => { setQ(''); L.setConds([]); }}>Clear all</button>}
			{(L.conds.length > 0 || L.q) && <SaveSearch name={name} title={[L.q && `"${L.q}"`, ...L.conds.map(shown)].filter(Boolean).join(', ') + ` in ${name}`} />}
		</div>
		</>
	);
}

// Page sizes; the server takes up to 1000 rows a page.
const PER = [25, 50, 100, 200, 500, 1000];

function pageList(page: number, pages: number): (number | '...')[] {
	const s = [...new Set([1, pages, page, page - 1, page + 1])].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b);
	return s.flatMap((n, i) => (i && n - (s[i - 1] as number) > 1 ? ['...' as const, n] : [n]));
}

export function Pager<T>({ L, noun }: { L: ListNav<T>; noun: string }) {
	const from = (L.page - 1) * L.per + 1;
	return (
		<div className="a-pager">
			<span className="a-muted">{L.total ? `${fmt(from)}-${fmt(from + L.rows.length - 1)} of ${L.total_exact === false ? 'about ' : ''}${fmt(L.total)} ${noun}` : `0 ${noun}`}</span>
			<span className="a-grow" />
			<label className="a-per">Rows
				<select value={L.per} onChange={(e) => L.setPer(Number(e.target.value))}>{[...new Set([...PER, L.per])].sort((a, b) => a - b).map((n) => <option key={n}>{n}</option>)}</select>
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

// A long text in a table cell: its first lines, and the whole of it
// in a dialog when it does not fit (the Drogon lists' long-text modal).
export function LongText({ text, title, limit = 120 }: { text: string; title: string; limit?: number }) {
	const [open, setOpen] = useState(false);
	if (!text) return null;
	const long = text.length > limit || text.includes('\n');
	return (
		<>
			<span className="a-longtext" title={long ? undefined : text}>{text}</span>
			{long && <button type="button" className="a-longmore" onClick={() => setOpen(true)} aria-label={`Show all of ${title}`}>more</button>}
			{open && <Modal title={title} onClose={() => setOpen(false)}><div className="a-longfull">{text}</div></Modal>}
		</>
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
