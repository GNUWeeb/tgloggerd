// The stand-in's group files and polls tabs, as the Go service is to
// answer them: newest message first, 50 to a page, paged by ?before=,
// only the kinds a group has, and 404 where the viewer has no group.
import { describe, expect, it } from 'vitest';
import { groupTabApi, pageData } from './fixtures.ts';
import { CHANNEL_GROUP, FILES_GROUP, GROUP_FILES_PAGE, SOME_KINDS_GROUP } from './groupfiles.ts';

type Row = { message_id: number; message_row_id: number; file_type: string; sender: { id: number } | null; chat_link: string; stored: boolean; url: string | null };
type Files = { kinds: { kind: string; count: number | null }[]; kind: string; rows: Row[]; next_before: number | null };
type Polls = { rows: { message_id: number; poll: { kind: string; type: string; closed: boolean; options: unknown[]; options_total: number } }[]; next_before: number | null };
const q = (s: string) => new URLSearchParams(s);
const files = (id: number, s = '', who: Parameters<typeof groupTabApi>[3] = 'superadmin') => groupTabApi('files', String(id), q(s), who);

describe('stand-in group files', () => {
	it('lists every kind of the sample group, newest first, a page at a time', () => {
		const [status, body] = files(FILES_GROUP.id);
		expect(status).toBe(200);
		const p = body as Files;
		expect(p.kinds.map((k) => k.kind)).toEqual(['photo', 'video', 'animation', 'audio', 'voice', 'document', 'sticker', 'video_note']);
		expect(p.kinds.every((k) => typeof k.count === 'number')).toBe(true);
		expect(p.rows.length).toBeGreaterThanOrEqual(GROUP_FILES_PAGE);
		const ids = p.rows.map((r) => r.message_id);
		expect([...ids].sort((a, b) => b - a)).toEqual(ids);
		expect(p.rows.some((r) => !r.stored)).toBe(true);
		expect(p.rows[0]!.chat_link).toBe(`/groups/${FILES_GROUP.id}/chat?at=${p.rows[0]!.message_id}`);
		expect(p.next_before).toBe(ids[ids.length - 1]);
		const next = files(FILES_GROUP.id, `before=${p.next_before}`)[1] as Files;
		expect(next.rows.length).toBeGreaterThan(0);
		expect(next.rows.every((r) => r.message_id < p.next_before!)).toBe(true);
	});

	it('filters by kind and refuses a bad kind or cursor', () => {
		const p = files(FILES_GROUP.id, 'kind=photo')[1] as Files;
		expect(p.kind).toBe('photo');
		expect(p.rows.every((r) => r.file_type === 'photo')).toBe(true);
		expect(files(FILES_GROUP.id, 'kind=nope')[0]).toBe(400);
		expect(files(FILES_GROUP.id, 'before=x')[0]).toBe(400);
	});

	it('offers only the kinds a group has, uncounted, and a channel posts as itself', () => {
		const p = files(SOME_KINDS_GROUP.id)[1] as Files;
		expect(p.kinds.map((k) => k.kind)).not.toContain('animation');
		expect(p.kinds.every((k) => k.count === null)).toBe(true);
		const c = files(CHANNEL_GROUP.id)[1] as Files;
		expect(c.rows.every((r) => r.sender?.id === CHANNEL_GROUP.id)).toBe(true);
	});

	it('answers 404 where the viewer does not see the group', () => {
		const mine = (pageData('/groups', 'member').props as { rows: { id: number }[] }).rows.map((g) => g.id);
		const other = (pageData('/groups', 'superadmin').props as { rows: { id: number }[] }).rows.find((g) => !mine.includes(g.id))!;
		expect(files(other.id, '', 'member')[0]).toBe(404);
		expect(files(FILES_GROUP.id, '', 'public')[0]).toBe(404);
		expect(pageData(`/groups/${other.id}/files`, 'member').route).toBe('not_found');
		expect(pageData(`/groups/${FILES_GROUP.id}/files`, 'public').route).toBe('not_found');
	});

	it('serves the tab pages with the group beside the list', () => {
		const p = pageData(`/groups/${FILES_GROUP.id}/files?kind=sticker`, 'superadmin');
		expect(p.route).toBe('group_files');
		const props = p.props as Files & { group: { id: number } };
		expect(props.group.id).toBe(FILES_GROUP.id);
		expect(props.kind).toBe('sticker');
		// A bad kind on the page is read as none.
		expect((pageData(`/groups/${FILES_GROUP.id}/files?kind=nope`, 'superadmin').props as Files).kind).toBe('all');
		expect(pageData(`/groups/${FILES_GROUP.id}/polls`, 'superadmin').route).toBe('group_polls');
	});
});

describe('stand-in group polls', () => {
	it('lists the sample polls newest first: regular, a closed quiz, one over 20 options', () => {
		const [status, body] = groupTabApi('polls', String(FILES_GROUP.id), q(''), 'superadmin');
		expect(status).toBe(200);
		const p = body as Polls;
		expect(p.rows.length).toBe(3);
		expect(p.next_before).toBeNull();
		const ids = p.rows.map((r) => r.message_id);
		expect([...ids].sort((a, b) => b - a)).toEqual(ids);
		expect(p.rows.some((r) => r.poll.type === 'quiz' && r.poll.closed)).toBe(true);
		expect(p.rows.some((r) => r.poll.options_total > 20 && r.poll.options.length === 20)).toBe(true);
		expect(p.rows.some((r) => r.poll.type === 'regular' && !r.poll.closed)).toBe(true);
		const none = groupTabApi('polls', String(SOME_KINDS_GROUP.id), q(''), 'superadmin')[1] as Polls;
		expect(none.rows).toEqual([]);
	});
});
