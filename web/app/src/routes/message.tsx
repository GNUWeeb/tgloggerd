import { CornerUpLeft, Forward, Link2, Megaphone, MessageCircle, Pencil, Pin, Trash2 } from 'lucide-react';
import Shell from '../ui/Shell.tsx';
import { Link } from '../nav.tsx';
import { Avatar, Btn, Card, KV, Pill } from '../ui/primitives.tsx';
import { Thumb, TypeIcon, duration } from '../ui/media.tsx';
import { contentLabel, serviceLabel, typeLabel } from '../ui/labels.ts';
import { loggerName } from '../ui/logger.ts';
import { Timeline } from '../ui/timeline.tsx';
import { bytes, dateTime, fmt } from '../format.ts';
import { chatPath } from './messages.tsx';
import { MessageMedia, kindOf } from './chat-media.tsx';
import { Counts, EffectMark, KindDetail, KindFallback, Reactions, TopicIcon, hasFallback, richPage, textInside } from './chat-kinds.tsx';
import { LinkPreview } from './chat-preview.tsx';
import type { PageData } from '../page.ts';
import { RichText } from '../ui/richtext.tsx';
import { ServiceText, partsOf } from '../ui/serviceparts.tsx';
import { EditDetail, useEditList } from '../ui/editdetail.tsx';
import { ServiceDetailView, topicInText } from '../ui/servicedetail.tsx';
import type { RouteDef } from './index.tsx';
import type { S } from '../api/schema.ts';

export type MessageProps = S['MessageProps'];

const ORIGIN = { user: 'a user', hidden_user: 'a user who hides their account', chat: 'a group', channel: 'a channel' } as const;

