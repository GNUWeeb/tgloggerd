// One page scroll lock shared by every overlay (dialogs, viewers). The
// first lock saves the body's own overflow and hides it; only the last
// release puts the saved value back, so overlays opened inside each
// other can close in any order.
import { useEffect } from 'react';

let held = 0;
let saved = '';

export function lockScroll(): () => void {
	if (held++ === 0) {
		saved = document.body.style.overflow;
		document.body.style.overflow = 'hidden';
	}
	let done = false;
	return () => {
		if (done) return;
		done = true;
		if (--held === 0) document.body.style.overflow = saved;
	};
}

// Lock the page's scrolling while the calling component is mounted.
export function useScrollLock(): void {
	useEffect(() => lockScroll(), []);
}
