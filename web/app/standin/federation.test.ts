// The stand-in's federation details, as the Go service answers them:
// who sees one, its groups a page at a time, and who may change what.
import { describe, expect, it } from 'vitest';
import { adminApi, federationDetail, federationGroups } from './fixtures.ts';

const q = (s: string) => new URLSearchParams(s);
const patch = (id: number, body: object, who: 'superadmin' | 'admin' | 'member' = 'superadmin') => adminApi('PATCH', `federations/${id}`, body, who);

describe('stand-in federation details', () => {
	it('shows a superadmin every federation, an admin its own, a member none', () => {
		expect(federationDetail(1, 'superadmin')).not.toBeNull();
		expect(federationDetail(0, 'superadmin')!.federation.default).toBe(true);
		const own = federationDetail(1, 'admin') ?? federationDetail(2, 'admin') ?? federationDetail(3, 'admin');
		expect(own).not.toBeNull();
		expect(federationDetail(1, 'member')).toBeNull();
		expect(adminApi('GET', 'federations/99', {}, 'superadmin')[0]).toBe(404);
	});

	it('pages its groups busiest first, and searches them', () => {
		const [, all] = federationGroups(0, q('limit=3'), 'superadmin') as [number, { items: { msg_count: number; title: string }[]; next_cursor: string | null; total: number }];
		expect(all.items).toHaveLength(3);
		expect(all.items[0]!.msg_count).toBeGreaterThanOrEqual(all.items[1]!.msg_count);
		const [, next] = federationGroups(0, q(`limit=3&cursor=${all.next_cursor}`), 'superadmin') as [number, { items: unknown[] }];
		expect(next.items.length).toBeGreaterThan(0);
		const term = all.items[0]!.title.slice(0, 4).toUpperCase();
		const [, hit] = federationGroups(0, q(`q=${term}`), 'superadmin') as [number, { items: { title: string }[] }];
		expect(hit.items.every((g) => g.title.toUpperCase().includes(term))).toBe(true);
		expect(federationGroups(0, q('limit=0'), 'superadmin')[0]).toBe(400);
		expect(federationGroups(0, q('cursor=nope'), 'superadmin')[0]).toBe(400);
		expect(federationGroups(0, q(`q=${'x'.repeat(513)}`), 'superadmin')[0]).toBe(400);
		// A leading @ is dropped, as for a username.
		const [, at] = federationGroups(0, q(`q=@${term}`), 'superadmin') as [number, { total: number }];
		expect(at.total).toBe((federationGroups(0, q(`q=${term}`), 'superadmin')[1] as { total: number }).total);
	});

	it('lets a superadmin change anything but Default\'s name', () => {
		expect(federationDetail(0, 'superadmin')!.can_edit).toEqual({ name: false, description: true, groups: false, admins: true, delete: false });
		expect(federationDetail(1, 'superadmin')!.can_edit).toEqual({ name: true, description: true, groups: true, admins: true, delete: true });
		expect(patch(0, { name: 'Elsewhere' })[0]).toBe(400);
		const [st, row] = patch(1, { name: '  GNU/Weeb  ', description: '  The network.\r\nAll of it.  ' });
		expect(st).toBe(200);
		expect(row).toEqual({ id: 1 });
		expect(federationDetail(1, 'superadmin')!.federation).toMatchObject({ name: 'GNU/Weeb', description: 'The network.\nAll of it.' });
		patch(1, { description: '' });
		expect(federationDetail(1, 'superadmin')!.federation.description).toBe('');
		expect(patch(1, { name: 'x'.repeat(65) })[1]).toMatchObject({ error: 'bad_name' });
	});

	it('lets an admin of a federation rename and describe it, nothing more', () => {
		const id = [1, 2, 3].find((i) => federationDetail(i, 'admin'))!;
		expect(federationDetail(id, 'admin')!.can_edit).toEqual({ name: true, description: true, groups: false, admins: false, delete: false });
		expect(patch(id, { description: 'Ours' }, 'admin')[0]).toBe(200);
		expect(federationDetail(id, 'admin')!.federation.description).toBe('Ours');
		expect(patch(id, { group_ids: [] }, 'admin')[0]).toBe(403);
		expect(patch(id, { admin_ids: [] }, 'admin')[0]).toBe(403);
	});

	it('refuses a long description or one with control characters', () => {
		expect(patch(1, { description: 'x'.repeat(2001) })[1]).toMatchObject({ error: 'bad_description' });
		expect(patch(1, { description: 'bell\u0007' })[1]).toMatchObject({ error: 'bad_description' });
		expect(patch(1, { description: 'lone\rreturn' })[1]).toMatchObject({ error: 'bad_description' });
		expect(patch(1, { description: 'tab\tand\nnew line' })[0]).toBe(200);
	});

	it('deletes a federation for a superadmin only, never Default', () => {
		expect(adminApi('DELETE', 'federations/0', {}, 'superadmin')[0]).toBe(400);
		expect(adminApi('DELETE', 'federations/3', {}, 'admin')[0]).toBe(404);
		expect(adminApi('DELETE', 'federations/3', {}, 'superadmin')[0]).toBe(204);
		expect(federationDetail(3, 'superadmin')).toBeNull();
		// Its groups are Default's again.
		const [, d] = federationGroups(0, new URLSearchParams('limit=200'), 'superadmin') as [number, { total: number }];
		expect(d.total).toBeGreaterThan(0);
	});
});
