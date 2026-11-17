// Throttled refetches (web-live-client.md section 5). Each key runs at
// most once per its interval: a request inside the interval schedules
// one trailing run, never several. A burst of requests in the same tick
// becomes one run. At most `max` runs at once; the rest wait. While the
// tab is hidden nothing runs: keys are only marked stale and run once
// when it shows again.
import type { Clock } from './manager.ts';

export interface Visibility {
	hidden(): boolean;
	onVisible(fn: () => void): () => void;
}

interface Entry { fn: () => Promise<unknown>; interval: number; last: number; timer: unknown; running: boolean; again: boolean; queued: boolean }

const realVisibility: Visibility = {
	hidden: () => typeof document !== 'undefined' && document.hidden,
	onVisible: (fn) => {
		const on = () => { if (!document.hidden) fn(); };
		document.addEventListener('visibilitychange', on);
		return () => document.removeEventListener('visibilitychange', on);
	},
};

export class Refetcher {
	private entries = new Map<string, Entry>();
	private stale = new Set<string>();
	private queue: string[] = [];
	private running = 0;
	private stopVisible: () => void;

	constructor(private clock: Clock, private vis: Visibility = realVisibility, private max = 4) {
		this.stopVisible = vis.onVisible(() => {
			const keys = [...this.stale];
			this.stale.clear();
			for (const k of keys) this.schedule(k);
		});
	}

	// Ask for `fn` to run under `key`. The latest fn for a key wins.
	request(key: string, interval: number, fn: () => Promise<unknown>): void {
		let e = this.entries.get(key);
		if (!e) { e = { fn, interval, last: -Infinity, timer: null, running: false, again: false, queued: false }; this.entries.set(key, e); }
		e.fn = fn;
		e.interval = interval;
		if (this.vis.hidden()) { this.stale.add(key); return; }
		this.schedule(key);
	}

	// Drop a key, e.g. when the component that asked for it unmounts.
	forget(key: string): void {
		const e = this.entries.get(key);
		if (e) this.clock.clearTimeout(e.timer);
		this.entries.delete(key);
		this.stale.delete(key);
		this.queue = this.queue.filter((k) => k !== key);
	}

	dispose(): void {
		for (const e of this.entries.values()) this.clock.clearTimeout(e.timer);
		this.entries.clear();
		this.stopVisible();
	}

	private schedule(key: string): void {
		const e = this.entries.get(key);
		if (!e) return;
		if (e.running) { e.again = true; return; }
		if (e.timer !== null || e.queued) return;
		const wait = Math.max(0, e.last + e.interval - this.clock.now());
		e.timer = this.clock.setTimeout(() => { e.timer = null; this.start(key); }, wait);
	}

	private start(key: string): void {
		const e = this.entries.get(key);
		if (!e) return;
		if (this.vis.hidden()) { this.stale.add(key); return; }
		if (this.running >= this.max) { e.queued = true; this.queue.push(key); return; }
		e.running = true;
		e.last = this.clock.now();
		this.running++;
		const done = () => {
			this.running--;
			e.running = false;
			const next = this.queue.shift();
			if (next) { const n = this.entries.get(next); if (n) n.queued = false; this.start(next); }
			if (e.again && this.entries.get(key) === e) { e.again = false; this.schedule(key); }
		};
		e.fn().catch(() => undefined).finally(done);
	}
}
