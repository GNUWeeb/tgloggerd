import { afterEach, describe, expect, it, vi } from 'vitest';
import { searchAllChats, searchChats } from './search.ts';

const item = (id: number) => ({ id, title: `g${id}`, kind: 'group' as const });

afterEach(() => vi.unstubAllGlobals());

describe('chat search', () => {
	it('builds the query string', async () => {
		const f = vi.fn(async () => new Response(JSON.stringify({ items: [], next_cursor: null })));
		vi.stubGlobal('fetch', f);
		await searchChats({ q: 'rust', kind: 'group', federation: 0, cursor: 'c1' });
		const url = (n: number) => { const u = new URL((f.mock.calls[n] as unknown as [Request])[0].url); return u.pathname + u.search; };
		expect(url(0)).toBe('/api/chats/search?q=rust&kind=group&federation=0&limit=20&cursor=c1');
		await searchChats({ kind: 'any' });
		expect(url(1)).toBe('/api/chats/search?limit=20');
	});

	it('follows the cursor to the end', async () => {
		const pages: Record<string, unknown> = { '': { items: [item(1), item(2)], next_cursor: 'a' }, a: { items: [item(3)], next_cursor: null } };
		vi.stubGlobal('fetch', vi.fn(async (req: Request) => new Response(JSON.stringify(pages[new URL(req.url).searchParams.get('cursor') ?? '']), { headers: { 'content-type': 'application/json' } })));
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
		await expect(searchChats({})).rejects.toMatchObject({ status: 500 });
	});
});
