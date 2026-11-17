import { MessageCircle, Forward, Pencil, Trash2, CornerUpLeft, Copy, Link2 } from 'lucide-react';
import { Link } from '../router.jsx';
import Shell from '../ui/Shell.jsx';
import { Avatar, Pill, Card, KV, Btn } from '../ui/primitives.jsx';
import { Timeline } from '../ui/Timeline.jsx';
import { Thumb, TypeIcon, duration } from '../ui/Media.jsx';
import { getMessage, senderOf } from '../data/messages.js';
import { bytes, fmt } from '../data/seed.js';
import { snippet, chatHref } from './Messages.jsx';
import NotFound from './NotFound.jsx';
import { useViewer } from '../viewer.jsx';

function chatLink(m) {
	return m.scope === 'group' ? `/groups/${m.chat.id}` : `/users/${m.chat.id}`;
}

export default function Message({ scope, id }) {
	const m = getMessage(scope, id);
	const v = useViewer();
	if (!m || (scope === 'group' && !v.group(m.chat))) return <NotFound what="message" />;
	const s = senderOf(m);
	const listTitle = scope === 'group' ? 'Group messages' : 'Private messages';
	const f = m.media ? { ...m.media, file_type: m.content_type } : null;
	return (
		<Shell crumbs={[[listTitle, scope === 'group' ? '/group_messages' : '/private_messages'], [`#${m.message_id}`, null]]}>
			<div className="a-pagehead">
				<div>
					<h1>Message #{m.message_id}</h1>
					<p className="a-sub">in <Link to={chatLink(m)} className="a-rowlink">{m.chat.title || m.chat.name}</Link> . {m.date}</p>
				</div>
				<div className="a-head-actions">
					<Btn primary to={chatHref(m)} icon={MessageCircle}>Show in chat</Btn>
					<Btn ghost icon={Link2}>Copy link</Btn>
				</div>
			</div>
			<div className="a-msgflagsrow">
				<Pill><TypeIcon type={m.content_type} size={11} />{m.content_type}</Pill>
				{m.is_outgoing && <Pill tone="accent">outgoing</Pill>}
				{m.forward && <Pill tone="info"><Forward size={11} />forwarded</Pill>}
				{m.edits && <Pill><Pencil size={11} />edited</Pill>}
				{m.deleted_at && <Pill tone="danger"><Trash2 size={11} />deleted</Pill>}
			</div>
			{m.deleted_at && (
				<div className="a-banner warn"><Trash2 size={15} /><span>Deleted on Telegram at {m.deleted_at}. The archive keeps the last version it saw.</span></div>
			)}

			<div className="a-cols">
				<div className="a-stack">
					<Card>
						<div className="a-msgcard-head">
							<Avatar e={s} size={36} />
							<div className="a-grow">
								{m.is_outgoing ? <b>You (logger account)</b> : <Link to={`/users/${s.id}`} className="a-rowlink"><b>{s.name}</b></Link>}
								<div className="a-muted sm">{m.date}</div>
							</div>
						</div>
						{m.reply_to && (
							<Link to={`/messages/${scope}/${m.reply_to.id}`} className="a-replybox">
								<CornerUpLeft size={14} />
								<span><b>{senderOf(m.reply_to)?.name}</b><span className="a-ellipsis a-muted">{snippet(m.reply_to)}</span></span>
							</Link>
						)}
						{m.forward && <p className="a-muted sm a-fwdline"><Forward size={13} />Forwarded from <b>{m.forward.from}</b></p>}
						{f && (
							<div className="a-msgmedia">
								{(f.file_type === 'photo' || f.file_type === 'video')
									? <Thumb f={{ ...f, stored: f.stored }} size={260} radius={10} />
									: <div className="a-filerow"><Thumb f={f} size={44} /><div><b>{f.name || f.file_type}</b><div className="a-muted sm">{f.size ? bytes(f.size) : ''}{f.duration ? ' . ' + duration(f.duration) : ''}</div></div></div>}
							</div>
						)}
						{m.text && <pre className="a-msgtext">{m.text}</pre>}
						{!m.text && !f && <p className="a-muted">(no text)</p>}
					</Card>
					{m.edits && (
						<Card title="Edit history">
							<Timeline items={[
								...m.edits.map((e) => ({ icon: Pencil, at: e.at, title: 'Original', body: e.text })),
								{ icon: Pencil, at: m.edited_at, title: 'Edited to', body: m.text, tone: 'accent' },
							]} />
						</Card>
					)}
				</div>
				<Card title="Details">
					<KV rows={[
						[null, 'Chat', <Link to={chatLink(m)} className="a-rowlink">{m.chat.title || m.chat.name}</Link>],
						[null, 'Chat ID', m.chat.id, true],
						[null, 'Sender', m.is_outgoing ? 'You (logger account)' : <Link to={`/users/${s.id}`} className="a-rowlink">{s.name}</Link>],
						[null, 'Message ID', <>{m.message_id} <button type="button" className="a-icon-btn sm" aria-label="Copy"><Copy size={12} /></button></>, true],
						[null, 'Type', m.content_type],
						[null, 'Sent', m.date, true],
						m.edited_at && [null, 'Edited', m.edited_at, true],
						m.deleted_at && [null, 'Deleted', m.deleted_at, true],
						f?.fileId && [null, 'File', <Link to={`/files?file=${f.fileId}&view=list`} className="a-rowlink">file #{fmt(f.fileId)}</Link>],
						m.forward && [null, 'Forwarded from', m.forward.from],
					]} />
				</Card>
			</div>
		</Shell>
	);
}
