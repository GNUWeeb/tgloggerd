// "Search everything" (Ctrl-K, or Cmd-K): jump to a page of the menu or
// to a chat (a group, a channel or a private chat) by typing part of its
// name. Chats come from GET /api/chats/search, within the viewer's scope.
import { useEffect, useRef, useState, type ComponentType, type KeyboardEvent } from 'react';
import { createPortal } from 'react-dom';
import { CornerDownLeft, Search } from 'lucide-react';
import { useNav } from '../nav.tsx';
import { searchChats } from '../api/search.ts';
import { Avatar } from './primitives.tsx';
import type { PickItem } from '../types.ts';

export type JumpPage = { href: string; label: string; icon: ComponentType<{ size?: number }> };
type Hit = { kind: 'page'; page: JumpPage } | { kind: 'chat'; chat: PickItem };

const hrefOf = (h: Hit) => (h.kind === 'page' ? h.page.href : h.chat.kind === 'group' ? `/groups/${h.chat.id}` : `/users/${h.chat.id}`);

export function QuickJump({ pages, onClose }: { pages: JumpPage[]; onClose: () => void }) {
	const { navigate } = useNav();
	const [q, setQ] = useState('');
	const [chats, setChats] = useState<PickItem[]>([]);
	const [busy, setBusy] = useState(false);
	const [failed, setFailed] = useState(false);
	const [at, setAt] = useState(0);
	const input = useRef<HTMLInputElement>(null);
	useEffect(() => { input.current?.focus(); }, []);
	// The chat search, a moment after typing stops; an older answer
	// never replaces a newer one.
	useEffect(() => {
		const ac = new AbortController();
		const t = setTimeout(() => {
			setBusy(true);
			searchChats({ q: q.trim(), kind: 'any', limit: 8 }, ac.signal)
				.then((r) => { setChats(r.items); setFailed(false); }, (x: Error) => { if (x.name !== 'AbortError') setFailed(true); })
				.finally(() => { if (!ac.signal.aborted) setBusy(false); });
		}, 150);
		return () => { clearTimeout(t); ac.abort(); };
	}, [q]);
	const term = q.trim().toLowerCase();
	const hits: Hit[] = [
		...pages.filter((p) => term && p.label.toLowerCase().includes(term)).map((page) => ({ kind: 'page' as const, page })),
		...chats.map((chat) => ({ kind: 'chat' as const, chat })),
	];
	useEffect(() => { setAt(0); }, [q, chats]);
	const go = (h: Hit | undefined) => { if (!h) return; onClose(); navigate(hrefOf(h)); };
	const key = (e: KeyboardEvent) => {
		if (e.key === 'Escape') { e.preventDefault(); onClose(); }
		else if (e.key === 'ArrowDown') { e.preventDefault(); setAt((i) => Math.min(hits.length - 1, i + 1)); }
		else if (e.key === 'ArrowUp') { e.preventDefault(); setAt((i) => Math.max(0, i - 1)); }
		else if (e.key === 'Enter') { e.preventDefault(); go(hits[at]); }
	};
	const box = (
		<div className="a-modal-wrap a-jump-wrap" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
			<div className="a-jump" role="dialog" aria-modal="true" aria-label="Search everything">
				<div className="a-jump-in">
					<Search size={16} />
					<input ref={input} value={q} onChange={(e) => setQ(e.target.value)} onKeyDown={key} placeholder="Jump to a page or a chat"
						role="combobox" aria-expanded="true" aria-controls="a-jump-list" aria-activedescendant={hits[at] ? `a-jump-${at}` : undefined} aria-autocomplete="list" />
					<kbd>Esc</kbd>
				</div>
				<ul id="a-jump-list" className="a-jump-list" role="listbox" aria-label="Results">
					{hits.map((h, i) => (
						<li key={hrefOf(h)} id={`a-jump-${i}`} role="option" aria-selected={i === at} className={i === at ? 'on' : ''}
							onMouseEnter={() => setAt(i)} onMouseDown={(e) => { e.preventDefault(); go(h); }}>
							{h.kind === 'page'
								? <><span className="a-jump-icon"><h.page.icon size={16} /></span><span className="a-grow">{h.page.label}</span><span className="a-muted sm">page</span></>
								: <><Avatar id={h.chat.id} name={h.chat.title} photo={h.chat.photo} size={24} square={h.chat.kind === 'group'} /><span className="a-grow a-ellipsis">{h.chat.title}</span><span className="a-muted sm">{h.chat.sub ?? (h.chat.kind === 'group' ? 'group' : 'private chat')}</span></>}
							{i === at && <CornerDownLeft size={13} className="a-muted" />}
						</li>
					))}
				</ul>
				{!hits.length && <div className="a-jump-empty a-muted">{failed ? 'The search could not reach the server.' : busy ? 'Searching...' : term ? 'Nothing matches.' : 'Type to search.'}</div>}
			</div>
		</div>
	);
	return createPortal(box, document.querySelector('.app') ?? document.body);
}

// Ctrl-K or Cmd-K, anywhere on the page, toggles it.
export function useQuickJumpKey(toggle: () => void) {
	useEffect(() => {
		const on = (e: globalThis.KeyboardEvent) => {
			if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 'k') { e.preventDefault(); toggle(); }
		};
		addEventListener('keydown', on);
		return () => removeEventListener('keydown', on);
	}, [toggle]);
}
