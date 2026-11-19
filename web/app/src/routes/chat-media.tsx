// A chat message's media: photos, videos, animations, stickers, voice
// notes, audio and documents, each stored, fetchable from Telegram, or
// kept as metadata only (web/docs/media.md, "Chat pages"). A media
// object has a url exactly when its bytes are stored or fetchable.
import { useEffect, useRef, useState, type MouseEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { CloudDownload, CloudOff, Download, ExternalLink, Pause, Play, Trash2, X } from 'lucide-react';
import { duration } from '../ui/media.tsx';
import { playTgs } from '../ui/lottie.ts';
import { bytes } from '../format.ts';
import type { ChatMedia, ChatMessage } from '../types.ts';

export type MediaKind = 'photo' | 'video' | 'animation' | 'sticker' | 'voice' | 'audio' | 'document';

export type Media = ChatMedia;

const ratioOf = (m: Pick<Media, 'w' | 'h'>, def = '4 / 3') => (m.w && m.h ? `${m.w} / ${m.h}` : def);

// A plain left click opens the viewer; other clicks (new tab, middle
// button) follow the link to the file itself.
const plainClick = (e: MouseEvent) => e.button === 0 && !e.metaKey && !e.ctrlKey && !e.shiftKey && !e.altKey;

// The full-size viewer: a picture or a playing video over the page.
// Esc, the close button or a click beside the media closes it. It needs
// only the file's URL, name and size, so a file row can open it too.
export function Lightbox({ m, kind, onClose }: { m: Pick<Media, 'url' | 'name' | 'w' | 'h'>; kind: 'picture' | 'video'; onClose: () => void }) {
	const close = useRef<HTMLButtonElement>(null);
	useEffect(() => {
		const on = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
		addEventListener('keydown', on);
		const prev = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
		close.current?.focus();
		return () => { removeEventListener('keydown', on); document.body.style.overflow = prev; };
	}, [onClose]);
	const url = m.url ?? undefined;
	const box = (
		<div className="c-lightbox" role="dialog" aria-modal="true" aria-label="Media viewer" onMouseDown={(e) => e.target === e.currentTarget && onClose()}>
			<div className="c-lb-bar">
				{m.name && <span className="a-ellipsis">{m.name}</span>}
				<a className="c-lb-btn" href={url} target="_blank" rel="noopener" aria-label="Open the file"><ExternalLink size={18} /></a>
				<button ref={close} type="button" className="c-lb-btn" onClick={onClose} aria-label="Close"><X size={20} /></button>
			</div>
			{kind === 'picture'
				? <img className="c-lb-media" src={url} alt="" style={{ aspectRatio: ratioOf(m, 'auto') }} />
				: <video className="c-lb-media" src={url} controls autoPlay playsInline style={{ aspectRatio: ratioOf(m, 'auto') }} />}
		</div>
	);
	// Into the app root, so a transformed ancestor cannot trap the overlay.
	return createPortal(box, document.querySelector('.app') ?? document.body);
}

function Missing({ m, ratio, label = 'Not stored', children }: { m: Media; ratio?: string; label?: string; children?: ReactNode }) {
	// A blurred preview (TDLib's minithumb) shows even without the bytes.
	const bg = m.thumb ? { backgroundImage: `url("${m.thumb}")` } : {};
	return (
		<div className={`c-media c-missing ${m.thumb ? 'c-blurred' : ''}`} style={{ ...(ratio ? { aspectRatio: ratio } : {}), ...bg }} title="Not stored, only metadata kept">
			<CloudOff size={22} /><span>{label}</span>
			{m.size ? <span className="c-sub">{bytes(m.size)}</span> : null}
			{children}
		</div>
	);
}

function DeletedMark({ m }: { m: Media }) {
	return m.deleted_at ? <span className="c-itemdel" title={`Deleted ${m.deleted_at}`}><Trash2 size={11} />deleted</span> : null;
}

// A photo: the picture at its real shape; a click opens the viewer.
export function Picture({ m, single, onClick = false }: { m: Media; single?: boolean; onClick?: boolean }) {
	const [shown, setShown] = useState(!onClick);
	const [open, setOpen] = useState(false);
	const ratio = ratioOf(m);
	const cls = `c-media ${single ? 'single' : ''} ${m.deleted_at ? 'deleted' : ''}`;
	if (!m.url) return <Missing m={m} ratio={ratio} />;
	// "Load media on click": a placeholder until asked for.
	if (!shown) return <button type="button" className={`${cls} c-missing c-reveal`} style={{ aspectRatio: ratio }} onClick={() => setShown(true)}><Download size={20} /><span>Show</span></button>;
	return (
		<>
			<a className={cls} style={{ aspectRatio: ratio }} href={m.url} target="_blank" rel="noopener" aria-label="Photo"
				onClick={(e) => { if (plainClick(e)) { e.preventDefault(); setOpen(true); } }}>
				<img src={m.thumb ?? m.url} alt="" loading="lazy" />
				<DeletedMark m={m} />
			</a>
			{open && <Lightbox m={m} kind="picture" onClose={() => setOpen(false)} />}
		</>
	);
}

// A video, or an animation when the viewer must ask for it: a still box
// that fetches nothing until clicked, then plays in the viewer. A stored
// video shows its first frame unless media load on click.
export function VideoBox({ m, single, animation = false, onClick = false }: { m: Media; single?: boolean; animation?: boolean; onClick?: boolean }) {
	const [open, setOpen] = useState(false);
	// A video note is round, as Telegram shows it.
	const ratio = m.round ? '1 / 1' : ratioOf(m, '16 / 9');
	const cls = `c-media c-videowrap ${single ? 'single' : ''} ${m.round ? 'round' : ''} ${m.deleted_at ? 'deleted' : ''}`;
	const info = <span className="c-dur">{animation ? 'GIF' : m.duration ? duration(m.duration) : ''}{m.size ? `${animation || m.duration ? ' . ' : ''}${bytes(m.size)}` : ''}</span>;
	if (!m.url) return <Missing m={m} ratio={ratio}>{m.duration ? <span className="c-sub">{duration(m.duration)}</span> : null}</Missing>;
	const lazy = !m.stored || onClick;
	return (
		<>
			<a className={cls} style={{ aspectRatio: ratio }} href={m.url} target="_blank" rel="noopener" aria-label={animation ? 'Animation' : 'Video'}
				onClick={(e) => { if (plainClick(e)) { e.preventDefault(); setOpen(true); } }}>
				{m.thumb
					? <img src={m.thumb} alt="" loading="lazy" />
					: <video src={lazy ? undefined : m.url} preload={lazy ? 'none' : 'metadata'} muted playsInline tabIndex={-1} />}
				<span className="c-playbig">{m.stored ? <Play size={22} fill="currentColor" /> : <CloudDownload size={22} />}</span>
				{!m.stored && <span className="c-fetch">Load from Telegram</span>}
				{info}
				<DeletedMark m={m} />
			</a>
			{open && <Lightbox m={m} kind="video" onClose={() => setOpen(false)} />}
		</>
	);
}

// An animation (GIF as mp4): plays inline, muted and looping, once its
// bytes may load; otherwise the click-to-play box.
export function Animation({ m, onClick = false }: { m: Media; onClick?: boolean }) {
	const [open, setOpen] = useState(false);
	if (!m.url || !m.stored || onClick) return <VideoBox m={m} single animation onClick={onClick} />;
	return (
		<>
			<a className="c-media single c-videowrap" style={{ aspectRatio: ratioOf(m, '1 / 1') }} href={m.url} target="_blank" rel="noopener" aria-label="Animation"
				onClick={(e) => { if (plainClick(e)) { e.preventDefault(); setOpen(true); } }}>
				<video src={m.url} autoPlay loop muted playsInline tabIndex={-1} />
				<span className="c-dur">GIF</span>
			</a>
			{open && <Lightbox m={m} kind="video" onClose={() => setOpen(false)} />}
		</>
	);
}

// A sticker's format: Go's, else from how it renders or its extension.
export const stickerFormat = (m: Media) => m.format ?? (m.render === 'lottie' || m.ext === 'tgs' ? 'tgs' : m.render === 'animation' || m.ext === 'webm' ? 'webm' : 'webp');

// An animated (.tgs) sticker: its emoji until the Lottie player draws
// it, and if it cannot.
function LottieSticker({ m }: { m: Media }) {
	const box = useRef<HTMLDivElement>(null);
	const [state, setState] = useState<'wait' | 'ready' | 'failed'>('wait');
	useEffect(() => {
		if (!box.current || !m.url) return;
		setState('wait');
		return playTgs(box.current, m.url, () => setState('ready'), () => setState('failed'));
	}, [m.url]);
	const label = `sticker ${m.emoji ?? ''}`.trim();
	return (
		<div className={`c-sticker c-lottie ${state}`} role="img" aria-label={label} title={state === 'failed' ? 'This animated sticker could not be played' : undefined}>
			<div ref={box} className="c-lottie-box" aria-hidden="true" />
			{state !== 'ready' && <span className="c-sticker-emoji c-lottie-emoji" aria-hidden="true">{m.emoji || '?'}</span>}
		</div>
	);
}

// A sticker: an image, a looping video, or a Lottie animation.
export function Sticker({ m }: { m: Media }) {
	const [open, setOpen] = useState(false);
	const f = stickerFormat(m);
	if (m.url && f === 'tgs') return <LottieSticker m={m} />;
	if (!m.url || f === 'tgs') return <div className="c-sticker c-sticker-emoji" role="img" aria-label={`sticker ${m.emoji ?? ''}`.trim()} title={m.url ? undefined : 'Not stored'}>{m.emoji || <CloudOff size={40} />}</div>;
	if (f === 'webm') return <video className="c-sticker" src={m.url} autoPlay loop muted playsInline aria-label={`sticker ${m.emoji ?? ''}`.trim()} />;
	return (
		<>
			<button type="button" className="c-sticker c-sticker-btn" onClick={() => setOpen(true)} aria-label={`sticker ${m.emoji ?? ''}`.trim()}>
				<img src={m.thumb ?? m.url} alt={m.emoji ?? ''} loading="lazy" />
			</button>
			{open && <Lightbox m={m} kind="picture" onClose={() => setOpen(false)} />}
		</>
	);
}

// A voice note: a play button over its waveform; the bars fill as it
// plays. Nothing is fetched before the first play.
export function Voice({ m }: { m: Media }) {
	const audio = useRef<HTMLAudioElement>(null);
	const [playing, setPlaying] = useState(false);
	const [at, setAt] = useState(0);
	const total = m.duration ?? 0;
	const wave = m.wave ?? [];
	const done = total > 0 ? at / total : 0;
	const toggle = () => {
		const a = audio.current;
		if (!a) return;
		if (a.paused) void a.play().catch(() => setPlaying(false));
		else a.pause();
	};
	return (
		<div className="c-voice">
			<button type="button" className="c-play" aria-label={playing ? 'Pause' : 'Play'} disabled={!m.url} onClick={toggle}>
				{playing ? <Pause size={16} fill="currentColor" /> : m.url ? <Play size={16} fill="currentColor" /> : <CloudOff size={16} />}
			</button>
			<div>
				<div className="c-wave" aria-hidden="true">{wave.map((w, i) => <span key={i} className={(i + 0.5) / wave.length <= done ? 'on' : ''} style={{ height: 4 + (w / 31) * 18 }} />)}</div>
				<span className="c-sub">{playing || at > 0 ? `${duration(Math.floor(at))} / ` : ''}{duration(total)}{!m.stored && (m.url ? ' . from Telegram' : ' . not stored')}</span>
			</div>
			{m.url && (
				<audio ref={audio} src={m.url} preload="none"
					onPlay={() => setPlaying(true)} onPause={() => setPlaying(false)}
					onEnded={() => { setPlaying(false); setAt(0); }}
					onTimeUpdate={(e) => setAt(e.currentTarget.currentTime)} />
			)}
		</div>
	);
}

// A document, or an audio file (with a player): a link with its name
// and size; an unstored one that may be fetched downloads from Telegram.
export function DocFile({ m, audio = false }: { m: Media; audio?: boolean }) {
	// Music shows as its performer and title when the file says them.
	const tag = audio && (m.title || m.performer) ? [m.performer, m.title].filter(Boolean).join(' - ') : '';
	const label = tag || m.name;
	const sub = <span className="c-sub">{bytes(m.size ?? 0)}{audio && m.duration ? ` . ${duration(m.duration)}` : ''}{!m.stored && (m.url ? ' . load from Telegram' : ' . not stored')}</span>;
	return (
		<div className={`c-docwrap ${m.deleted_at ? 'deleted' : ''}`}>
			{m.url
				? <a className="c-doc" href={m.url} download={m.name || undefined}>
					<span className="c-docicon">{m.stored ? <Download size={18} /> : <CloudDownload size={18} />}</span>
					<span className="c-docmeta"><b>{label}</b>{tag && m.name ? <span className="c-sub">{m.name}</span> : null}{sub}</span>
				</a>
				: <div className="c-doc" title="Not stored, only metadata kept">
					<span className="c-docicon off"><CloudOff size={18} /></span>
					<span className="c-docmeta"><b>{label}</b>{tag && m.name ? <span className="c-sub">{m.name}</span> : null}{sub}</span>
				</div>}
			{audio && m.url && <audio className="c-audio" src={m.url} controls preload="none" aria-label={label ? `Play ${label}` : 'Play'} />}
			<DeletedMark m={m} />
		</div>
	);
}

// One media object as its kind.
export function MediaOf({ m, kind, single, onClick }: { m: Media; kind: MediaKind; single?: boolean; onClick: boolean }) {
	switch (kind) {
		case 'photo': return <Picture m={m} single={single} onClick={onClick} />;
		case 'video': return <VideoBox m={m} single={single} onClick={onClick} />;
		case 'animation': return single ? <Animation m={m} onClick={onClick} /> : <VideoBox m={m} animation onClick={onClick} />;
		case 'sticker': return <Sticker m={m} />;
		case 'voice': return <Voice m={m} />;
		case 'audio': return <DocFile m={m} audio />;
		default: return <DocFile m={m} />;
	}
}

const KINDS: readonly string[] = ['photo', 'video', 'animation', 'sticker', 'voice', 'audio', 'document'];
const tile = (k: MediaKind) => k === 'photo' || k === 'video' || k === 'animation';

// How a media object draws: by the content type, which keeps stickers
// and voice notes their own look, corrected by Go's render where it
// says more (a "document" that is really a picture or a video).
export function kindOf(ct: string, m: Media): MediaKind | null {
	const k = KINDS.includes(ct) ? (ct as MediaKind) : null;
	if (k === 'sticker' || k === 'voice') return k;
	switch (m.render) {
		case 'image': return 'photo';
		case 'video': return 'video';
		case 'animation': return 'animation';
		case 'lottie': return 'sticker';
		case 'audio': return 'audio';
		case 'file': return 'document';
		default: return k;
	}
}

// A message's media and album: one item as itself; an album of photos
// and videos as a grid, any other items (documents, audio) one below
// the other. Each item has its own kind and deletion.
export function MessageMedia({ msg, onClick }: { msg: ChatMessage; onClick: boolean }) {
	const ct = msg.content_type;
	const base = msg.media ? kindOf(ct, msg.media) : null;
	if (!msg.media || !base) return null;
	const items = [msg.media, ...msg.album].map((m) => ({ m, kind: (m.kind ? kindOf(m.kind, m) : null) ?? base }));
	if (items.length === 1) return <MediaOf m={msg.media} kind={base} single onClick={onClick} />;
	const grid = items.filter((x) => tile(x.kind));
	const rest = items.filter((x) => !tile(x.kind));
	return (
		<div className="c-albumwrap" aria-label={`album . ${items.length} items`}>
			{grid.length > 0 && (
				<div className={`c-album n${Math.min(4, grid.length)}`}>
					{grid.map((x, i) => <MediaOf key={i} m={x.m} kind={x.kind} onClick={onClick} />)}
				</div>
			)}
			{rest.map((x, i) => <MediaOf key={i} m={x.m} kind={x.kind} onClick={onClick} />)}
		</div>
	);
}
