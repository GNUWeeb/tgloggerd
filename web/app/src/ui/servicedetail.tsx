// What a service message's stored detail adds to its line: the new
// group photo, how long a video chat ran or when it is planned, what was
// pinned, the poll option added or removed, the topic and the community
// named, the title a group was created with, a background set for the
// actor alone (docs/todo/msg-service-detail.md). The chat's service
// line and the message page both draw it.
import { useEffect, useState, type ReactNode } from 'react';
import { CloudOff, Download, Play } from 'lucide-react';
import { BackgroundDetail } from './background.tsx';
import { Link } from '../nav.tsx';
import { Avatar } from './primitives.tsx';
import { Lightbox, plainClick } from '../routes/chat-media.tsx';
import { TopicIcon } from '../routes/chat-kinds.tsx';
import { GiftCard, ThemeDetail } from './servicegift.tsx';
import { dateTime } from '../format.ts';
import type { Viewer } from '../page.ts';
import type { ChatMedia } from '../types.ts';
import { FetchButton, needsFetch, useFetched } from './fetchfile.tsx';
import type { S } from '../api/schema.ts';

// The detail, tagged by kind (webapi/openapi.yaml ServiceDetail);
// absent or null where there is none.
export type ServiceDetail = S['ServiceDetail'];
type Topic = S['ServiceTopic'];

// A video chat's length in words: "45 s", "12 min", "1 h 2 min".
export function lasted(s: number): string {
	if (s < 60) return `${s} s`;
	const m = Math.floor(s / 60);
	if (m < 60) return `${m} min`;
	return m % 60 ? `${Math.floor(m / 60)} h ${m % 60} min` : `${m / 60} h`;
}

// The new photo, small and round as Telegram shows a group's photo; a
// plain click opens the viewer. Media loaded on click wait to be asked.
// An animated photo's video plays in the viewer, the still as its
// poster; the line still shows the still.
// A big photo or animation not stored, with a fetch path, has the
// fetch button beside it (the animation's first: it is what plays);
// once fetched by it, it shows and opens at once.
function NewPhoto({ m: givenPhoto, anim: givenAnim, size, onClick }: { m: ChatMedia; anim: ChatMedia | null; size: number; onClick: boolean }) {
	const m = useFetched(givenPhoto);
	const anim = useFetched(givenAnim);
	const [asked, setShown] = useState(!onClick);
	const [open, setOpen] = useState(false);
	const [fresh, setFresh] = useState(false);
	const shown = asked || fresh;
	useEffect(() => { if (fresh) setOpen(true); }, [fresh]);
	const box = { width: size, height: size };
	const video = anim?.url ? anim : null;
	if (!m.url && !video) {
		const pic = m.minithumb ?? m.thumb;
		const path = anim && needsFetch(anim) ? anim.fetch! : needsFetch(m) ? m.fetch! : null;
		const label = path ? 'Photo on Telegram' : 'Photo not stored';
		return <>
			<span className={`c-svcphoto c-missing ${pic ? 'c-blurred' : ''}`} style={{ ...box, ...(pic ? { backgroundImage: `url("${pic}")` } : {}) }} title={label} role="img" aria-label={label}><CloudOff size={Math.round(size / 3)} /></span>
			{path && <FetchButton path={path} compact onDone={() => setFresh(true)} />}
		</>;
	}
	if (!shown) return <button type="button" className="c-svcphoto c-missing" style={box} onClick={() => setShown(true)} aria-label="Show the photo"><Download size={Math.round(size / 3)} /></button>;
	const still = m.url ? (m.thumb ?? m.url) : null;
	return (
		<>
			<a className={`c-svcphoto ${video ? 'c-svcanim' : ''}`} style={box} href={(video ?? m).url!} target="_blank" rel="noopener" aria-label={video ? 'The new animated photo' : 'The new photo'}
				onClick={(e) => { if (plainClick(e)) { e.preventDefault(); setOpen(true); } }}>
				{still && <img src={still} alt="" width={size} height={size} loading="lazy" />}
				{video && <Play className="c-svcplay" size={Math.max(10, Math.round(size / 3))} fill="currentColor" aria-hidden="true" />}
			</a>
			{open && (video
				? <Lightbox m={{ ...video, poster: video.poster ?? m.url ?? undefined }} kind="video" onClose={() => setOpen(false)} />
				: <Lightbox m={m} kind="picture" onClose={() => setOpen(false)} />)}
		</>
	);
}

