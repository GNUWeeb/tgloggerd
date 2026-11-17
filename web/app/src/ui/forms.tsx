// Form parts for the settings and administration pages.
import { useState, type ReactNode } from 'react';
import { Check, Search, X } from 'lucide-react';
import { Avatar } from './primitives.tsx';

export function Field({ label, hint, children }: { label: ReactNode; hint?: ReactNode; children: ReactNode }) {
	return (
		<label className="a-field">
			<span className="a-field-label">{label}</span>
			{children}
			{hint && <span className="a-field-hint">{hint}</span>}
		</label>
	);
}

export function Switch({ checked, onChange, label, disabled }: { checked: boolean; onChange: (v: boolean) => void; label?: ReactNode; disabled?: boolean }) {
	return (
		<label className="a-switchrow">
			<input type="checkbox" className="a-switch" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
			{label && <span>{label}</span>}
		</label>
	);
}

export interface PickItem { id: number; title: string; kind: 'group' | 'user'; sub?: string; photo?: string | null }

// One chat (group, channel or user), searched by name.
export function ChatPicker({ value, onChange, options }: { value: PickItem | null; onChange: (v: PickItem) => void; options: PickItem[] }) {
	const [q, setQ] = useState('');
	const [open, setOpen] = useState(false);
	const list = options.filter((o) => o.title.toLowerCase().includes(q.toLowerCase())).slice(0, 7);
	return (
		<div className="a-picker">
			<button type="button" className="a-picker-btn" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
				{value ? <><Avatar id={value.id} name={value.title} photo={value.photo} size={22} square={value.kind === 'group'} /><span className="a-ellipsis">{value.title}</span>{value.sub && <span className="a-muted sm">{value.sub}</span>}</>
					: <span className="a-muted">Choose a chat...</span>}
			</button>
			{open && (
				<div className="a-picker-pop">
					<div className="a-picker-search"><Search size={14} /><input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search groups, channels, users" /></div>
					<ul>
						{list.map((o) => (
							<li key={o.kind + o.id}>
								<button type="button" onClick={() => { onChange(o); setOpen(false); }}>
									<Avatar id={o.id} name={o.title} photo={o.photo} size={26} square={o.kind === 'group'} />
									<span className="a-grow a-ellipsis">{o.title}{o.sub && <span className="a-muted sm"> . {o.sub}</span>}</span>
									{value?.id === o.id && <Check size={15} />}
								</button>
							</li>
						))}
						{!list.length && <li className="a-muted a-picker-empty">No chat matches.</li>}
					</ul>
				</div>
			)}
		</div>
	);
}

// Several items as removable chips.
export function MultiPicker({ value, onChange, options, placeholder = 'Add...' }: { value: PickItem[]; onChange: (v: PickItem[]) => void; options: PickItem[]; placeholder?: string }) {
	const [q, setQ] = useState('');
	const [open, setOpen] = useState(false);
	const ids = new Set(value.map((v) => v.id));
	const list = options.filter((o) => !ids.has(o.id) && o.title.toLowerCase().includes(q.toLowerCase())).slice(0, 6);
	return (
		<div className="a-multi">
			<div className="a-multi-box" onClick={() => setOpen(true)}>
				{value.map((v) => (
					<span key={v.id} className="a-chip a-chip-entity">
						<Avatar id={v.id} name={v.title} photo={v.photo} size={18} square={v.kind === 'group'} />{v.title}
						<button type="button" aria-label={`Remove ${v.title}`} onClick={(e) => { e.stopPropagation(); onChange(value.filter((x) => x.id !== v.id)); }}><X size={12} /></button>
					</span>
				))}
				<input value={q} onChange={(e) => { setQ(e.target.value); setOpen(true); }} onFocus={() => setOpen(true)} placeholder={value.length ? '' : placeholder} />
			</div>
			{open && list.length > 0 && (
				<ul className="a-multi-pop">
					{list.map((o) => (
						<li key={o.id}>
							<button type="button" onClick={() => { onChange([...value, o]); setQ(''); }}>
								<Avatar id={o.id} name={o.title} photo={o.photo} size={22} square={o.kind === 'group'} />
								<span className="a-grow a-ellipsis">{o.title}</span>
							</button>
						</li>
					))}
				</ul>
			)}
		</div>
	);
}

export function CheckChips({ items, value, onChange }: { items: [string, ReactNode][]; value: string[]; onChange: (v: string[]) => void }) {
	return (
		<div className="a-checkgrid">
			{items.map(([k, label]) => {
				const on = value.includes(k);
				return (
					<label key={k} className={`a-checkchip ${on ? 'on' : ''}`}>
						<input type="checkbox" checked={on} onChange={() => onChange(on ? value.filter((x) => x !== k) : [...value, k])} />
						{on && <Check size={13} />}{label}
					</label>
				);
			})}
		</div>
	);
}

export function Seg<K extends string>({ value, onChange, items }: { value: K; onChange: (k: K) => void; items: [K, ReactNode][] }) {
	return (
		<div className="a-seg" role="tablist">
			{items.map(([k, label]) => <button key={k} type="button" role="tab" aria-selected={value === k} className={value === k ? 'on' : ''} onClick={() => onChange(k)}>{label}</button>)}
		</div>
	);
}
