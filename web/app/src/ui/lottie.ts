// Animated (.tgs) stickers: gzipped Lottie JSON, drawn by lottie-web.
// The player is imported on first use, so pages without such a
// sticker never load it. It is the "light" build: SVG only, and no
// expression support, so a sticker file can never run code.
import type { AnimationItem } from 'lottie-web';

type Lottie = typeof import('lottie-web').default;
let player: Promise<Lottie> | null = null;

export function loadPlayer(): Promise<Lottie> {
	player ??= import('lottie-web/build/player/esm/lottie_light.min.js').then((m) => m.default);
	return player;
}

// A .tgs file's animation: the bytes are gzip (Telegram's format), or
// plain JSON from a server that already unpacked them.
export async function loadTgs(url: string, signal?: AbortSignal): Promise<unknown> {
	const r = await fetch(url, { signal });
	if (!r.ok) throw new Error(`sticker: HTTP ${r.status}`);
	const bytes = new Uint8Array(await r.arrayBuffer());
	const gzip = bytes[0] === 0x1f && bytes[1] === 0x8b;
	const text = gzip
		? await new Response(new Blob([bytes]).stream().pipeThrough(new DecompressionStream('gzip'))).text()
		: new TextDecoder().decode(bytes);
	return JSON.parse(text);
}

// Play the sticker at url in el, from when el first comes into view;
// it pauses out of view, and stays on its first frame for a viewer who
// asked for reduced motion. onReady/onError report the outcome. The
// returned function stops and frees it.
export function playTgs(el: HTMLElement, url: string, onReady: () => void, onError: () => void): () => void {
	const abort = new AbortController();
	let anim: AnimationItem | null = null;
	let started = false;
	const still = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
	const start = async () => {
		started = true;
		try {
			const [lottie, data] = await Promise.all([loadPlayer(), loadTgs(url, abort.signal)]);
			if (abort.signal.aborted) return;
			anim = lottie.loadAnimation({ container: el, renderer: 'svg', loop: true, autoplay: !still, animationData: data });
			if (still) anim.goToAndStop(0, true);
			onReady();
		} catch {
			if (!abort.signal.aborted) onError();
		}
	};
	let io: IntersectionObserver | null = null;
	if (typeof IntersectionObserver === 'function') {
		io = new IntersectionObserver((entries) => {
			const seen = entries.some((e) => e.isIntersecting);
			if (seen && !started) void start();
			else if (anim && !still) { if (seen) anim.play(); else anim.pause(); }
		});
		io.observe(el);
	} else void start();
	return () => { abort.abort(); io?.disconnect(); anim?.destroy(); };
}