// Telegram's first topic colour, for a topic whose colour is not known.
const DEFAULT_TOPIC_COLOR = 0x6fb9f0;
const topicName = (t: Topic) => t.name || 'a topic';

// The topic as a link to it, with its icon (or colour dot); only its
// name when its id is not known.
export function TopicLink({ t, hrefOf }: { t: Topic; hrefOf: (id: number) => string }) {
	const inner = <><TopicIcon t={{ icon_color: t.icon_color ?? DEFAULT_TOPIC_COLOR, icon_custom_emoji: t.icon_custom_emoji }} />{topicName(t)}</>;
	return t.id !== null ? <Link to={hrefOf(t.id)} className="c-svctopic">{inner}</Link> : <span className="c-svctopic">{inner}</span>;
}

// A topic's name in a service text ('created topic "Help"'), the name
// made the topic's link; null when the text does not quote it.
const quoted = (text: string, d: ServiceDetail | null | undefined) => (d?.kind === 'topic' && d.topic.name ? text.indexOf(`"${d.topic.name}"`) : -1);
export function topicInText(text: string, d: ServiceDetail | null | undefined, hrefOf: (id: number) => string): ReactNode | null {
	const at = quoted(text, d);
	if (at < 0 || d?.kind !== 'topic') return null;
	return <>{text.slice(0, at)}"<TopicLink t={d.topic} hrefOf={hrefOf} />"{text.slice(at + d.topic.name!.length + 2)}</>;
}

// The detail after the line's text, starting with its own space or
// colon. text is the line's text, so a topic it already names (and
// links, by topicInText) is not repeated; self says the actor is the
// viewer, for "for yourself".
export function ServiceDetailView({ service, text, v, topicHref, self = false, photoSize = 28 }: { service: S['ServiceInfo'] | null | undefined; text: string; v: Viewer; topicHref: (id: number) => string; self?: boolean; photoSize?: number }) {
	const d = service?.detail;
	if (!d) return null;
	const onClick = v.chat?.media === 'click';
	switch (d.kind) {
		case 'photo':
			return <> <NewPhoto m={d.photo} anim={d.animation} size={photoSize} onClick={onClick} /></>;
		case 'video_chat':
			if (d.duration !== null && d.duration > 0) return <span className="c-svcdetail"> after {lasted(d.duration)}</span>;
			if (d.start_date) return <span className="c-svcdetail"> for <time dateTime={d.start_date}>{dateTime(d.start_date, v)}</time></span>;
			return null;
		case 'pin':
			return d.snippet ? <> <q className="c-svcquote">{d.snippet}</q></> : null;
		case 'poll_option':
			return <>: <q className="c-svcquote">{d.text}</q></>;
		case 'topic':
			return quoted(text, d) >= 0 ? null : <>: <TopicLink t={d.topic} hrefOf={topicHref} /></>;
		// A community has no page of its own (its id is TDLib's, not a
		// chat's): its name with its photo, unlinked.
		case 'community':
			return <>: <span className="c-svccommunity"><Avatar id={d.community.id} name={d.community.name} photo={d.community.photo} size={16} />{d.community.name}</span></>;
		case 'title': {
			const name = d.title || d.basic_group?.title;
			return name ? <> <q className="c-svcquote">{d.basic_group ? <Link to={`/groups/${d.basic_group.id}`} className="c-svcref">{name}</Link> : name}</q></> : null;
		}
		case 'background':
			// Its swatch (src/ui/background.tsx): 48px high on a line,
			// the photo's size on the message page.
			return <><BackgroundDetail d={d} size={Math.max(48, photoSize)} onClick={onClick} />{d.only_for_self && <span className="c-svcdetail"> for {self ? 'yourself' : 'themselves'}</span>}</>;
		case 'gift':
			return <GiftCard d={d} v={v} />;
		case 'theme':
			return <ThemeDetail d={d} />;
	}
}
