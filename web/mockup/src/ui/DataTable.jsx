// Search + filter chips, a sortable table that turns into a list on
// phones, and a pager. Shared by every advanced-search page.
import { useState } from 'react';
import { Search, Plus, X, ChevronUp, ChevronDown, ChevronLeft, ChevronRight, Columns3 } from 'lucide-react';
import { Link } from '../router.jsx';
import { OPS, fmt } from '../data/seed.js';

function FilterAdd({ fields, onAdd }) {
	const [open, setOpen] = useState(false);
	const [field, setField] = useState(fields[0].key);
	const f = fields.find((x) => x.key === field);
	const [op, setOp] = useState(OPS[f.type][0]);
	const [value, setValue] = useState(f.type === 'bool' ? 'yes' : f.type === 'enum' ? f.values[0] : '');
	const changeField = (k) => {
		const nf = fields.find((x) => x.key === k);
		setField(k); setOp(OPS[nf.type][0]);
		setValue(nf.type === 'bool' ? 'yes' : nf.type === 'enum' ? nf.values[0] : '');
	};
	return (
		<div className="a-pop-wrap">
			<button type="button" className="a-btn a-btn-ghost" onClick={() => setOpen((o) => !o)} aria-expanded={open}><Plus size={14} />Filter</button>
			{open && (
				<form className="a-pop" onSubmit={(e) => { e.preventDefault(); onAdd({ field, op, value }); setOpen(false); }}>
					<label>Field
						<select value={field} onChange={(e) => changeField(e.target.value)}>
							{fields.map((x) => <option key={x.key} value={x.key}>{x.label}</option>)}
						</select>
					</label>
					<label>Condition
						<select value={op} onChange={(e) => setOp(e.target.value)}>
							{OPS[f.type].map((o) => <option key={o}>{o}</option>)}
						</select>
					</label>
					{op !== 'empty' && (
						<label>Value
							{f.type === 'bool' ? (
								<select value={value} onChange={(e) => setValue(e.target.value)}><option>yes</option><option>no</option></select>
							) : f.type === 'enum' ? (
								<select value={value} onChange={(e) => setValue(e.target.value)}>{f.values.map((v) => <option key={v}>{v}</option>)}</select>
							) : (
								<input autoFocus value={value} onChange={(e) => setValue(e.target.value)} placeholder={f.type === 'date' ? 'YYYY-MM-DD' : ''} />
							)}
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

export function FilterBar({ L, fields, placeholder, quick, right }) {
	const [q, setQ] = useState(L.q);
	const label = (c) => fields.find((x) => x.key === c.field)?.label || c.field;
	return (
		<div className="a-toolbar">
			<form className="a-search" role="search" onSubmit={(e) => { e.preventDefault(); L.update({ q }); }}>
				<Search size={15} />
				<input value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} aria-label="Search" />
				{q && <button type="button" className="a-search-x" aria-label="Clear search" onClick={() => { setQ(''); L.update({ q: '' }); }}><X size={13} /></button>}
			</form>
			{quick && quick.map(([t, c]) => {
				const on = JSON.stringify(c) === JSON.stringify(L.conds);
				return <button key={t} type="button" className={`a-quick ${on ? 'on' : ''}`} onClick={() => L.update({ conds: on ? [] : c })}>{t}</button>;
			})}
			{L.conds.map((c, i) => (
				<span key={i} className="a-chip">
					<b>{label(c)}</b> {c.op} {c.op !== 'empty' && <b>{String(c.value)}</b>}
					<button type="button" aria-label="Remove filter" onClick={() => L.update({ conds: L.conds.filter((_, j) => j !== i) })}><X size={12} /></button>
				</span>
			))}
			<FilterAdd fields={fields} onAdd={(c) => L.update({ conds: [...L.conds, c] })} />
			{(L.conds.length > 0 || L.q) && <button type="button" className="a-link" onClick={() => { setQ(''); L.update({ conds: [], q: '' }); }}>Clear all</button>}
			<span className="a-grow" />
			{right}
			<button type="button" className="a-btn a-btn-ghost a-hide-sm"><Columns3 size={14} />Columns</button>
		</div>
	);
}

export function pageList(page, pages) {
	const out = new Set([1, pages, page, page - 1, page + 1]);
	return [...out].filter((n) => n >= 1 && n <= pages).sort((a, b) => a - b)
		.flatMap((n, i, a) => (i && n - a[i - 1] > 1 ? ['...', n] : [n]));
}

export function Pager({ L, noun = 'results' }) {
	const from = (L.page - 1) * L.per + 1;
	return (
		<div className="a-pager">
			<span className="a-muted">{L.total ? `${fmt(from)}-${fmt(from + L.rows.length - 1)} of ${fmt(L.total)} ${noun}` : `0 ${noun}`}</span>
			<span className="a-grow" />
			<label className="a-per">Rows
				<select value={L.per} onChange={(e) => L.update({ per: e.target.value })}>
					{[25, 50, 100].map((n) => <option key={n}>{n}</option>)}
				</select>
			</label>
			<button type="button" className="a-icon-btn" disabled={L.page <= 1} onClick={() => L.update({ page: L.page - 1 })} aria-label="Previous page"><ChevronLeft size={16} /></button>
			<span className="a-pages">
				{pageList(L.page, L.pages).map((n, i) => n === '...'
					? <span key={'e' + i} className="a-muted">...</span>
					: <button type="button" key={n} className={n === L.page ? 'on' : ''} onClick={() => L.update({ page: n })} aria-current={n === L.page ? 'page' : undefined}>{n}</button>)}
			</span>
			<button type="button" className="a-icon-btn" disabled={L.page >= L.pages} onClick={() => L.update({ page: L.page + 1 })} aria-label="Next page"><ChevronRight size={16} /></button>
		</div>
	);
}

// cols: [{ key, label, sort?: bool, align?: 'right', cls?, render(row) }]
// mobile(row) -> { lead, title, sub, extra, side }
export function DataTable({ L, cols, href, mobile, empty = 'Nothing matches these filters.', noun }) {
	const Sort = ({ k }) => (L.sort === k ? (L.dir === 'asc' ? <ChevronUp size={13} /> : <ChevronDown size={13} />) : null);
	return (
		<div className="a-card a-tablecard">
			<div className="a-tablescroll">
				<table className="a-table">
					<thead>
						<tr>
							{cols.map((c) => (
								<th key={c.key} className={`${c.cls || ''} ${c.align === 'right' ? 'r' : ''}`} aria-sort={L.sort === c.key ? (L.dir === 'asc' ? 'ascending' : 'descending') : undefined}>
									{c.sort ? <button type="button" onClick={() => L.toggleSort(c.key)}>{c.label}<Sort k={c.key} /></button> : c.label}
								</th>
							))}
						</tr>
					</thead>
					<tbody>
						{L.rows.map((row) => (
							<tr key={row.id}>
								{cols.map((c) => <td key={c.key} className={`${c.cls || ''} ${c.align === 'right' ? 'r' : ''}`}>{c.render ? c.render(row) : row[c.key]}</td>)}
							</tr>
						))}
					</tbody>
				</table>
			</div>
			{L.rows.length === 0 && <div className="a-empty">{empty}</div>}
			<ul className="a-list">
				{L.rows.map((row) => {
					const m = mobile(row);
					const body = (
						<>
							{m.lead}
							<span className="a-list-main">
								<span className="a-uname">{m.title}</span>
								{m.sub && <span className="a-handle">{m.sub}</span>}
								{m.extra && <span className="a-list-flags">{m.extra}</span>}
							</span>
							{m.side && <span className="a-list-side">{m.side}</span>}
						</>
					);
					return <li key={row.id}>{href ? <Link to={href(row)}>{body}</Link> : <div className="a-list-row">{body}</div>}</li>;
				})}
			</ul>
			<Pager L={L} noun={noun} />
		</div>
	);
}
