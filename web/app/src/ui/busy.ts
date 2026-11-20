// Loads someone is waiting for: page navigations, and data an area of
// the page asked for on request (a search, "load more", a photo, older
// messages). Each is tracked while it runs; the loading pill shows once
// one has run for a while. Background work (live updates, polling,
// periodic refreshes) is not tracked: nobody waits for it.
type Listener = () => void;

let running = 0;
const listeners = new Set<Listener>();
const tell = () => { for (const fn of listeners) fn(); };

// Count `p` as running until it settles; it is returned as it is.
export function track<T>(p: Promise<T>): Promise<T> {
	running++;
	tell();
	const done = () => { running--; tell(); };
	p.then(done, done);
	return p;
}

export function busyCount(): number {
	return running;
}

export function onBusy(fn: Listener): () => void {
	listeners.add(fn);
	return () => { listeners.delete(fn); };
}
