// React side of the entity cache: one store per tab, fed by the live
// events for the users and groups on screen, and hooks that show a
// fetched value in place of the page's when it is newer than the page.
import { createContext, useContext, useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from 'react';
import { EntityStore, type EntityKind } from './entities.ts';
import { useLive } from './react.tsx';
import { useNav } from '../nav.tsx';
import { api, ok } from '../api/client.ts';
import type { GroupSummary, UserSummary } from '../types.ts';

interface Ctx { store: EntityStore; pageAt: { current: number } }
const EntityCtx = createContext<Ctx | null>(null);

const clock = { now: () => Date.now(), setTimeout: (fn: () => void, ms: number) => setTimeout(fn, ms), clearTimeout: (id: unknown) => clearTimeout(id as ReturnType<typeof setTimeout>) };
const visibility = {
	hidden: () => document.hidden,
	onVisible: (fn: () => void) => { const on = () => { if (!document.hidden) fn(); }; document.addEventListener('visibilitychange', on); return () => document.removeEventListener('visibilitychange', on); },
};

const fetchEntities = (users: number[], groups: number[]) => ok(api.GET('/api/entities', {
	params: { query: { users: users.length ? users.join(',') : undefined, groups: groups.length ? groups.join(',') : undefined } },
})).then((r) => ({ users: r.users ?? [], groups: r.groups ?? [] }));

export function EntityProvider({ enabled, children }: { enabled: boolean; children: ReactNode }) {
	const { page } = useNav();
	const [ctx, setCtx] = useState<Ctx | null>(null);
	const [topics, setTopics] = useState<string[] | null>(null);
	useEffect(() => {
		if (!enabled) return;
		const store = new EntityStore(fetchEntities, clock, visibility);
		const sync = () => {
			const t = [...store.mountedIds('user').map((id) => `user:${id}`), ...store.mountedIds('group').map((id) => `group:${id}`)];
			setTopics(t.length ? [...t, 'stats'] : null);
		};
		const off = store.onMountsChanged(sync);
		setCtx({ store, pageAt: { current: Date.now() } });
		return () => { off(); store.dispose(); setCtx(null); setTopics(null); };
	}, [enabled]);
	useEffect(() => { if (ctx) { ctx.pageAt.current = Date.now(); ctx.store.pageLoaded(); } }, [ctx, page]);
	useLive(topics, (e) => {
		const s = ctx?.store;
		if (!s) return;
		if (e.kind === 'user.updated' && e.user_id !== undefined) s.invalidate('user', [e.user_id]);
		else if (e.kind === 'group.updated' && e.chat_id !== undefined) s.invalidate('group', [e.chat_id]);
		else if (e.kind === 'stats.changed') {
			s.invalidate('user', e.user_ids ?? []);
			s.invalidate('group', (e.chat_ids ?? []).filter((id) => id < 0));
		} else if (e.kind === 'resync') s.invalidateMounted();
	});
	return <EntityCtx.Provider value={ctx}>{children}</EntityCtx.Provider>;
}

const noop = () => () => {};

function useStoreVersion(store: EntityStore | undefined): number {
	return useSyncExternalStore(store ? (fn) => store.subscribe(fn) : noop, () => store?.version ?? 0, () => 0);
}

function useMounted(kind: EntityKind, ids: number[]): void {
	const ctx = useContext(EntityCtx);
	const key = ids.join(',');
	useEffect(() => {
		if (!ctx || !key) return;
		const offs = key.split(',').map((id) => ctx.store.mount(kind, Number(id)));
		return () => offs.forEach((f) => f());
	}, [ctx, kind, key]);
}

// The rows of a list, each with the cache's value where it is newer than
// the page. Rows keep their order: a list never reorders on its own.
export function useFreshRows<T extends { id: number }>(kind: EntityKind, rows: T[]): T[] {
	const ctx = useContext(EntityCtx);
	const version = useStoreVersion(ctx?.store);
	useMounted(kind, rows.map((r) => r.id));
	return useMemo(() => {
		if (!ctx) return rows;
		return rows.map((r) => {
			const e = ctx.store.get(kind, r.id);
			return e && e.at > ctx.pageAt.current ? { ...r, ...e.v } : r;
		});
		// `version` changes when the cache does.
	}, [ctx, kind, rows, version]);
}

export function useFreshUser<T extends UserSummary | null | undefined>(u: T): T {
	const rows = useMemo(() => (u ? [u] : []), [u]);
	const fresh = useFreshRows('user', rows as UserSummary[]);
	return (u ? fresh[0] : u) as T;
}

export function useFreshGroup<T extends GroupSummary | null | undefined>(g: T): T {
	const rows = useMemo(() => (g ? [g] : []), [g]);
	const fresh = useFreshRows('group', rows as GroupSummary[]);
	return (g ? fresh[0] : g) as T;
}
