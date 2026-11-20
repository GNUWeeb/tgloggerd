// Chat search for the pickers: GET /api/chats/search. The server
// filters by the viewer's scope; an empty `q` lists the busiest chats
// first. Paged by an opaque cursor.
import type { PickItem } from '../types.ts';
import type { S } from './schema.ts';
import { api, ok } from './client.ts';

export interface ChatQuery {
	kind?: 'group' | 'user' | 'any';
	// Only the groups of one federation (0 is Default).
	federation?: number;
}

export type ChatSearchPage = S['ChatSearchPage'];

export async function searchChats(query: ChatQuery & { q?: string; limit?: number; cursor?: string | null }, signal?: AbortSignal): Promise<ChatSearchPage> {
	return ok(api.GET('/api/chats/search', {
		params: {
			query: {
				q: query.q || undefined,
				kind: query.kind && query.kind !== 'any' ? query.kind : undefined,
				federation: query.federation,
				limit: query.limit ?? 20,
				cursor: query.cursor || undefined,
			},
		},
		signal,
	}));
}

// Every match, following the cursor, for editors that must send a
// complete list back (a federation's groups). Stops at `max` so a
// runaway cursor cannot loop forever; `truncated` says it did.
export async function searchAllChats(query: ChatQuery, max = 5000, signal?: AbortSignal): Promise<{ items: PickItem[]; truncated: boolean }> {
	const items: PickItem[] = [];
	let cursor: string | null = null;
	do {
		const page: ChatSearchPage = await searchChats({ ...query, limit: 200, cursor }, signal);
		items.push(...page.items);
		cursor = page.next_cursor;
	} while (cursor && items.length < max);
	return { items, truncated: !!cursor };
}
