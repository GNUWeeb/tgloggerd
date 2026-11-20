// The entity cache (web-live-client.md section 2): users' and groups'
// display fields and counters, fetched again through GET /api/entities
// when an event names them, so a rename or a new count shows in every
// list, cell and bubble without fetching whole pages again.
//
// Only ids on screen (mounted) are fetched. Requests are collected for
// a moment and sent together, at most 200 ids per kind per request.
// The server leaves out ids the viewer may not see and unknown ones; a
// missing id is "not available" until the next page load and is never
// asked for again in the meantime. The cache keeps at most `cap`
// entries per kind, dropping the least recently used ones that are not
// on screen.
import type { Clock } from './manager.ts';
import type { Visibility } from './refetch.ts';
import type { GroupSummary, UserSummary } from '../types.ts';

export type EntityKind = 'user' | 'group';
type Value<K extends EntityKind> = K extends 'user' ? UserSummary : GroupSummary;
interface Entry<V> { v: V; at: number }

export type EntityFetcher = (users: number[], groups: number[]) => Promise<{ users: UserSummary[]; groups: GroupSummary[] }>;

export const BATCH_MS = 500;
export const MAX_IDS = 200;
export const CAP = 2000;

export class EntityStore {
	private data = { user: new Map<number, Entry<UserSummary>>(), group: new Map<number, Entry<GroupSummary>>() };
	private missing = { user: new Set<number>(), group: new Set<number>() };
	private mounted = { user: new Map<number, number>(), group: new Map<number, number>() };
	private pending = { user: new Set<number>(), group: new Set<number>() };
	private timer: unknown = null;
	private deferred = false;
	private listeners = new Set<() => void>();
	private mountListeners = new Set<() => void>();
	private stopVisible: () => void;
	version = 0;

	constructor(private fetcher: EntityFetcher, private clock: Clock, private vis: Visibility, private cap = CAP) {
		this.stopVisible = vis.onVisible(() => { if (this.deferred) { this.deferred = false; void this.flush(); } });
	}

	get<K extends EntityKind>(kind: K, id: number): Entry<Value<K>> | undefined {
		const m = this.data[kind] as Map<number, Entry<Value<K>>>;
		const e = m.get(id);
		if (e) { m.delete(id); m.set(id, e); } // most recently used last
		return e;
	}

	// While mounted, an id is fetched again when an event names it.
	mount(kind: EntityKind, id: number): () => void {
		const m = this.mounted[kind];
		const first = !m.has(id);
		m.set(id, (m.get(id) ?? 0) + 1);
		if (first) this.mountsChanged();
		return () => {
			const n = (m.get(id) ?? 1) - 1;
			if (n > 0) m.set(id, n);
			else { m.delete(id); this.mountsChanged(); }
		};
	}

	mountedIds(kind: EntityKind): number[] {
		return [...this.mounted[kind].keys()];
	}

	invalidate(kind: EntityKind, ids: number[]): void {
		let any = false;
		for (const id of ids) {
			if (!this.mounted[kind].has(id) || this.missing[kind].has(id)) continue;
			this.pending[kind].add(id);
			any = true;
		}
		if (any && this.timer === null) this.timer = this.clock.setTimeout(() => { this.timer = null; void this.flush(); }, BATCH_MS);
	}

	invalidateMounted(): void {
		this.invalidate('user', this.mountedIds('user'));
		this.invalidate('group', this.mountedIds('group'));
	}

	// A new page load: what was missing may be visible again.
	pageLoaded(): void {
		this.missing.user.clear();
		this.missing.group.clear();
	}

	subscribe(fn: () => void): () => void {
		this.listeners.add(fn);
		return () => this.listeners.delete(fn);
	}

	onMountsChanged(fn: () => void): () => void {
		this.mountListeners.add(fn);
		return () => this.mountListeners.delete(fn);
	}

	dispose(): void {
		this.clock.clearTimeout(this.timer);
		this.stopVisible();
		this.listeners.clear();
		this.mountListeners.clear();
	}

	private mountsChanged(): void {
		for (const fn of this.mountListeners) fn();
	}

	private async flush(): Promise<void> {
		if (this.vis.hidden()) { this.deferred = true; return; }
		const users = [...this.pending.user];
		const groups = [...this.pending.group];
		this.pending.user.clear();
		this.pending.group.clear();
		for (let i = 0; i < Math.max(users.length, groups.length); i += MAX_IDS) {
			const u = users.slice(i, i + MAX_IDS);
			const g = groups.slice(i, i + MAX_IDS);
			let got;
			try { got = await this.fetcher(u, g); } catch { continue; } // the next event tries again
			const at = this.clock.now();
			this.store('user', u, got.users, at);
			this.store('group', g, got.groups, at);
		}
		this.version++;
		for (const fn of this.listeners) fn();
	}

	private store<K extends EntityKind>(kind: K, asked: number[], got: Value<K>[], at: number): void {
		const m = this.data[kind] as Map<number, Entry<Value<K>>>;
		const seen = new Set<number>();
		for (const v of got) {
			seen.add(v.id);
			m.delete(v.id);
			m.set(v.id, { v, at });
		}
		for (const id of asked) {
			if (seen.has(id)) continue;
			this.missing[kind].add(id);
			m.delete(id);
		}
		for (const id of m.keys()) {
			if (m.size <= this.cap) break;
			if (!this.mounted[kind].has(id)) m.delete(id);
		}
	}
}
