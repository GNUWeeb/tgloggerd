// The stand-in's MCP note edit, as the Go service answers it: the note
// alone changes, clipped to 255, for viewers who may expose groups.
import { describe, expect, it } from 'vitest';
import { adminApi, pageData } from './fixtures.ts';
import { groups } from '../../mockup/src/data/groups.js';

type Row = { group: { id: number }; note: string; added: string };
const rows = () => (pageData('/mcp-admin/groups', 'superadmin').props as { groups: Row[] }).groups;
const patch = (id: number, body: object, who: 'superadmin' | 'admin' | 'member' = 'superadmin') => adminApi('PATCH', `mcp/groups/${id}`, body, who);

describe('stand-in MCP group note', () => {
	it('changes the note and keeps the date', () => {
		const before = rows()[0]!;
		expect(patch(before.group.id, { note: 'Reviewed again' })).toEqual([204, null]);
		const after = rows().find((r) => r.group.id === before.group.id)!;
		expect(after.note).toBe('Reviewed again');
		expect(after.added).toBe(before.added);
		expect(patch(before.group.id, { note: '' })[0]).toBe(204);
		expect(rows().find((r) => r.group.id === before.group.id)!.note).toBe('');
	});

	it('clips the note to 255 characters', () => {
		const id = rows()[0]!.group.id;
		expect(patch(id, { note: 'x'.repeat(300) })[0]).toBe(204);
		expect(rows().find((r) => r.group.id === id)!.note).toHaveLength(255);
	});

	it('clips by runes after trimming, on PATCH and on POST alike', () => {
		const id = rows()[0]!.group.id;
		// A face is two UTF-16 units but one rune.
		expect(patch(id, { note: '  ' + '\u{1F600}'.repeat(300) + '  ' })[0]).toBe(204);
		expect(Array.from(rows().find((r) => r.group.id === id)!.note)).toHaveLength(255);
		expect(patch(id, { note: '  kept  ' })[0]).toBe(204);
		expect(rows().find((r) => r.group.id === id)!.note).toBe('kept');
		const g = groups.find((x: { exposed: boolean }) => !x.exposed)!;
		expect(adminApi('POST', 'mcp/groups', { group_id: g.id, note: '  ' + 'y'.repeat(300) }, 'superadmin')[0]).toBe(201);
		expect(rows().find((r) => r.group.id === g.id)!.note).toBe('y'.repeat(255));
	});

	it('answers 400 for a missing, null or non-string note, before looking at the group', () => {
		const id = rows()[0]!.group.id;
		for (const body of [{}, { note: null }, { note: 7 }]) expect(patch(id, body)[0]).toBe(400);
		expect(patch(-1, {})[0]).toBe(400);
	});

	it('answers 404 for an unknown, hidden or not exposed group, and to a member', () => {
		const id = rows()[0]!.group.id;
		expect(patch(-1, { note: 'x' })[0]).toBe(404);
		expect(patch(id, { note: 'x' }, 'member')[0]).toBe(404);
		// An admin sees only some exposed groups; the others are hidden.
		const seen = new Set((pageData('/mcp-admin/groups', 'admin').props as { groups: Row[] }).groups.map((r) => r.group.id));
		const hidden = rows().find((r) => !seen.has(r.group.id))!;
		expect(patch(hidden.group.id, { note: 'x' }, 'admin')[0]).toBe(404);
		expect(rows().find((r) => r.group.id === hidden.group.id)!.note).toBe(hidden.note);
		const shown = rows().find((r) => seen.has(r.group.id))!;
		expect(patch(shown.group.id, { note: 'x' }, 'admin')[0]).toBe(204);
	});

	it('removes a group only on DELETE; another method is 405', () => {
		const id = rows()[0]!.group.id;
		for (const method of ['GET', 'PUT', 'POST']) expect(adminApi(method, `mcp/groups/${id}`, {}, 'superadmin')[0]).toBe(405);
		expect(adminApi('GET', 'mcp/groups', {}, 'superadmin')[0]).toBe(405);
		expect(rows().some((r) => r.group.id === id)).toBe(true);
		expect(adminApi('DELETE', `mcp/groups/${id}`, {}, 'superadmin')[0]).toBe(204);
		expect(rows().some((r) => r.group.id === id)).toBe(false);
	});
});
