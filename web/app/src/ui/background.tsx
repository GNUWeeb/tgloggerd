// A chat background as a small swatch on its service line and its
// message page (docs/todo/msg-chat-backgrounds.md): the wallpaper or
// pattern image over its fill, a fill's colours alone, or a chat
// theme's name. Only colours built from the fill's integers go into
// inline style; images are <img> elements.
import { useState, type CSSProperties } from 'react';
import { Download } from 'lucide-react';
import { Lightbox, plainClick } from '../routes/chat-media.tsx';
import { FetchButton, needsFetch, useFetched } from './fetchfile.tsx';
import type { S } from '../api/schema.ts';

// A service line's background or a link preview's (keyed alike; a
// preview's has no only_for_self and says dark only for a theme's).
type Background = Omit<S['ServiceBackgroundDetail'], 'kind' | 'only_for_self' | 'dark'> & { dark?: boolean };
type Fill = S['BackgroundFill'];

// 0xRRGGBB as #rrggbb; null for anything that is not such an integer.
export function rgb(n: unknown): string | null {
	if (typeof n !== 'number' || !Number.isInteger(n) || n < 0 || n > 0xffffff) return null;
	return `#${n.toString(16).padStart(6, '0')}`;
}

// Where Telegram puts a freeform gradient's colours (as fractions of the
// width and height), for three and for four colours.
const FREEFORM: Record<number, [number, number][]> = {
	3: [[0.8, 0.1], [0.2, 0.5], [0.6, 0.9]],
	4: [[0.8, 0.1], [0.35, 0.25], [0.2, 0.9], [0.65, 0.75]],
};

// The fill as a CSS background. A gradient's top colour is at the top
// at angle 0 and the angle turns it clockwise, so CSS's angle (which
// points to where the gradient ends) is 180 degrees on. A freeform
// gradient is approximated by a soft radial spot of each colour at its
// point, over the last colour.
export function fillCss(f: Fill | null | undefined): string | null {
	const cs = (f?.colors ?? []).map(rgb).filter((c): c is string => c !== null);
	if (!f || !cs.length) return null;
	if (f.type === 'solid' || cs.length === 1) return cs[0]!;
	if (f.type === 'gradient') {
		const turn = Number.isInteger(f.rotation_angle) ? f.rotation_angle : 0;
		return `linear-gradient(${(180 + turn) % 360}deg, ${cs[0]}, ${cs[1] ?? cs[0]})`;
	}
	const at = FREEFORM[cs.length] ?? FREEFORM[4]!;
	const spots = cs.slice(0, at.length).map((c, i) => `radial-gradient(circle at ${at[i]![0] * 100}% ${at[i]![1] * 100}%, ${c} 0, ${c}00 70%)`);
	return [...spots, cs[cs.length - 1]].join(', ');
}

// What the swatch shows, in words: its label and tooltip.
function describe(d: Background): string {
	const fill = d.fill ? { solid: 'a solid colour', gradient: 'a gradient', freeform_gradient: 'a freeform gradient' }[d.fill.type] : null;
	const bits: string[] = [];
	if (d.type === 'wallpaper') {
		bits.push('Wallpaper');
		if (d.is_blurred) bits.push('blurred');
	} else if (d.type === 'pattern') {
		bits.push(`Pattern${fill ? ` over ${fill}` : ''}`);
		if (d.intensity !== null) bits.push(`${d.intensity}% intensity`);
		if (d.is_inverted) bits.push('inverted');
	} else bits.push(fill ? `Fill of ${fill}` : 'Fill');
	if (d.is_moving) bits.push('moving');
	if (d.dark) bits.push('for a dark theme');
	return bits.join(', ');
}

// The swatch, size px high and three quarters of that wide (a phone's
// shape, as a background is). An image that can be shown opens in the
// viewer on a plain click; media loaded on click wait to be asked.
export function BackgroundSwatch({ d, size, onClick }: { d: Background; size: number; onClick: boolean }) {
	const [asked, setShown] = useState(!onClick);
	const [open, setOpen] = useState(false);
	// A big image not stored has the fetch button beside the swatch; once
	// fetched by it, it shows.
	const img = useFetched(d.image);
	const [fresh, setFresh] = useState(false);
	const shown = asked || fresh;
	if (!d.type || d.type === 'chat_theme') return null;
	// A PNG or JPEG draws as an image; Telegram's own pattern format (a
	// gzipped SVG, render file) cannot be drawn here.
	const drawable = !!img && img.render === 'image';
	const url = drawable ? img.url : null;
	const pic = drawable ? (url ? (img.thumb ?? url) : (img.minithumb ?? img.thumb ?? null)) : null;
	const pattern = d.type === 'pattern';
	const fill = fillCss(d.fill);
	const box: CSSProperties = { width: Math.round(size * 3 / 4), height: size, ...(fill ? { background: fill } : {}) };
	// A pattern is drawn at its intensity, as the image's opacity, over
	// the fill. An inverted pattern shows the fill through the pattern's
	// shapes on a dark ground: the pattern (black shapes in Telegram's
	// files) is inverted to white shapes on a black ground (c-bgpat) and
	// multiplied onto the fill, which keeps the fill under the shapes and
	// darkens it elsewhere; the intensity is that layer's opacity, so a
	// weak one leaves the ground less dark.
	const opacity = pattern ? Math.min(100, Math.max(0, d.intensity ?? 50)) / 100 : 1;
	const inverted = pattern && d.is_inverted;
	const lost = !!img && !img.url;
	const fetchable = drawable && needsFetch(img);
	const cls = ['c-bgswatch', d.is_blurred ? 'c-bgblur' : '', pattern && img && !drawable ? 'c-bghint' : '', !fill && !pic ? 'c-bgnone' : ''].filter(Boolean).join(' ');
	const label = `${describe(d)}${pattern && img && !drawable ? ' (the pattern is not drawn)' : fetchable ? ' (on Telegram)' : lost ? ' (not stored)' : ''}`;
	const fade = opacity < 1 ? { opacity } : undefined;
	const layer = pic && (inverted
		? <span className="c-bgpat" style={fade}><img src={pic} alt="" loading="lazy" /></span>
		: <img src={pic} alt="" loading="lazy" style={fade} />);
	if (!url) return <><span className={cls} style={box} role="img" aria-label={label} title={label}>{layer}</span>{fetchable && <FetchButton path={img.fetch!} compact onDone={() => setFresh(true)} />}</>;
	if (!shown) return <button type="button" className={`${cls} c-bgask`} style={box} onClick={() => setShown(true)} aria-label="Show the background" title={label}><Download size={Math.round(size / 3)} /></button>;
	return (
		<>
			<a className={cls} style={box} href={url} target="_blank" rel="noopener" aria-label={label} title={label}
				onClick={(e) => { if (plainClick(e)) { e.preventDefault(); setOpen(true); } }}>
				{layer}
			</a>
			{open && <Lightbox m={img!} kind="picture" onClose={() => setOpen(false)} />}
		</>
	);
}

// The background after the service line's text: the swatch, or the
// chat theme's name; nothing for a row logged before the background
// was kept.
export function BackgroundDetail({ d, size, onClick }: { d: Background; size: number; onClick: boolean }) {
	if (d.type === 'chat_theme') return d.theme_name ? <span className="c-svcdetail"> to the chat theme <q className="c-svcquote">{d.theme_name}</q></span> : <span className="c-svcdetail"> to a chat theme</span>;
	if (!d.type) return null;
	return <> <BackgroundSwatch d={d} size={size} onClick={onClick} /></>;
}
