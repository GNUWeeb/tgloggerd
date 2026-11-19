// A file as the files list draws it: the grid's card, the list's name
// cell and the preview dialog. The files page (/files) and a group's
// Files tab share them.
import { useCallback, useState, type ReactNode } from 'react';
import { CloudDownload, CloudOff, Copy, Download, MessageSquare, Play } from 'lucide-react';
import { Link } from '../nav.tsx';
import { Btn, KV } from './primitives.tsx';
import { Modal } from './Modal.tsx';
import { Thumb, TypeIcon, drawableThumb, duration, isVideoType } from './media.tsx';
import { ago, bytes, dateTime, fmt } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { FileRow } from '../types.ts';
import { Lightbox, MediaOf, type Media, type MediaKind } from '../routes/chat-media.tsx';

const noop = () => {};

// A file the archive saw only in messages the viewer cannot see has no
// chat and no message (both null); it is shown, without links.
export const NO_MESSAGE = 'no visible message';
const chatHref = (c: NonNullable<FileRow['chat']>) => (c.kind === 'group' ? `/groups/${c.id}` : `/users/${c.id}`);

// What the card, the name cell and the preview draw of a file: a
// FileRow, or a group's file (api/groupfiles.ts), which has these too.
type ShownFile = Pick<FileRow, 'name' | 'ext' | 'size' | 'file_type' | 'stored' | 'fetchable' | 'url' | 'thumb' | 'minithumb' | 'duration'>;

// The grid's card: the file's picture or icon in a fixed box, its name,
// size and a date (the files page: when it was first seen; a group's
// files: when it was sent).
export function FileCard({ f, at, v, onOpen }: { f: ShownFile; at: string; v: Viewer; onOpen: () => void }) {
	return (
		<button type="button" className="a-filecard" onClick={onOpen}>
			<span className="a-filecard-media"><Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={150} radius={0} fill />
				{f.duration ? <span className="a-dur">{duration(f.duration)}</span> : null}
			</span>
			<span className="a-filecard-meta"><span className="a-ellipsis a-uname">{f.name}</span><span className="a-muted sm">{bytes(f.size)} . {ago(at, v)}</span></span>
		</button>
	);
}

// The list's name cell: a small thumbnail, the name and a line under it
// (the files page: the chat).
export function FileNameCell({ f, sub, onOpen }: { f: ShownFile; sub: ReactNode; onOpen: () => void }) {
	return (
		<button type="button" className="a-usercell a-plain" onClick={onOpen}>
			<Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={30} radius={6} />
			<span><span className="a-uname a-clip">{f.name}</span><span className="a-handle">{sub}</span></span>
		</button>
	);
}

// The file's picture (a photo, or a video's poster) whole, at its own
// shape: never cropped, within the pane and most of the window's height.
// A stored picture opens bigger, in the chat's viewer, on a click.
function PreviewImage({ f, src, onZoom }: { f: ShownFile; src: string; onZoom: (() => void) | null }) {
	const video = isVideoType(f.file_type);
	const img = <img className="a-preview-img" src={src} alt="" />;
	if (onZoom) return <button type="button" className="a-preview-media a-preview-zoom" onClick={onZoom} aria-label="View bigger">{img}</button>;
	return (
		<span className="a-preview-media">
			{img}
			{video && <span className="a-thumb-play"><Play size={36} fill="currentColor" /></span>}
		</span>
	);
}

// How the chat plays a file; null for a picture, a .webp sticker or a
// document, which the preview shows as a picture or offers to download.
function playKind(f: ShownFile): MediaKind | null {
	switch (f.file_type) {
		case 'video': case 'video_note': return 'video';
		case 'animation': return 'animation';
		case 'audio': return 'audio';
		case 'voice': return 'voice';
		case 'sticker': return /^(webm|tgs)$/i.test(f.ext) ? 'sticker' : null;
		default: return null;
	}
}

// A file row has no waveform: an even one makes the voice player's bars
// a plain progress bar.
const EVEN_WAVE: number[] = Array(48).fill(14);

// The file as the chat's media: its URL, its still and blurred preview
// for a video (as the grid draws them), and what the players need.
function mediaOf(f: ShownFile): Media {
	const ext = f.ext.toLowerCase();
	const sticker = f.file_type === 'sticker';
	const poster = (isVideoType(f.file_type) || ext === 'webm') ? drawableThumb(f.thumb, { url: f.url, ext: f.ext, type: f.file_type }) ?? undefined : undefined;
	return {
		stored: f.stored, fetchable: f.fetchable, url: f.url, name: f.name, size: f.size, duration: f.duration, ext,
		thumb: poster ?? f.minithumb ?? null, poster, minithumb: f.minithumb,
		...(sticker ? { format: ext === 'tgs' ? 'tgs' : 'webm' } as const : {}),
		...(f.file_type === 'video_note' ? { round: true } : {}),
		...(f.file_type === 'voice' ? { wave: EVEN_WAVE } : {}),
	};
}

