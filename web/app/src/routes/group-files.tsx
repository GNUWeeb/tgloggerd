// A group's Files tab (route group_files, /groups/{id}/files?kind=):
// the files of its messages, newest message first, in sub tabs by kind,
// drawn as the files page draws them (ui/filegrid.tsx), each with its
// sender and links to its message in the chat and to its message page.
// Older files come a page at a time from GET /api/groups/{id}/files.
import { useCallback, useState } from 'react';
import { LayoutGrid, List, LoaderCircle, MessageCircle, MessageSquare, TriangleAlert, ChevronsDown } from 'lucide-react';
import { Link, setQuery, useNav, useQueryParams } from '../nav.tsx';
import { Avatar, Btn, Empty, Pill } from '../ui/primitives.tsx';
import { FileCard, FileNameCell, FilePreview } from '../ui/filegrid.tsx';
import { TypeIcon, duration } from '../ui/media.tsx';
import { useHistoryPages } from '../ui/history.tsx';
import { ago, bytes, dateTime, fmt } from '../format.ts';
import type { Viewer } from '../page.ts';
import { fetchGroupFiles, type FileKind, type GroupFileRow, type GroupFilesPage, type GroupMessageRef, type GroupSender } from '../api/groupfiles.ts';

const KIND_LABEL: Record<'all' | FileKind, string> = {
	all: 'All', photo: 'Photos', video: 'Videos', animation: 'GIF', audio: 'Audio', voice: 'Voice',
	document: 'Documents', sticker: 'Stickers', video_note: 'Video notes',
};

// Who sent it: a user's page, or for a post as a chat that chat's.
export function SentBy({ s }: { s: GroupSender | null }) {
	if (!s) return <span className="a-muted">hidden sender</span>;
	return (
		<Link to={s.id < 0 ? `/groups/${s.id}` : `/users/${s.id}`} className="a-usercell a-sentby">
			<Avatar id={s.id} name={s.name} photo={s.photo} size={20} square={s.id < 0} />
			<span className="a-uname a-ellipsis">{s.name}</span>
		</Link>
	);
}

// The two ways to the message: its place in the chat and its page.
export function MessageLinks({ r }: { r: GroupMessageRef }) {
	return (
		<span className="a-msglinks">
			<Link to={r.chat_link} className="a-link"><MessageCircle size={13} />In the chat</Link>
			<Link to={`/messages/group/${r.message_row_id}`} className="a-link"><MessageSquare size={13} />Message</Link>
		</span>
	);
}

// The end of a backwards-paged list: a button for the next page, or
// that all is shown.
export function LoadMore({ h, noun }: { h: { hasMore: boolean; busy: boolean; err: string; more: () => Promise<void>; entries: unknown[] }; noun: string }) {
	return (
		<div className="a-loadmore">
			{h.err && <span className="a-banner warn" role="alert"><TriangleAlert size={14} /><span>{h.err}</span></span>}
			{h.hasMore
				? <button type="button" className="a-btn a-btn-ghost" disabled={h.busy} onClick={() => void h.more()}>{h.busy ? <LoaderCircle size={14} className="a-spin" /> : <ChevronsDown size={14} />}{h.busy ? 'Loading...' : 'Load more'}</button>
				: h.entries.length > 0 && <span className="a-muted">All {fmt(h.entries.length)} {noun} shown</span>}
		</div>
	);
}

// A cursor as the paging hook keeps it (a string, null at the end).
export const cursorOf = (n: number | null) => (n === null ? null : String(n));

// Each row's key: a message can carry more than one file.
const keyOf = (f: GroupFileRow, i: number) => `${f.message_id}:${i}`;

function Preview({ f, v, onClose }: { f: GroupFileRow; v: Viewer; onClose: () => void }) {
	return (
		<FilePreview f={f} v={v} onClose={onClose}
			links={<>
				<Btn to={f.chat_link} ghost icon={MessageCircle}>In the chat</Btn>
				<Btn to={`/messages/group/${f.message_row_id}`} ghost icon={MessageSquare}>Message</Btn>
			</>}
			details={[
				[null, 'Type', f.file_type],
				[null, 'Size', `${bytes(f.size)} (${fmt(f.size)} bytes)`],
				f.duration ? [null, 'Duration', duration(f.duration)] : null,
				[null, 'Stored', f.stored ? 'yes' : f.url ? 'no, can be fetched from Telegram' : 'no'],
				[null, 'Sender', <SentBy s={f.sender} />],
				[null, 'Sent', dateTime(f.date, v)],
			]} />
	);
}

