// A photo that may load slowly or not at all: a photo whose bytes are
// no longer stored is fetched from Telegram on the fly, which can take a
// while or fail. A neutral box stands in while it loads and a placeholder
// titled "Photo not available" when it fails, never the browser's broken
// image. The box (className, width, height) keeps the photo's size in
// every state, so nothing moves when it arrives.
import { useEffect, useRef, useState } from 'react';
import { ImageOff } from 'lucide-react';

export type PhotoState = 'loading' | 'shown' | 'failed';

export const NOT_AVAILABLE = 'Photo not available';

// The load state of one picture, started over when its src changes.
// `ref` goes on the <img>: a picture rendered on the server may load or
// fail before the page is hydrated, its event fired before React
// listened, so it is checked once on mount. A finished picture with no
// width is broken if it cannot be decoded.
export function usePhotoState(src: string) {
	const [state, setState] = useState<{ src: string; s: PhotoState }>({ src, s: 'loading' });
	const ref = useRef<HTMLImageElement>(null);
	const s = state.src === src ? state.s : 'loading';
	useEffect(() => {
		const i = ref.current;
		if (!i?.complete) return;
		let live = true;
		const settle = (to: PhotoState) => { if (live) setState((x) => (x.src === src && x.s !== 'loading' ? x : { src, s: to })); };
		if (i.naturalWidth > 0) settle('shown');
		else i.decode().then(() => { if (i.naturalWidth > 0) settle('shown'); }, () => settle('failed'));
		return () => { live = false; };
	}, [src]);
	return {
		state: s, ref,
		onLoad: () => setState({ src, s: 'shown' }),
		onError: () => setState({ src, s: 'failed' }),
	};
}

export function PhotoImg({ src, alt, className = '', width, height, lazy, iconSize = 18 }: { src: string; alt: string; className?: string; width?: number; height?: number; lazy?: boolean; iconSize?: number }) {
	const { state, ref, onLoad, onError } = usePhotoState(src);
	const style = width !== undefined ? { width, height } : undefined;
	if (state === 'failed') {
		return <span className={`a-photo failed ${className}`} style={style} role="img" aria-label={NOT_AVAILABLE} title={NOT_AVAILABLE}><ImageOff size={iconSize} aria-hidden="true" /></span>;
	}
	return (
		<span className={`a-photo ${state} ${className}`} style={style} aria-busy={state === 'loading' || undefined}>
			<img ref={ref} src={src} alt={alt} width={width} height={height} loading={lazy ? 'lazy' : undefined} draggable={false} onLoad={onLoad} onError={onError} />
		</span>
	);
}
