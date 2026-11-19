// A message's link preview, shared by the chat and the message page.
import type { CSSProperties } from 'react';
import { ExternalLink, MessageSquare, Play } from 'lucide-react';
import { Link } from '../nav.tsx';
import { duration } from '../ui/media.tsx';
import { safeHref } from '../ui/richtext.tsx';
import { BackgroundSwatch, fillCss, rgb } from '../ui/background.tsx';
import { GiftCard } from '../ui/servicegift.tsx';
import { CustomEmoji } from '../ui/customemoji.tsx';
import { dateTime, fmt } from '../format.ts';
import { MediaOf, MessageMedia, Sticker, kindOf, type Media, type MediaKind } from './chat-media.tsx';
import { RefLink } from './chat-kinds.tsx';
import type { ChatMessage } from '../types.ts';
import type { Viewer } from '../page.ts';
import type { S } from '../api/schema.ts';

type LP = NonNullable<ChatMessage['link_preview']>;

const EXT = { target: '_blank', rel: 'noopener noreferrer nofollow' } as const;

// What a preview's type is called where saying it helps; an article, a
// type the daemon did not know and any newer one say nothing.
const TYPES: Record<string, string> = {
	photo: 'Photo', video: 'Video', animation: 'Animation', audio: 'Audio', document: 'Document', sticker: 'Sticker',
	embedded_video_player: 'Video player', embedded_animation_player: 'Animation player', embedded_audio_player: 'Audio player',
	external_video: 'Video', external_audio: 'Audio',
	message: 'Telegram message', story: 'Telegram story', chat: 'Telegram chat', user: 'Telegram user',
	video_note: 'Video message', voice_note: 'Voice message', album: 'Album', background: 'Chat background',
	sticker_set: 'Sticker set', gift_collection: 'Gift collection', gift_auction: 'Gift auction', upgraded_gift: 'Upgraded gift',
	live_story: 'Live story', story_album: 'Story album', theme: 'Theme', text_composition_style: 'Text style',
	direct_messages_chat: 'Direct messages', premium_gift_code: 'Premium gift code', request_managed_bot: 'Managed bot request',
	shareable_chat_folder: 'Chat folder',
};

// How the preview's own medium draws: by its render, as a message's
// would, but a sticker or a voice note keeps its own look and a video
// note is a (round) video. A sticker preview logged before its sticker
// was kept has only its picture (no format), which stays a picture.
function ownKind(type: string, m: Media): MediaKind | null {
	if (type === 'sticker') return m.format || m.render !== 'image' ? 'sticker' : 'photo';
	return kindOf(type === 'voice_note' ? 'voice' : type === 'video_note' ? 'video_note' : '', m);
}

// An album's items in the chat's album grid, each as its own kind;
// items TDLib left empty or not recorded are left out.
function Album({ a, onClick }: { a: NonNullable<LP['album']>; onClick: boolean }) {
	const items = a.items.filter((x): x is Media => !!x).map((m) => ({ ...m, kind: m.kind ?? kindOf('', m) ?? 'photo' }));
	if (!items.length) return null;
	return <div className="c-lp-album"><MessageMedia msg={{ content_type: items[0]!.kind, media: items[0]!, album: items.slice(1) }} onClick={onClick} /></div>;
}

// A sticker set's first stickers or a gift collection's icons, in a row.
function Stickers({ list }: { list: NonNullable<LP['stickers']> }) {
	const shown = list.filter((x): x is Media => !!x);
	if (!shown.length) return null;
	return <div className="c-lp-stickers">{shown.map((m, i) => <Sticker key={i} m={m} />)}</div>;
}

// A gift auction's or an upgraded gift's gift, drawn with the service
// lines' gift card (a preview names no sender, receiver or text), and
// an auction's dates and gifts per round.
function PreviewGift({ g, v }: { g: S['LinkPreviewGift']; v: Viewer }) {
	const d: S['ServiceGiftDetail'] = { kind: 'gift', sticker: g.sticker, symbol_sticker: g.symbol_sticker, star_count: g.star_count, price: null, sender: null, receiver: null, text: '', entities: [], upgraded: g.upgraded };
	const a = g.auction;
	const when = !!a && !!(a.start_date || a.end_date);
	const per = a?.gifts_per_round ?? null;
	return (
		<div className="c-lp-gift">
			<GiftCard d={d} v={v} />
			{a && (when || per !== null) && (
				<span className="c-lp-auction c-sub">
					{when && <>Auction{a.start_date && <> from <time dateTime={a.start_date}>{dateTime(a.start_date, v)}</time></>}{a.end_date && <> until <time dateTime={a.end_date}>{dateTime(a.end_date, v)}</time></>}</>}
					{when && per !== null && ' . '}
					{per !== null && `${fmt(per)} gift${per === 1 ? '' : 's'} per round`}
				</span>
			)}
		</div>
	);
}

