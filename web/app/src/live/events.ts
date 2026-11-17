// Events of GET /api/events (web-live.md section 3). They name what
// changed, never the content; the page refetches through the API.
export type LiveKind =
	| 'message.new' | 'message.edited' | 'message.deleted' | 'file.stored'
	| 'user.updated' | 'group.updated' | 'stats.changed'
	| 'resync' | 'session';

export const LIVE_KINDS: LiveKind[] = ['message.new', 'message.edited', 'message.deleted', 'file.stored', 'user.updated', 'group.updated', 'stats.changed', 'resync', 'session'];

export interface LiveEvent {
	seq: number;
	kind: LiveKind;
	chat_id?: number;
	message_id?: number;
	user_id?: number;
	file_id?: number;
	chat_ids?: number[];
	user_ids?: number[];
	fields?: string[];
	// session events: why the stream ends.
	reason?: 'scope_changed' | 'revoked';
}

// The topics an event belongs to (web-live-client.md section 3).
// resync and session reach every subscriber.
export function topicsOf(e: LiveEvent): string[] | 'all' {
	switch (e.kind) {
		case 'message.new':
		case 'message.edited':
		case 'message.deleted':
		case 'file.stored':
			return [`chat:${e.chat_id}`, 'messages'];
		case 'user.updated':
			return [`user:${e.user_id}`];
		case 'group.updated':
			return [`group:${e.chat_id}`];
		case 'stats.changed':
			return ['stats'];
		default:
			return 'all';
	}
}
