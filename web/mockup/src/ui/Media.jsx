// Stand-ins for media: gradients for photos/videos, icons for documents.
import { FileText, Image, Video, Mic, Music, Sticker, Film, Play, CloudOff, Type as TextIcon, Info } from 'lucide-react';

export const TYPE_ICON = {
	text: TextIcon, photo: Image, video: Video, document: FileText, audio: Music,
	voice: Mic, sticker: Sticker, animation: Film, service: Info, unknown: FileText,
};

export function TypeIcon({ type, size = 14 }) {
	const I = TYPE_ICON[type] || FileText;
	return <I size={size} />;
}

export function duration(s) {
	return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}

// f: { file_type, hue, ext, emoji, duration, stored }
export function Thumb({ f, size = 36, radius = 8, className = '' }) {
	const style = { width: size, height: size, borderRadius: radius };
	if (!f.stored) {
		return <span className={`a-thumb a-thumb-missing ${className}`} style={style} title="Not stored"><CloudOff size={Math.round(size * 0.4)} /></span>;
	}
	if (f.file_type === 'photo' || f.file_type === 'video' || f.file_type === 'animation') {
		return (
			<span className={`a-thumb ${className}`} style={{ ...style, background: `linear-gradient(135deg, hsl(${f.hue} 65% 64%), hsl(${(f.hue + 50) % 360} 60% 42%))` }}>
				{f.file_type !== 'photo' && <Play size={Math.round(size * 0.36)} fill="currentColor" />}
			</span>
		);
	}
	if (f.file_type === 'sticker') return <span className={`a-thumb a-thumb-sticker ${className}`} style={{ ...style, fontSize: size * 0.6 }}>{f.emoji || '🙂'}</span>;
	return (
		<span className={`a-thumb a-thumb-doc a-thumb-${f.file_type} ${className}`} style={style}>
			<TypeIcon type={f.file_type} size={Math.round(size * 0.42)} />
			{size >= 48 && f.ext && <span className="a-thumb-ext">{f.ext}</span>}
		</span>
	);
}
