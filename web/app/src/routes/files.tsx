import { useCallback, useEffect, useRef, useState } from 'react';
import { CloudDownload, CloudOff, Copy, Download, LayoutGrid, List, MessageSquare, Play } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, setQuery, useNav, useQueryParams } from '../nav.tsx';
import { Btn, KV, PageHead, Pill } from '../ui/primitives.tsx';
import { DataTable, emptyText, FilterBar, useListNav, type Col } from '../ui/list.tsx';
import { Modal } from '../ui/Modal.tsx';
import { Thumb, TypeIcon, drawableThumb, duration, isVideoType } from '../ui/media.tsx';
import { ago, bytes, dateTime, fmt } from '../format.ts';
import type { PageData, Viewer } from '../page.ts';
import type { Cond, FieldDef, FileRow, ListProps } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { Lightbox, MediaOf, type Media, type MediaKind } from './chat-media.tsx';
import { fetchPage } from '../api/page.ts';
import { track } from '../ui/busy.ts';

const noop = () => {};

export type FilesProps = S['FilesProps'];

const FIELDS: FieldDef[] = [
	{ key: 'id', label: 'File ID', type: 'number' },
	{ key: 'file_type', label: 'Type', type: 'enum', values: ['photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation', 'unknown'] },
	{ key: 'name', label: 'Name', type: 'text' },
	{ key: 'ext', label: 'Extension', type: 'text' },
	{ key: 'tg_file_id', label: 'Telegram file ID', type: 'text' },
	{ key: 'sha256', label: 'SHA-256', type: 'text', ops: ['=', '!='] },
	{ key: 'size', label: 'Size (bytes)', type: 'number' },
	{ key: 'hits', label: 'Hits', type: 'number' },
	{ key: 'stored', label: 'Stored', type: 'bool' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
	{ key: 'updated_at', label: 'Updated', type: 'date' },
	{ key: 'accessed_at', label: 'Last access', type: 'date' },
];
const QUICK: [string, Cond[]][] = [
	['Photos', [{ field: 'file_type', op: '=', value: 'photo' }]],
	['Videos', [{ field: 'file_type', op: '=', value: 'video' }]],
	['Documents', [{ field: 'file_type', op: '=', value: 'document' }]],
	['Not stored', [{ field: 'stored', op: 'is', value: 'no' }]],
];

// A file the archive saw only in messages the viewer cannot see has no
// chat and no message (both null); it is shown, without links.
const NO_MESSAGE = 'no visible message';
const chatHref = (c: NonNullable<FileRow['chat']>) => (c.kind === 'group' ? `/groups/${c.id}` : `/users/${c.id}`);

// The file's picture (a photo, or a video's poster) whole, at its own
// shape: never cropped, within the pane and most of the window's height.
// A stored picture opens bigger, in the chat's viewer, on a click.
function PreviewImage({ f, src, onZoom }: { f: FileRow; src: string; onZoom: (() => void) | null }) {
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
function playKind(f: FileRow): MediaKind | null {
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
function mediaOf(f: FileRow): Media {
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

function Preview({ f, v, onClose }: { f: FileRow; v: Viewer; onClose: () => void }) {
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
				{f.message
					? <Btn to={`/messages/${f.message.scope}/${f.message.id}`} ghost icon={MessageSquare}>Open message</Btn>
					: <Btn ghost icon={MessageSquare} disabled title={`Sent in ${NO_MESSAGE}`}>Open message</Btn>}
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
				<KV rows={[
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
			</div>
			{zoom && <Lightbox m={f} kind="picture" onClose={unzoom} />}
		</Modal>
	);
}

// The grid's batch: a multiple of every column count it uses (2, 3, 4,
// 6, 8), so a batch always ends on a full row. It follows the rows per
// page setting: 25 -> 24, 50 -> 48, 100 and more -> 96.
export function gridBatch(per: number): number {
	return per <= 25 ? 24 : per <= 50 ? 48 : 96;
}

// How near the grid's end (in px below the window) the next batch loads.
const NEAR = 600;

// The grid scrolls for more instead of paging: it starts with the page
// data's first batch and appends further ones, each the page data of the
// same list at the batch's page, fetched without navigating. Batches
// belong to the list (its URL without the open file): another query,
// filter or sort starts again from the page data.
function useGridRows(L: { rows: FileRow[]; page: number; per: number; total: number; total_exact?: boolean }, url: string, enabled: boolean) {
	const key = setQuery(url, { file: undefined });
	const batch = gridBatch(L.per);
	const start = (L.page - 1) * L.per;
	const base = L.rows.slice(0, batch);
	// Fewer rows than the server's page holds: the list ends there.
	const baseDone = L.rows.length < L.per && L.rows.length <= batch;
	const fresh = { key, rows: [] as FileRow[], offset: start + base.length, done: baseDone, failed: false };
	const [st, setSt] = useState(fresh);
	const cur = st.key === key ? st : fresh;
	const [loading, setLoading] = useState(false);
	const ac = useRef<AbortController | null>(null);
	// Each new list drops the load still running for the one before.
	useEffect(() => () => { ac.current?.abort(); ac.current = null; setLoading(false); }, [key]);
	const seen = new Set<number>();
	const rows = [...base, ...cur.rows].filter((f) => (seen.has(f.id) ? false : (seen.add(f.id), true)));
	const more = !cur.done && (L.total_exact === false || cur.offset < L.total);
	const next = useCallback(async () => {
		if (ac.current || !more || !enabled) return;
		const c = new AbortController();
		ac.current = c;
		setLoading(true);
		const at = cur.offset;
		const n = Math.floor(at / batch) + 1;
		try {
			const r = await track(fetchPage(setQuery(key, { page: n > 1 ? n : undefined, per: batch }), c.signal));
			if (c.signal.aborted) return;
			if (r.kind !== 'page' || r.page.route !== 'files') throw new Error('not the files list');
			const got = (r.page.props as FilesProps).rows;
			const add = got.slice(at % batch);
			setSt({ key, rows: [...cur.rows, ...add], offset: at + add.length, done: got.length < batch || !add.length, failed: false });
		} catch {
			// The button stands in; a click tries again.
			if (!c.signal.aborted) setSt({ ...cur, failed: true });
		} finally {
			if (ac.current === c) { ac.current = null; setLoading(false); }
		}
	}, [key, batch, more, enabled, cur]);
	return { rows, more, loading, failed: cur.failed, next };
}

function GridFoot({ g, auto, shown }: { g: ReturnType<typeof useGridRows>; auto: boolean; shown: number }) {
	const foot = useRef<HTMLDivElement>(null);
	const { more, loading, failed, next } = g;
	// Load the next batch as the grid's end nears the window's bottom; on
	// a scroll, a resize, and after each batch (one too short to fill the
	// window gives no scroll). A batch that adds nothing ends the list, so
	// this cannot loop.
	useEffect(() => {
		if (!auto || !more || failed) return;
		const check = () => {
			const el = foot.current;
			if (el && el.getBoundingClientRect().top - innerHeight < NEAR) void next();
		};
		check();
		addEventListener('scroll', check, { passive: true });
		addEventListener('resize', check);
		return () => { removeEventListener('scroll', check); removeEventListener('resize', check); };
	}, [auto, more, failed, next]);
	if (!shown) return null;
	return (
		<div className="a-filegrid-foot" ref={foot}>
			{more
				? <Btn ghost disabled={loading} onClick={() => void next()}>{loading ? 'Loading more files...' : 'Load more'}</Btn>
				: <span className="a-muted" role="status">All {fmt(shown)} files shown</span>}
		</div>
	);
}

function Files({ page }: { page: PageData<FilesProps> }) {
	const L = useListNav(page.props);
	const { navigate, replaceUrl } = useNav();
	const q = useQueryParams();
	const v = page.viewer;
	const grid = q.get('view') !== 'list';
	const g = useGridRows(L, page.url, grid);
	const rows = grid ? g.rows : L.rows;
	// The preview opens and closes in place: the URL names the file, but
	// the list stays as it is, so nothing is fetched. The server's
	// preview serves a direct visit to ?file= (a link, a reload, or a
	// file not on this page).
	const fileId = Number(q.get('file')) || null;
	const server = page.props.preview;
	const preview = fileId === null ? null : server?.id === fileId ? server : rows.find((r) => r.id === fileId) ?? null;
	const show = (f: FileRow | null) => replaceUrl(setQuery(page.url, { file: f ? f.id : undefined }));
	const member = v.account?.role === 'member';
	const cols: Col<FileRow>[] = [
		{ key: 'id', label: 'ID', sort: true, align: 'right', cls: 'a-hide-md', render: (f) => <span className="num">{f.id}</span> },
		{ key: 'name', label: 'File', sort: true, cls: 'c-name', render: (f) => (
			<button type="button" className="a-usercell a-plain" onClick={() => show(f)}>
				<Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={30} radius={6} />
				<span><span className="a-uname a-clip">{f.name}</span><span className="a-handle">{f.chat?.title ?? NO_MESSAGE}</span></span>
			</button>
		) },
		{ key: 'file_type', label: 'Type', sort: true, render: (f) => <span className="a-typecell"><TypeIcon type={f.file_type} />{f.file_type}</span> },
		{ key: 'ext', label: 'Ext', sort: true, cls: 'a-hide-md', render: (f) => <span className="mono">{f.ext}</span> },
		{ key: 'size', label: 'Size', sort: true, align: 'right', render: (f) => <span className="num">{bytes(f.size)}</span> },
		{ key: 'hits', label: 'Hits', sort: true, align: 'right', cls: 'a-hide-md', render: (f) => <span className="num">{fmt(f.hits)}</span> },
		{ key: 'stored', label: 'Stored', render: (f) => (f.stored ? <Pill tone="ok">stored</Pill> : f.url ? <Pill tone="info">fetchable</Pill> : <Pill tone="danger">missing</Pill>) },
		{ key: 'sender', label: 'Sender', cls: 'a-hide-md', render: (f) => (f.sender ? <Link to={`/users/${f.sender.id}`} className="a-rowlink">{f.sender.name}</Link> : null) },
		{ key: 'created_at', label: 'First seen', sort: true, render: (f) => <span title={f.created_at}>{ago(f.created_at, v)}</span> },
		{ key: 'accessed_at', label: 'Last access', sort: true, cls: 'a-hide-md', render: (f) => (f.accessed_at ? <span title={f.accessed_at}>{ago(f.accessed_at, v)}</span> : null) },
	];
	const toggle = (
		<div className="a-seg a-seg-icons" role="group" aria-label="View">
			<button type="button" className={grid ? 'on' : ''} onClick={() => navigate(setQuery(page.url, { view: undefined, page: undefined }), { replace: true, scroll: false })} aria-label="Grid view"><LayoutGrid size={15} /></button>
			<button type="button" className={!grid ? 'on' : ''} onClick={() => navigate(setQuery(page.url, { view: 'list', page: undefined }), { replace: true, scroll: false })} aria-label="List view"><List size={15} /></button>
		</div>
	);
	return (
		<Shell crumbs={[['Files', null]]}>
			<PageHead title="Files" sub={member ? 'Files shared in your groups' : `${fmt(page.props.archive_total ?? L.total)} files${page.props.stored_bytes ? `, ${bytes(page.props.stored_bytes)} stored on disk` : ''}`} actions={toggle} />
			<FilterBar L={L} fields={FIELDS} placeholder="File name, chat or ID" name="Files" quick={QUICK} />
			{grid ? (
				// No pages in the grid: it loads more as it is scrolled.
				<div className="a-card a-tablecard a-filegrid-card">
					<div className="a-filegrid">{g.rows.map((f) => (
						<button type="button" key={f.id} className="a-filecard" onClick={() => show(f)}>
							<span className="a-filecard-media"><Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={150} radius={0} fill />
								{f.duration ? <span className="a-dur">{duration(f.duration)}</span> : null}
							</span>
							<span className="a-filecard-meta"><span className="a-ellipsis a-uname">{f.name}</span><span className="a-muted sm">{bytes(f.size)} . {ago(f.created_at, v)}</span></span>
						</button>
					))}</div>
					{!L.rows.length && <div className="a-empty">{emptyText(L, member, 'files', 'No files match these filters.')}</div>}
					<GridFoot g={g} auto={v.chat?.auto_load !== false} shown={g.rows.length} />
				</div>
			) : (
				<DataTable L={L} cols={cols} noun="files" empty={emptyText(L, member, 'files', 'No files match these filters.')}
					href={(f) => setQuery(page.url, { file: f.id })} open={show}
					mobile={(f) => ({ lead: <Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={40} />, title: f.name, sub: `${f.file_type} . ${bytes(f.size)} . ${f.chat?.title ?? NO_MESSAGE}`, side: <span className="a-muted sm">{ago(f.created_at, v)}</span> })} />
			)}
			{preview && <Preview f={preview} v={v} onClose={() => show(null)} />}
		</Shell>
	);
}

export const filesRoute: RouteDef<FilesProps> = { Page: Files, head: () => ({ title: 'Files - tgloggerd', robots: 'noindex' }) };