const BASE: Record<string, string> = { classic: 'Classic', day: 'Day', night: 'Night', tinted: 'Tinted', arctic: 'Arctic' };

// A theme as a small swatch of its colours: its background's fill, an
// incoming and an outgoing bubble (in the outgoing fill, else the
// accent colour), and its accent colour. Its files are not kept, so
// nothing more of it can be drawn; it is never applied to the page.
function ThemeSwatch({ t }: { t: S['LinkPreviewTheme'] }) {
	const accent = rgb(t.accent_color);
	const out = fillCss(t.outgoing_message_fill) ?? rgb(t.outgoing_message_accent_color) ?? accent;
	const bg = fillCss(t.background?.fill);
	const base = t.base_theme ? BASE[t.base_theme] : null;
	const label = [base ? `Based on ${base}` : 'A theme', accent ? `accent ${accent}` : '', t.animate_outgoing_message_fill ? 'animated' : ''].filter(Boolean).join(', ');
	const dark = t.base_theme === 'night' || t.base_theme === 'tinted' || !!t.background?.dark;
	return (
		<div className="c-lp-themewrap">
			<span className={`c-lp-theme${dark ? ' dark' : ''}`} style={bg ? { background: bg } as CSSProperties : undefined} role="img" aria-label={label} title={label}>
				<span className="c-lp-theme-in" />
				<span className="c-lp-theme-out" style={out ? { background: out } : undefined} />
				{accent && <span className="c-lp-theme-accent" style={{ background: accent }} />}
			</span>
			<span className="c-sub">{label}{t.background?.name ? ` . ${t.background.name}` : ''}</span>
		</div>
	);
}

// A story's poster and a link to the story on Telegram: stories are
// not archived. The poster's username is one path segment.
function Story({ s }: { s: NonNullable<LP['story']> }) {
	const href = s.username ? safeHref(`https://t.me/${encodeURIComponent(s.username)}/s/${s.story_id}`) : null;
	if (!s.poster && !href) return null;
	return (
		<div className="c-lp-story">
			{s.poster && <span>Story by <RefLink r={s.poster} /></span>}
			{href && <a className="c-lp-embed" href={href} {...EXT}><ExternalLink size={12} />Open the story on Telegram</a>}
		</div>
	);
}

// A link preview: its site, title and description, its type, author and
// length, and its picture, linking to the page, or for a Telegram
// message link to the archived message when there is one. A preview
// with its own medium plays it with the chat's components, and the
// later types draw what they carry (an album, a background, stickers,
// a gift, a story's poster, a theme's colours), all beside the link:
// links do not nest. An embedded player only opens in a new tab: it is
// never run here. A field or type this card does not know leaves the
// plain card. Media load on click when onClick says so, as in the chat.
export function LinkPreview({ lp, v, onClick = false }: { lp: LP; v: Viewer; onClick?: boolean }) {
	const m0 = lp.media;
	const m = m0 && lp.type === 'video_note' && !m0.round ? { ...m0, round: true } : m0;
	const kind = m ? ownKind(lp.type, m) : null;
	// The medium itself, or else the preview's picture.
	const own = m && kind && kind !== 'photo' ? m : null;
	const pic = !own && m?.thumb ? m.thumb : null;
	const meta = [TYPES[lp.type], lp.author, lp.duration ? duration(lp.duration) : ''].filter(Boolean).join(' . ');
	const href = lp.url ? safeHref(lp.url) : null;
	const embed = lp.embed ? safeHref(lp.embed.url) : null;
	const bg = lp.background;
	const caption = lp.album?.caption && lp.album.caption !== lp.desc ? lp.album.caption : '';
	const inner = (
		<>
			<b className="c-lp-site">{lp.site}</b>
			<b>{lp.custom_emoji && <span className="c-lp-emoji"><CustomEmoji e={lp.custom_emoji} /></span>}{lp.title}</b>
			{lp.desc && <span className="c-lp-desc">{lp.desc}</span>}
			{caption && <span className="c-lp-desc">{caption}</span>}
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
			{lp.album && <Album a={lp.album} onClick={onClick} />}
			{bg && (bg.type === 'chat_theme'
				? bg.theme_name && <span className="c-lp-meta">The chat theme <q className="c-svcquote">{bg.theme_name}</q></span>
				: <div className="c-lp-bg"><BackgroundSwatch d={bg} size={120} onClick={onClick} /></div>)}
			{lp.stickers && <Stickers list={lp.stickers} />}
			{lp.gift && <PreviewGift g={lp.gift} v={v} />}
			{lp.story && <Story s={lp.story} />}
			{lp.theme && <ThemeSwatch t={lp.theme} />}
			{embed && <a className="c-lp-embed" href={embed} {...EXT}><Play size={12} fill="currentColor" />Open the player in a new tab</a>}
		</div>
	);
}
