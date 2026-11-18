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

export function TypeIcon({ type, size = 14 }: { type: ContentType; size?: number }) {
	const I = ICON[type] ?? FileText;
	return <I size={size} />;
}

export function duration(s: number): string {
	return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

export function Thumb({ type, thumb, stored, ext, size = 36, radius = 8, fill = false }: { type: ContentType; thumb?: string | null; stored: boolean; ext?: string; size?: number; radius?: number; fill?: boolean }) {
	const style = fill ? { borderRadius: radius } : { width: size, height: size, borderRadius: radius };
	const cls = `a-thumb ${fill ? 'a-fill' : ''}`;
	if (!stored) return <span className={`${cls} a-thumb-missing`} style={style} title="Not stored"><CloudOff size={Math.round(size * 0.4)} /></span>;
	if (thumb) {
		return (
			<span className={cls} style={style}>
				<img src={thumb} alt="" loading="lazy" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
				{(type === 'video' || type === 'animation') && <span className="a-thumb-play"><Play size={Math.round(size * 0.3)} fill="currentColor" /></span>}
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
