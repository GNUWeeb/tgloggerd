// A window of a chat's messages, oldest first, for scrolling and live
// updates. Hand-written until the generated client exists; the Go port
// of today's /v1/chats/{scope}/{id}/messages answers it.
import type { ChatWindow } from '../types.ts';

export type Anchor = { before: number } | { after: number } | { around: number } | { latest: true };

export async function fetchChatWindow(scope: 'group' | 'private', chatId: number, anchor: Anchor, limit = 60, signal?: AbortSignal): Promise<ChatWindow> {
	const q = new URLSearchParams({ limit: String(limit) });
	if ('before' in anchor) q.set('before', String(anchor.before));
	if ('after' in anchor) q.set('after', String(anchor.after));
	if ('around' in anchor) q.set('around', String(anchor.around));
	const res = await fetch(`/api/chats/${scope}/${chatId}/messages?${q}`, { credentials: 'same-origin', signal });
	if (!res.ok) throw new Error(`chat window: HTTP ${res.status}`);
	return (await res.json()) as ChatWindow;
}