export function GroupFiles({ groupId, url, p, v }: { groupId: number; url: string; p: GroupFilesPage; v: Viewer }) {
	const { navigate } = useNav();
	const q = useQueryParams();
	const grid = q.get('view') !== 'list';
	const kind = p.kind;
	const load = useCallback(async (before: string) => {
		const r = await fetchGroupFiles(groupId, kind, Number(before));
		return { entries: r.rows, next: cursorOf(r.next_before) };
	}, [groupId, kind]);
	const h = useHistoryPages(p.rows, cursorOf(p.next_before), load);
	const [open, setOpen] = useState<GroupFileRow | null>(null);
	const close = useCallback(() => setOpen(null), []);
	// All, then the kinds the group has; All counts them when each does.
	const known = p.kinds.filter((k) => k.kind !== ('all' as string));
	const all = p.kinds.find((k) => k.kind === ('all' as string))?.count ?? (known.length && known.every((k) => k.count !== null) ? known.reduce((a, k) => a + k.count!, 0) : null);
	const tabs: [('all' | FileKind), number | null][] = [['all', all], ...known.map((k): [FileKind, number | null] => [k.kind, k.count])];
	const tabHref = (k: 'all' | FileKind) => setQuery(url, { kind: k === 'all' ? undefined : k, before: undefined });
	const view = (list: boolean) => navigate(setQuery(url, { view: list ? 'list' : undefined }), { replace: true, scroll: false });
	return (
		<div className="a-groupfiles">
			<div className="a-groupfiles-bar">
				<nav className="a-seg a-seg-links" aria-label="File kinds">
					{tabs.map(([k, n]) => (
						<Link key={k} to={tabHref(k)} className={kind === k ? 'on' : ''} aria-current={kind === k ? 'page' : undefined}>
							{KIND_LABEL[k]}{n !== null && <span className="a-tabcount">{fmt(n)}</span>}
						</Link>
					))}
				</nav>
				<span className="a-grow" />
				<div className="a-seg a-seg-icons" role="group" aria-label="View">
					<button type="button" className={grid ? 'on' : ''} onClick={() => view(false)} aria-label="Grid view"><LayoutGrid size={15} /></button>
					<button type="button" className={!grid ? 'on' : ''} onClick={() => view(true)} aria-label="List view"><List size={15} /></button>
				</div>
			</div>
			{grid ? (
				h.entries.length > 0 && <div className="a-card a-tablecard">
					<div className="a-filegrid">{h.entries.map((f, i) => (
						<div key={keyOf(f, i)} className="a-groupfile">
							<FileCard f={f} at={f.date} v={v} onOpen={() => setOpen(f)} />
							<div className="a-groupfile-from"><SentBy s={f.sender} /><MessageLinks r={f} /></div>
						</div>
					))}</div>
				</div>
			) : h.entries.length > 0 && (
				<div className="a-card a-tablecard">
					<div className="a-tablescroll">
						<table className="a-table">
							<thead><tr><th>File</th><th>Type</th><th className="r">Size</th><th>Stored</th><th>Sender</th><th>Sent</th><th>Message</th></tr></thead>
							<tbody>{h.entries.map((f, i) => (
								<tr key={keyOf(f, i)}>
									<td className="c-name"><FileNameCell f={f} sub={dateTime(f.date, v)} onOpen={() => setOpen(f)} /></td>
									<td><span className="a-typecell"><TypeIcon type={f.file_type} />{f.file_type}</span></td>
									<td className="r"><span className="num">{bytes(f.size)}</span></td>
									<td>{f.stored ? <Pill tone="ok">stored</Pill> : f.url ? <Pill tone="info">fetchable</Pill> : <Pill tone="danger">missing</Pill>}</td>
									<td><SentBy s={f.sender} /></td>
									<td><span title={f.date}>{ago(f.date, v)}</span></td>
									<td><MessageLinks r={f} /></td>
								</tr>
							))}</tbody>
						</table>
					</div>
				</div>
			)}
			{!h.entries.length && <Empty>{kind === 'all' ? 'No files in this group.' : `No ${KIND_LABEL[kind].toLowerCase()} in this group.`}</Empty>}
			<LoadMore h={h} noun="files" />
			{open && <Preview f={open} v={v} onClose={close} />}
		</div>
	);
}
