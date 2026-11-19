// A group's files and polls in the stand-in (the group_files and
// group_polls page routes and their JSON twins GET
// /api/groups/{id}/files and /polls), as the Go service is to answer
// them: the files of the group's messages and its poll messages, newest
// message first, 50 to a page, paged backwards by ?before=<message id>.
// GroupFileRow has no duration in webapi/openapi.yaml yet; the rows
// carry it as the service is to send it.
//
// The files follow what the chat shows: a message's media and its album
// items, kinds as the chat renders them (every fourth video is a GIF, a
// sticker is webp, webm or tgs by its file id), and the video note of
// the kinds sample. The kinds sample group (Assembly Lounge) has every
// kind and more than one page; it counts its kinds, the other groups do
// not (null). A channel's posts are sent as the channel.
import { groups } from '../../mockup/src/data/groups.js';
import { chatStream, senderOf } from '../../mockup/src/data/messages.js';
import { KINDS_GROUP, kindsOf, sampleOf, type View } from './kinds.ts';

export const GROUP_FILES_PAGE = 50;
export const FILE_KINDS = ['photo', 'video', 'animation', 'audio', 'voice', 'document', 'sticker', 'video_note'] as const;
type Kind = (typeof FILE_KINDS)[number];

// What the mockup data gives a message, loosely.
type Msg = {
	id: number; message_id: number; date: string; is_outgoing?: boolean; content_type: string;
	chat: { id: number; kind: string; type?: string; title?: string; username?: string | null };
	media?: { fileId: number; ext?: string; name?: string; size?: number; stored?: boolean; w?: number; h?: number; duration?: number } | null;
	album?: { fileId: number; size?: number; name?: string }[];
};
type Sender = { id: number | null; name: string; username: string; photo: string | null };

const iso = (s: string) => s.replace(' ', 'T') + 'Z';
const STICKER_FORMATS = ['webp', 'webm', 'tgs'];
const EXT_OF: Record<string, string> = { photo: 'jpg', video: 'mp4', animation: 'mp4', voice: 'ogg', audio: 'mp3', document: 'bin', video_note: 'mp4' };

// Who sent a message, as the viewer may see them: a channel's post is
// sent as the channel; one the viewer may not see is named but has no
// id, as Go sends a hidden chat ("(hidden chat)"). Never null; no
// username is an empty one. Every third sender has a profile photo.
function senderFor(m: Msg, v: View): Sender {
	if (m.chat.type === 'channel') return { id: m.chat.id, name: m.chat.title ?? '', username: m.chat.username ?? '', photo: null };
	const s = senderOf(m) as { id: number; name: string; username?: string | null } | null;
	if (!s || !s.id || !v.user(s.id)) return { id: null, name: '(hidden chat)', username: '', photo: null };
	return { id: s.id, name: s.name, username: s.username ?? '', photo: s.id % 3 === 0 ? `/files/standin-${800000 + (s.id % 100000)}` : null };
}

// A file as the list carries it: FileRow's own fields and the message.
function fileOf(m: Msg, md: { fileId: number; ext?: string; name?: string; size?: number; stored?: boolean; duration?: number }, kind: Kind, v: View) {
	const stored = md.stored !== false;
	const fetchable = !stored && v.fetch;
	const ext = kind === 'sticker' ? STICKER_FORMATS[md.fileId % 3]! : md.ext ?? EXT_OF[kind] ?? 'bin';
	const url = stored || fetchable ? `/files/standin-${md.fileId}${ext === 'tgs' ? '.tgs' : ''}` : null;
	// Only a stored picture an <img> can draw has a thumb here.
	const picture = kind === 'photo' || (kind === 'sticker' && ext === 'webp');
	return {
		id: md.fileId, file_type: kind === 'video_note' ? 'video_note' : kind, name: md.name || `${kind}_${md.fileId}.${ext}`, ext,
		size: md.size || 30000 + (md.fileId % 997) * 311, stored, fetchable, url, thumb: stored && picture ? url : null,
		...(md.duration ? { duration: md.duration } : {}),
		message_id: m.message_id, message_row_id: m.id, date: iso(m.date), sender: senderFor(m, v),
		chat_link: `/groups/${m.chat.id}/chat?at=${m.message_id}`,
	};
}