type KVRows = Parameters<typeof KV>[0]['rows'];

// The preview dialog: the file playing, its picture (or icon, or why it
// is missing), its details beside it, and in the footer the caller's
// links with the download.
export function FilePreview({ f, v, details, links, onClose }: { f: ShownFile; v: Viewer; details: KVRows; links: ReactNode; onClose: () => void }) {
	const big = f.file_type === 'photo' || isVideoType(f.file_type) || f.file_type === 'sticker';
	const [zoom, setZoom] = useState(false);
	const pic = drawableThumb(f.thumb, { url: f.url, ext: f.ext, type: f.file_type });
	// The bigger view shows the file itself: only a picture's. A video
	// sticker's still is not the sticker.
	const zoomable = (f.file_type === 'photo' || f.file_type === 'sticker') && !!pic && !!f.url && !/^(webm|tgs)$/i.test(f.ext);
	const unzoom = useCallback(() => setZoom(false), []);
	// A file the chat can play plays here with the chat's own players,
	// when its bytes are stored or may be fetched; "media on click" is
	// kept as in the chat.
	const play = f.url ? playKind(f) : null;
	return (
		// While the bigger view is open, Esc and clicks belong to it.
		<Modal wide title={f.name} icon={<TypeIcon type={f.file_type} />} onClose={zoom ? noop : onClose}
			footer={<>
				{links}
				<span className="a-grow" />
				{f.url && (f.stored
					? <a className="a-btn a-btn-primary" href={f.url} download><Download size={14} />Download</a>
					// Opening an unstored file's URL fetches it from Telegram.
					: <a className="a-btn a-btn-primary" href={f.url} download><CloudDownload size={14} />Fetch from Telegram</a>)}
			</>}>
			<div className="a-previewgrid">
				<div className={`a-preview ${big ? 'big' : ''}`}>
					{/* A video's still or blurred preview shows even when the
					    video itself is not stored. */}
					{play ? <div className="a-preview-play"><MediaOf m={mediaOf(f)} kind={play} single onClick={v.chat?.media === 'click'} /></div>
						: pic ? <PreviewImage f={f} src={pic} onZoom={zoomable && f.stored ? () => setZoom(true) : null} />
						: f.stored || f.minithumb ? <Thumb type={f.file_type} thumb={null} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={big ? 280 : 96} radius={big ? 12 : 16} />
						: null}
					{!f.stored && <div className="a-preview-missing"><CloudOff size={28} /><span>{f.url ? 'Not stored. Fetch from Telegram downloads it from there, if Telegram still has it.' : 'Not stored in the archive; only its details were kept.'}</span></div>}
				</div>
				<KV rows={details} />
			</div>
			{zoom && <Lightbox m={f} kind="picture" onClose={unzoom} />}
		</Modal>
	);
}

// A file of the files page, with all the archive has of it.
export function Preview({ f, v, onClose }: { f: FileRow; v: Viewer; onClose: () => void }) {
	return (
		<FilePreview f={f} v={v} onClose={onClose}
			links={f.message
				? <Btn to={`/messages/${f.message.scope}/${f.message.id}`} ghost icon={MessageSquare}>Open message</Btn>
				: <Btn ghost icon={MessageSquare} disabled title={`Sent in ${NO_MESSAGE}`}>Open message</Btn>}
			details={[
				[null, 'Type', f.file_type],
				[null, 'Size', `${bytes(f.size)} (${fmt(f.size)} bytes)`],
				f.duration ? [null, 'Duration', duration(f.duration)] : null,
				[null, 'Hits', fmt(f.hits)],
				[null, 'Stored', f.stored ? 'yes' : f.url ? 'no, can be fetched from Telegram' : 'no'],
				[null, 'Sent in', f.chat ? <Link to={chatHref(f.chat)} className="a-rowlink">{f.chat.title}</Link> : <span className="a-muted">{NO_MESSAGE}</span>],
				[null, 'Sender', f.sender ? <Link to={`/users/${f.sender.id}`} className="a-rowlink">{f.sender.name}</Link> : ''],
				[null, 'File ID', f.id, true],
				[null, 'Telegram ID', <span className="a-break">{f.tg_file_id} <button type="button" className="a-icon-btn sm" aria-label="Copy Telegram ID" onClick={() => void navigator.clipboard?.writeText(f.tg_file_id)}><Copy size={12} /></button></span>, true],
				[null, 'SHA-256', f.sha256 ? <span className="a-break mono">{f.sha256} <button type="button" className="a-icon-btn sm" aria-label="Copy SHA-256" onClick={() => void navigator.clipboard?.writeText(f.sha256!)}><Copy size={12} /></button></span> : <span className="a-muted">not recorded</span>, !!f.sha256],
				[null, 'First seen', dateTime(f.created_at, v)],
				f.accessed_at ? [null, 'Last access', dateTime(f.accessed_at, v)] : null,
			]} />
	);
}
