import { CornerUpLeft, Forward, Link2, MessageCircle, Pencil, Trash2 } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, Btn, Card, KV, Pill } from '../ui/primitives.tsx';
import { Thumb, TypeIcon, duration } from '../ui/media.tsx';
import { Timeline } from '../ui/timeline.tsx';
import { bytes, dateTime, fmt } from '../format.ts';
import { chatPath } from './messages.tsx';
import type { PageData } from '../page.ts';
import type { MessageFull } from '../types.ts';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type MessageProps = S['MessageProps'];

function MessagePage({ page }: { page: PageData<MessageProps> }) {
	const m = page.props.message;
	const v = page.viewer;
	const chatHref = m.chat.kind === 'group' ? `/groups/${m.chat.id}` : `/users/${m.chat.id}`;
	const list = m.scope === 'group' ? ['Group messages', '/group_messages'] as const : ['Private messages', '/private_messages'] as const;
	const big = m.media && (m.content_type === 'photo' || m.content_type === 'video');
	return (
		<Shell crumbs={[[list[0], list[1]], [`#${m.message_id}`, null]]}>
			<div className="a-pagehead">
				<div>
					<h1>Message #{m.message_id}</h1>
					<p className="a-sub">in <Link to={chatHref} className="a-rowlink">{m.chat.title}</Link> . {dateTime(m.date, v)}</p>
				</div>
				<div className="a-head-actions">
					<Btn primary to={chatPath(m)} icon={MessageCircle}>Show in chat</Btn>
					<Btn ghost icon={Link2} onClick={() => void navigator.clipboard?.writeText(location.href)}>Copy link</Btn>
				</div>
			</div>
			<div className="a-msgflagsrow">
				<Pill><TypeIcon type={m.content_type} size={11} />{m.content_type}</Pill>
				{!m.sender && <Pill tone="accent">outgoing</Pill>}
				{m.forwarded && <Pill tone="info"><Forward size={11} />forwarded</Pill>}
				{m.edited && <Pill><Pencil size={11} />edited</Pill>}
				{m.deleted_at && <Pill tone="danger"><Trash2 size={11} />deleted</Pill>}
			</div>
			{m.deleted_at && <div className="a-banner warn"><Trash2 size={15} /><span>Deleted on Telegram at {dateTime(m.deleted_at, v)}. The archive keeps the last version it saw.</span></div>}
			<div className="a-cols">
				<div className="a-stack">
					<Card>
						<div className="a-msgcard-head">
							{m.sender ? <Avatar id={m.sender.id} name={m.sender.name} photo={m.sender.photo} size={36} /> : <Avatar id={0} name="You" size={36} />}
							<div className="a-grow">
								{m.sender ? <Link to={`/users/${m.sender.id}`} className="a-rowlink"><b>{m.sender.name}</b></Link> : <b>You (logging account)</b>}
								<div className="a-muted sm">{dateTime(m.date, v)}</div>
							</div>
						</div>
						{m.reply_to && (
							<Link to={`/messages/${m.reply_to.scope}/${m.reply_to.id}`} className="a-replybox">
								<CornerUpLeft size={14} />
								<span><b>{m.reply_to.sender_name}</b><span className="a-ellipsis a-muted">{m.reply_to.snippet}</span></span>
							</Link>
						)}
						{m.forwarded && <p className="a-muted sm a-fwdline"><Forward size={13} />Forwarded from <b>{m.forwarded}</b></p>}
						{m.media && (
							<div className="a-msgmedia">
								{big ? <Thumb type={m.content_type} thumb={m.media.thumb} stored={m.media.stored} size={260} radius={10} />
									: <div className="a-filerow"><Thumb type={m.content_type} thumb={m.media.thumb} stored={m.media.stored} size={44} /><div><b>{m.media.name || m.content_type}</b><div className="a-muted sm">{bytes(m.media.size)}{m.media.duration ? ' . ' + duration(m.media.duration) : ''}</div></div></div>}
							</div>
						)}
						{m.text ? <pre className="a-msgtext">{m.text}</pre> : !m.media && <p className="a-muted">(no text)</p>}
					</Card>
					{m.edits.length > 0 && (
						<Card title="Edit history">
							<Timeline v={v} items={[
								...m.edits.map((e) => ({ icon: Pencil, at: e.at, title: 'Earlier version', body: e.text })),
								...(m.edited_at ? [{ icon: Pencil, at: m.edited_at, title: 'Current version', body: m.text, tone: 'accent' as const }] : []),
							]} />
						</Card>
					)}
				</div>
				<Card title="Details">
					<KV rows={[
						[null, 'Chat', <Link to={chatHref} className="a-rowlink">{m.chat.title}</Link>],
						[null, 'Chat ID', m.chat.id, true],
						[null, 'Sender', m.sender ? <Link to={`/users/${m.sender.id}`} className="a-rowlink">{m.sender.name}</Link> : 'You (logging account)'],
						[null, 'Message ID', m.message_id, true],
						[null, 'Type', m.content_type],
						[null, 'Sent', dateTime(m.date, v)],
						m.edited_at ? [null, 'Edited', dateTime(m.edited_at, v)] : null,
						m.deleted_at ? [null, 'Deleted', dateTime(m.deleted_at, v)] : null,
						m.media ? [null, 'File', <Link to={`/files?view=list&file=${m.media.file_id}`} className="a-rowlink">file #{fmt(m.media.file_id)}</Link>] : null,
						m.forwarded ? [null, 'Forwarded from', m.forwarded] : null,
					]} />
				</Card>
			</div>
		</Shell>
	);
}

export const messageRoute: RouteDef<MessageProps> = {
	Page: MessagePage,
	head: (p) => ({ title: `Message #${p.props.message.message_id} in ${p.props.message.chat.title} - tgloggerd`, robots: 'noindex' }),
};
