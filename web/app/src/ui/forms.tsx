// Form parts for the settings and administration pages.
import { useEffect, useState, type ReactNode } from 'react';
import { track } from './busy.ts';
import { Check, Eye, EyeOff, Search, X } from 'lucide-react';
import { Avatar } from './primitives.tsx';
import { searchChats, type ChatQuery } from '../api/search.ts';
import type { PickItem } from '../types.ts';

export function Field({ label, hint, children }: { label: ReactNode; hint?: ReactNode; children: ReactNode }) {
	return (
		<label className="a-field">
			<span className="a-field-label">{label}</span>
			{children}
			{hint && <span className="a-field-hint">{hint}</span>}
		</label>
	);
}

// Attributes that keep browsers and password managers from treating a
// field as part of a login form and filling it with the saved site
// login. Spread on every field of a form that holds a token or secret.
export const NO_FILL = {
	autoComplete: 'off',
	'data-1p-ignore': 'true',
	'data-lpignore': 'true',
	'data-form-type': 'other',
} as const;

// A token, webhook URL or other secret that is not the account's own
// password. It is never type=password: browsers and password managers
// ignore autocomplete=off there and fill in the site login. It is
// masked by CSS instead (.a-secret), with a toggle button to show it
// (a stable name; aria-pressed says whether it is shown). Name and
// id must not look like a login field (never name, user, password...).
export function SecretInput({ id, value, onChange, what, placeholder }: { id: string; value: string; onChange: (v: string) => void; what: string; placeholder?: string }) {
	const [shown, setShown] = useState(false);
	return (
		<span className="a-inputwrap">
			<input type="text" id={id} name={id} className={shown ? '' : 'a-secret'} value={value} onChange={(e) => onChange(e.target.value)} placeholder={placeholder}
				{...NO_FILL} autoCorrect="off" autoCapitalize="off" spellCheck={false} />
			<button type="button" className="a-icon-btn sm" aria-pressed={shown} aria-label={`Show ${what}`} title={shown ? `Hide ${what}` : `Show ${what}`} onClick={() => setShown((x) => !x)}>{shown ? <EyeOff size={14} /> : <Eye size={14} />}</button>
		</span>
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

export type { PickItem };

// Search results for a picker, fetched while it is open: debounced, the
// previous request aborted, `exclude` dropped from the list.
// Where a picker's choices come from: the chat search, or another
// search of the server's, named by `key` (a new key searches again).
export interface Finder { key: string; find: (q: string, signal: AbortSignal) => Promise<PickItem[]> }
export type PickSource = ChatQuery | Finder;
const isFinder = (x: PickSource): x is Finder => typeof (x as Finder).find === 'function';

function useChatResults(query: PickSource | undefined, q: string, open: boolean, exclude: ReadonlySet<number>) {
	const [state, setState] = useState<{ items: PickItem[]; loading: boolean; failed: boolean }>({ items: [], loading: false, failed: false });
	const key = !query ? '' : isFinder(query) ? `find|${query.key}` : `${query.kind ?? ''}|${query.federation ?? ''}`;
	useEffect(() => {
		if (!query || !open) return;
		const ac = new AbortController();
		setState((s) => ({ ...s, loading: true, failed: false }));
		const t = setTimeout(() => {
			track(isFinder(query) ? query.find(q.trim(), ac.signal) : searchChats({ ...query, q: q.trim(), limit: 20 }, ac.signal).then((page) => page.items))
				.then((items) => setState({ items, loading: false, failed: false }))
				.catch(() => { if (!ac.signal.aborted) setState({ items: [], loading: false, failed: true }); });
		}, q ? 200 : 0);
		return () => { clearTimeout(t); ac.abort(); };
	}, [key, q, open]);
	return { ...state, items: state.items.filter((o) => !exclude.has(o.id)) };
}

const NO_IDS: ReadonlySet<number> = new Set();

function PickEmpty({ loading, failed, q }: { loading: boolean; failed: boolean; q: string }) {
	return <li className="a-muted a-picker-empty">{loading ? 'Searching...' : failed ? 'The search failed; try again.' : q ? 'No chat matches.' : 'Nothing to choose.'}</li>;
}

// One chat (group, channel or user), searched on the server. Its
// buttons name themselves: inside a Field's <label> they would otherwise
// be named after the label's whole text, hint and listed chats included.
export function ChatPicker({ value, onChange, search, exclude = NO_IDS, placeholder = 'Search groups, channels, users' }: { value: PickItem | null; onChange: (v: PickItem) => void; search: ChatQuery; exclude?: ReadonlySet<number>; placeholder?: string }) {
	const [q, setQ] = useState('');
	const [open, setOpen] = useState(false);
	const r = useChatResults(search, q, open, exclude);
	const list = r.items.slice(0, 8);
	return (
		<div className="a-picker">
			<button type="button" className="a-picker-btn" onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-label={value ? `Chat: ${value.title}. Change` : 'Choose a chat'}>
				{value ? <><Avatar id={value.id} name={value.title} photo={value.photo} size={22} square={value.kind === 'group'} /><span className="a-ellipsis">{value.title}</span>{value.sub && <span className="a-muted sm">{value.sub}</span>}</>
					: <span className="a-muted">Choose a chat...</span>}
			</button>
			{open && (
				<div className="a-picker-pop">
					<div className="a-picker-search"><Search size={14} /><input autoFocus name="chat-search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={placeholder} {...NO_FILL} /></div>
					<ul aria-busy={r.loading}>
						{list.map((o) => (
							<li key={o.kind + o.id}>
								<button type="button" aria-label={o.sub ? `${o.title}, ${o.sub}` : o.title} onClick={() => { onChange(o); setOpen(false); }}>
									<Avatar id={o.id} name={o.title} photo={o.photo} size={26} square={o.kind === 'group'} />
									<span className="a-grow a-ellipsis">{o.title}{o.sub && <span className="a-muted sm"> . {o.sub}</span>}</span>
									{value?.id === o.id && <Check size={15} />}
								</button>
							</li>
						))}
						{!list.length && <PickEmpty loading={r.loading} failed={r.failed} q={q} />}
					</ul>
				</div>
			)}
		</div>
	);
}

// Several items as removable chips: chats searched on the server
// (`search`), or a short list the page already holds (`options`).
export function MultiPicker({ value, onChange, options, search, placeholder = 'Add...' }: { value: PickItem[]; onChange: (v: PickItem[]) => void; options?: PickItem[]; search?: PickSource; placeholder?: string }) {
	const [q, setQ] = useState('');
	const [open, setOpen] = useState(false);
	const ids = new Set(value.map((v) => v.id));
	const r = useChatResults(search, q, open, ids);
	const list = (search ? r.items : (options ?? []).filter((o) => !ids.has(o.id) && o.title.toLowerCase().includes(q.toLowerCase()))).slice(0, 6);
	return (
		<div className="a-multi" onBlur={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setOpen(false); }}>
			<div className="a-multi-box" onClick={() => setOpen(true)}>
				{value.map((v) => (
					<span key={v.id} className="a-chip a-chip-entity">
						<Avatar id={v.id} name={v.title} photo={v.photo} size={18} square={v.kind === 'group'} />{v.title}
						<button type="button" aria-label={`Remove ${v.title}`} onClick={(e) => { e.stopPropagation(); onChange(value.filter((x) => x.id !== v.id)); }}><X size={12} /></button>
					</span>
				))}
				<input value={q} onChange={(e) => { setQ(e.target.value); setOpen(true); }} onFocus={() => setOpen(true)} placeholder={value.length ? '' : placeholder} aria-label={placeholder} />
			</div>
			{open && (list.length > 0 || (search && (r.loading || r.failed || q))) && (
				<ul className="a-multi-pop" aria-busy={search ? r.loading : undefined}>
					{list.map((o) => (
						<li key={o.id}>
							<button type="button" aria-label={`Add ${o.title}`} onClick={() => { onChange([...value, o]); setQ(''); }}>
								<Avatar id={o.id} name={o.title} photo={o.photo} size={22} square={o.kind === 'group'} />
								<span className="a-grow a-ellipsis">{o.title}</span>
							</button>
						</li>
					))}
					{!list.length && <PickEmpty loading={r.loading} failed={r.failed} q={q} />}
				</ul>
			)}
		</div>
	);
}

export function CheckChips<K extends string>({ items, value, onChange }: { items: [K, ReactNode][]; value: K[]; onChange: (v: K[]) => void }) {
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
