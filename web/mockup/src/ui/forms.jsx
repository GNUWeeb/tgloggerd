// Form parts for the admin pages: modal, switch, chat picker.
import { useEffect, useState } from 'react';
import { X, Search, Check } from 'lucide-react';
import { Avatar } from './primitives.jsx';

// A centered dialog: the one pattern for every add/edit form and preview.
export function Modal({ title, icon, onClose, children, footer, wide = false }) {
	useEffect(() => {
		const on = (e) => e.key === 'Escape' && onClose();
		addEventListener('keydown', on);
		const prev = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		return () => { removeEventListener('keydown', on); document.body.style.overflow = prev; };
	}, [onClose]);
	return (
		<div className="a-modal-wrap" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
			<div className={`a-modal ${wide ? 'wide' : ''}`} role="dialog" aria-modal="true" aria-label={title}>
				<header className="a-modal-head">
					{icon}
					<b className="a-grow a-ellipsis">{title}</b>
					<button type="button" className="a-icon-btn" onClick={onClose} aria-label="Close"><X size={16} /></button>
				</header>
				<div className="a-modal-body a-form">{children}</div>
				{footer && <footer className="a-modal-foot">{footer}</footer>}
			</div>
		</div>
	);
}

export function Field({ label, hint, children }) {
	return (
		<label className="a-field">
			<span className="a-field-label">{label}</span>
			{children}
			{hint && <span className="a-field-hint">{hint}</span>}
		</label>
	);
}

export function Switch({ checked, onChange, label }) {
	return (
		<label className="a-switchrow">
			<input type="checkbox" className="a-switch" checked={checked} onChange={(e) => onChange(e.target.checked)} />
			{label && <span>{label}</span>}
		</label>
	);
}

// Searchable picker over chats (users and groups), busiest first.
export function ChatPicker({ value, onChange, options }) {
	const [q, setQ] = useState('');
	const [open, setOpen] = useState(false);
	const list = options.filter((o) => (o.title || o.name).toLowerCase().includes(q.toLowerCase())).slice(0, 7);
	return (
		<div className="a-picker">
			<button type="button" className="a-picker-btn" onClick={() => setOpen((o) => !o)} aria-expanded={open}>
				{value ? <><Avatar e={value} size={22} square={value.kind === 'group'} /><span className="a-ellipsis">{value.title || value.name}</span><span className="a-muted sm">{value.kind === 'group' ? value.type : 'user'}</span></>
					: <span className="a-muted">Choose a chat...</span>}
			</button>
			{open && (
				<div className="a-picker-pop">
					<div className="a-picker-search"><Search size={14} /><input autoFocus value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search groups, channels, users" /></div>
					<ul>
						{list.map((o) => (
							<li key={o.kind + o.id}>
								<button type="button" onClick={() => { onChange(o); setOpen(false); }}>
									<Avatar e={o} size={26} square={o.kind === 'group'} />
									<span className="a-grow a-ellipsis">{o.title || o.name}<span className="a-muted sm"> . {o.kind === 'group' ? o.type : 'user'}</span></span>
									{value && value.id === o.id && <Check size={15} />}
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

export const CONTENT_KINDS = [
	['sticker', 'Sticker'], ['text', 'Text message'], ['photo', 'Photo'],
	['video', 'Video'], ['document', 'Document'], ['other', 'Others'],
];

export function TypeChecks({ value, onChange, kinds = CONTENT_KINDS }) {
	return (
		<div className="a-checkgrid">
			{kinds.map(([k, label]) => (
				<label key={k} className={`a-checkchip ${value.includes(k) ? 'on' : ''}`}>
					<input type="checkbox" checked={value.includes(k)} onChange={() => onChange(value.includes(k) ? value.filter((x) => x !== k) : [...value, k])} />
					{value.includes(k) && <Check size={13} />}{label}
				</label>
			))}
		</div>
	);
}
