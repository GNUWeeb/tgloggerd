import { CornerUpLeft, Forward, Link2, Megaphone, MessageCircle, Pencil, Trash2 } from 'lucide-react';
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

const ORIGIN = { user: 'a user', hidden_user: 'a user who hides their account', chat: 'a group', channel: 'a channel' } as const;

function MessagePage({ page }: { page: PageData<MessageProps> }) {
	const m = page.props.message;
	const v = page.viewer;
	const chatHref = m.chat.kind === 'group' ? `/groups/${m.chat.id}` : `/users/${m.chat.id}`;
	const list = m.scope === 'group' ? ['Group messages', '/group_messages'] as const : ['Private messages', '/private_messages'] as const;
	const big = m.media && (m.content_type === 'photo' || m.content_type === 'video');
	// Posted as a chat (a channel, an anonymous admin): link that chat.
	const asChat = m.sender_chat;
	const senderLink = asChat ? <Link to={`/groups/${asChat.id}`} className="a-rowlink"><b>{asChat.title}</b></Link>
		: m.sender ? <Link to={`/users/${m.sender.id}`} className="a-rowlink"><b>{m.sender.name}</b></Link> : <b>You (logging account)</b>;
	const f = m.forward_from;
	const fromLink = f?.user_id !== undefined ? <Link to={`/users/${f.user_id}`} className="a-rowlink">{f.name}</Link>
		: f?.chat_id !== undefined ? <Link to={`/groups/${f.chat_id}`} className="a-rowlink">{f.name}</Link> : <>{f?.name ?? m.forwarded}</>;
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
				{m.is_channel_post && <Pill tone="info"><Megaphone size={11} />channel post</Pill>}
				{m.forwarded && <Pill tone="info"><Forward size={11} />forwarded</Pill>}
				{m.edited && <Pill><Pencil size={11} />edited</Pill>}
				{m.deleted_at && <Pill tone="danger"><Trash2 size={11} />deleted</Pill>}
			</div>
			{m.deleted_at && <div className="a-banner warn"><Trash2 size={15} /><span>Deleted on Telegram at {dateTime(m.deleted_at, v)}. The archive keeps the last version it saw.</span></div>}
			<div className="a-cols">
				<div className="a-stack">
					<Card>
						<div className="a-msgcard-head">
							{asChat ? <Avatar id={asChat.id} name={asChat.title} photo={asChat.photo} size={36} square /> : m.sender ? <Avatar id={m.sender.id} name={m.sender.name} photo={m.sender.photo} size={36} /> : <Avatar id={0} name="You" size={36} />}
							<div className="a-grow">
								{senderLink}
								<div className="a-muted sm">{dateTime(m.date, v)}</div>
							</div>
						</div>
						{m.reply_to && (
							<Link to={`/messages/${m.reply_to.scope}/${m.reply_to.id}`} className="a-replybox">
								<CornerUpLeft size={14} />
								<span><b>{m.reply_to.sender_name}</b><span className="a-ellipsis a-muted">{m.reply_to.snippet}</span></span>
							</Link>
						)}
						{m.forwarded && <p className="a-muted sm a-fwdline"><Forward size={13} />Forwarded from <b>{fromLink}</b></p>}
						{m.media && (
							<div className="a-msgmedia">
								{big ? <Thumb type={m.content_type} thumb={m.media.thumb} stored={m.media.stored} size={260} radius={10} />
									: <div className="a-filerow"><Thumb type={m.content_type} thumb={m.media.thumb} stored={m.media.stored} size={44} /><div><b>{m.media.name || m.content_type}</b><div className="a-muted sm">{bytes(m.media.size)}{m.media.duration ? ' . ' + duration(m.media.duration) : ''}</div></div></div>}
							</div>
						)}
						{m.text ? <pre className="a-msgtext">{m.text}</pre> : !m.media && <p className="a-muted">{m.service_type ? m.service_type.replace(/_/g, ' ') : '(no text)'}</p>}
						{m.author_signature && <p className="a-muted sm a-signline">- {m.author_signature}</p>}
					</Card>
					{f && (
						<Card title="Forwarded from">
							<KV rows={[
								[null, 'Origin', ORIGIN[f.type]],
								[null, f.type === 'channel' || f.type === 'chat' ? 'Chat' : 'Sender', fromLink],
								f.chat_id !== undefined && f.message_id !== undefined ? [null, 'Original post', <Link to={`/groups/${f.chat_id}/chat?at=${f.message_id}`} className="a-rowlink">#{f.message_id} in the chat</Link>] : null,
								f.date ? [null, 'Originally sent', dateTime(f.date, v)] : null,
							]} />
						</Card>
					)}
					{m.edits.length > 0 && (
						<Card title="Edit history">
							<Timeline v={v} items={[
								...m.edits.map((e) => ({
									icon: Pencil, at: e.at, title: e.content_type && e.content_type !== m.content_type ? `Earlier version (${e.content_type})` : 'Earlier version',
									body: <>
										{e.media && (e.media.thumb
											? <a href={e.media.url ?? undefined} target="_blank" rel="noopener" className="a-editmedia" aria-label="Earlier file"><img src={e.media.thumb} alt="" width={72} height={72} loading="lazy" /></a>
											: <div className="a-muted sm">{e.media.url ? <a href={e.media.url} className="a-link">{e.media.name || 'earlier file'}</a> : `${e.media.name || 'earlier file'} (not stored)`}</div>)}
										{e.text || (!e.media && <span className="a-muted">(no text)</span>)}
									</>,
								})),
								...(m.edited_at ? [{ icon: Pencil, at: m.edited_at, title: 'Current version', body: m.text, tone: 'accent' as const }] : []),
							]} />
						</Card>
					)}
				</div>
				<Card title="Details">
					<KV rows={[
						[null, 'Chat', <Link to={chatHref} className="a-rowlink">{m.chat.title}</Link>],
						[null, 'Chat ID', m.chat.id, true],
						[null, asChat ? 'Posted as' : 'Sender', senderLink],
						[null, 'Message ID', m.message_id, true],
						[null, 'Type', m.content_type],
						[null, 'Sent', dateTime(m.date, v)],
						m.edited_at ? [null, 'Edited', dateTime(m.edited_at, v)] : null,
						m.deleted_at ? [null, 'Deleted', dateTime(m.deleted_at, v)] : null,
						m.media ? [null, 'File', <Link to={`/files?view=list&file=${m.media.file_id}`} className="a-rowlink">file #{fmt(m.media.file_id)}</Link>] : null,
						m.forwarded ? [null, 'Forwarded from', fromLink] : null,
						m.author_signature ? [null, 'Signature', m.author_signature] : null,
						m.service_type ? [null, 'Service action', m.service_type, true] : null,
						m.tdlib_type ? [null, 'TDLib type', m.tdlib_type, true] : null,
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
