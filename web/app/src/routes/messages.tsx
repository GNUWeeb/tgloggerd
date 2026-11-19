import { ArrowUpRight, Forward, Pencil, Trash2 } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, PageHead } from '../ui/primitives.tsx';
import { DataTable, emptyText, FilterBar, useListNav, withMatches, type Col } from '../ui/list.tsx';
import { TypeIcon, CONTENT_TYPES } from '../ui/media.tsx';
import { CONTENT_LABELS, typeLabel } from '../ui/labels.ts';
import { ago, dateTime, fmt } from '../format.ts';
import type { PageData } from '../page.ts';
import type { Cond, FieldDef, ListProps, MessageRow } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';
import { safeFragment } from '../api/engine.ts';
import { BackendNote, EngineOptions, Facets, Highlighted } from '../ui/SearchEngine.tsx';

export type MessagesProps = S['MessagesProps'];

const TYPES: string[] = CONTENT_TYPES;
const FIELDS: Record<'group' | 'private', FieldDef[]> = {
	group: [
		{ key: 'text', label: 'Text', type: 'text' },
		{ key: 'content_type', label: 'Content type', type: 'enum', values: TYPES, labels: CONTENT_LABELS },
		{ key: 'chat_id', label: 'Group id', type: 'number' },
		{ key: 'sender_id', label: 'Sender user id', type: 'number' },
		{ key: 'is_forwarded', label: 'Forwarded', type: 'bool' },
		{ key: 'is_edited', label: 'Edited', type: 'bool' },
		{ key: 'is_deleted', label: 'Deleted', type: 'bool' },
		{ key: 'date', label: 'Sent', type: 'date' },
	],
	private: [
		{ key: 'text', label: 'Text', type: 'text' },
		{ key: 'content_type', label: 'Content type', type: 'enum', values: TYPES, labels: CONTENT_LABELS },
		{ key: 'chat_id', label: 'User (chat) id', type: 'number' },
		{ key: 'is_outgoing', label: 'Outgoing', type: 'bool' },
		{ key: 'is_deleted', label: 'Deleted', type: 'bool' },
		{ key: 'date', label: 'Sent', type: 'date' },
	],
};
// The kinds "Media" means: those with a file to show or play.
export const MEDIA: S['ContentType'][] = ['photo', 'video', 'video_note', 'document', 'audio', 'voice', 'sticker', 'animation', 'paid_media'];
const QUICK: Record<'group' | 'private', [string, Cond[]][]> = {
	group: [['Media', [{ field: 'content_type', op: 'in', value: MEDIA }]], ['Deleted', [{ field: 'is_deleted', op: 'is', value: 'yes' }]], ['Edited', [{ field: 'is_edited', op: 'is', value: 'yes' }]]],
	private: [['Outgoing', [{ field: 'is_outgoing', op: 'is', value: 'yes' }]], ['Deleted', [{ field: 'is_deleted', op: 'is', value: 'yes' }]]],
};

// Bolds the searched words inside a snippet.
export function Hl({ text, q }: { text: string; q: string }) {
	if (!q) return <>{text}</>;
	const i = text.toLowerCase().indexOf(q.toLowerCase());
	if (i < 0) return <>{text}</>;
	return <>{text.slice(0, i)}<mark>{text.slice(i, i + q.length)}</mark>{text.slice(i + q.length)}</>;
}

export function MsgFlags({ m }: { m: MessageRow }) {
	return (
		<span className="a-msgflags">
			{m.forwarded && <span title={`Forwarded from ${m.forwarded}`}><Forward size={13} /></span>}
			{m.edited && <span title="Edited"><Pencil size={13} /></span>}
			{m.deleted_at && <span className="a-del" title="Deleted"><Trash2 size={13} /></span>}
		</span>
	);
}

// A row's kind: a service row by its action.
const rowType = (m: MessageRow) => typeLabel(m.content_type, m.service_type);

export const chatPath = (m: MessageRow) => `${m.scope === 'group' ? '/groups' : '/users'}/${m.chat.id}/chat?at=${m.message_id}`;
const chatLink = (c: MessageRow['chat']) => (c.kind === 'group' ? `/groups/${c.id}` : `/users/${c.id}`);

