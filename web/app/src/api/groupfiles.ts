// A group's files and polls: the group_files and group_polls page
// routes (/groups/{id}/files?kind=, /groups/{id}/polls) and their JSON
// twins, GET /api/groups/{id}/files and /polls, which page backwards by
// ?before=<message id>, newest message first.
//
// LOCAL TYPES: the API description does not have these routes and
// shapes yet. They follow the agreed contract and live here, in one
// place; once webapi/openapi.yaml (and so the generated schema) has
// them, use the generated ones and drop these.
import type { Client } from 'openapi-fetch';
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

// The kinds a group's files are listed by (?kind=); all is every kind.
export const FILE_KINDS = ['photo', 'video', 'animation', 'audio', 'voice', 'document', 'sticker', 'video_note'] as const;
export type FileKind = (typeof FILE_KINDS)[number];

// Who sent the message: a user, or for a post as a chat that chat (a
// negative id); null when the viewer may not see them.
// id is null for a post as a chat the viewer does not see (named
// "(hidden chat)") and for a sender known only by its signature.
export interface GroupSender {
	id: number | null;
	name: string;
	username: string | null;
	photo: string | null;
}

// Where a file or poll was sent: the message in the group's chat
// (chat_link, "/groups/{id}/chat?at=<message_id>") and its message page
// (/messages/group/{message_row_id}).
export interface GroupMessageRef {
	message_id: number;
	message_row_id: number;
	date: string;
	sender: GroupSender;
	chat_link: string;
}

// A file of the group's messages: FileRow's own fields with the message.
export type GroupFileRow = Pick<S['FileRow'], 'name' | 'ext' | 'size' | 'file_type' | 'stored' | 'url' | 'thumb' | 'minithumb' | 'fetchable' | 'duration'> & GroupMessageRef;

export interface GroupFilesPage {
	// Only the kinds the group has; count null when it is not cheap.
	kinds: { kind: FileKind; count: number | null }[];
	kind: 'all' | FileKind;
	rows: GroupFileRow[];
	next_before: number | null;
}

// poll is null when the poll itself was never stored.
export type GroupPollRow = GroupMessageRef & { poll: S['PollDetail'] | null };

export interface GroupPollsPage {
	rows: GroupPollRow[];
	next_before: number | null;
}

// The two calls, typed as openapi-fetch takes them.
type Answer<R> = {
	200: { headers: { [name: string]: unknown }; content: { 'application/json': R } };
	default: { headers: { [name: string]: unknown }; content: { 'application/json': S['Error'] } };
};
type Get<Q, R> = {
	parameters: { query?: never; header?: never; path?: never; cookie?: never };
	get: { parameters: { query?: Q; header?: never; path: { id: number }; cookie?: never }; requestBody?: never; responses: Answer<R> };
	put?: never; post?: never; delete?: never; options?: never; head?: never; patch?: never; trace?: never;
};
interface LocalPaths {
	'/api/groups/{id}/files': Get<{ kind?: string; before?: number }, GroupFilesPage>;
	'/api/groups/{id}/polls': Get<{ before?: number }, GroupPollsPage>;
}
const local = api as unknown as Client<LocalPaths>;

export function fetchGroupFiles(id: number, kind: string, before: number): Promise<GroupFilesPage> {
	return ok(local.GET('/api/groups/{id}/files', { params: { path: { id }, query: { ...(kind !== 'all' ? { kind } : {}), before } } }));
}

export function fetchGroupPolls(id: number, before: number): Promise<GroupPollsPage> {
	return ok(local.GET('/api/groups/{id}/polls', { params: { path: { id }, query: { before } } }));
}
