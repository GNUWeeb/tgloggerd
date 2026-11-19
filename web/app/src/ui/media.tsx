// Media thumbnails and type icons. Real thumbnails come from the server
// (`thumb`, a /files/<token> URL); without one, an icon stands in.
import { FileText, Image, Video, Mic, Music, Sticker, Film, Play, CloudOff, Type as TextIcon, Info, CirclePlay, MapPin, Building2, Contact, Dices, Gamepad2, ChartColumn, ListChecks, CircleDashed, Star, Receipt, Gift, Phone, LayoutTemplate, AppWindow, TimerOff, CircleHelp, type LucideIcon } from 'lucide-react';
import type { S } from '../api/schema.ts';

export type ContentType = S['ContentType'];

// Every content type has an icon: a new one fails to compile here.
const ICON: Record<ContentType, LucideIcon> = {
	text: TextIcon, photo: Image, video: Video, document: FileText, audio: Music, voice: Mic, sticker: Sticker, animation: Film, service: Info, unknown: FileText,
	video_note: CirclePlay, location: MapPin, venue: Building2, contact: Contact, dice: Dices, game: Gamepad2, poll: ChartColumn, checklist: ListChecks,
	story: CircleDashed, paid_media: Star, invoice: Receipt, giveaway: Gift, call: Phone, rich: LayoutTemplate, widget: AppWindow, expired: TimerOff, unsupported: CircleHelp,
};

// Every content type, in the archive's order.
export const CONTENT_TYPES = Object.keys(ICON) as ContentType[];

export function TypeIcon({ type, size = 14 }: { type: ContentType; size?: number }) {
	const I = ICON[type] ?? FileText;
	return <I size={size} />;
}

export function duration(s: number): string {
	return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// The file types that play: their thumb is a still picture of them.
const VIDEO_TYPES: ReadonlySet<string> = new Set(['video', 'animation', 'video_note']);
export const isVideoType = (type: string) => VIDEO_TYPES.has(type);

// An <img> cannot draw a video or a .tgs (Lottie) sticker. Go sends a
// video's (and a .webm sticker's) still picture as its thumb; an older
// server sent such a file's own URL. So the check is on the thumb, not
// the file: a thumb that is a .webm or .tgs URL, or the file's own URL
// when the file plays or is a .webm or .tgs, is refused, and the type's
// icon stands in instead of a broken image.
const UNDRAWABLE = /\.(webm|tgs)$/i;
export type ThumbOf = { url?: string | null; ext?: string; type?: string };
export function drawableThumb(thumb: string | null | undefined, file: ThumbOf = {}): string | null {
	if (!thumb || UNDRAWABLE.test(new URL(thumb, 'http://x').pathname)) return null;
	const plays = (file.ext && UNDRAWABLE.test('.' + file.ext)) || (file.type && isVideoType(file.type));
	return plays && thumb === file.url ? null : thumb;
}

// A file's picture: its thumb, a video's still with the play mark over
// it, or a video's blurred preview (minithumb) when it has no still.
// The blurred preview also lies under the still while that loads. A
// video that is not stored keeps its still, marked as not stored.
export function Thumb({ type, thumb: given, url, minithumb, stored, ext, size = 36, radius = 8, fill = false }: { type: ContentType; thumb?: string | null; url?: string | null; minithumb?: string | null; stored: boolean; ext?: string; size?: number; radius?: number; fill?: boolean }) {
	const thumb = drawableThumb(given, { url, ext, type });
	const mini = minithumb || null;
	const style = fill ? { borderRadius: radius } : { width: size, height: size, borderRadius: radius };
	const cls = `a-thumb ${fill ? 'a-fill' : ''}`;
	if (!stored && !thumb && !mini) return <span className={`${cls} a-thumb-missing`} style={style} title="Not stored"><CloudOff size={Math.round(size * 0.4)} /></span>;
	if (thumb || mini) {
		const video = isVideoType(type);
		return (
			<span className={`${cls} ${thumb ? '' : 'a-thumb-blurred'}`} style={style} title={stored ? undefined : 'Not stored'}>
				{mini && <img className="a-thumb-mini" src={mini} alt="" />}
				{/* A filled box keeps its own size and shows the picture whole;
				    a small square thumbnail is cropped to fill it. */}
				{thumb && <img src={thumb} alt="" loading="lazy" style={{ width: '100%', height: '100%', objectFit: fill ? 'contain' : 'cover' }} />}
				{!stored
					? <span className="a-thumb-play"><CloudOff size={Math.round(size * 0.3)} /></span>
					: video && <span className="a-thumb-play"><Play size={Math.round(size * 0.3)} fill="currentColor" /></span>}
			</span>
		);
	}
	return (
		<span className={`${cls} a-thumb-doc a-thumb-${type}`} style={style}>
			<TypeIcon type={type} size={Math.round(size * 0.42)} />
			{size >= 48 && ext && <span className="a-thumb-ext">{ext}</span>}
		</span>
	);
}
