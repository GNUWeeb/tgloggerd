import { Forward, Pencil, Trash2, ArrowUpRight } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { useList } from '../ui/hooks.js';
import { Avatar, PageHead } from '../ui/primitives.jsx';
import { DataTable, FilterBar } from '../ui/DataTable.jsx';
import { TypeIcon } from '../ui/Media.jsx';
import { queryMessages, MSG_FIELDS, senderOf } from '../data/messages.js';
import { TOTALS } from '../data/totals.js';
import { fmt, ago } from '../data/seed.js';

export function snippet(m) {
	if (m.content_type === 'service') return m.text;
	if (m.text) return m.text;
	if (m.media?.name) return m.media.name;
	return `[${m.content_type}]`;
}

// Bolds the matched search terms inside a text snippet.
export function Hl({ text, q }) {
	if (!q) return text;
	const i = text.toLowerCase().indexOf(q.toLowerCase());
	if (i < 0) return text;
	return <>{text.slice(0, i)}<mark>{text.slice(i, i + q.length)}</mark>{text.slice(i + q.length)}</>;
}

export function MsgFlags({ m }) {
	return (
		<span className="a-msgflags">
			{m.forward && <span title={`Forwarded from ${m.forward.from}`}><Forward size={13} /></span>}
			{m.edits && <span title="Edited"><Pencil size={13} /></span>}
			{m.deleted_at && <span className="a-del" title={`Deleted ${m.deleted_at}`}><Trash2 size={13} /></span>}
		</span>
	);
}

export function chatHref(m) {
	return `${m.scope === 'group' ? '/groups' : '/users'}/${m.chat.id}/chat?at=${m.message_id}`;
}

const QUICK = {
	group: [
		['Media', [{ field: 'content_type', op: '!=', value: 'text' }]],
		['Deleted', [{ field: 'is_deleted', op: 'is', value: 'yes' }]],
		['Edited', [{ field: 'is_edited', op: 'is', value: 'yes' }]],
	],
	private: [
		['Outgoing', [{ field: 'is_outgoing', op: 'is', value: 'yes' }]],
		['Deleted', [{ field: 'is_deleted', op: 'is', value: 'yes' }]],
	],
};

export default function Messages({ scope }) {
	const L = useList((s) => queryMessages(scope, s), { sort: 'date', per: 50 });
	const title = scope === 'group' ? 'Group messages' : 'Private messages';
	const chatCol = scope === 'group'
		? { key: 'chat_title', label: 'Group', sort: true, cls: 'a-hide-md', render: (m) => <Link to={`/groups/${m.chat.id}`} className="a-chatcell"><Avatar e={m.chat} size={20} square /><span className="a-ellipsis">{m.chat.title}</span></Link> }
		: { key: 'chat_title', label: 'Chat with', sort: true, cls: 'a-hide-md', render: (m) => <Link to={`/users/${m.chat.id}`} className="a-chatcell"><Avatar e={m.chat} size={20} /><span className="a-ellipsis">{m.chat.name}</span></Link> };
	const cols = [
		{ key: 'date', label: 'Sent', sort: true, cls: 'mono a-dim', render: (m) => <span title={m.date}>{m.date.slice(5, 16)}</span> },
		chatCol,
		{ key: 'sender_name', label: scope === 'group' ? 'Sender' : 'From', sort: true, render: (m) => {
			const s = senderOf(m);
			return m.is_outgoing ? <span className="a-out">you</span>
				: <Link to={`/users/${s.id}`} className="a-chatcell"><Avatar e={s} size={20} /><span className="a-ellipsis">{s.name}</span></Link>;
		} },
		{ key: 'content_type', label: 'Type', sort: true, render: (m) => <span className="a-typecell"><TypeIcon type={m.content_type} />{m.content_type}</span> },
		{ key: 'text', label: 'Text', cls: 'a-textcell', render: (m) => (
			<Link to={`/messages/${scope}/${m.id}`} className={`a-snippet ${m.deleted_at ? 'is-deleted' : ''} ${m.content_type === 'service' ? 'is-service' : ''}`}>
				<Hl text={snippet(m)} q={L.q} />
			</Link>
		) },
		{ key: 'flags', label: '', render: (m) => <MsgFlags m={m} /> },
		{ key: 'open', label: '', render: (m) => <Link to={chatHref(m)} className="a-icon-btn sm" title="Show in chat" aria-label="Show in chat"><ArrowUpRight size={14} /></Link> },
	];
	return (
		<Shell crumbs={[[title, null]]}>
			<PageHead title={title} sub={`${fmt(scope === 'group' ? TOTALS.group_messages : TOTALS.private_messages)} messages. Full-text search over text and captions.`} />
			<FilterBar L={L} fields={MSG_FIELDS[scope]} placeholder="Search message text" quick={QUICK[scope]} />
			<DataTable
				L={L} cols={cols} noun="messages"
				href={(m) => `/messages/${scope}/${m.id}`}
				empty="No messages match. Full-text search matches whole words."
				mobile={(m) => {
					const s = senderOf(m);
					return {
						lead: <Avatar e={scope === 'private' ? m.chat : s} size={36} />,
						title: scope === 'private' ? m.chat.name : <>{s.name}<span className="a-muted"> in {m.chat.title}</span></>,
						sub: <span className={`a-snippet2 ${m.deleted_at ? 'is-deleted' : ''}`}>{m.is_outgoing && <b>You: </b>}<Hl text={snippet(m)} q={L.q} /></span>,
						side: <><span className="a-muted sm">{ago(m.date)}</span><MsgFlags m={m} /></>,
					};
				}}
			/>
		</Shell>
	);
}
