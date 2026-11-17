import { Download, LayoutGrid, List, MessageSquare, CloudOff, CloudDownload, Copy } from 'lucide-react';
import { Modal } from '../ui/forms.jsx';
import { Link, useQuery } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { useList } from '../ui/hooks.js';
import { Pill, PageHead, Btn, KV } from '../ui/primitives.jsx';
import { DataTable, FilterBar, Pager } from '../ui/DataTable.jsx';
import { Thumb, TypeIcon, duration } from '../ui/Media.jsx';
import { queryFiles, FILE_FIELDS, files } from '../data/files.js';
import { TOTALS } from '../data/totals.js';
import { fmt, bytes, ago } from '../data/seed.js';

const QUICK = [
	['Photos', [{ field: 'file_type', op: '=', value: 'photo' }]],
	['Videos', [{ field: 'file_type', op: '=', value: 'video' }]],
	['Documents', [{ field: 'file_type', op: '=', value: 'document' }]],
	['Not stored', [{ field: 'stored', op: 'is', value: 'no' }]],
];

function msgHref(f) {
	return `/messages/${f.msg.scope}/${f.msg.id}`;
}

function Preview({ f, onClose }) {
	const big = f.file_type === 'photo' || f.file_type === 'video' || f.file_type === 'animation' || f.file_type === 'sticker';
	return (
		<Modal wide title={f.name} icon={<TypeIcon type={f.file_type} />} onClose={onClose}
			footer={<><Btn to={msgHref(f)} ghost icon={MessageSquare}>Open message</Btn><span className="a-grow" />
				{f.stored ? <Btn primary icon={Download}>Download</Btn> : <Btn primary icon={CloudDownload} title="Needs the fetch-from-Telegram permission">Fetch from Telegram</Btn>}</>}>
			<div className="a-previewgrid">
				<div className={`a-preview ${big ? 'big' : ''}`}>
					{f.stored ? <Thumb f={f} size={big ? 280 : 96} radius={big ? 12 : 16} />
						: <div className="a-preview-missing"><CloudOff size={28} /><span>Not stored. Telegram may still have it; fetching downloads it into the archive.</span></div>}
				</div>
				<KV rows={[
					[null, 'Type', f.file_type],
					[null, 'Size', `${bytes(f.size)} (${fmt(f.size)} bytes)`],
					f.duration && [null, 'Duration', duration(f.duration)],
					[null, 'Hits', fmt(f.hits)],
					[null, 'Stored', f.stored ? 'yes' : 'no'],
					[null, 'Sent in', <Link to={f.msg.scope === 'group' ? `/groups/${f.msg.chat.id}` : `/users/${f.msg.chat.id}`} className="a-rowlink">{f.chat_title}</Link>],
					[null, 'Sender', f.sender_id ? <Link to={`/users/${f.sender_id}`} className="a-rowlink">{f.sender_name}</Link> : f.sender_name],
					[null, 'File ID', f.id, true],
					[null, 'Telegram ID', <span className="a-break">{f.tg_file_id} <button type="button" className="a-icon-btn sm" aria-label="Copy"><Copy size={12} /></button></span>, true],
					[null, 'First seen', f.created_at, true],
				]} />
			</div>
		</Modal>
	);
}

export default function Files() {
	const L = useList(queryFiles, { sort: 'created_at', per: 50 });
	const [p, set] = useQuery();
	const grid = p.view !== 'list';
	const open = p.file ? files().find((f) => String(f.id) === p.file) : null;
	const show = (f) => set({ file: f ? String(f.id) : undefined }, { replace: true });
	const stored = files().filter((f) => f.stored).reduce((a, f) => a + f.size, 0);
	const cols = [
		{ key: 'name', label: 'File', sort: true, cls: 'c-name', render: (f) => (
			<button type="button" className="a-usercell a-plain" onClick={() => show(f)}>
				<Thumb f={f} size={30} radius={6} />
				<span><span className="a-uname a-clip">{f.name}</span><span className="a-handle">{f.chat_title}</span></span>
			</button>
		) },
		{ key: 'file_type', label: 'Type', sort: true, render: (f) => <span className="a-typecell"><TypeIcon type={f.file_type} />{f.file_type}</span> },
		{ key: 'size', label: 'Size', sort: true, align: 'right', render: (f) => <span className="num">{bytes(f.size)}</span> },
		{ key: 'hits', label: 'Hits', sort: true, align: 'right', cls: 'a-hide-md', render: (f) => <span className="num">{fmt(f.hits)}</span> },
		{ key: 'stored', label: 'Stored', render: (f) => (f.stored ? <Pill tone="ok">stored</Pill> : <Pill tone="danger">missing</Pill>) },
		{ key: 'sender_name', label: 'Sender', sort: true, cls: 'a-hide-md', render: (f) => (f.sender_id ? <Link to={`/users/${f.sender_id}`} className="a-rowlink">{f.sender_name}</Link> : f.sender_name) },
		{ key: 'created_at', label: 'First seen', sort: true, render: (f) => <span title={f.created_at}>{ago(f.created_at)}</span> },
	];
	const toggle = (
		<div className="a-seg a-seg-icons" role="group" aria-label="View">
			<button type="button" className={grid ? 'on' : ''} onClick={() => set({ view: undefined })} aria-label="Grid view" title="Grid"><LayoutGrid size={15} /></button>
			<button type="button" className={!grid ? 'on' : ''} onClick={() => set({ view: 'list' })} aria-label="List view" title="List"><List size={15} /></button>
		</div>
	);
	return (
		<Shell crumbs={[['Files', null]]}>
			<PageHead title="Files" sub={`${fmt(TOTALS.files)} files, ${bytes(stored * 1900)} stored on disk`} />
			<FilterBar L={L} fields={FILE_FIELDS} placeholder="File name, chat or ID" quick={QUICK} right={toggle} />
			{grid ? (
				<div className="a-card a-tablecard">
					<div className="a-filegrid">
						{L.rows.map((f) => (
							<button type="button" key={f.id} className="a-filecard" onClick={() => show(f)}>
								<span className="a-filecard-media"><Thumb f={f} size={150} radius={0} className="a-fill" />
									{f.duration && <span className="a-dur">{duration(f.duration)}</span>}
								</span>
								<span className="a-filecard-meta">
									<span className="a-ellipsis a-uname">{f.name}</span>
									<span className="a-muted sm">{bytes(f.size)} . {ago(f.created_at)}</span>
								</span>
							</button>
						))}
					</div>
					{!L.rows.length && <div className="a-empty">No files match these filters.</div>}
					<Pager L={L} noun="files" />
				</div>
			) : (
				<DataTable
					L={L} cols={cols} noun="files" onRow={show}
					empty="No files match these filters."
					mobile={(f) => ({
						lead: <Thumb f={f} size={40} />,
						title: f.name,
						sub: `${f.file_type} . ${bytes(f.size)} . ${f.chat_title}`,
						side: <span className="a-muted sm">{ago(f.created_at)}</span>,
					})}
				/>
			)}
			{open && <Preview f={open} onClose={() => show(null)} />}
		</Shell>
	);
}
