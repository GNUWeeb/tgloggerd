// A group's files and polls: the group_files and group_polls page
// routes (/groups/{id}/files?kind=, /groups/{id}/polls) and their JSON
// twins, GET /api/groups/{id}/files and /polls, which page backwards by
// ?before=<message id>, newest message first.
import { api, ok } from './client.ts';
import type { S } from './schema.ts';

// The kinds a group's files are listed by (?kind=); all is every kind.
export type FileKind = S['FileKindCount']['kind'];

// Who sent the message (TabSender), and the message a file or poll
// comes from (TabMessage).
export type GroupSender = S['TabSender'];
export type GroupMessageRef = S['TabMessage'];

export type GroupFileRow = S['GroupFileRow'];
export type GroupFilesPage = S['GroupFilesPage'];

export type GroupPollRow = S['GroupPollRow'];
export type GroupPollsPage = S['GroupPollsPage'];

export function fetchGroupFiles(id: number, kind: 'all' | FileKind, before: number): Promise<GroupFilesPage> {
	return ok(api.GET('/api/groups/{id}/files', { params: { path: { id }, query: { ...(kind !== 'all' ? { kind } : {}), before } } }));
}

export function fetchGroupPolls(id: number, before: number): Promise<GroupPollsPage> {
	return ok(api.GET('/api/groups/{id}/polls', { params: { path: { id }, query: { before } } }));
}
