// A window of a chat's messages, oldest first, for scrolling and live
// updates: GET /api/chats/{scope}/{id}/messages.
import type { ChatWindow } from '../types.ts';
import { api, ok } from './client.ts';

export type Anchor = { before: number } | { after: number } | { around: number } | { latest: true };

export async function fetchChatWindow(scope: 'group' | 'private', chatId: number, anchor: Anchor, limit = 60, signal?: AbortSignal): Promise<ChatWindow> {
	return ok(api.GET('/api/chats/{scope}/{id}/messages', {
		params: {
			path: { scope, id: chatId },
			query: {
				limit,
				before: 'before' in anchor ? anchor.before : undefined,
				after: 'after' in anchor ? anchor.after : undefined,
				around: 'around' in anchor ? anchor.around : undefined,
			},
		},
		signal,
	}));
}
