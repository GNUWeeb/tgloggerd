// A "Loading..." pill at the top centre of the window while a page or
// a requested piece of data takes longer than DELAY_MS to arrive. Fast
// loads never show it, so it does not flicker; it goes as soon as
// everything waited for has arrived. Its slot is an .app of its own, for
// the theme's colours on every page.
import { useEffect, useState, useSyncExternalStore } from 'react';
import { LoaderCircle } from 'lucide-react';
import { busyCount, onBusy } from './busy.ts';
import { useNav } from '../nav.tsx';

export const DELAY_MS = 300;

export function LoadingPill() {
	const { loading } = useNav();
	const tracked = useSyncExternalStore(onBusy, () => busyCount() > 0, () => false);
	const waiting = loading || tracked;
	const [shown, setShown] = useState(false);
	useEffect(() => {
		if (!waiting) { setShown(false); return; }
		const t = setTimeout(() => setShown(true), DELAY_MS);
		return () => clearTimeout(t);
	}, [waiting]);
	return (
		<div className="app a-loadpill-slot" aria-live="polite">
			{shown && <span className="a-loadpill"><LoaderCircle size={14} className="a-spin" />Loading...</span>}
		</div>
	);
}
