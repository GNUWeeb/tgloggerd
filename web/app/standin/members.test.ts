// The stand-in's group members, as the group-members contract has the
// Go service answer them: the count, the list a page at a time, who
// gets 404, each reason a list is not available, and the hide switch.
import { describe, expect, it } from 'vitest';
import { MEMBER_SAMPLES as M, membersApi, patchGroup } from './members.ts';
import { pageData } from './fixtures.ts';

const q = (s: string) => new URLSearchParams(s);
const all = () => true;
type Page = { available: boolean; reason: string; total: number | null; members: { user: { id: number; name: string; username: string }; status: string }[] };
const list = (id: number, s = '', role: 'superadmin' | 'admin' | 'member' = 'superadmin') => membersApi(String(id), q(s), role, all) as [number, Page];

describe('stand-in group members', () => {
	it('pages the list by offset and limit, with every status', () => {
		const [st, first] = list(M.LONG);
		expect(st).toBe(200);
		expect(first.available).toBe(true);
		expect(first.members).toHaveLength(50);
		expect(first.total).toBeGreaterThan(100);
		const [, rest] = list(M.LONG, 'offset=100&limit=50');
		expect(rest.members.length).toBeGreaterThan(0);
		expect(rest.members.length).toBeLessThan(50);
		expect(rest.members[0]!.user.id).not.toBe(first.members[0]!.user.id);
		const statuses = new Set([...first.members, ...rest.members].map((x) => x.status));
		expect([...statuses].sort()).toEqual(['administrator', 'creator', 'member', 'restricted']);
		// A member the archive has not stored yet: its id alone.
		expect(rest.members.at(-1)!.user.name).toBe('');
		expect(list(M.LONG, 'limit=201')[0]).toBe(400);
	});

	it('is 404 for members, and for a group the viewer does not see', () => {
		expect(list(M.LONG, '', 'member')[0]).toBe(404);
		expect(list(M.LONG, '', 'admin')[0]).toBe(200);
		expect(membersApi(String(M.LONG), q(''), 'superadmin', () => false)[0]).toBe(404);
		expect(pageData(`/groups/${M.LONG}/members`, 'member').status).toBe(404);
	});

	it('gives each reason a list is not available, with the count', () => {
		expect(list(M.NOT_ALLOWED)[1]).toMatchObject({ available: false, reason: 'not_allowed', members: [] });
		expect(list(M.NOT_MEMBER)[1]).toMatchObject({ available: false, reason: 'not_member' });
		expect(list(M.FLOOD)[1]).toMatchObject({ available: false, reason: 'flood_wait' });
		expect(list(M.NOT_MEMBER)[1].total).toBeGreaterThan(0);
		// Hidden: for everyone but superadmins.
		expect(list(M.HIDDEN, '', 'admin')[1]).toMatchObject({ available: false, reason: 'hidden' });
		expect(list(M.HIDDEN, '', 'superadmin')[1].available).toBe(true);
		expect(list(M.NO_COUNT)[1]).toMatchObject({ available: true, total: null });
	});

	it('carries the count and the switch in the page data', () => {
		const g = pageData(`/groups/${M.HIDDEN}`, 'superadmin').props as { group: { member_count: number | null }; hide_member_list: boolean; can_hide_member_list: boolean };
		expect(g.group.member_count).toBeGreaterThan(0);
		expect(g).toMatchObject({ hide_member_list: true, can_hide_member_list: true });
		expect(pageData(`/groups/${M.HIDDEN}`, 'admin').props).toMatchObject({ can_hide_member_list: false });
		expect(pageData(`/groups/${M.LONG}`, 'superadmin').props).toMatchObject({ hide_member_list: false, can_hide_member_list: false });
		expect((pageData(`/groups/${M.NO_COUNT}/chat`, 'superadmin').props as { chat: { group: { member_count: unknown } } }).chat.group.member_count).toBeNull();
		const p = pageData(`/groups/${M.LONG}/members?offset=50`, 'admin');
		expect(p.route).toBe('group_members');
		expect(p.props).toMatchObject({ available: true, offset: 50, limit: 50 });
	});

	it('searches by name, @username and id, paging within the matches', () => {
		const every = [...list(M.LONG)[1].members, ...list(M.LONG, 'offset=50&limit=200')[1].members];
		const u = every.find((x) => x.user.username && x.user.name)!.user;
		// The username, with a leading @, in another case.
		const [st, byName] = list(M.LONG, `q=${encodeURIComponent('@' + u.username.toUpperCase())}`);
		expect(st).toBe(200);
		expect(byName.members.map((x) => x.user.id)).toContain(u.id);
		expect(byName.total).toBe(byName.members.length);
		// A part of the name.
		const part = u.name.split(' ')[0]!.toLowerCase();
		const named = list(M.LONG, `q=${encodeURIComponent(part)}`)[1];
		expect(named.members.length).toBeGreaterThan(0);
		for (const x of named.members) expect(x.user.name.toLowerCase().includes(part) || x.user.username.toLowerCase().includes(part)).toBe(true);
		// An exact id comes first; one nobody has finds nothing.
		const last = every.at(-1)!.user;
		const byId = list(M.LONG, `q=${last.id}`)[1].members;
		expect(byId[0]).toMatchObject({ user: { id: last.id }, matched_by: 'id' });
		// The id match comes on the first page in addition to limit, and
		// counts in total; later pages leave it out.
		const one = list(M.LONG, `q=${last.id}&limit=1`)[1];
		expect(one.members[0]).toMatchObject({ user: { id: last.id }, matched_by: 'id' });
		expect(one.total).toBe(one.members.length);
		expect(list(M.LONG, `q=${last.id}&offset=1`)[1].members).toEqual([]);
		expect(named.members.every((x) => (x as { matched_by?: string }).matched_by === 'search')).toBe(true);
		// Without a search, no member says how it was found.
		expect(every.some((x) => 'matched_by' in x)).toBe(false);
		expect(list(M.LONG, 'q=zzzqqqxxx')[1]).toMatchObject({ available: true, total: 0, members: [] });
		// Paging within the matches.
		const a = list(M.LONG, 'q=a&limit=5')[1];
		const b = list(M.LONG, 'q=a&limit=5&offset=5')[1];
		expect(a.total).toBeGreaterThan(10);
		expect(b.members[0]!.user.id).not.toBe(a.members[0]!.user.id);
		// Too long for the API; the page ignores it.
		expect(list(M.LONG, `q=${'x'.repeat(65)}`)[0]).toBe(400);
		expect(list(M.LONG, `q=${'x'.repeat(64)}`)[0]).toBe(200);
		expect((pageData(`/groups/${M.LONG}/members?q=${'x'.repeat(65)}`, 'admin').props as Page).members).toHaveLength(50);
		expect((pageData(`/groups/${M.LONG}/members?q=zzzqqqxxx`, 'admin').props as Page).total).toBe(0);
		// A search of a list that is not available has no total.
		expect(list(M.FLOOD, 'q=a')[1]).toMatchObject({ available: false, total: null });
		expect(list(M.FLOOD)[1].total).not.toBeNull();
	});

	it('lets only a superadmin hide a channel\'s list', () => {
		expect(patchGroup(String(M.NOT_ALLOWED), { hide_member_list: true }, 'admin', all)[0]).toBe(403);
		expect(patchGroup(String(M.LONG), { hide_member_list: true }, 'superadmin', all)[0]).toBe(400);
		expect(patchGroup(String(M.NOT_ALLOWED), {}, 'superadmin', all)[0]).toBe(400);
		expect(patchGroup(String(M.HIDDEN), { hide_member_list: false }, 'superadmin', all)[0]).toBe(200);
		expect(list(M.HIDDEN, '', 'admin')[1].available).toBe(true);
		expect(patchGroup(String(M.HIDDEN), { hide_member_list: true }, 'superadmin', all)[0]).toBe(200);
		expect(list(M.HIDDEN, '', 'admin')[1].reason).toBe('hidden');
	});
});
