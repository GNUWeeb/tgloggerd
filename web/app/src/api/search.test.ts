import { afterEach, describe, expect, it, vi } from 'vitest';
import { searchAllChats, searchChats } from './search.ts';

const item = (id: number) => ({ id, title: `g${id}`, kind: 'group' as const });

afterEach(() => vi.unstubAllGlobals());

describe('chat search', () => {
	it('builds the query string', async () => {
		const f = vi.fn(async () => new Response(JSON.stringify({ items: [], next_cursor: null })));
		vi.stubGlobal('fetch', f);
		await searchChats({ q: 'rust', kind: 'group', federation: 0, cursor: 'c1' });
		expect(f).toHaveBeenCalledWith('/api/chats/search?q=rust&kind=group&federation=0&limit=20&cursor=c1', expect.anything());
		await searchChats({ kind: 'any' });
		expect(f).toHaveBeenLastCalledWith('/api/chats/search?limit=20', expect.anything());
	});

	it('follows the cursor to the end', async () => {
		const pages: Record<string, unknown> = { '': { items: [item(1), item(2)], next_cursor: 'a' }, a: { items: [item(3)], next_cursor: null } };
		vi.stubGlobal('fetch', vi.fn(async (url: string) => new Response(JSON.stringify(pages[new URL(url, 'http://x').searchParams.get('cursor') ?? '']))));
		const r = await searchAllChats({ kind: 'group', federation: 1 });
		expect(r.items.map((i) => i.id)).toEqual([1, 2, 3]);
		expect(r.truncated).toBe(false);
	});

	it('stops at the cap', async () => {
		vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({ items: [item(1), item(2)], next_cursor: 'again' }))));
		const r = await searchAllChats({}, 5);
		expect(r.items).toHaveLength(6);
		expect(r.truncated).toBe(true);
	});

	it('throws on an error status', async () => {
		vi.stubGlobal('fetch', vi.fn(async () => new Response('{}', { status: 500 })));
		await expect(searchChats({})).rejects.toThrow('HTTP 500');
	});
});
