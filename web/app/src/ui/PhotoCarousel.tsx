// A user's or a group's recorded photos in a carousel over the page
// (GET /api/users/{id}/photos, /api/groups/{id}/photos): newest first,
// opened at the photo asked for or else the current one. The arrow keys,
// Home and End, the buttons and a swipe move through them; Esc, the close
// button or a click beside the photo closes it.
import { useCallback, useEffect, useRef, useState, type PointerEvent } from 'react';
import { createPortal } from 'react-dom';
import { ChevronLeft, ChevronRight, CloudOff, ExternalLink, LoaderCircle, X } from 'lucide-react';
import { api, ApiError, ok } from '../api/client.ts';
import { dateTime } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type Photos = S['UserPhotos'];

const SWIPE = 50;

export function PhotoCarousel({ kind, id, start, title, viewer, onClose }: { kind: 'users' | 'groups'; id: number; start: number | null; title: string; viewer: Viewer; onClose: () => void }) {
	const [photos, setPhotos] = useState<Photos | null>(null);
	const [err, setErr] = useState('');
	const [at, setAt] = useState(0);
	const close = useRef<HTMLButtonElement>(null);
	const down = useRef<number | null>(null);
	useEffect(() => {
		let live = true;
		const call = kind === 'users'
			? ok(api.GET('/api/users/{id}/photos', { params: { path: { id } } }))
			: ok(api.GET('/api/groups/{id}/photos', { params: { path: { id } } }));
		call.then((p) => {
			if (!live) return;
			setPhotos(p);
			// The photo asked for, else the current one, else the newest.
			const want = start ?? p.current_id;
			const i = want === null ? -1 : p.items.findIndex((x) => x.id === want);
			setAt(Math.max(0, i));
		}, (x) => { if (live) setErr(x instanceof ApiError ? x.message : 'The server could not be reached.'); });
		return () => { live = false; };
	}, [kind, id, start]);
	const n = photos?.items.length ?? 0;
	const go = useCallback((i: number) => setAt((cur) => (n ? Math.min(n - 1, Math.max(0, i)) : cur)), [n]);
	useEffect(() => {
		const on = (e: KeyboardEvent) => {
			if (e.key === 'Escape') onClose();
			// Newest first: left is newer, right is older, as a timeline reads.
			else if (e.key === 'ArrowLeft') setAt((i) => Math.max(0, i - 1));
			else if (e.key === 'ArrowRight') setAt((i) => Math.min(n - 1, i + 1));
			else if (e.key === 'Home') setAt(0);
			else if (e.key === 'End') setAt(n - 1);
			else return;
			e.preventDefault();
		};
		addEventListener('keydown', on);
		const prev = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		close.current?.focus();
		return () => { removeEventListener('keydown', on); document.body.style.overflow = prev; };
	}, [n, onClose]);
	const onDown = (e: PointerEvent) => { down.current = e.clientX; };
	const onUp = (e: PointerEvent) => {
		if (down.current === null) return;
		const dx = e.clientX - down.current;
		down.current = null;
		if (dx > SWIPE) go(at - 1);
		else if (dx < -SWIPE) go(at + 1);
	};
	const p = photos?.items[at];
	const box = (
		<div className="a-carousel" role="dialog" aria-modal="true" aria-label={`Photos of ${title}`} onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
			<div className="a-carousel-bar">
				<span className="a-grow a-ellipsis"><b>{title}</b>{n > 0 && <span className="a-carousel-count"> . {at + 1} of {n}</span>}</span>
				{p?.url && <a className="a-carousel-btn" href={p.url} target="_blank" rel="noopener" aria-label="Open the file"><ExternalLink size={18} /></a>}
				<button ref={close} type="button" className="a-carousel-btn" onClick={onClose} aria-label="Close"><X size={20} /></button>
			</div>
			<div className="a-carousel-stage" onPointerDown={onDown} onPointerUp={onUp} onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
				{err ? <div className="a-carousel-note" role="alert">{err}</div>
					: !photos ? <div className="a-carousel-note"><LoaderCircle size={22} className="a-spin" /></div>
						: !p ? <div className="a-carousel-note">No photo is recorded.</div>
							: p.url ? <img key={p.id} className="a-carousel-img" src={p.url} alt={`Photo of ${title}, set ${dateTime(p.at, viewer)}`} draggable={false} />
								: <div className="a-carousel-note"><CloudOff size={28} /><span>This photo's file is not stored.</span></div>}
				{n > 1 && <button type="button" className="a-carousel-nav prev" onClick={() => go(at - 1)} disabled={at === 0} aria-label="Newer photo"><ChevronLeft size={26} /></button>}
				{n > 1 && <button type="button" className="a-carousel-nav next" onClick={() => go(at + 1)} disabled={at === n - 1} aria-label="Older photo"><ChevronRight size={26} /></button>}
			</div>
			{p && (
				<div className="a-carousel-caption" aria-live="polite">
					Set {dateTime(p.at, viewer)}{p.id === photos?.current_id ? <span className="a-carousel-current">current photo</span> : null}
				</div>
			)}
			{n > 1 && (
				<div className="a-carousel-strip" role="group" aria-label="All photos">
					{photos!.items.map((x, i) => (
						<button key={x.id} type="button" className={i === at ? 'on' : ''} aria-current={i === at ? 'true' : undefined} aria-label={`Photo set ${dateTime(x.at, viewer)}`} onClick={() => go(i)}>
							{x.url ? <img src={x.url} alt="" loading="lazy" /> : <CloudOff size={14} />}
						</button>
					))}
				</div>
			)}
		</div>
	);
	return createPortal(box, document.querySelector('.app') ?? document.body);
}
