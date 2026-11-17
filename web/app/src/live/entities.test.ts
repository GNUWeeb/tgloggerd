import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EntityStore, BATCH_MS } from './entities.ts';
import type { Clock } from './manager.ts';
import type { GroupSummary, UserSummary } from '../types.ts';

const clock: Clock = { now: () => Date.now(), setTimeout: (fn, ms) => setTimeout(fn, ms), clearTimeout: (id) => clearTimeout(id as never) };
const user = (id: number, name = `u${id}`) => ({ id, name, username: '', type: 'regular', photo: null }) as UserSummary;
const group = (id: number, title = `g${id}`) => ({ id, title, username: '', type: 'supergroup', photo: null }) as GroupSummary;

function vis() {
	let hidden = false; let fns: (() => void)[] = [];
	return { hidden: () => hidden, onVisible: (fn: () => void) => { fns.push(fn); return () => { fns = fns.filter((f) => f !== fn); }; }, set(h: boolean) { hidden = h; if (!h) fns.forEach((f) => f()); } };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe('EntityStore', () => {
	it('batches mounted ids into one request after a pause and stores the answer', async () => {
		const fetcher = vi.fn(async (u: number[], g: number[]) => ({ users: u.map((id) => user(id, 'renamed')), groups: g.map((id) => group(id)) }));
		const s = new EntityStore(fetcher, clock, vis());
		s.mount('user', 1); s.mount('user', 2); s.mount('group', -5);
		s.invalidate('user', [1]); s.invalidate('user', [2, 3]); s.invalidate('group', [-5]);
		await vi.advanceTimersByTimeAsync(BATCH_MS - 1);
		expect(fetcher).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(1);
		expect(fetcher).toHaveBeenCalledTimes(1);
		expect(fetcher).toHaveBeenCalledWith([1, 2], [-5]); // 3 is not on screen
		expect(s.get('user', 1)?.v.name).toBe('renamed');
		expect(s.version).toBe(1);
	});

	it('sends at most 200 ids per request', async () => {
		const fetcher = vi.fn(async (u: number[]) => ({ users: u.map((id) => user(id)), groups: [] }));
		const s = new EntityStore(fetcher, clock, vis());
		const ids = Array.from({ length: 450 }, (_, i) => i + 1);
		ids.forEach((id) => s.mount('user', id));
		s.invalidate('user', ids);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		expect(fetcher.mock.calls.map((c) => c[0].length)).toEqual([200, 200, 50]);
	});

	it('never asks again for an id the server left out, until the next page', async () => {
		const fetcher = vi.fn(async (u: number[]) => ({ users: u.filter((id) => id !== 9).map((id) => user(id)), groups: [] }));
		const s = new EntityStore(fetcher, clock, vis());
		s.mount('user', 9); s.mount('user', 1);
		s.invalidate('user', [9, 1]);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		expect(s.get('user', 9)).toBeUndefined();
		s.invalidate('user', [9]);
		await vi.advanceTimersByTimeAsync(BATCH_MS * 4);
		expect(fetcher).toHaveBeenCalledTimes(1);
		s.pageLoaded();
		s.invalidate('user', [9]);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		expect(fetcher).toHaveBeenCalledTimes(2);
	});

	it('ignores ids no longer on screen', async () => {
		const fetcher = vi.fn(async () => ({ users: [], groups: [] }));
		const s = new EntityStore(fetcher, clock, vis());
		const off = s.mount('user', 1);
		off();
		s.invalidate('user', [1]);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		expect(fetcher).not.toHaveBeenCalled();
	});

	it('waits while the tab is hidden', async () => {
		const v = vis();
		const fetcher = vi.fn(async (u: number[]) => ({ users: u.map((id) => user(id)), groups: [] }));
		const s = new EntityStore(fetcher, clock, v);
		s.mount('user', 1);
		v.set(true);
		s.invalidate('user', [1]);
		await vi.advanceTimersByTimeAsync(BATCH_MS * 4);
		expect(fetcher).not.toHaveBeenCalled();
		v.set(false);
		await vi.advanceTimersByTimeAsync(0);
		expect(fetcher).toHaveBeenCalledTimes(1);
	});

	it('keeps at most `cap` entries, dropping the least recently used off screen', async () => {
		const fetcher = vi.fn(async (u: number[]) => ({ users: u.map((id) => user(id)), groups: [] }));
		const s = new EntityStore(fetcher, clock, vis(), 3);
		const offs = [1, 2, 3, 4].map((id) => s.mount('user', id));
		s.invalidate('user', [1, 2, 3]);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		s.get('user', 1); // used: now the most recent
		offs.forEach((f) => f());
		s.mount('user', 4);
		s.invalidate('user', [4]);
		await vi.advanceTimersByTimeAsync(BATCH_MS);
		expect([1, 2, 3, 4].map((id) => !!s.get('user', id))).toEqual([true, false, true, true]);
	});

	it('reports when the ids on screen change', () => {
		const s = new EntityStore(async () => ({ users: [], groups: [] }), clock, vis());
		const fn = vi.fn();
		s.onMountsChanged(fn);
		const a = s.mount('user', 1);
		const b = s.mount('user', 1);
		a();
		expect(fn).toHaveBeenCalledTimes(1);
		b();
		expect(fn).toHaveBeenCalledTimes(2);
		expect(s.mountedIds('user')).toEqual([]);
	});
});
