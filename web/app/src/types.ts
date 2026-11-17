// Shapes of the page props the server sends. Hand-written until the
// OpenAPI description generates them (src/api/generated/).

export interface UserSummary {
	id: number;
	name: string;
	username: string;
	type: 'regular' | 'bot' | 'deleted' | 'unknown';
	photo: string | null;
	contact_name?: string;
	// Full-access viewers only; absent for members.
	msg_count?: number;
	is_verified?: boolean;
	is_premium?: boolean;
	is_scam?: boolean;
	is_fake?: boolean;
	is_support?: boolean;
	language?: string;
	created_at?: string;
	updated_at?: string;
}

export interface GroupSummary {
	id: number;
	title: string;
	username: string;
	type: 'supergroup' | 'basic_group' | 'channel';
	photo: string | null;
	msg_count?: number;
	exposed?: boolean;
}

export type Op = '=' | '!=' | '>' | '<' | 'contains' | 'empty' | 'is';
export interface Cond { field: string; op: Op; value: string }
export type FieldType = 'number' | 'text' | 'enum' | 'bool' | 'date';
export interface FieldDef { key: string; label: string; type: FieldType; values?: string[] }

export interface ListProps<T> {
	rows: T[];
	total: number;
	page: number;
	pages: number;
	per: number;
	sort: string;
	dir: 'asc' | 'desc';
	q: string;
	conds: Cond[];
}

export interface HistoryEntry {
	kind: 'name' | 'username' | 'bio' | 'phone' | 'photo';
	at: string;
	detail?: string;
	action?: 'added' | 'removed';
	photo?: string | null;
	removed?: boolean;
}

export interface GroupFull extends GroupSummary {
	description: string;
	members: number;
	msg_count: number;
	admin_count: number;
	created_at: string;
	updated_at: string;
}

export interface GroupAdmin {
	user: UserSummary;
	status: 'creator' | 'administrator';
	custom_title: string;
	is_anonymous: boolean;
	perms: string[];
	since: string;
}

export interface GroupHistoryEntry {
	kind: 'title' | 'description' | 'username' | 'photo' | 'admin';
	at: string;
	detail?: string;
	action?: string;
	photo?: string | null;
	removed?: boolean;
	user?: UserSummary;
}

export interface FileRow {
	id: number;
	file_type: 'photo' | 'video' | 'document' | 'audio' | 'voice' | 'sticker' | 'animation';
	name: string;
	ext: string;
	size: number;
	hits: number;
	stored: boolean;
	created_at: string;
	duration?: number;
	tg_file_id: string;
	url: string | null;
	thumb: string | null;
	chat: { id: number; title: string; kind: 'group' | 'user' };
	sender: { id: number; name: string } | null;
	message: { scope: 'group' | 'private'; id: number };
}

export interface ChatRef { id: number; title: string; kind: 'group' | 'user'; photo: string | null }

export interface MessageRow {
	id: number;
	scope: 'group' | 'private';
	message_id: number;
	date: string;
	chat: ChatRef;
	// null for messages the logging account sent (private chats).
	sender: UserSummary | null;
	content_type: 'text' | 'photo' | 'video' | 'document' | 'audio' | 'voice' | 'sticker' | 'animation' | 'service' | 'unknown';
	snippet: string;
	forwarded: string | null;
	edited: boolean;
	deleted_at: string | null;
}

export interface MessageFull extends MessageRow {
	text: string;
	reply_to: { scope: 'group' | 'private'; id: number; sender_name: string; snippet: string } | null;
	edits: { at: string; text: string }[];
	edited_at: string | null;
	media: { file_id: number; name: string; size: number; duration?: number; stored: boolean; thumb: string | null; url: string | null } | null;
}

export interface ChatMedia {
	stored: boolean;
	thumb: string | null;
	url: string | null;
	w?: number;
	h?: number;
	duration?: number;
	name?: string;
	size?: number;
	emoji?: string;
	wave?: number[];
}

export interface ChatMessage {
	id: number;
	message_id: number;
	date: string;
	scope: 'group' | 'private';
	// null: sent by the logging account.
	sender: UserSummary | null;
	content_type: 'text' | 'photo' | 'video' | 'document' | 'audio' | 'voice' | 'sticker' | 'animation' | 'service' | 'unknown';
	text: string;
	media: ChatMedia | null;
	album: ChatMedia[];
	reply_to: { message_id: number; sender_id: number; sender_name: string; snippet: string } | null;
	forward: string | null;
	edits: { at: string; text: string }[];
	edited_at: string | null;
	deleted_at: string | null;
	link_preview: { site: string; title: string; desc: string } | null;
}

export interface ChatWindow {
	messages: ChatMessage[];
	has_older: boolean;
	has_newer: boolean;
}
