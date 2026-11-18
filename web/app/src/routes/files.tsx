import { CloudDownload, CloudOff, Copy, Download, LayoutGrid, List, MessageSquare } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link, setQuery, useNav, useQueryParams } from '../nav.tsx';
import { Btn, KV, PageHead, Pill } from '../ui/primitives.tsx';
import { DataTable, FilterBar, Pager, useListNav, type Col } from '../ui/list.tsx';
import { Modal } from '../ui/Modal.tsx';
import { Thumb, TypeIcon, duration } from '../ui/media.tsx';
import { ago, bytes, dateTime, fmt } from '../format.ts';
import type { PageData, Viewer } from '../page.ts';
import type { Cond, FieldDef, FileRow, ListProps } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type FilesProps = S['FilesProps'];

const FIELDS: FieldDef[] = [
	{ key: 'id', label: 'File ID', type: 'number' },
	{ key: 'file_type', label: 'Type', type: 'enum', values: ['photo', 'video', 'document', 'audio', 'voice', 'sticker', 'animation'] },
	{ key: 'name', label: 'Name', type: 'text' },
	{ key: 'ext', label: 'Extension', type: 'text' },
	{ key: 'size', label: 'Size (bytes)', type: 'number' },
	{ key: 'hits', label: 'Hits', type: 'number' },
	{ key: 'stored', label: 'Stored', type: 'bool' },
	{ key: 'created_at', label: 'First seen', type: 'date' },
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

function Preview({ f, canFetch, v, onClose }: { f: FileRow; canFetch: boolean; v: Viewer; onClose: () => void }) {
	const big = f.file_type === 'photo' || f.file_type === 'video' || f.file_type === 'animation' || f.file_type === 'sticker';
	return (
		<Modal wide title={f.name} icon={<TypeIcon type={f.file_type} />} onClose={onClose}
			footer={<>
				{f.message
					? <Btn to={`/messages/${f.message.scope}/${f.message.id}`} ghost icon={MessageSquare}>Open message</Btn>
					: <Btn ghost icon={MessageSquare} disabled title={`Sent in ${NO_MESSAGE}`}>Open message</Btn>}
				<span className="a-grow" />
				{f.stored && f.url ? <a className="a-btn a-btn-primary" href={f.url} download><Download size={14} />Download</a>
					: canFetch && <Btn primary icon={CloudDownload}>Fetch from Telegram</Btn>}
			</>}>
			<div className="a-previewgrid">
				<div className={`a-preview ${big ? 'big' : ''}`}>
					{f.stored ? <Thumb type={f.file_type} thumb={f.thumb} stored ext={f.ext} size={big ? 280 : 96} radius={big ? 12 : 16} />
						: <div className="a-preview-missing"><CloudOff size={28} /><span>{canFetch ? 'Not stored. Telegram may still have it; fetching downloads it into the archive.' : 'Not stored in the archive.'}</span></div>}
				</div>
				<KV rows={[
					[null, 'Type', f.file_type],
					[null, 'Size', `${bytes(f.size)} (${fmt(f.size)} bytes)`],
					f.duration ? [null, 'Duration', duration(f.duration)] : null,
					[null, 'Hits', fmt(f.hits)],
					[null, 'Stored', f.stored ? 'yes' : 'no'],
					[null, 'Sent in', f.chat ? <Link to={chatHref(f.chat)} className="a-rowlink">{f.chat.title}</Link> : <span className="a-muted">{NO_MESSAGE}</span>],
					[null, 'Sender', f.sender ? <Link to={`/users/${f.sender.id}`} className="a-rowlink">{f.sender.name}</Link> : ''],
					[null, 'File ID', f.id, true],
					[null, 'Telegram ID', <span className="a-break">{f.tg_file_id} <button type="button" className="a-icon-btn sm" aria-label="Copy" onClick={() => void navigator.clipboard?.writeText(f.tg_file_id)}><Copy size={12} /></button></span>, true],
					[null, 'First seen', dateTime(f.created_at, v)],
				]} />
			</div>
		</Modal>
	);
}

function Files({ page }: { page: PageData<FilesProps> }) {
	const L = useListNav(page.props);
	const { navigate } = useNav();
	const q = useQueryParams();
	const v = page.viewer;
	const grid = q.get('view') !== 'list';
	const show = (f: FileRow | null) => navigate(setQuery(page.url, { file: f ? f.id : undefined }), { replace: true, scroll: false });
	const member = v.account?.role === 'member';
	const cols: Col<FileRow>[] = [
		{ key: 'name', label: 'File', sort: true, cls: 'c-name', render: (f) => (
			<button type="button" className="a-usercell a-plain" onClick={() => show(f)}>
				<Thumb type={f.file_type} thumb={f.thumb} stored={f.stored} size={30} radius={6} />
				<span><span className="a-uname a-clip">{f.name}</span><span className="a-handle">{f.chat?.title ?? NO_MESSAGE}</span></span>
			</button>
		) },
		{ key: 'file_type', label: 'Type', sort: true, render: (f) => <span className="a-typecell"><TypeIcon type={f.file_type} />{f.file_type}</span> },
		{ key: 'size', label: 'Size', sort: true, align: 'right', render: (f) => <span className="num">{bytes(f.size)}</span> },
		{ key: 'hits', label: 'Hits', sort: true, align: 'right', cls: 'a-hide-md', render: (f) => <span className="num">{fmt(f.hits)}</span> },
		{ key: 'stored', label: 'Stored', render: (f) => (f.stored ? <Pill tone="ok">stored</Pill> : <Pill tone="danger">missing</Pill>) },
		{ key: 'sender', label: 'Sender', cls: 'a-hide-md', render: (f) => (f.sender ? <Link to={`/users/${f.sender.id}`} className="a-rowlink">{f.sender.name}</Link> : null) },
		{ key: 'created_at', label: 'First seen', sort: true, render: (f) => <span title={f.created_at}>{ago(f.created_at, v)}</span> },
	];
	const toggle = (
		<div className="a-seg a-seg-icons" role="group" aria-label="View">
			<button type="button" className={grid ? 'on' : ''} onClick={() => navigate(setQuery(page.url, { view: undefined }), { replace: true, scroll: false })} aria-label="Grid view"><LayoutGrid size={15} /></button>
			<button type="button" className={!grid ? 'on' : ''} onClick={() => navigate(setQuery(page.url, { view: 'list' }), { replace: true, scroll: false })} aria-label="List view"><List size={15} /></button>
		</div>
	);
	return (
		<Shell crumbs={[['Files', null]]}>
			<PageHead title="Files" sub={member ? 'Files shared in your groups' : `${fmt(page.props.archive_total ?? L.total)} files${page.props.stored_bytes ? `, ${bytes(page.props.stored_bytes)} stored on disk` : ''}`} actions={toggle} />
			<FilterBar L={L} fields={FIELDS} placeholder="File name, chat or ID" name="Files" quick={QUICK} />
			{grid ? (
				<div className="a-card a-tablecard">
					<div className="a-filegrid">{L.rows.map((f) => (
						<button type="button" key={f.id} className="a-filecard" onClick={() => show(f)}>
							<span className="a-filecard-media"><Thumb type={f.file_type} thumb={f.thumb} stored={f.stored} ext={f.ext} size={150} radius={0} fill />
								{f.duration ? <span className="a-dur">{duration(f.duration)}</span> : null}
							</span>
							<span className="a-filecard-meta"><span className="a-ellipsis a-uname">{f.name}</span><span className="a-muted sm">{bytes(f.size)} . {ago(f.created_at, v)}</span></span>
						</button>
					))}</div>
					{!L.rows.length && <div className="a-empty">No files match these filters.</div>}
					<Pager L={L} noun="files" />
				</div>
			) : (
				<DataTable L={L} cols={cols} noun="files" empty="No files match these filters."
					href={(f) => setQuery(page.url, { file: f.id })}
					mobile={(f) => ({ lead: <Thumb type={f.file_type} thumb={f.thumb} stored={f.stored} size={40} />, title: f.name, sub: `${f.file_type} . ${bytes(f.size)} . ${f.chat?.title ?? NO_MESSAGE}`, side: <span className="a-muted sm">{ago(f.created_at, v)}</span> })} />
			)}
			{page.props.preview && <Preview f={page.props.preview} canFetch={page.props.can_fetch} v={v} onClose={() => show(null)} />}
		</Shell>
	);
}

export const filesRoute: RouteDef<FilesProps> = { Page: Files, head: () => ({ title: 'Files - tgloggerd', robots: 'noindex' }) };