function MessagePage({ page }: { page: PageData<MessageProps> }) {
	const m = page.props.message;
	const v = page.viewer;
	const chatHref = m.chat.kind === 'group' ? `/groups/${m.chat.id}` : `/users/${m.chat.id}`;
	// Hashtags search the message's chat.
	const chat = { scope: m.scope, id: m.chat.id };
	const ents = m.entities;
	const list = m.scope === 'group' ? ['Group messages', '/group_messages'] as const : ['Private messages', '/private_messages'] as const;
	// The media as the chat draws it, by kind; only media no kind fits
	// (an unknown type without a render) falls back to a plain file row.
	const drawn = m.media && kindOf(m.content_type, m.media) !== null;
	// A kind without a card to draw says what it is instead, as in the chat.
	const fallback = !m.detail && hasFallback(m.content_type);
	// A service message's action, and the message it is about (a pin).
	const svcType = m.service?.type ?? m.service_type;
	const svcAbout = m.service?.refs.message_id ?? null;
	// What its stored detail adds (the new photo, what was pinned, ...),
	// after its text; a topic the text quotes links to the topic.
	const svcDetail = m.content_type === 'service' ? m.service?.detail : null;
	const topicHref = (id: number) => `/groups/${m.chat.id}/chat?topic=${id}`;
	const svcLine = svcDetail && <ServiceDetailView service={m.service} text={m.text} v={v} topicHref={topicHref} self={!m.sender && !m.sender_chat && v.signed_in} photoSize={96} />;
	// Posted as a chat (a channel, an anonymous admin): link that chat.
	const asChat = m.sender_chat;
	// No sender: the logging account ("You" when signed in).
	const logger = v.signed_in ? null : v.logger;
	const senderLink = asChat ? <Link to={`/groups/${asChat.id}`} className="a-rowlink"><b>{asChat.title}</b></Link>
		: m.sender ? (m.sender.id !== 0 ? <Link to={`/users/${m.sender.id}`} className="a-rowlink"><b>{m.sender.name}</b></Link> : <b className="a-muted">{m.sender.name}</b>) : <b>{v.signed_in ? 'You (logging account)' : loggerName(v)}</b>;
	const f = m.forward_from;
	const editList = useEditList(m.edits, 'a-link sm a-editmore');
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
				<Pill><TypeIcon type={m.content_type} size={11} />{typeLabel(m.content_type, svcType)}</Pill>
				{!m.sender && <Pill tone="accent">outgoing</Pill>}
				{m.is_channel_post && <Pill tone="info"><Megaphone size={11} />channel post</Pill>}
				{m.forwarded && <Pill tone="info"><Forward size={11} />forwarded</Pill>}
				{m.edited && <Pill><Pencil size={11} />edited</Pill>}
				{m.pinned && <Pill tone="accent"><Pin size={11} />pinned</Pill>}
				{m.deleted_at && <Pill tone="danger"><Trash2 size={11} />deleted</Pill>}
			</div>
			{m.deleted_at && <div className="a-banner warn"><Trash2 size={15} /><span>Deleted on Telegram at {dateTime(m.deleted_at, v)}. The archive keeps the last version it saw.</span></div>}
			<div className="a-cols">
				<div className="a-stack">
					<Card>
						<div className="a-msgcard-head">
							{asChat ? <Avatar id={asChat.id} name={asChat.title} photo={asChat.photo} size={36} square /> : m.sender ? <Avatar id={m.sender.id} name={m.sender.name} photo={m.sender.photo} size={36} /> : <Avatar id={logger?.id ?? 0} name={loggerName(v)} photo={logger?.photo ?? null} size={36} />}
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
								{drawn ? <MessageMedia msg={{ content_type: m.content_type, media: m.media, album: m.album ?? [] }} onClick={v.chat?.media === 'click'} />
									: <div className="a-filerow"><Thumb type={m.content_type} thumb={m.media.thumb} url={m.media.url} stored={m.media.stored} ext={m.media.ext} size={44} /><div><b>{m.media.name || contentLabel(m.content_type)}</b><div className="a-muted sm">{bytes(m.media.size)}{m.media.duration ? ' . ' + duration(m.media.duration) : ''}</div></div></div>}
							</div>
						)}
						{m.detail ? <div className="a-msgkind"><KindDetail d={m.detail} text={m.text} entities={ents} chat={chat} v={v} tdlib={m.tdlib_type} full /></div>
							: fallback && <div className="a-msgkind"><KindFallback ct={m.content_type} tdlib={m.tdlib_type} /></div>}
						{m.text && !textInside(m.detail) ? <div className="a-msgtext">{partsOf(m.service, m.text) ? <ServiceText service={m.service} text={m.text} linkClass="a-link" /> : topicInText(m.text, svcDetail, topicHref) ?? <RichText text={m.text} entities={ents} chat={chat} />}{svcLine}</div> : !m.media && !m.detail && !fallback && <p className="a-muted">{svcType ? serviceLabel(svcType) : '(no text)'}{svcLine}</p>}
						{m.link_preview && <LinkPreview lp={m.link_preview} onClick={v.chat?.media === 'click'} />}
						{svcAbout !== null && <p className="a-muted sm"><Link to={`${chatPath(m).split('?')[0]}?at=${svcAbout}`} className="a-link">The message this is about</Link></p>}
						{(m.reactions.length > 0 || m.views !== null || m.forwards || m.replies || m.effect_id !== null) && (
							<div className="a-msginteract">
								<EffectMark id={m.effect_id} effect={m.effect} />
								<Reactions list={m.reactions} />
								<span className="a-muted sm a-msgcounts"><Counts k={{ ...m, pinned: false }} /></span>
							</div>
						)}
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
							{editList.more}
							<Timeline v={v} items={[
								...editList.shown.map((e) => ({
									icon: Pencil, at: e.at, title: e.content_type && e.content_type !== m.content_type ? `Earlier version (${contentLabel(e.content_type)})` : 'Earlier version',
									body: <>
										{e.media && (e.media.thumb
											? <a href={e.media.url ?? undefined} target="_blank" rel="noopener" className="a-editmedia" aria-label="Earlier file"><img src={e.media.thumb} alt="" width={72} height={72} loading="lazy" /></a>
											: <div className="a-muted sm">{e.media.url ? <a href={e.media.url} className="a-link">{e.media.name || 'earlier file'}</a> : `${e.media.name || 'earlier file'} (not stored)`}</div>)}
										{e.text && !richPage(e.detail) ? <RichText text={e.text} entities={e.entities} chat={chat} /> : !e.media && !e.detail && <span className="a-muted">(no text)</span>}
										{e.detail && <EditDetail d={e.detail} text={e.text} entities={e.entities} chat={chat} v={v} />}
									</>,
								})),
								...(m.edited_at ? [{ icon: Pencil, at: m.edited_at, title: 'Current version', body: <RichText text={m.text} entities={ents} chat={chat} />, tone: 'accent' as const }] : []),
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
						[null, 'Type', contentLabel(m.content_type)],
						[null, 'Sent', dateTime(m.date, v)],
						m.edited_at ? [null, 'Edited', dateTime(m.edited_at, v)] : null,
						m.deleted_at ? [null, 'Deleted', dateTime(m.deleted_at, v)] : null,
						m.media ? [null, 'File', <Link to={`/files?view=list&file=${m.media.file_id}`} className="a-rowlink">file #{fmt(m.media.file_id)}</Link>] : null,
						m.forwarded ? [null, 'Forwarded from', fromLink] : null,
						m.author_signature ? [null, 'Signature', m.author_signature] : null,
						svcType ? [null, 'Service action', <>{serviceLabel(svcType)} <span className="mono a-muted sm">{svcType}</span></>] : null,
						m.topic_id !== null && m.scope === 'group' ? [null, 'Forum topic', <Link to={`/groups/${m.chat.id}/chat?topic=${m.topic_id}`} className="a-rowlink">{m.topic ? <><TopicIcon t={m.topic} className="a-topicdot" />{m.topic.name}</> : `topic ${m.topic_id}`}</Link>] : null,
						m.views !== null ? [null, 'Views', fmt(m.views)] : null,
						m.forwards !== null ? [null, 'Forwards', fmt(m.forwards)] : null,
						m.replies !== null ? [null, 'Replies', fmt(m.replies)] : null,
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
