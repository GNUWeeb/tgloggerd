// What each page listens to, and what an event does to it
// (web-live-client.md section 4), until the entity cache exists:
// - 'refresh' refetches the page's data in place (values change where
//   they are; detail pages and the dashboard);
// - 'notice' leaves the page alone and offers a refresh (lists, whose
//   rows must not move under the reader).
// The chat view handles its own events.
import type { PageData } from '../page.ts';
import type { LiveEvent } from './events.ts';
import type { S } from '../api/schema.ts';

export interface PageLive {
	topics: string[];
	mode: 'refresh' | 'notice';
	interval: number; // minimum ms between refetches of this page
	match: (e: LiveEvent) => boolean;
}

const idIn = (url: string, re: RegExp) => { const m = re.exec(new URL(url, 'http://x').pathname); return m ? Number(m[1]) : null; };
const firstPage = (url: string) => { const q = new URL(url, 'http://x').searchParams; return !q.get('page') || q.get('page') === '1'; };

export function pageLive(page: PageData): PageLive | null {
	switch (page.route) {
		case 'dashboard':
			// Its counts follow new messages themselves (routes/dashboard.tsx);
			// the rest of the page refreshes when the stats change.
			return { topics: ['stats'], mode: 'refresh', interval: 5000, match: (e) => e.kind === 'stats.changed' };
		case 'user':
		case 'user_history': {
			const id = idIn(page.url, /^\/users\/(\d+)/);
			return id === null ? null : { topics: [`user:${id}`], mode: 'refresh', interval: 2000, match: (e) => e.kind === 'user.updated' };
		}
		case 'group':
		case 'group_admins':
		case 'group_history': {
			const id = idIn(page.url, /^\/groups\/(-?\d+)/);
			return id === null ? null : { topics: [`group:${id}`], mode: 'refresh', interval: 2000, match: (e) => e.kind === 'group.updated' };
		}
		case 'message': {
			const m = (page.props as { message?: { chat: { id: number }; message_id: number } }).message;
			if (!m) return null;
			return {
				topics: [`chat:${m.chat.id}`], mode: 'refresh', interval: 1000,
				match: (e) => e.message_id === m.message_id && (e.kind === 'message.edited' || e.kind === 'message.deleted' || e.kind === 'file.stored'),
			};
		}
		case 'group_messages':
		case 'private_messages': {
			const groups = page.route === 'group_messages';
			// New rows go on the first page of the default order only.
			return {
				topics: ['messages'], mode: 'notice', interval: 2000,
				match: (e) => e.kind === 'message.new' && e.chat_id !== undefined && (e.chat_id < 0) === groups && firstPage(page.url),
			};
		}
		case 'bots':
			return { topics: ['bots'], mode: 'refresh', interval: 1000, match: (e) => e.kind === 'bot.updated' };
		case 'fwd_discord_telegram': {
			// The routes show their bots' status: follow the bots they use.
			const ids = new Set((page.props as S['D2TProps']).routes.flatMap((r) => [r.discord_bot?.id, r.telegram_bot.id]));
			return { topics: ['bots'], mode: 'refresh', interval: 1000, match: (e) => e.kind === 'bot.updated' && e.bot_id !== undefined && ids.has(e.bot_id) };
		}
		case 'files':
			return { topics: ['messages'], mode: 'notice', interval: 2000, match: (e) => e.kind === 'file.stored' && firstPage(page.url) };
		default:
			return null;
	}
}
