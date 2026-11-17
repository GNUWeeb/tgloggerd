import { describe, expect, it } from 'vitest';
import { pageLive } from './pages.ts';
import type { PageData } from '../page.ts';

const page = (route: string, url: string, props: unknown = {}) => ({ route, url, props, viewer: { signed_in: true } }) as unknown as PageData;

describe('pageLive', () => {
	it('refreshes a user page on that user only', () => {
		const s = pageLive(page('user', '/users/42'))!;
		expect(s.topics).toEqual(['user:42']);
		expect(s.mode).toBe('refresh');
		expect(s.match({ seq: 1, kind: 'user.updated', user_id: 42 })).toBe(true);
		expect(s.match({ seq: 2, kind: 'group.updated', chat_id: -1 })).toBe(false);
	});

	it('offers a refresh on the first page of group messages for group chats only', () => {
		const s = pageLive(page('group_messages', '/group_messages'))!;
		expect(s.mode).toBe('notice');
		expect(s.match({ seq: 1, kind: 'message.new', chat_id: -100 })).toBe(true);
		expect(s.match({ seq: 2, kind: 'message.new', chat_id: 7 })).toBe(false);
		expect(pageLive(page('group_messages', '/group_messages?page=3'))!.match({ seq: 3, kind: 'message.new', chat_id: -100 })).toBe(false);
	});

	it('follows one message on its page', () => {
		const s = pageLive(page('message', '/messages/group/9', { message: { chat: { id: -5 }, message_id: 77 } }))!;
		expect(s.topics).toEqual(['chat:-5']);
		expect(s.match({ seq: 1, kind: 'message.edited', chat_id: -5, message_id: 77 })).toBe(true);
		expect(s.match({ seq: 2, kind: 'message.edited', chat_id: -5, message_id: 78 })).toBe(false);
	});

	it('leaves settings and admin pages alone', () => {
		for (const r of ['settings', 'accounts', 'mcp_tokens', 'not_found']) expect(pageLive(page(r, '/x'))).toBeNull();
	});
});
