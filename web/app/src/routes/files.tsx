import { useCallback, useEffect, useRef, useState } from 'react';
import { LayoutGrid, List } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, setQuery, useNav, useQueryParams } from '../nav.tsx';
import { Btn, PageHead, Pill } from '../ui/primitives.tsx';
import { DataTable, emptyText, FilterBar, useListNav, type Col } from '../ui/list.tsx';
import { Thumb, TypeIcon } from '../ui/media.tsx';
import { FileCard, FileNameCell, NO_MESSAGE, Preview } from '../ui/filegrid.tsx';
import { ROLE_LABELS, ROLE_VALUES, RoleTag, roleLabel, roleOf } from '../ui/fileroles.tsx';
import { ago, bytes, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { Cond, FieldDef, FileRow, ListProps } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { fetchPage } from '../api/page.ts';
import { track } from '../ui/busy.ts';

export type FilesProps = S['FilesProps'];

const FIELDS: FieldDef[] = [
	{ key: 'id', label: 'File ID', type: 'number' },
	{ key: 'file_type', label: 'Type', type: 'enum', values: ['photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation', 'video_note', 'thumbnail', 'unknown'] },
	{ key: 'role', label: 'Role in its message', type: 'enum', values: ROLE_VALUES, labels: ROLE_LABELS, ops: ['=', '!=', 'in'] },
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
	['Voice', [{ field: 'file_type', op: '=', value: 'voice' }]],
	['Audio', [{ field: 'file_type', op: '=', value: 'audio' }]],
	['Stickers', [{ field: 'file_type', op: '=', value: 'sticker' }]],
	['Animations', [{ field: 'file_type', op: '=', value: 'animation' }]],
	['Video notes', [{ field: 'file_type', op: '=', value: 'video_note' }]],
	['Link previews', [{ field: 'role', op: '=', value: 'link_preview' }]],
	['Not stored', [{ field: 'stored', op: 'is', value: 'no' }]],
];

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
		{ key: 'name', label: 'File', sort: true, cls: 'c-name', render: (f) => <FileNameCell f={f} sub={<><RoleTag role={roleOf(f)} />{f.chat?.title ?? NO_MESSAGE}</>} onOpen={() => show(f)} /> },
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
					<div className="a-filegrid">{g.rows.map((f) => <FileCard key={f.id} f={f} at={f.created_at} v={v} onOpen={() => show(f)} tag={<RoleTag role={roleOf(f)} />} />)}</div>
					{!L.rows.length && <div className="a-empty">{emptyText(L, member, 'files', 'No files match these filters.')}</div>}
					<GridFoot g={g} auto={v.chat?.auto_load !== false} shown={g.rows.length} />
				</div>
			) : (
				<DataTable L={L} cols={cols} noun="files" empty={emptyText(L, member, 'files', 'No files match these filters.')}
					href={(f) => setQuery(page.url, { file: f.id })} open={show}
					mobile={(f) => ({ lead: <Thumb type={f.file_type} thumb={f.thumb} url={f.url} minithumb={f.minithumb} stored={f.stored} ext={f.ext} size={40} />, title: f.name, sub: `${f.file_type}${roleOf(f) ? ` (${roleLabel(roleOf(f)!.role)})` : ''} . ${bytes(f.size)} . ${f.chat?.title ?? NO_MESSAGE}`, side: <span className="a-muted sm">{ago(f.created_at, v)}</span> })} />
			)}
			{preview && <Preview f={preview} v={v} onClose={() => show(null)} />}
		</Shell>
	);
}

export const filesRoute: RouteDef<FilesProps> = { Page: Files, head: () => ({ title: 'Files - tgloggerd', robots: 'noindex' }) };
