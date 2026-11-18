// The stand-in's list search: it models the Go lists' rules, which the
// list pages are tested against.
import { describe, expect, it } from 'vitest';
import { REGISTRY, compile, legacyQuery, type Cond } from './search.ts';

const rows = [
	{ id: 1, name: 'Dewi', first_name: 'Dewi', bio: 'kernel hacker', phone: '', is_premium: true, has_photo: true, created_at: '2026-05-08 11:00:00' },
	{ id: 2, name: 'Bima', first_name: 'Bima', bio: '', phone: '+62', is_premium: false, has_photo: false, created_at: '2026-09-01 10:00:00' },
	{ id: 3, name: 'Sari', first_name: 'Sari', bio: 'rust and C', phone: '', is_premium: false, has_photo: true, created_at: '2026-10-01 10:00:00' },
];
// The list's own filter keys; the rest come from its registry.
const own = ['id', 'name'];
const ids = (conds: Cond[]) => {
	const t = compile(conds, own, REGISTRY.users!);
	if (typeof t !== 'function') throw new Error(t.message);
	return rows.filter(t).map((r) => r.id);
};

describe('stand-in list search', () => {
	it('binds AND tighter than OR', () => {
		// id = 1 OR id = 2 AND is_premium: 1, or (2 and premium) = [1]
		expect(ids([{ field: 'id', op: '=', value: '1', n: 'OR' }, { field: 'id', op: '=', value: '2' }, { field: 'is_premium', op: '=', value: '1' }])).toEqual([1]);
		expect(ids([{ field: 'id', op: '=', value: '1', n: 'OR' }, { field: 'id', op: '=', value: '3' }])).toEqual([1, 3]);
	});

	it('runs the search operators', () => {
		expect(ids([{ field: 'first_name', op: 'LIKE', value: 'D%' }])).toEqual([1]);
		expect(ids([{ field: 'first_name', op: 'NOT LIKE', value: '_ima' }])).toEqual([1, 3]);
		expect(ids([{ field: 'bio', op: '%LIKE%', value: 'RUST' }])).toEqual([3]);
		expect(ids([{ field: 'bio', op: 'IS NULL', value: '' }])).toEqual([2]);
		expect(ids([{ field: 'has_photo', op: 'IS NOT NULL', value: '' }])).toEqual([1, 3]);
		expect(ids([{ field: 'created_at', op: '>=', value: '2026-09-01 10:00:00' }])).toEqual([2, 3]);
		expect(ids([{ field: 'id', op: '<=', value: '2' }])).toEqual([1, 2]);
	});

	it('refuses an operator a field does not take, a bad value and an unknown field', () => {
		for (const c of [{ field: 'is_premium', op: 'LIKE', value: 'x' }, { field: 'created_at', op: '>', value: 'soon' }, { field: 'nope', op: '=', value: '1' }]) {
			const t = compile([c], own, REGISTRY.users!);
			expect(typeof t).toBe('object');
			expect((t as { code: string }).code).toBe('bad_search');
		}
	});

	it('reads a Drogon list URL as the same list', () => {
		const q = legacyQuery(new URLSearchParams({ search: JSON.stringify([{ c: 'first_name', o: 'LIKE', v: 'D%', n: 'OR' }, { c: 'id', o: '=', v: '2', n: 'AND' }]), offset: '100', limit: '50', sort: 'id', order: 'ASC' }))!;
		expect(JSON.parse(q.get('f')!)).toEqual([{ field: 'first_name', op: 'LIKE', value: 'D%', n: 'OR' }, { field: 'id', op: '=', value: '2' }]);
		expect([q.get('per'), q.get('page'), q.get('sort'), q.get('dir')]).toEqual(['50', '3', 'id', 'asc']);
		expect(legacyQuery(new URLSearchParams({ offset: '100' }))!.get('per')).toBe('50');
		expect(legacyQuery(new URLSearchParams({ q: 'x' }))).toBeNull();
		expect(legacyQuery(new URLSearchParams({ offset: '5', page: '2' }))).toBeNull();
	});
});
