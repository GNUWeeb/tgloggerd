// A message's link preview, shared by the chat and the message page.
import { ExternalLink, MessageSquare, Play } from 'lucide-react';
import { Link } from '../nav.tsx';
import { duration } from '../ui/media.tsx';
import { safeHref } from '../ui/richtext.tsx';
import { MediaOf, kindOf } from './chat-media.tsx';
import type { ChatMessage } from '../types.ts';

type LP = NonNullable<ChatMessage['link_preview']>;

const EXT = { target: '_blank', rel: 'noopener noreferrer nofollow' } as const;

// What a preview's type is called where saying it helps; an article, a
// type the daemon did not know and any newer one say nothing.
const TYPES: Record<string, string> = {
	photo: 'Photo', video: 'Video', animation: 'Animation', audio: 'Audio', document: 'Document', sticker: 'Sticker',
	embedded_video_player: 'Video player', embedded_animation_player: 'Animation player', embedded_audio_player: 'Audio player',
	external_video: 'Video', external_audio: 'Audio',
	message: 'Telegram message', story: 'Telegram story', chat: 'Telegram chat', user: 'Telegram user',
};

// A link preview: its site, title and description, its type, author and
// length, and its picture, linking to the page, or for a Telegram
// message link to the archived message when there is one. A preview
// with its own video, animation or audio plays it with the chat's
// components (beside the link: links do not nest), and an embedded
// player only opens in a new tab: it is never run here. Media load on
// click when onClick says so, as in the chat.
export function LinkPreview({ lp, onClick = false }: { lp: LP; onClick?: boolean }) {
	const m = lp.media;
	const kind = m ? kindOf('', m) : null;
	// The medium itself, or else the preview's picture.
	const own = m && kind && kind !== 'photo' ? m : null;
	const pic = !own && m?.thumb ? m.thumb : null;
	const meta = [TYPES[lp.type], lp.author, lp.duration ? duration(lp.duration) : ''].filter(Boolean).join(' . ');
	const href = lp.url ? safeHref(lp.url) : null;
	const embed = lp.embed ? safeHref(lp.embed.url) : null;
	const inner = (
		<>
			<b className="c-lp-site">{lp.site}</b><b>{lp.title}</b>
			{lp.desc && <span className="c-lp-desc">{lp.desc}</span>}
			{meta && <span className="c-lp-meta">{meta}</span>}
			{pic && <img className="c-lp-img" src={pic} alt="" loading="lazy" style={m?.w && m.h ? { aspectRatio: `${m.w} / ${m.h}` } : undefined} />}
		</>
	);
	const main = lp.target
		? <Link className="c-lp-main" to={`/groups/${lp.target.chat_id}/chat?at=${lp.target.message_id}`}>{inner}<span className="c-lp-url"><MessageSquare size={11} />Open the message in the archive</span></Link>
		: href
			? <a className="c-lp-main" href={href} {...EXT}>{inner}<span className="c-lp-url"><ExternalLink size={11} />{href.replace(/^https?:\/\//, '')}</span></a>
			: <div className="c-lp-main">{inner}</div>;
	return (
		<div className="c-lp">
			{main}
			{own && kind && <MediaOf m={own} kind={kind} single onClick={onClick} />}
			{embed && <a className="c-lp-embed" href={embed} {...EXT}><Play size={12} fill="currentColor" />Open the player in a new tab</a>}
		</div>
	);
}