type FileItem = ReturnType<typeof fileOf>;

// Every file of a group, newest message first.
const fileCache = new Map<string, FileItem[]>();
function filesOf(g: { id: number }, v: View, key: string): FileItem[] {
	const k = `${g.id}:${key}`;
	const hit = fileCache.get(k);
	if (hit) return hit;
	const out: FileItem[] = [];
	for (const m of [...(chatStream(g) as Msg[])].reverse()) {
		const s = sampleOf(m);
		// A kinds sample is that kind alone: only the video note has a file.
		if (s) {
			if (s.media?.round) out.push(fileOf(m, { fileId: 970000 + (m.message_id % 10000), size: 420000, duration: 9 }, 'video_note', v));
			continue;
		}
		if (m.media) {
			const t = m.content_type;
			const kind = (t === 'video' && m.media.fileId % 4 === 0 ? 'animation' : t === 'document' && m.media.ext === 'mp3' ? 'audio' : t) as Kind;
			if (FILE_KINDS.includes(kind)) out.push(fileOf(m, m.media, kind, v));
		}
		// An album's items are the same message's here (the stand-in has no
		// message ids of their own for them in the chat).
		for (const a of m.album ?? []) {
			const kind = a.fileId % 5 === 0 ? 'video' : 'photo';
			out.push(fileOf(m, { ...a, stored: a.fileId % 6 !== 0, ...(kind === 'video' ? { duration: 12 + (a.fileId % 50) } : {}) }, kind, v));
		}
	}
	fileCache.set(k, out);
	return out;
}

// One page of rows older than `before`, never splitting a message's
// files between pages.
function pageOf<T extends { message_id: number }>(all: T[], before: number | null) {
	const rest = before === null ? all : all.filter((r) => r.message_id < before);
	let n = Math.min(GROUP_FILES_PAGE, rest.length);
	while (n < rest.length && rest[n]!.message_id === rest[n - 1]!.message_id) n++;
	const rows = rest.slice(0, n);
	return { rows, next_before: n < rest.length ? rows[rows.length - 1]!.message_id : null };
}

const parseBefore = (q: URLSearchParams): number | null | false => {
	const b = q.get('before');
	if (b === null || b === '') return null;
	return /^\d+$/.test(b) ? Number(b) : false;
};

// The files part of the props or the API answer; a string is a bad
// query parameter's name.
export function groupFiles(g: { id: number }, q: URLSearchParams, v: View, viewKey: string): Record<string, unknown> | string {
	const kind = q.get('kind') || 'all';
	if (kind !== 'all' && !FILE_KINDS.includes(kind as Kind)) return 'kind';
	const before = parseBefore(q);
	if (before === false) return 'before';
	const all = filesOf(g, v, viewKey);
	// Counting is cheap only for the sample group here.
	const counted = g.id === KINDS_GROUP.id;
	const kinds = FILE_KINDS.filter((k) => all.some((f) => f.file_type === k)).map((k) => ({ kind: k, count: counted ? all.filter((f) => f.file_type === k).length : null }));
	const p = pageOf(kind === 'all' ? all : all.filter((f) => f.file_type === kind), before);
	return { kinds, kind, ...p };
}

// The group's polls, with the chat's poll detail; a poll whose poll
// was never stored is not one of them.
export function groupPolls(g: { id: number }, q: URLSearchParams, v: View): Record<string, unknown> | string {
	const before = parseBefore(q);
	if (before === false) return 'before';
	const all = [...(chatStream(g) as Msg[])].reverse().filter((m) => sampleOf(m)?.content_type === 'poll' && sampleOf(m)?.detail).map((m) => ({
		message_id: m.message_id, message_row_id: m.id, date: iso(m.date), sender: senderFor(m, v),
		chat_link: `/groups/${m.chat.id}/chat?at=${m.message_id}`, poll: kindsOf({ ...m, content_type: 'poll' }, v).base.detail,
	}));
	return pageOf(all, before);
}

// The sample groups the tests name: every kind and more than a page;
// some kinds only (no GIFs); a channel.
export const FILES_GROUP = KINDS_GROUP;
export const SOME_KINDS_GROUP = groups[8]!;
export const CHANNEL_GROUP = groups.find((x: { type: string }) => x.type === 'channel')!;
