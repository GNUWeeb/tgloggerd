// Custom emoji, and message effects, drawn at the size of the text
// around them: a WebP sticker as an image, a WebM one as a muted
// looping video, a TGS one with the chat's Lottie player. Without a
// file (url null, or a format this view does not know) the fallback
// emoji is drawn instead, and it is the alt text of the others.
//
// A page can hold hundreds of them, so they stay cheap: one shared
// IntersectionObserver watches them all, nothing is fetched or played
// until one is in view or hovered, they pause out of view, and each
// .tgs file is fetched and unpacked once however often it appears.
import { useEffect, useRef, useState, type ReactNode } from 'react';
import type { AnimationItem } from 'lottie-web';
import type { S } from '../api/schema.ts';
import { loadPlayer, loadTgs } from './lottie.ts';

// A CustomEmoji, or a MessageEffect (which is never repainted).
export type EmojiFile = Pick<S['CustomEmoji'], 'emoji' | 'format' | 'url'> & { needs_repainting?: boolean };

type Seen = (inView: boolean) => void;
const watched = new Map<Element, Seen>();
let observer: IntersectionObserver | null = null;

// Call seen with whether el is in view, now and on every change; the
// returned function stops. Without IntersectionObserver it counts as
// in view.
function watch(el: Element, seen: Seen): () => void {
	if (typeof IntersectionObserver !== 'function') {
		seen(true);
		return () => {};
	}
	observer ??= new IntersectionObserver((entries) => {
		for (const e of entries) watched.get(e.target)?.(e.isIntersecting);
	});
	watched.set(el, seen);
	observer.observe(el);
	return () => { watched.delete(el); observer?.unobserve(el); };
}

// lottie-web only reads an animation's data (it marks it done the
// first time), so every copy of an emoji shares one.
const animations = new Map<string, Promise<unknown>>();
function animationOf(url: string): Promise<unknown> {
	let p = animations.get(url);
	if (!p) {
		p = loadTgs(url);
		animations.set(url, p);
		p.catch(() => animations.delete(url));
	}
	return p;
}

const reducedMotion = () => typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;

// A .tgs emoji: the fallback until it first comes into view or under
// the pointer, then the animation, playing while in view (or, for a
// viewer who asked for reduced motion, only while hovered).
function LottieEmoji({ url, alt, className }: { url: string; alt: string; className: string }) {
	const box = useRef<HTMLSpanElement>(null);
	const ctl = useRef<{ hover: (on: boolean) => void } | null>(null);
	const [state, setState] = useState<'still' | 'ready' | 'failed'>('still');
	useEffect(() => {
		const el = box.current;
		if (!el) return;
		setState('still');
		const still = reducedMotion();
		let anim: AnimationItem | null = null;
		let started = false;
		let gone = false;
		let inView = false;
		let hovered = false;
		const sync = () => {
			if (!anim) return;
			if (hovered || (inView && !still)) anim.play();
			else if (still) anim.goToAndStop(0, true);
			else anim.pause();
		};
		const start = async () => {
			started = true;
			try {
				const [lottie, data] = await Promise.all([loadPlayer(), animationOf(url)]);
				if (gone) return;
				anim = lottie.loadAnimation({ container: el, renderer: 'svg', loop: true, autoplay: false, animationData: data });
				anim.goToAndStop(0, true);
				sync();
				setState('ready');
			} catch {
				if (!gone) setState('failed');
			}
		};
		const nudge = () => { if (!started) void start(); else sync(); };
		const stop = watch(el, (v) => { inView = v; if (v || started) nudge(); });
		ctl.current = { hover: (on) => { hovered = on; if (on || started) nudge(); } };
		return () => { gone = true; ctl.current = null; stop(); anim?.destroy(); };
	}, [url]);
	return (
		<span className={`${className} ce-tgs ${state}`} role="img" aria-label={alt} title={state === 'failed' ? `${alt} (could not be played)` : undefined}
			onMouseEnter={() => ctl.current?.hover(true)} onMouseLeave={() => ctl.current?.hover(false)}>
			<span ref={box} className="ce-box" aria-hidden="true" />
			{state !== 'ready' && <span className="ce-alt" aria-hidden="true">{alt}</span>}
		</span>
	);
}

// A .webm emoji: loads nothing until in view, plays while in view (or
// while hovered, for reduced motion), and the fallback if it fails.
function VideoEmoji({ url, alt, className, onFail }: { url: string; alt: string; className: string; onFail: () => void }) {
	const ref = useRef<HTMLVideoElement>(null);
	useEffect(() => {
		const v = ref.current;
		if (!v) return;
		const still = reducedMotion();
		return watch(v, (inView) => {
			if (inView && v.preload === 'none') v.preload = 'auto';
			if (inView && !still) v.play()?.catch(() => {});
			else v.pause();
		});
	}, [url]);
	const hover = (on: boolean) => {
		const v = ref.current;
		if (!v || !reducedMotion()) return;
		if (on) v.play()?.catch(() => {});
		else v.pause();
	};
	return <video ref={ref} className={`${className} ce-webm`} src={url} muted loop playsInline preload="none" aria-label={alt} onError={onFail} onMouseEnter={() => hover(true)} onMouseLeave={() => hover(false)} />;
}

// CSS url() for a URL: quoted, its quotes and backslashes escaped.
const cssUrl = (u: string) => `url("${u.replace(/["\\\n]/g, (c) => `\\${c === '\n' ? 'a ' : c}`)}")`;

// A custom emoji or effect e, at text size. alt is the emoji it stands
// for; children (default: alt) are drawn when e cannot be.
//
// needs_repainting: the sticker is a one-colour template drawn in the
// text colour. A WebP one becomes a CSS mask over a box of
// currentColor, which is exact. A TGS one has its SVG's fills and
// strokes set to currentColor by CSS (lottie-web sets them as
// attributes, which CSS overrides). A video cannot be a mask, so a
// WebM one is filtered to the theme's text colour as near as filters
// get: black in the light theme, white in the dark.
export function CustomEmoji({ e, alt, children }: { e: EmojiFile | null | undefined; alt?: string; children?: ReactNode }) {
	const [failed, setFailed] = useState(false);
	const text = alt || e?.emoji || '';
	const fallback = children ?? text;
	useEffect(() => setFailed(false), [e?.url]);
	if (!e?.url || !e.format || failed) return <>{fallback}</>;
	const cls = `ce${e.needs_repainting ? ' ce-repaint' : ''}`;
	switch (e.format) {
		case 'tgs': return <LottieEmoji url={e.url} alt={text} className={cls} />;
		case 'webm': return <VideoEmoji url={e.url} alt={text} className={cls} onFail={() => setFailed(true)} />;
		case 'webp': return e.needs_repainting
			? <span className={`${cls} ce-mask`} role="img" aria-label={text} style={{ maskImage: cssUrl(e.url), WebkitMaskImage: cssUrl(e.url) }} />
			: <img className={cls} src={e.url} alt={text} loading="lazy" decoding="async" draggable={false} onError={() => setFailed(true)} />;
		default: return <>{fallback}</>;
	}
}