function Messages({ page }: { page: PageData<MessagesProps> }) {
	const L = useListNav(page.props);
	const v = page.viewer;
	const scope = page.props.scope;
	const title = scope === 'group' ? 'Group messages' : 'Private messages';
	// The engine's matched fragments of each row, when it sent them; the
	// rows without any keep the plain snippet.
	const hls = new Map((page.props.highlights ?? []).map((h, i) => [page.props.rows[i]?.id, h]));
	const snippet = (m: MessageRow) => (hls.get(m.id)?.some((f) => safeFragment(f) !== null) ? <Highlighted fragments={hls.get(m.id)} /> : <Hl text={m.snippet} q={L.q} />);
	const cols: Col<MessageRow>[] = [
		{ key: 'date', label: 'Sent', sort: true, cls: 'mono a-dim', render: (m) => <span title={m.date}>{dateTime(m.date, v)}</span> },
		{ key: 'chat', label: scope === 'group' ? 'Group' : 'Chat with', cls: 'a-hide-md', render: (m) => <Link to={chatLink(m.chat)} className="a-chatcell"><Avatar id={m.chat.id} name={m.chat.title} photo={m.chat.photo} size={20} square={m.chat.kind === 'group'} /><span className="a-ellipsis">{m.chat.title}</span></Link> },
		{ key: 'sender', label: scope === 'group' ? 'Sender' : 'From', render: (m) => (m.sender
			? m.sender.id === 0 ? <span className="a-chatcell a-muted"><span className="a-ellipsis">{m.sender.name}</span></span> : <Link to={`/users/${m.sender.id}`} className="a-chatcell"><Avatar id={m.sender.id} name={m.sender.name} photo={m.sender.photo} size={20} /><span className="a-ellipsis">{m.sender.name}</span></Link>
			: <span className="a-out">you</span>) },
		{ key: 'content_type', label: 'Type', render: (m) => <span className="a-typecell"><TypeIcon type={m.content_type} />{rowType(m)}</span> },
		{ key: 'text', label: 'Text', cls: 'a-textcell', render: (m) => <Link to={`/messages/${scope}/${m.id}`} className={`a-snippet ${m.deleted_at ? 'is-deleted' : ''} ${m.content_type === 'service' ? 'is-service' : ''}`}>{snippet(m)}</Link> },
		{ key: 'flags', label: '', render: (m) => <MsgFlags m={m} /> },
		{ key: 'open', label: '', render: (m) => <Link to={chatPath(m)} className="a-icon-btn sm" title="Show in chat" aria-label="Show in chat"><ArrowUpRight size={14} /></Link> },
	];
	// The own text field hides the search's, so it takes the search's
	// matches, which the search engine answers.
	const fields = withMatches(FIELDS[scope], L.fields, 'text');
	return (
		<Shell crumbs={[[title, null]]}>
			<PageHead title={title} sub={v.account?.role === 'member' ? 'Messages in your groups. Full-text search over text and captions.' : `${fmt(page.props.archive_total ?? L.total)} messages. Full-text search over text and captions.`} />
			<FilterBar L={L} fields={fields} placeholder="Search message text" name={title} quick={QUICK[scope]} />
			<EngineOptions p={page.props} />
			<BackendNote b={page.props.backend} />
			<Facets facets={page.props.facets} conds={L.conds} onConds={L.setConds} />
			<DataTable L={L} cols={cols} noun="messages" empty={emptyText(L, v.account?.role === 'member', 'messages', 'No messages match. Full-text search matches whole words.')}
				href={(m) => `/messages/${scope}/${m.id}`}
				mobile={(m) => ({
					lead: scope === 'private' ? <Avatar id={m.chat.id} name={m.chat.title} photo={m.chat.photo} size={36} /> : <Avatar id={m.sender?.id ?? 0} name={m.sender?.name ?? '?'} photo={m.sender?.photo} size={36} />,
					title: scope === 'private' ? m.chat.title : <>{m.sender?.name}<span className="a-muted"> in {m.chat.title}</span></>,
					sub: <span className={`a-snippet2 ${m.deleted_at ? 'is-deleted' : ''}`}>{!m.sender && <b>You: </b>}{m.content_type !== 'text' && <span className="a-muted">{rowType(m)}: </span>}{snippet(m)}</span>,
					side: <><span className="a-muted sm">{ago(m.date, v)}</span><MsgFlags m={m} /></>,
				})} />
		</Shell>
	);
}

export const messagesRoute: RouteDef<MessagesProps> = {
	Page: Messages,
	head: (p) => ({ title: `${p.props.scope === 'group' ? 'Group' : 'Private'} messages - tgloggerd`, robots: 'noindex' }),
};
